// Copyright 2026 Abera/Corteza contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package assistant

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/cortezaproject/corteza/server/pkg/auth"
	"github.com/cortezaproject/corteza/server/pkg/logger"
	"github.com/cortezaproject/corteza/server/store"
	"github.com/cortezaproject/corteza/server/system/service"
	"github.com/cortezaproject/corteza/server/system/types"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

type Actor struct {
	UserID uint64   `json:"userID,string"`
	Email  string   `json:"email"`
	Name   string   `json:"name"`
	Handle string   `json:"handle"`
	Roles  []string `json:"roles"`
}

type ActorResolver interface {
	Resolve(context.Context, auth.Identifiable) (Actor, bool, error)
}

type StoreActorResolver struct {
	RequiredRole string
}

func (resolver StoreActorResolver) Resolve(ctx context.Context, identity auth.Identifiable) (Actor, bool, error) {
	if identity == nil || !identity.Valid() {
		return Actor{}, false, nil
	}
	user, err := store.LookupUserByID(ctx, service.DefaultStore, identity.Identity())
	if err != nil {
		return Actor{}, false, err
	}
	if !user.Valid() {
		return Actor{}, false, nil
	}
	required, err := store.LookupRoleByHandle(ctx, service.DefaultStore, resolver.RequiredRole)
	if err != nil {
		return Actor{}, false, err
	}
	members, _, err := store.SearchRoleMembers(ctx, service.DefaultStore, types.RoleMemberFilter{
		RoleID:   required.ID,
		Resource: fmt.Sprintf("corteza::system:user/%d", user.ID),
		Limit:    1,
	})
	if err != nil {
		return Actor{}, false, err
	}
	// Both checks are intentional: the store check makes revocation immediate,
	// while the token check prevents newly granted access before token refresh.
	allowed := len(members) > 0 && containsRole(identity.Roles(), required.ID)
	actor := Actor{
		UserID: user.ID,
		Email:  user.Email,
		Name:   user.Name,
		Handle: user.Handle,
		Roles:  []string{},
	}
	if allowed {
		actor.Roles = append(actor.Roles, resolver.RequiredRole)
	}
	return actor, allowed, nil
}

func containsRole(roles []uint64, expected uint64) bool {
	for _, roleID := range roles {
		if roleID == expected {
			return true
		}
	}
	return false
}

type Handler struct {
	config   Config
	resolver ActorResolver
	signer   RequestSigner
	client   *http.Client
}

func MountRoutes() func(chi.Router) {
	config, err := ConfigFromEnvironment()
	if err != nil {
		panic(fmt.Errorf("invalid Abera AI configuration: %w", err))
	}
	handler, err := NewHandler(config, nil, nil, nil)
	if err != nil {
		panic(fmt.Errorf("initialize Abera AI bridge: %w", err))
	}
	return handler.Routes()
}

func NewHandler(config Config, resolver ActorResolver, signer RequestSigner, client *http.Client) (*Handler, error) {
	if config.Timeout == 0 {
		config.Timeout = defaultTimeout
	}
	if config.MaxJSONBody == 0 {
		config.MaxJSONBody = maxJSONBody
	}
	if config.MaxUploadBody == 0 {
		config.MaxUploadBody = maxUploadBody
	}
	if config.PublicBridgePath == "" {
		config.PublicBridgePath = "/api/system/assistant"
	}
	if config.Enabled && config.AgentURL == nil {
		return nil, fmt.Errorf("agent URL is required when the bridge is enabled")
	}
	if resolver == nil {
		resolver = StoreActorResolver{RequiredRole: config.RequiredRole}
	}
	if config.Enabled && signer == nil {
		switch config.AuthMode {
		case "local_token":
			signer = BearerSigner{Token: config.LocalToken}
		case "aws_iam":
			signer = SigV4Signer{
				Region:      config.AWSRegion,
				Credentials: NewDefaultCredentialProvider(),
			}
		default:
			return nil, fmt.Errorf("unsupported authentication mode %q", config.AuthMode)
		}
	}
	if client == nil {
		client = &http.Client{
			Timeout: config.Timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	return &Handler{config: config, resolver: resolver, signer: signer, client: client}, nil
}

func (handler *Handler) Routes() func(chi.Router) {
	return func(router chi.Router) {
		router.Get("/auth/context", handler.authContext)
		router.Get("/conversations", handler.proxy("/conversations"))
		router.Post("/conversations", handler.proxy("/conversations"))
		router.Get("/conversations/{conversationID}", handler.withUUID("conversationID", conversationPath))
		router.Delete("/conversations/{conversationID}", handler.withUUID("conversationID", conversationPath))
		router.Post("/conversations/{conversationID}/messages", handler.withUUID("conversationID", func(request *http.Request) string {
			return conversationPath(request) + "/messages"
		}))
		router.Post("/conversations/{conversationID}/approvals/{approvalID}", handler.withUUIDs([]string{"conversationID", "approvalID"}, func(request *http.Request) string {
			return conversationPath(request) + "/approvals/" + chi.URLParam(request, "approvalID")
		}))
		router.Post("/conversations/{conversationID}/files/prepare", handler.withUUID("conversationID", func(request *http.Request) string {
			return conversationPath(request) + "/files/prepare"
		}))
		router.Put("/conversations/{conversationID}/files/{fileID}/content", handler.withUUIDs([]string{"conversationID", "fileID"}, func(request *http.Request) string {
			return conversationPath(request) + "/files/" + chi.URLParam(request, "fileID") + "/content"
		}))
		router.Post("/conversations/{conversationID}/files/{fileID}/complete", handler.withUUIDs([]string{"conversationID", "fileID"}, func(request *http.Request) string {
			return conversationPath(request) + "/files/" + chi.URLParam(request, "fileID") + "/complete"
		}))
	}
}

func conversationPath(request *http.Request) string {
	return "/conversations/" + chi.URLParam(request, "conversationID")
}

func (handler *Handler) withUUID(parameter string, path func(*http.Request) string) http.HandlerFunc {
	return handler.withUUIDs([]string{parameter}, path)
}

func (handler *Handler) withUUIDs(parameters []string, path func(*http.Request) string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		for _, parameter := range parameters {
			if !uuidPattern.MatchString(chi.URLParam(request, parameter)) {
				writeError(writer, http.StatusBadRequest, "Identificador inválido")
				return
			}
		}
		handler.proxyFromParams(path)(writer, request)
	}
}

func (handler *Handler) authContext(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	if !handler.config.Enabled {
		writeJSON(writer, http.StatusOK, map[string]any{"enabled": false, "allowed": false})
		return
	}
	actor, allowed, err := handler.resolveActor(request)
	if err != nil {
		handler.log(request.Context(), "failed to resolve assistant actor", err)
		writeError(writer, http.StatusInternalServerError, "No fue posible verificar el acceso al asistente")
		return
	}
	payload := map[string]any{
		"enabled":  true,
		"allowed":  allowed,
		"tenantID": handler.config.TenantID,
	}
	if allowed {
		payload["user"] = map[string]any{
			"userID": fmt.Sprintf("%d", actor.UserID),
			"email":  actor.Email,
			"name":   actor.Name,
			"handle": actor.Handle,
		}
		payload["roles"] = actor.Roles
	}
	writeJSON(writer, http.StatusOK, payload)
}

func (handler *Handler) proxy(agentPath string) http.HandlerFunc {
	return handler.proxyFromParams(func(*http.Request) string { return agentPath })
}

func (handler *Handler) proxyFromParams(agentPath func(*http.Request) string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if !handler.config.Enabled {
			writeError(writer, http.StatusServiceUnavailable, "El asistente no está habilitado")
			return
		}
		actor, allowed, err := handler.resolveActor(request)
		if err != nil {
			handler.log(request.Context(), "failed to resolve assistant actor", err)
			writeError(writer, http.StatusInternalServerError, "No fue posible verificar el acceso al asistente")
			return
		}
		if !allowed {
			writeError(writer, http.StatusForbidden, "No tiene permiso para usar el asistente")
			return
		}
		tracked := &trackingWriter{ResponseWriter: writer}
		if err = handler.forward(tracked, request, actor, agentPath(request)); err != nil {
			handler.log(request.Context(), "assistant upstream request failed", err)
			if !tracked.written {
				writeError(writer, http.StatusBadGateway, "El asistente no está disponible temporalmente")
			}
		}
	}
}

func (handler *Handler) resolveActor(request *http.Request) (Actor, bool, error) {
	return handler.resolver.Resolve(request.Context(), auth.GetIdentityFromContext(request.Context()))
}

func (handler *Handler) forward(writer http.ResponseWriter, incoming *http.Request, actor Actor, agentPath string) error {
	maximum := handler.config.MaxJSONBody
	if incoming.Method == http.MethodPut && strings.HasSuffix(agentPath, "/content") {
		maximum = handler.config.MaxUploadBody
	}
	if incoming.ContentLength > maximum {
		writeError(writer, http.StatusRequestEntityTooLarge, "La solicitud supera el tamaño permitido")
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(incoming.Body, maximum+1))
	if err != nil {
		return fmt.Errorf("read request body: %w", err)
	}
	if int64(len(body)) > maximum {
		writeError(writer, http.StatusRequestEntityTooLarge, "La solicitud supera el tamaño permitido")
		return nil
	}

	target := *handler.config.AgentURL
	target.Path = strings.TrimRight(target.Path, "/") + "/v1/internal" + agentPath
	target.RawPath = ""
	target.RawQuery = canonicalQuery(incoming.URL.Query())
	request, err := http.NewRequestWithContext(incoming.Context(), incoming.Method, target.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create upstream request: %w", err)
	}
	if contentType := incoming.Header.Get("Content-Type"); contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	request.Header.Set("Accept", incoming.Header.Get("Accept"))
	request.Header.Set("User-Agent", "abera-corteza-assistant-bridge/1")
	request.Header.Set("X-Abera-Tenant-Id", safeHeader(handler.config.TenantID))
	request.Header.Set("X-Abera-User-Id", fmt.Sprintf("%d", actor.UserID))
	request.Header.Set("X-Abera-User-Email", safeHeader(actor.Email))
	request.Header.Set("X-Abera-User-Name", safeHeader(actor.Name))
	request.Header.Set("X-Abera-User-Handle", safeHeader(actor.Handle))
	request.Header.Set("X-Abera-User-Roles", safeHeader(strings.Join(actor.Roles, ",")))
	request.Header.Set("X-Request-ID", requestID(incoming))
	if handler.config.APIKey != "" {
		request.Header.Set("X-API-Key", handler.config.APIKey)
	}
	if err = handler.signer.Sign(incoming.Context(), request, body); err != nil {
		return fmt.Errorf("authenticate upstream request: %w", err)
	}

	response, err := handler.client.Do(request)
	if err != nil {
		return fmt.Errorf("call agent: %w", err)
	}
	defer response.Body.Close()
	copyResponseHeaders(writer.Header(), response.Header)

	if strings.HasSuffix(agentPath, "/files/prepare") && response.StatusCode >= 200 && response.StatusCode < 300 {
		return handler.writeUploadTarget(writer, response)
	}

	writer.WriteHeader(response.StatusCode)
	if strings.HasPrefix(strings.ToLower(response.Header.Get("Content-Type")), "text/event-stream") {
		return copyFlushed(writer, response.Body)
	}
	_, err = io.Copy(writer, response.Body)
	return err
}

func (handler *Handler) writeUploadTarget(writer http.ResponseWriter, response *http.Response) error {
	payload, err := io.ReadAll(io.LimitReader(response.Body, 256*1024+1))
	if err != nil {
		return err
	}
	if len(payload) > 256*1024 {
		return fmt.Errorf("agent upload target response is too large")
	}
	var target map[string]any
	if err = json.Unmarshal(payload, &target); err != nil {
		return fmt.Errorf("decode upload target: %w", err)
	}
	if uploadURL, ok := target["uploadURL"].(string); ok && strings.HasPrefix(uploadURL, "/v1/internal/") {
		target["uploadURL"] = strings.TrimRight(handler.config.PublicBridgePath, "/") + strings.TrimPrefix(uploadURL, "/v1/internal")
	}
	writer.WriteHeader(response.StatusCode)
	return json.NewEncoder(writer).Encode(target)
}

func copyResponseHeaders(destination, source http.Header) {
	for _, name := range []string{"Content-Type", "Cache-Control", "Retry-After", "X-Request-ID"} {
		if value := source.Get(name); value != "" {
			destination.Set(name, value)
		}
	}
	destination.Set("X-Content-Type-Options", "nosniff")
}

func copyFlushed(writer http.ResponseWriter, source io.Reader) error {
	flusher, _ := writer.(http.Flusher)
	buffer := make([]byte, 16*1024)
	for {
		read, readErr := source.Read(buffer)
		if read > 0 {
			if _, err := writer.Write(buffer[:read]); err != nil {
				return err
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func requestID(request *http.Request) string {
	if id := safeHeader(middleware.GetReqID(request.Context())); id != "" {
		return id
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err == nil {
		return hex.EncodeToString(random)
	}
	return "corteza-assistant"
}

func safeHeader(value string) string {
	value = strings.Map(func(character rune) rune {
		if character < 32 || character == 127 {
			return -1
		}
		return character
	}, value)
	if len(value) > 1024 {
		cut := 1024
		for cut > 0 && !utf8.RuneStart(value[cut]) {
			cut--
		}
		return value[:cut]
	}
	return value
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]string{"error": message})
}

type trackingWriter struct {
	http.ResponseWriter
	written bool
}

func (writer *trackingWriter) WriteHeader(status int) {
	writer.written = true
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *trackingWriter) Write(payload []byte) (int, error) {
	writer.written = true
	return writer.ResponseWriter.Write(payload)
}

func (writer *trackingWriter) Flush() {
	writer.written = true
	if flusher, ok := writer.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (handler *Handler) log(ctx context.Context, message string, err error) {
	logger.ContextValue(ctx).Error(message, zap.Error(err))
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

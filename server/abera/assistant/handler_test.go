// Copyright 2026 Abera/Corteza contributors
// Licensed under the Apache License, Version 2.0.

package assistant

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/cortezaproject/corteza/server/pkg/auth"
	"github.com/go-chi/chi/v5"
)

type fixedResolver struct {
	actor   Actor
	allowed bool
	err     error
}

func (resolver fixedResolver) Resolve(context.Context, auth.Identifiable) (Actor, bool, error) {
	return resolver.actor, resolver.allowed, resolver.err
}

type fixedCredentials struct {
	value AWSCredentials
}

func (provider fixedCredentials) Credentials(context.Context) (AWSCredentials, error) {
	return provider.value, nil
}

func TestContextOnlyReturnsActorWhenAllowed(t *testing.T) {
	const userID = uint64(507144411485306881)
	config := Config{Enabled: true, AgentURL: mustURL(t, "http://agent.invalid"), AuthMode: "local_token", LocalToken: strings.Repeat("x", 32)}
	denied, err := NewHandler(config, fixedResolver{allowed: false}, BearerSigner{Token: config.LocalToken}, nil)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/assistant/auth/context", nil)
	request = request.WithContext(auth.SetIdentityToContext(request.Context(), auth.Authenticated(42)))
	recorder := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Route("/assistant", denied.Routes())
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || strings.Contains(recorder.Body.String(), "user") {
		t.Fatalf("unexpected denied response: %d %s", recorder.Code, recorder.Body.String())
	}

	allowed, err := NewHandler(config, fixedResolver{
		allowed: true,
		actor:   Actor{UserID: userID, Email: "ana@example.test", Roles: []string{"abera-ai-user"}},
	}, BearerSigner{Token: config.LocalToken}, nil)
	if err != nil {
		t.Fatal(err)
	}
	recorder = httptest.NewRecorder()
	router = chi.NewRouter()
	router.Route("/assistant", allowed.Routes())
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "ana@example.test") ||
		!strings.Contains(recorder.Body.String(), "tenantID") {
		t.Fatalf("unexpected allowed response: %d %s", recorder.Code, recorder.Body.String())
	}
	payload := decodeJSON(t, recorder.Body)
	user := payload["user"].(map[string]any)
	if user["userID"] != "507144411485306881" {
		t.Fatalf("user ID must be serialized without numeric precision loss: %#v", user["userID"])
	}
}

func TestContainerCredentialURLIsRestricted(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1:9911/credentials",
		"http://169.254.170.2/v2/credentials/id",
		"http://169.254.170.23/v1/credentials",
	} {
		if err := validateContainerCredentialURL(raw); err != nil {
			t.Fatalf("expected %s to be accepted: %v", raw, err)
		}
	}
	for _, raw := range []string{
		"https://169.254.170.2/credentials",
		"http://169.254.169.254/latest/meta-data",
		"http://169.254.1.50/credentials",
		"http://example.test/credentials",
	} {
		if err := validateContainerCredentialURL(raw); err == nil {
			t.Fatalf("expected %s to be rejected", raw)
		}
	}
}

func TestSafeHeaderDoesNotSplitUTF8(t *testing.T) {
	value := strings.Repeat("a", 1023) + "á" + "tail"
	got := safeHeader(value)
	if !utf8.ValidString(got) || got != strings.Repeat("a", 1023) {
		t.Fatalf("header was not truncated at a UTF-8 boundary")
	}
}

func TestProxyUsesBridgeIdentityAndWhitelistedPath(t *testing.T) {
	type captured struct {
		path, authorization, apiKey, userID, tenant, body string
	}
	capture := make(chan captured, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		payload, _ := io.ReadAll(request.Body)
		capture <- captured{
			path: request.URL.RequestURI(), authorization: request.Header.Get("Authorization"),
			apiKey: request.Header.Get("X-API-Key"), userID: request.Header.Get("X-Abera-User-Id"),
			tenant: request.Header.Get("X-Abera-Tenant-Id"), body: string(payload),
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"id":"71ce67d4-5b1e-4ad7-83e7-28eff323a732","title":"Nueva conversación"}`))
	}))
	defer upstream.Close()

	config := Config{
		Enabled: true, AgentURL: mustURL(t, upstream.URL), TenantID: "tenant-1",
		RequiredRole: "abera-ai-user", AuthMode: "local_token", LocalToken: strings.Repeat("b", 40),
		APIKey: "metering-key", Timeout: 5 * time.Second,
	}
	handler, err := NewHandler(config, fixedResolver{
		allowed: true,
		actor:   Actor{UserID: 42, Email: "ana@example.test", Name: "Ana", Roles: []string{"abera-ai-user"}},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	router.Route("/assistant", handler.Routes())
	request := httptest.NewRequest(http.MethodPost, "/assistant/conversations?trace=yes", strings.NewReader(`{"title":"Ventas"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer browser-token-must-not-leak")
	request = request.WithContext(auth.SetIdentityToContext(request.Context(), auth.Authenticated(42)))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
	}
	got := <-capture
	if got.path != "/v1/internal/conversations?trace=yes" || got.authorization != "Bearer "+config.LocalToken {
		t.Fatalf("unexpected upstream path or auth: %#v", got)
	}
	if got.apiKey != "metering-key" || got.userID != "42" || got.tenant != "tenant-1" || !strings.Contains(got.body, "Ventas") {
		t.Fatalf("missing trusted bridge context: %#v", got)
	}
}

func TestProxyRewritesOnlyInternalUploadURL(t *testing.T) {
	conversationID := "71ce67d4-5b1e-4ad7-83e7-28eff323a732"
	fileID := "3825802a-0d44-4d94-b8b8-73f72093425b"
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"uploadURL":"/v1/internal/conversations/` + conversationID + `/files/` + fileID + `/content","method":"PUT"}`))
	}))
	defer upstream.Close()
	config := Config{Enabled: true, AgentURL: mustURL(t, upstream.URL), TenantID: "tenant-1", AuthMode: "local_token", LocalToken: strings.Repeat("c", 40)}
	handler, err := NewHandler(config, fixedResolver{allowed: true, actor: Actor{UserID: 42}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	router.Route("/assistant", handler.Routes())
	request := httptest.NewRequest(http.MethodPost, "/assistant/conversations/"+conversationID+"/files/prepare", strings.NewReader(`{"name":"a.txt","contentType":"text/plain","size":1}`))
	request = request.WithContext(auth.SetIdentityToContext(request.Context(), auth.Authenticated(42)))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if !strings.Contains(recorder.Body.String(), "/api/system/assistant/conversations/") || strings.Contains(recorder.Body.String(), "/v1/internal/") {
		t.Fatalf("upload URL was not rewritten: %s", recorder.Body.String())
	}
}

func TestSigV4Signer(t *testing.T) {
	request, err := http.NewRequest(http.MethodPost, "https://abc.execute-api.us-east-1.amazonaws.com/prod/v1/internal/conversations?z=2&a=1", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Abera-Tenant-Id", "tenant-1")
	request.Header.Set("X-Abera-User-Id", "42")
	request.Header.Set("X-API-Key", "metering-key")
	request.Header.Set("X-Request-ID", "request-1")
	signer := SigV4Signer{
		Region: "us-east-1",
		Credentials: fixedCredentials{value: AWSCredentials{
			AccessKeyID: "AKIDEXAMPLE", SecretAccessKey: "secret", SessionToken: "session",
		}},
		Now: func() time.Time { return time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC) },
	}
	if err = signer.Sign(context.Background(), request, []byte("{}")); err != nil {
		t.Fatal(err)
	}
	authorization := request.Header.Get("Authorization")
	if !strings.Contains(authorization, "Credential=AKIDEXAMPLE/20260826/us-east-1/execute-api/aws4_request") ||
		!strings.Contains(authorization, "SignedHeaders=host;x-abera-tenant-id;x-abera-user-id;x-amz-content-sha256;x-amz-date;x-amz-security-token;x-api-key;x-request-id") {
		t.Fatalf("unexpected SigV4 authorization: %s", authorization)
	}
	if request.Header.Get("X-Amz-Security-Token") != "session" {
		t.Fatal("session token was not signed")
	}
}

func TestCanonicalQueryUsesAWSURIEncodingAndStableOrdering(t *testing.T) {
	values := url.Values{
		"a":         {"first"},
		"a-":        {"second"},
		"space key": {"value with space"},
		"slash":     {"a/b"},
		"duplicate": {"z", "a"},
		"unicode":   {"Bogotá"},
		"empty":     {},
	}
	got := canonicalQuery(values)
	want := "a=first&a-=second&duplicate=a&duplicate=z&empty=&slash=a%2Fb&space%20key=value%20with%20space&unicode=Bogot%C3%A1"
	if got != want {
		t.Fatalf("unexpected canonical query:\nwant %s\n got %s", want, got)
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func decodeJSON(t *testing.T, payload io.Reader) map[string]any {
	t.Helper()
	output := map[string]any{}
	if err := json.NewDecoder(payload).Decode(&output); err != nil {
		t.Fatal(err)
	}
	return output
}

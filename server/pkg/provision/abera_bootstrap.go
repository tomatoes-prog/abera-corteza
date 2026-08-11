package provision

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	automationTypes "github.com/cortezaproject/corteza/server/automation/types"
	composeTypes "github.com/cortezaproject/corteza/server/compose/types"
	"github.com/cortezaproject/corteza/server/pkg/actionlog"
	internalAuth "github.com/cortezaproject/corteza/server/pkg/auth"
	"github.com/cortezaproject/corteza/server/pkg/errors"
	"github.com/cortezaproject/corteza/server/pkg/handle"
	"github.com/cortezaproject/corteza/server/pkg/mail"
	"github.com/cortezaproject/corteza/server/pkg/rbac"
	"github.com/cortezaproject/corteza/server/store"
	systemTypes "github.com/cortezaproject/corteza/server/system/types"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const (
	aberaBootstrapStateSetting = "abera.bootstrap.oauth.v1"
	aberaOAuthClientHandle     = "abera-mcp"
	aberaEnvelopeAlgorithm     = "RSA-OAEP-256+A256GCM"
	aberaEnvelopeAAD           = "abera-bootstrap-v1"
	aberaSecretHashPrefix      = "sha256:"
)

type (
	aberaMCPMode string

	aberaBootstrapConfig struct {
		Enabled       bool
		AdminEmail    string
		AdminName     string
		AdminHandle   string
		Mode          aberaMCPMode
		PublicKeyFile string
		OutputFile    string
	}

	aberaBootstrapPayload struct {
		Version int               `json:"version"`
		Admin   aberaAdminPayload `json:"admin"`
		OAuth   aberaOAuthPayload `json:"oauth"`
	}

	aberaAdminPayload struct {
		UserID   string `json:"userID"`
		Email    string `json:"email"`
		Handle   string `json:"handle"`
		Password string `json:"password"`
	}

	aberaOAuthPayload struct {
		ClientID                 string       `json:"clientID"`
		ClientSecret             string       `json:"clientSecret"`
		GrantType                string       `json:"grantType"`
		Scope                    string       `json:"scope"`
		TokenURL                 string       `json:"tokenURL"`
		Mode                     aberaMCPMode `json:"mode"`
		ExplicitApprovalRequired bool         `json:"explicitApprovalRequired"`
	}

	aberaEncryptedEnvelope struct {
		Version      int    `json:"version"`
		Algorithm    string `json:"algorithm"`
		EncryptedKey string `json:"encryptedKey"`
		Nonce        string `json:"nonce"`
		Ciphertext   string `json:"ciphertext"`
	}

	aberaBootstrapState struct {
		Version              int                    `json:"version"`
		Status               string                 `json:"status"`
		AdminUserID          uint64                 `json:"adminUserID,string"`
		AuthClientID         uint64                 `json:"authClientID,string"`
		Mode                 aberaMCPMode           `json:"mode"`
		PublicKeyFingerprint string                 `json:"publicKeyFingerprint"`
		EnvelopeHash         string                 `json:"envelopeHash"`
		Envelope             aberaEncryptedEnvelope `json:"envelope"`
		UpdatedAt            time.Time              `json:"updatedAt"`
	}
)

const (
	aberaMCPModeBasic aberaMCPMode = "basic"
	aberaMCPModePro   aberaMCPMode = "pro"
	aberaMCPModeAdmin aberaMCPMode = "admin"
)

var aberaBootstrapEnvironmentVariables = []string{
	"ABERA_INITIAL_ADMIN_EMAIL",
	"ABERA_INITIAL_ADMIN_NAME",
	"ABERA_INITIAL_ADMIN_HANDLE",
	"ABERA_MCP_MODE",
	"ABERA_BOOTSTRAP_PUBLIC_KEY_FILE",
	"ABERA_BOOTSTRAP_OUTPUT_FILE",
}

var aberaHandleInvalidCharacters = regexp.MustCompile(`[^a-z0-9_.-]+`)

var aberaRoleNames = map[string]string{
	"abera-mcp-basic": "Integración Abera básica",
	"abera-mcp-pro":   "Integración Abera profesional",
	"abera-mcp-admin": "Integración Abera administrativa",
	"abera-consulta":  "Consulta Abera",
}

func loadAberaBootstrapConfig(getenv func(string) string) (cfg aberaBootstrapConfig, err error) {
	for _, name := range aberaBootstrapEnvironmentVariables {
		if strings.TrimSpace(getenv(name)) != "" {
			cfg.Enabled = true
			break
		}
	}

	if !cfg.Enabled {
		return cfg, nil
	}

	cfg.AdminEmail = strings.TrimSpace(getenv("ABERA_INITIAL_ADMIN_EMAIL"))
	cfg.AdminName = strings.TrimSpace(getenv("ABERA_INITIAL_ADMIN_NAME"))
	cfg.AdminHandle = strings.TrimSpace(getenv("ABERA_INITIAL_ADMIN_HANDLE"))
	cfg.Mode = aberaMCPMode(strings.ToLower(strings.TrimSpace(getenv("ABERA_MCP_MODE"))))
	cfg.PublicKeyFile = filepath.Clean(strings.TrimSpace(getenv("ABERA_BOOTSTRAP_PUBLIC_KEY_FILE")))
	cfg.OutputFile = filepath.Clean(strings.TrimSpace(getenv("ABERA_BOOTSTRAP_OUTPUT_FILE")))

	if cfg.AdminEmail == "" {
		return cfg, fmt.Errorf("ABERA_INITIAL_ADMIN_EMAIL es obligatoria cuando el bootstrap OAuth está habilitado")
	}
	if !mail.IsValidAddress(cfg.AdminEmail) {
		return cfg, fmt.Errorf("ABERA_INITIAL_ADMIN_EMAIL no contiene un correo válido")
	}
	if cfg.AdminName == "" {
		return cfg, fmt.Errorf("ABERA_INITIAL_ADMIN_NAME es obligatoria cuando el bootstrap OAuth está habilitado")
	}
	if cfg.AdminHandle == "" {
		cfg.AdminHandle = deriveAberaAdminHandle(cfg.AdminEmail)
	}
	if !handle.IsValid(cfg.AdminHandle) {
		return cfg, fmt.Errorf("ABERA_INITIAL_ADMIN_HANDLE no contiene un handle válido")
	}
	switch cfg.Mode {
	case aberaMCPModeBasic, aberaMCPModePro, aberaMCPModeAdmin:
	default:
		return cfg, fmt.Errorf("ABERA_MCP_MODE debe ser basic, pro o admin")
	}
	if cfg.PublicKeyFile == "." || !filepath.IsAbs(cfg.PublicKeyFile) {
		return cfg, fmt.Errorf("ABERA_BOOTSTRAP_PUBLIC_KEY_FILE debe ser una ruta absoluta")
	}
	if cfg.OutputFile == "." || !filepath.IsAbs(cfg.OutputFile) {
		return cfg, fmt.Errorf("ABERA_BOOTSTRAP_OUTPUT_FILE debe ser una ruta absoluta")
	}
	if samePath(cfg.PublicKeyFile, cfg.OutputFile) {
		return cfg, fmt.Errorf("los archivos de clave pública y salida cifrada deben ser diferentes")
	}

	return cfg, nil
}

func deriveAberaAdminHandle(email string) string {
	base := strings.SplitN(strings.ToLower(email), "@", 2)[0]
	base = strings.NewReplacer(".", "-", "_", "-").Replace(base)
	base = strings.Trim(aberaHandleInvalidCharacters.ReplaceAllString(base, "-"), "-_.")
	if base == "" || base[0] < 'a' || base[0] > 'z' {
		base = "admin-" + base
	}
	if len(base) < 2 {
		base += "-admin"
	}
	if handle.IsValid(base) {
		return base
	}
	out, _ := handle.Cast(func(string) bool { return true }, base)
	return strings.ToLower(out)
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func provisionAberaBootstrap(
	ctx context.Context,
	log *zap.Logger,
	s store.Storer,
	cfg aberaBootstrapConfig,
) error {
	if !cfg.Enabled {
		return nil
	}

	pub, fingerprint, err := loadAberaBootstrapPublicKey(cfg.PublicKeyFile)
	if err != nil {
		return err
	}

	state, err := loadAberaBootstrapState(ctx, s)
	if err != nil {
		return err
	}

	if state == nil {
		return createAberaBootstrap(ctx, log, s, cfg, pub, fingerprint)
	}

	return resumeAberaBootstrap(ctx, log, s, cfg, state, fingerprint)
}

func createAberaBootstrap(
	ctx context.Context,
	log *zap.Logger,
	s store.Storer,
	cfg aberaBootstrapConfig,
	pub *rsa.PublicKey,
	fingerprint string,
) (err error) {
	if err = assertAberaBootstrapResourcesAvailable(ctx, s, cfg); err != nil {
		return err
	}

	password, err := generateAberaPassword()
	if err != nil {
		return fmt.Errorf("no se pudo generar la contraseña administrativa: %w", err)
	}
	defer zeroBytes(password)

	clientSecret, err := randomBase64URLBytes(48)
	if err != nil {
		return fmt.Errorf("no se pudo generar el secreto OAuth: %w", err)
	}
	defer zeroBytes(clientSecret)

	var (
		createdAt = *now()
		user      = &systemTypes.User{
			ID:             nextID(),
			Email:          cfg.AdminEmail,
			Name:           cfg.AdminName,
			Handle:         cfg.AdminHandle,
			EmailConfirmed: true,
			Meta: &systemTypes.UserMeta{
				PreferredLanguage: "es",
			},
			CreatedAt: createdAt,
		}
		client = &systemTypes.AuthClient{
			ID:         nextID(),
			Handle:     aberaOAuthClientHandle,
			Meta:       &systemTypes.AuthClientMeta{Name: "Integración externa Abera", Description: "Cliente OAuth de aprovisionamiento para integraciones externas"},
			Secret:     hashAberaOAuthSecretBytes(clientSecret),
			Scope:      "api",
			ValidGrant: "client_credentials",
			Trusted:    true,
			Enabled:    true,
			OwnedBy:    user.ID,
			CreatedBy:  user.ID,
			CreatedAt:  createdAt,
			Security:   &systemTypes.AuthClientSecurity{ImpersonateUser: user.ID},
		}
	)

	passwordHash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("no se pudo proteger la contraseña administrativa: %w", err)
	}
	credential := &systemTypes.Credential{
		ID:          nextID(),
		OwnerID:     user.ID,
		Kind:        "password",
		Credentials: string(passwordHash),
		CreatedAt:   createdAt,
	}
	zeroBytes(passwordHash)

	roles := make(map[string]*systemTypes.Role, len(aberaRoleNames))
	for roleHandle, name := range aberaRoleNames {
		roles[roleHandle] = &systemTypes.Role{
			ID:        nextID(),
			Name:      name,
			Handle:    roleHandle,
			Meta:      &systemTypes.RoleMeta{Description: "Rol estable administrado por el bootstrap OAuth de Abera"},
			CreatedAt: createdAt,
		}
	}

	selectedRole := roles[aberaModeRoleHandle(cfg.Mode)]
	selectedRoleID := strconv.FormatUint(selectedRole.ID, 10)
	client.Security.PermittedRoles = []string{selectedRoleID}
	// Keep the selected role both permitted and forced. The administrator also
	// belongs to the bypass role for interactive login, but that role must never
	// reach a client-credentials token. Forcing the selected common role also
	// makes the token independent from role-membership cache warm-up order.
	client.Security.ForcedRoles = []string{selectedRoleID}

	payload := aberaBootstrapPayload{
		Version: 1,
		Admin: aberaAdminPayload{
			UserID:   strconv.FormatUint(user.ID, 10),
			Email:    user.Email,
			Handle:   user.Handle,
			Password: string(password),
		},
		OAuth: aberaOAuthPayload{
			ClientID:                 strconv.FormatUint(client.ID, 10),
			ClientSecret:             string(clientSecret),
			GrantType:                "client_credentials",
			Scope:                    "api",
			TokenURL:                 "/auth/oauth2/token",
			Mode:                     cfg.Mode,
			ExplicitApprovalRequired: true,
		},
	}

	envelope, err := encryptAberaBootstrapPayload(pub, &payload)
	payload.Admin.Password = ""
	payload.OAuth.ClientSecret = ""
	zeroBytes(password)
	zeroBytes(clientSecret)
	if err != nil {
		return fmt.Errorf("no se pudo cifrar la entrega del bootstrap: %w", err)
	}

	envelopeBytes, envelopeHash, err := marshalAberaEnvelope(envelope)
	if err != nil {
		return err
	}
	defer zeroBytes(envelopeBytes)

	state := &aberaBootstrapState{
		Version:              1,
		Status:               "pending",
		AdminUserID:          user.ID,
		AuthClientID:         client.ID,
		Mode:                 cfg.Mode,
		PublicKeyFingerprint: fingerprint,
		EnvelopeHash:         envelopeHash,
		Envelope:             *envelope,
		UpdatedAt:            createdAt,
	}

	err = store.Tx(ctx, s, func(ctx context.Context, tx store.Storer) error {
		for _, roleHandle := range aberaRoleHandles() {
			if err := store.CreateRole(ctx, tx, roles[roleHandle]); err != nil {
				return err
			}
		}
		if err := replaceAberaRBACRules(ctx, tx, roles); err != nil {
			return err
		}
		if err := store.CreateUser(ctx, tx, user); err != nil {
			return err
		}
		if err := store.CreateCredential(ctx, tx, credential); err != nil {
			return err
		}
		if err := syncAberaUserMemberships(ctx, tx, user.ID, cfg.Mode, roles); err != nil {
			return err
		}
		if err := store.CreateAuthClient(ctx, tx, client); err != nil {
			return err
		}
		return saveAberaBootstrapState(ctx, tx, state)
	})
	if err != nil {
		return fmt.Errorf("no se pudo guardar el bootstrap OAuth: %w", err)
	}

	if err = writeAberaEnvelopeAtomic(cfg.OutputFile, envelopeBytes); err != nil {
		return err
	}

	state.Status = "complete"
	state.UpdatedAt = *now()
	if err = saveAberaBootstrapState(ctx, s, state); err != nil {
		return fmt.Errorf("la entrega se escribió, pero no se pudo completar el marcador del bootstrap: %w", err)
	}

	log.Info(
		"bootstrap OAuth de Abera completado",
		zap.Uint64("userID", user.ID),
		zap.Uint64("authClientID", client.ID),
		zap.String("mode", string(cfg.Mode)),
	)
	return nil
}

func resumeAberaBootstrap(
	ctx context.Context,
	log *zap.Logger,
	s store.Storer,
	cfg aberaBootstrapConfig,
	state *aberaBootstrapState,
	fingerprint string,
) error {
	if state.Version != 1 {
		return fmt.Errorf("el estado del bootstrap OAuth usa una versión no compatible: %d", state.Version)
	}
	if state.PublicKeyFingerprint != fingerprint {
		return fmt.Errorf("la clave pública no coincide con la utilizada para crear este bootstrap")
	}
	if state.Status != "pending" && state.Status != "complete" {
		return fmt.Errorf("el estado del bootstrap OAuth no es válido: %q", state.Status)
	}
	if state.Status == "pending" && state.Mode != cfg.Mode {
		return fmt.Errorf("el bootstrap está pendiente en modo %s; complételo antes de cambiar a %s", state.Mode, cfg.Mode)
	}

	envelopeBytes, envelopeHash, err := marshalAberaEnvelope(&state.Envelope)
	if err != nil {
		return err
	}
	defer zeroBytes(envelopeBytes)
	if envelopeHash != state.EnvelopeHash {
		return fmt.Errorf("el sobre cifrado guardado no coincide con su hash")
	}

	user, err := store.LookupUserByID(ctx, s, state.AdminUserID)
	if err != nil {
		return fmt.Errorf("no se pudo recuperar el administrador del bootstrap: %w", err)
	}
	if !strings.EqualFold(user.Email, cfg.AdminEmail) || user.Handle != cfg.AdminHandle {
		return fmt.Errorf("el administrador configurado no coincide con el estado de este volumen")
	}
	if user.DeletedAt != nil {
		return fmt.Errorf("el administrador del bootstrap fue eliminado")
	}

	client, err := store.LookupAuthClientByID(ctx, s, state.AuthClientID)
	if err != nil {
		return fmt.Errorf("no se pudo recuperar el cliente OAuth del bootstrap: %w", err)
	}
	if client.Handle != aberaOAuthClientHandle || client.Security == nil || client.Security.ImpersonateUser != user.ID {
		return fmt.Errorf("el cliente OAuth del bootstrap fue modificado de forma incompatible")
	}

	roles, err := loadAberaRoles(ctx, s)
	if err != nil {
		return err
	}

	if state.Status == "pending" {
		if err = syncAberaMode(ctx, s, user, client, cfg.Mode, roles); err != nil {
			return err
		}
		if err = writeAberaEnvelopeAtomic(cfg.OutputFile, envelopeBytes); err != nil {
			return err
		}
		state.Status = "complete"
		state.UpdatedAt = *now()
		if err = saveAberaBootstrapState(ctx, s, state); err != nil {
			return fmt.Errorf("la entrega se escribió, pero no se pudo completar el marcador del bootstrap: %w", err)
		}
		log.Info("bootstrap OAuth de Abera recuperado", zap.String("mode", string(cfg.Mode)))
		return nil
	}

	if err = verifyAberaEnvelopeIfPresent(cfg.OutputFile, envelopeBytes); err != nil {
		return err
	}

	oldMode := state.Mode
	if oldMode != cfg.Mode {
		err = store.Tx(ctx, s, func(ctx context.Context, tx store.Storer) error {
			if err := syncAberaModeInStore(ctx, tx, user, client, cfg.Mode, roles); err != nil {
				return err
			}
			state.Mode = cfg.Mode
			state.UpdatedAt = *now()
			if err := saveAberaBootstrapState(ctx, tx, state); err != nil {
				return err
			}
			if oldMode != aberaMCPModeAdmin && cfg.Mode == aberaMCPModeAdmin {
				if err := recordAberaAdminPromotion(ctx, tx, user.ID, client.ID, oldMode); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("no se pudo aplicar atómicamente el cambio de modo OAuth: %w", err)
		}
		log.Info("modo OAuth de Abera actualizado", zap.String("oldMode", string(oldMode)), zap.String("mode", string(cfg.Mode)))
		return nil
	}

	return syncAberaMode(ctx, s, user, client, cfg.Mode, roles)
}

func syncAberaMode(
	ctx context.Context,
	s store.Storer,
	user *systemTypes.User,
	client *systemTypes.AuthClient,
	mode aberaMCPMode,
	roles map[string]*systemTypes.Role,
) error {
	return store.Tx(ctx, s, func(ctx context.Context, tx store.Storer) error {
		return syncAberaModeInStore(ctx, tx, user, client, mode, roles)
	})
}

func syncAberaModeInStore(
	ctx context.Context,
	s store.Storer,
	user *systemTypes.User,
	client *systemTypes.AuthClient,
	mode aberaMCPMode,
	roles map[string]*systemTypes.Role,
) error {
	if err := replaceAberaRBACRules(ctx, s, roles); err != nil {
		return err
	}
	if err := syncAberaUserMemberships(ctx, s, user.ID, mode, roles); err != nil {
		return err
	}

	selected := roles[aberaModeRoleHandle(mode)]
	selectedRoleID := strconv.FormatUint(selected.ID, 10)
	client.Security.PermittedRoles = []string{selectedRoleID}
	client.Security.ProhibitedRoles = nil
	client.Security.ForcedRoles = []string{selectedRoleID}
	client.Security.ImpersonateUser = user.ID
	client.UpdatedAt = now()
	return store.UpdateAuthClient(ctx, s, client)
}

func assertAberaBootstrapResourcesAvailable(ctx context.Context, s store.Storer, cfg aberaBootstrapConfig) error {
	checks := []struct {
		description string
		lookup      func() error
	}{
		{"correo del administrador", func() error {
			_, err := store.LookupUserByEmail(ctx, s, cfg.AdminEmail)
			return err
		}},
		{"handle del administrador", func() error {
			_, err := store.LookupUserByHandle(ctx, s, cfg.AdminHandle)
			return err
		}},
		{"cliente OAuth abera-mcp", func() error {
			_, err := store.LookupAuthClientByHandle(ctx, s, aberaOAuthClientHandle)
			return err
		}},
	}
	for _, roleHandle := range aberaRoleHandles() {
		h := roleHandle
		checks = append(checks, struct {
			description string
			lookup      func() error
		}{
			"rol " + h,
			func() error {
				_, err := store.LookupRoleByHandle(ctx, s, h)
				return err
			},
		})
	}

	for _, check := range checks {
		err := check.lookup()
		if err == nil {
			return fmt.Errorf("ya existe %s sin un marcador de bootstrap; se detuvo para no enlazar recursos accidentalmente", check.description)
		}
		if !errors.IsNotFound(err) {
			return fmt.Errorf("no se pudo comprobar %s: %w", check.description, err)
		}
	}

	if _, err := store.LookupRoleByHandle(ctx, s, "super-admin"); err != nil {
		return fmt.Errorf("no se encontró el rol super-admin requerido por el bootstrap: %w", err)
	}
	return nil
}

func loadAberaRoles(ctx context.Context, s store.Storer) (map[string]*systemTypes.Role, error) {
	roles := make(map[string]*systemTypes.Role, len(aberaRoleNames))
	for _, roleHandle := range aberaRoleHandles() {
		role, err := store.LookupRoleByHandle(ctx, s, roleHandle)
		if err != nil {
			return nil, fmt.Errorf("no se pudo recuperar el rol %s: %w", roleHandle, err)
		}
		roles[roleHandle] = role
	}
	return roles, nil
}

func aberaRoleHandles() []string {
	return []string{"abera-mcp-basic", "abera-mcp-pro", "abera-mcp-admin", "abera-consulta"}
}

func aberaModeRoleHandle(mode aberaMCPMode) string {
	return "abera-mcp-" + string(mode)
}

func syncAberaUserMemberships(
	ctx context.Context,
	s store.Storer,
	userID uint64,
	mode aberaMCPMode,
	roles map[string]*systemTypes.Role,
) error {
	resource := systemTypes.UserRbacResource(userID)
	selectedHandle := aberaModeRoleHandle(mode)

	for _, roleHandle := range aberaRoleHandles() {
		if roleHandle == "abera-consulta" {
			continue
		}
		member := &systemTypes.RoleMember{RoleID: roles[roleHandle].ID, Resource: resource}
		if roleHandle == selectedHandle {
			if err := ensureAberaRoleMember(ctx, s, member); err != nil {
				return err
			}
		} else {
			if err := store.DeleteRoleMember(ctx, s, member); err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
	}

	superAdmin, err := store.LookupRoleByHandle(ctx, s, "super-admin")
	if err != nil {
		return err
	}
	return ensureAberaRoleMember(ctx, s, &systemTypes.RoleMember{RoleID: superAdmin.ID, Resource: resource})
}

func ensureAberaRoleMember(ctx context.Context, s store.Storer, member *systemTypes.RoleMember) error {
	found, _, err := store.SearchRoleMembers(ctx, s, systemTypes.RoleMemberFilter{
		RoleID:   member.RoleID,
		Resource: member.Resource,
		Limit:    1,
	})
	if err != nil {
		return err
	}
	if len(found) > 0 {
		return nil
	}
	return store.CreateRoleMember(ctx, s, member)
}

func replaceAberaRBACRules(ctx context.Context, s store.Storer, roles map[string]*systemTypes.Role) error {
	all, _, err := store.SearchRbacRules(ctx, s, rbac.RuleFilter{})
	if err != nil {
		return err
	}

	roleIDs := make(map[uint64]bool, len(roles))
	for _, role := range roles {
		roleIDs[role.ID] = true
	}
	remove := make(rbac.RuleSet, 0)
	for _, rule := range all {
		if roleIDs[rule.RoleID] {
			remove = append(remove, rule)
		}
	}
	if len(remove) > 0 {
		if err = store.DeleteRbacRule(ctx, s, remove...); err != nil {
			return err
		}
	}

	expected := make(rbac.RuleSet, 0)
	for _, roleHandle := range aberaRoleHandles() {
		expected = append(expected, aberaRulesForRole(roles[roleHandle].ID, roleHandle, roles["abera-consulta"].ID)...)
	}
	return store.UpsertRbacRule(ctx, s, expected...)
}

func aberaRulesForRole(roleID uint64, roleHandle string, consultaRoleID uint64) rbac.RuleSet {
	allow := func(resource string, operations ...string) (out rbac.RuleSet) {
		for _, operation := range operations {
			out = append(out, rbac.AllowRule(roleID, resource, operation))
		}
		return out
	}
	deny := func(resource string, operations ...string) (out rbac.RuleSet) {
		for _, operation := range operations {
			out = append(out, rbac.DenyRule(roleID, resource, operation))
		}
		return out
	}

	rules := make(rbac.RuleSet, 0, 64)

	// Corteza grants a few discovery operations to every authenticated user.
	// These explicit common-role denials take precedence and keep MCP tokens
	// inside the narrower contract declared below.
	rules = append(rules, deny(
		systemTypes.ComponentRbacResource(),
		"grant",
		"action-log.read",
		"settings.read",
		"settings.manage",
		"auth-client.create",
		"auth-clients.search",
	)...)
	rules = append(rules, deny(systemTypes.AuthClientRbacResource(0), "read", "update", "delete", "authorize")...)
	rules = append(rules, deny(systemTypes.UserRbacResource(0), "update", "delete", "suspend", "unsuspend", "impersonate", "credentials.manage")...)
	rules = append(rules, deny(systemTypes.RoleRbacResource(0), "update", "delete", "members.manage")...)
	rules = append(rules, deny(composeTypes.ComponentRbacResource(), "grant", "settings.read", "settings.manage")...)
	rules = append(rules, deny(composeTypes.ModuleRbacResource(0, 0), "delete")...)

	if roleHandle != "abera-mcp-admin" {
		rules = append(rules, deny(systemTypes.ComponentRbacResource(), "user.create", "users.search", "role.create", "roles.search")...)
		rules = append(rules, deny(systemTypes.UserRbacResource(0), "read", "email.unmask", "name.unmask")...)
		rules = append(rules, deny(systemTypes.RoleRbacResource(0), "read")...)
	}

	rules = append(rules, allow(composeTypes.ComponentRbacResource(), "namespaces.search")...)
	rules = append(rules, allow(composeTypes.NamespaceRbacResource(0), "read", "modules.search", "charts.search", "pages.search")...)
	rules = append(rules, allow(composeTypes.ModuleRbacResource(0, 0), "read", "records.search")...)
	rules = append(rules, allow(composeTypes.ModuleFieldRbacResource(0, 0, 0), "record.value.read")...)
	rules = append(rules, allow(composeTypes.RecordRbacResource(0, 0, 0), "read")...)
	rules = append(rules, allow(composeTypes.ChartRbacResource(0, 0), "read")...)
	rules = append(rules, allow(composeTypes.PageRbacResource(0, 0), "read")...)
	rules = append(rules, allow(composeTypes.PageLayoutRbacResource(0, 0, 0), "read")...)

	if roleHandle == "abera-consulta" {
		return rules
	}

	rules = append(rules, allow(composeTypes.ModuleRbacResource(0, 0), "record.create")...)
	rules = append(rules, allow(composeTypes.ModuleFieldRbacResource(0, 0, 0), "record.value.update")...)
	rules = append(rules, allow(composeTypes.RecordRbacResource(0, 0, 0), "update")...)

	if roleHandle != "abera-consulta" {
		rules = append(rules, allow(
			automationTypes.ComponentRbacResource(),
			"workflow.create",
			"workflows.search",
		)...)
	}

	if roleHandle == "abera-mcp-pro" || roleHandle == "abera-mcp-admin" {
		rules = append(rules, allow(
			automationTypes.WorkflowRbacResource(0),
			"update",
		)...)
	}

	if roleHandle == "abera-mcp-pro" || roleHandle == "abera-mcp-admin" {
		rules = append(rules, allow(composeTypes.RecordRbacResource(0, 0, 0), "delete")...)
	}

	if roleHandle == "abera-mcp-admin" {
		rules = append(rules, allow(
			automationTypes.WorkflowRbacResource(0),
			"delete",
			"execute",
		)...)
		rules = append(rules, allow(composeTypes.NamespaceRbacResource(0), "module.create")...)
		rules = append(rules, allow(composeTypes.ModuleRbacResource(0, 0), "update")...)
		rules = append(rules, allow(systemTypes.ComponentRbacResource(), "user.create", "users.search", "role.create", "roles.search")...)
		rules = append(rules, allow(systemTypes.UserRbacResource(0), "read")...)
		rules = append(rules, allow(systemTypes.RoleRbacResource(0), "read")...)
		rules = append(rules, allow(systemTypes.RoleRbacResource(consultaRoleID), "members.manage")...)
	}

	return rules
}

func loadAberaBootstrapPublicKey(filename string) (*rsa.PublicKey, string, error) {
	if err := rejectSymlinkPath(filename, false); err != nil {
		return nil, "", fmt.Errorf("la ruta de la clave pública no es segura: %w", err)
	}
	info, err := os.Lstat(filename)
	if err != nil {
		return nil, "", fmt.Errorf("no se pudo leer la clave pública: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, "", fmt.Errorf("la clave pública debe ser un archivo regular, no un enlace simbólico")
	}

	raw, err := os.ReadFile(filename)
	if err != nil {
		return nil, "", fmt.Errorf("no se pudo leer la clave pública: %w", err)
	}
	defer zeroBytes(raw)

	block, rest := pem.Decode(raw)
	if block == nil || len(bytes.TrimSpace(rest)) != 0 {
		return nil, "", fmt.Errorf("la clave pública no contiene un único bloque PEM válido")
	}

	var pub *rsa.PublicKey
	if parsed, parseErr := x509.ParsePKIXPublicKey(block.Bytes); parseErr == nil {
		var ok bool
		pub, ok = parsed.(*rsa.PublicKey)
		if !ok {
			return nil, "", fmt.Errorf("la clave pública debe ser RSA")
		}
	} else {
		pub, err = x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, "", fmt.Errorf("la clave pública RSA no es válida")
		}
	}
	if pub.N.BitLen() < 3072 {
		return nil, "", fmt.Errorf("la clave pública RSA debe tener al menos 3072 bits")
	}

	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, "", fmt.Errorf("no se pudo calcular la huella de la clave pública: %w", err)
	}
	sum := sha256.Sum256(der)
	return pub, hex.EncodeToString(sum[:]), nil
}

func encryptAberaBootstrapPayload(pub *rsa.PublicKey, payload *aberaBootstrapPayload) (*aberaEncryptedEnvelope, error) {
	plaintext, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(plaintext)

	key := make([]byte, 32)
	if _, err = io.ReadFull(crand.Reader, key); err != nil {
		return nil, err
	}
	defer zeroBytes(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err = io.ReadFull(crand.Reader, nonce); err != nil {
		return nil, err
	}
	defer zeroBytes(nonce)

	ciphertext := aead.Seal(nil, nonce, plaintext, []byte(aberaEnvelopeAAD))
	encryptedKey, err := rsa.EncryptOAEP(sha256.New(), crand.Reader, pub, key, nil)
	if err != nil {
		return nil, err
	}

	encode := base64.RawURLEncoding.EncodeToString
	return &aberaEncryptedEnvelope{
		Version:      1,
		Algorithm:    aberaEnvelopeAlgorithm,
		EncryptedKey: encode(encryptedKey),
		Nonce:        encode(nonce),
		Ciphertext:   encode(ciphertext),
	}, nil
}

func marshalAberaEnvelope(envelope *aberaEncryptedEnvelope) ([]byte, string, error) {
	if envelope.Version != 1 || envelope.Algorithm != aberaEnvelopeAlgorithm {
		return nil, "", fmt.Errorf("el sobre cifrado tiene una versión o algoritmo no compatible")
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return nil, "", err
	}
	raw = append(raw, '\n')
	sum := sha256.Sum256(raw)
	return raw, hex.EncodeToString(sum[:]), nil
}

func writeAberaEnvelopeAtomic(filename string, expected []byte) error {
	if err := rejectSymlinkPath(filepath.Dir(filename), false); err != nil {
		return fmt.Errorf("el directorio de salida no es seguro: %w", err)
	}
	parent, err := os.Lstat(filepath.Dir(filename))
	if err != nil {
		return fmt.Errorf("el directorio de salida no existe o no se puede leer: %w", err)
	}
	if !parent.IsDir() || parent.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("el directorio de salida debe ser un directorio real")
	}

	if info, err := os.Lstat(filename); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("el archivo de salida existente no es un archivo regular")
		}
		current, readErr := os.ReadFile(filename)
		if readErr != nil {
			return fmt.Errorf("no se pudo verificar el archivo de salida existente: %w", readErr)
		}
		defer zeroBytes(current)
		if !bytes.Equal(current, expected) {
			return fmt.Errorf("el archivo de salida ya existe y no coincide con el sobre esperado")
		}
		if err = os.Chmod(filename, 0o600); err != nil {
			return fmt.Errorf("no se pudieron restringir los permisos del archivo de salida: %w", err)
		}
		return syncAberaDirectory(filepath.Dir(filename))
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("no se pudo comprobar el archivo de salida: %w", err)
	}

	tempDir, err := os.MkdirTemp(filepath.Dir(filename), ".abera-bootstrap-")
	if err != nil {
		return err
	}
	removeTempDir := true
	defer func() {
		if removeTempDir {
			_ = os.RemoveAll(tempDir)
		}
	}()

	tmpName := filepath.Join(tempDir, "envelope")
	f, err := os.OpenFile(tmpName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("no se pudo crear el archivo temporal de entrega: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	if _, err = f.Write(expected); err != nil {
		return fmt.Errorf("no se pudo escribir el archivo temporal de entrega: %w", err)
	}
	if err = f.Sync(); err != nil {
		return fmt.Errorf("no se pudo sincronizar el archivo temporal de entrega: %w", err)
	}
	if err = f.Chmod(0o600); err != nil {
		return fmt.Errorf("no se pudieron restringir los permisos del archivo temporal: %w", err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("no se pudo cerrar el archivo temporal de entrega: %w", err)
	}

	// A hard link publishes the already-synced file atomically and fails if the
	// destination appeared after the initial check. Rename would replace that
	// destination on Unix and would violate the no-overwrite guarantee.
	if err = os.Link(tmpName, filename); err != nil {
		return fmt.Errorf("no se pudo publicar el sobre cifrado sin sobrescribir el destino: %w", err)
	}
	if err = os.Remove(tmpName); err != nil {
		return fmt.Errorf("no se pudo retirar el archivo temporal de entrega: %w", err)
	}
	if err = os.Remove(tempDir); err != nil {
		return fmt.Errorf("no se pudo retirar el directorio temporal de entrega: %w", err)
	}
	removeTempDir = false
	return syncAberaDirectory(filepath.Dir(filename))
}

func verifyAberaEnvelopeIfPresent(filename string, expected []byte) error {
	if err := rejectSymlinkPath(filepath.Dir(filename), false); err != nil {
		return fmt.Errorf("el directorio de salida no es seguro: %w", err)
	}
	info, err := os.Lstat(filename)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("no se pudo comprobar el archivo de salida: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("el archivo de salida existente no es un archivo regular")
	}
	current, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("no se pudo verificar el archivo de salida existente: %w", err)
	}
	defer zeroBytes(current)
	if !bytes.Equal(current, expected) {
		return fmt.Errorf("el archivo de salida existente fue manipulado o pertenece a otro bootstrap")
	}
	if err = os.Chmod(filename, 0o600); err != nil {
		return fmt.Errorf("no se pudieron restringir los permisos del archivo de salida: %w", err)
	}
	return nil
}

func rejectSymlinkPath(path string, allowMissingLeaf bool) error {
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return fmt.Errorf("se requiere una ruta absoluta")
	}

	volume := filepath.VolumeName(clean)
	remainder := strings.TrimPrefix(clean, volume)
	remainder = strings.TrimPrefix(remainder, string(filepath.Separator))
	current := volume + string(filepath.Separator)
	if volume == "" {
		current = string(filepath.Separator)
	}

	parts := strings.Split(remainder, string(filepath.Separator))
	for index, part := range parts {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) && allowMissingLeaf && index == len(parts)-1 {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s contiene un enlace simbólico", current)
		}
	}
	return nil
}

func syncAberaDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	if err = dir.Sync(); err != nil && runtime.GOOS != "windows" {
		return err
	}
	return nil
}

func loadAberaBootstrapState(ctx context.Context, s store.Storer) (*aberaBootstrapState, error) {
	setting, err := store.LookupSettingValueByNameOwnedBy(ctx, s, aberaBootstrapStateSetting, 0)
	if errors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer el estado del bootstrap OAuth: %w", err)
	}

	state := &aberaBootstrapState{}
	if err = setting.Value.Unmarshal(state); err != nil {
		return nil, fmt.Errorf("el estado del bootstrap OAuth no es válido: %w", err)
	}
	return state, nil
}

func saveAberaBootstrapState(ctx context.Context, s store.Storer, state *aberaBootstrapState) error {
	setting := systemTypes.MakeSettingValue(aberaBootstrapStateSetting, state)
	setting.UpdatedAt = *now()
	return store.UpsertSettingValue(ctx, s, setting)
}

func recordAberaAdminPromotion(
	ctx context.Context,
	s store.Storer,
	userID uint64,
	clientID uint64,
	oldMode aberaMCPMode,
) error {
	actorID := internalAuth.GetIdentityFromContext(ctx).Identity()
	return store.CreateActionlog(ctx, s, &actionlog.Action{
		ID:            nextID(),
		Timestamp:     *now(),
		RequestOrigin: actionlog.RequestOriginFromContext(ctx),
		ActorID:       actorID,
		Resource:      systemTypes.AuthClientRbacResource(clientID),
		Action:        "abera.mcp.mode.promote",
		Severity:      actionlog.Notice,
		Description:   "El cliente OAuth de Abera fue ascendido al modo admin",
		Meta: actionlog.Meta{
			"userID":  strconv.FormatUint(userID, 10),
			"oldMode": string(oldMode),
			"newMode": string(aberaMCPModeAdmin),
		},
	})
}

func generateAberaPassword() ([]byte, error) {
	const (
		upper   = "ABCDEFGHJKLMNPQRSTUVWXYZ"
		lower   = "abcdefghijkmnopqrstuvwxyz"
		digits  = "23456789"
		special = "!@#$%^&*()-_=+"
		all     = upper + lower + digits + special
		length  = 32
	)

	password := []byte{
		upper[0],
		lower[0],
		digits[0],
		special[0],
	}
	completed := false
	defer func() {
		if !completed {
			zeroBytes(password)
		}
	}()
	groups := []string{upper, lower, digits, special}
	for index, group := range groups {
		value, err := randomIndex(len(group))
		if err != nil {
			return nil, err
		}
		password[index] = group[value]
	}
	for len(password) < length {
		value, err := randomIndex(len(all))
		if err != nil {
			return nil, err
		}
		password = append(password, all[value])
	}
	for i := len(password) - 1; i > 0; i-- {
		j, err := randomIndex(i + 1)
		if err != nil {
			return nil, err
		}
		password[i], password[j] = password[j], password[i]
	}
	completed = true
	return password, nil
}

func randomIndex(max int) (int, error) {
	if max <= 0 || max > 255 {
		return 0, fmt.Errorf("límite aleatorio inválido")
	}
	var one [1]byte
	limit := 256 - (256 % max)
	for {
		if _, err := io.ReadFull(crand.Reader, one[:]); err != nil {
			return 0, err
		}
		if int(one[0]) < limit {
			return int(one[0]) % max, nil
		}
	}
}

func randomBase64URLBytes(size int) (encoded []byte, err error) {
	raw := make([]byte, size)
	defer zeroBytes(raw)
	if _, err = io.ReadFull(crand.Reader, raw); err != nil {
		return nil, err
	}
	encoded = make([]byte, base64.RawURLEncoding.EncodedLen(len(raw)))
	base64.RawURLEncoding.Encode(encoded, raw)
	return encoded, nil
}

func randomBase64URL(size int) (string, error) {
	encoded, err := randomBase64URLBytes(size)
	if err != nil {
		return "", err
	}
	defer zeroBytes(encoded)
	return string(encoded), nil
}

func hashAberaOAuthSecretBytes(secret []byte) string {
	sum := sha256.Sum256(secret)
	return aberaSecretHashPrefix + base64.RawURLEncoding.EncodeToString(sum[:])
}

func hashAberaOAuthSecret(secret string) string {
	return hashAberaOAuthSecretBytes([]byte(secret))
}

func zeroBytes(value []byte) {
	for i := range value {
		value[i] = 0
	}
}

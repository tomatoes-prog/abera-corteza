//go:build cgo

package provision

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	automationTypes "github.com/cortezaproject/corteza/server/automation/types"
	composeTypes "github.com/cortezaproject/corteza/server/compose/types"
	"github.com/cortezaproject/corteza/server/pkg/actionlog"
	"github.com/cortezaproject/corteza/server/pkg/id"
	"github.com/cortezaproject/corteza/server/pkg/options"
	"github.com/cortezaproject/corteza/server/pkg/rbac"
	"github.com/cortezaproject/corteza/server/store"
	"github.com/cortezaproject/corteza/server/store/adapters/rdbms/drivers/sqlite"
	systemTypes "github.com/cortezaproject/corteza/server/system/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

var aberaTestIDOnce sync.Once

func TestLoadAberaBootstrapConfig(t *testing.T) {
	t.Run("disabled when no bootstrap variables exist", func(t *testing.T) {
		cfg, err := loadAberaBootstrapConfig(func(string) string { return "" })
		require.NoError(t, err)
		require.False(t, cfg.Enabled)
	})

	t.Run("valid and derives handle", func(t *testing.T) {
		dir := t.TempDir()
		values := map[string]string{
			"ABERA_INITIAL_ADMIN_EMAIL":       "Admin.Cliente@example.test",
			"ABERA_INITIAL_ADMIN_NAME":        "Administrador Cliente",
			"ABERA_MCP_MODE":                  "BASIC",
			"ABERA_BOOTSTRAP_PUBLIC_KEY_FILE": filepath.Join(dir, "public.pem"),
			"ABERA_BOOTSTRAP_OUTPUT_FILE":     filepath.Join(dir, "bootstrap.enc.json"),
		}
		cfg, err := loadAberaBootstrapConfig(func(name string) string { return values[name] })
		require.NoError(t, err)
		require.True(t, cfg.Enabled)
		require.Equal(t, "admin-cliente", cfg.AdminHandle)
		require.Equal(t, aberaMCPModeBasic, cfg.Mode)
	})

	for name, mutate := range map[string]func(map[string]string){
		"missing email": func(values map[string]string) { values["ABERA_INITIAL_ADMIN_EMAIL"] = "" },
		"missing name":  func(values map[string]string) { values["ABERA_INITIAL_ADMIN_NAME"] = "" },
		"invalid mode":  func(values map[string]string) { values["ABERA_MCP_MODE"] = "owner" },
		"relative key":  func(values map[string]string) { values["ABERA_BOOTSTRAP_PUBLIC_KEY_FILE"] = "public.pem" },
		"relative out":  func(values map[string]string) { values["ABERA_BOOTSTRAP_OUTPUT_FILE"] = "bootstrap.json" },
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			values := map[string]string{
				"ABERA_INITIAL_ADMIN_EMAIL":       "admin@example.test",
				"ABERA_INITIAL_ADMIN_NAME":        "Administrador",
				"ABERA_MCP_MODE":                  "basic",
				"ABERA_BOOTSTRAP_PUBLIC_KEY_FILE": filepath.Join(dir, "public.pem"),
				"ABERA_BOOTSTRAP_OUTPUT_FILE":     filepath.Join(dir, "bootstrap.enc.json"),
			}
			mutate(values)
			_, err := loadAberaBootstrapConfig(func(name string) string { return values[name] })
			require.Error(t, err)
		})
	}
}

func TestAberaBootstrapMutuallyExclusiveWithLegacySuperUser(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ABERA_INITIAL_ADMIN_EMAIL", "admin@example.test")
	t.Setenv("ABERA_INITIAL_ADMIN_NAME", "Administrador")
	t.Setenv("ABERA_MCP_MODE", "basic")
	t.Setenv("ABERA_BOOTSTRAP_PUBLIC_KEY_FILE", filepath.Join(dir, "public.pem"))
	t.Setenv("ABERA_BOOTSTRAP_OUTPUT_FILE", filepath.Join(dir, "bootstrap.enc.json"))

	err := Run(
		context.Background(),
		zap.NewNop(),
		nil,
		options.ProvisionOpt{},
		options.AuthOpt{ProvisionSuperUser: "admin@example.test"},
	)
	require.EqualError(t, err, "AUTH_PROVISION_SUPER_USER y el bootstrap OAuth de Abera son mutuamente excluyentes")
}

func TestAberaBootstrapEncryptedHandoffAndIdempotency(t *testing.T) {
	ctx, s := newAberaTestStore(t)
	privateKey, cfg := newAberaTestConfig(t, aberaMCPModeBasic, 3072)
	core, observed := observer.New(zap.DebugLevel)
	log := zap.New(core)

	require.NoError(t, provisionAberaBootstrap(ctx, log, s, cfg))

	raw, err := os.ReadFile(cfg.OutputFile)
	require.NoError(t, err)
	var envelope aberaEncryptedEnvelope
	require.NoError(t, json.Unmarshal(raw, &envelope))
	require.Equal(t, aberaEnvelopeAlgorithm, envelope.Algorithm)

	payload := decryptAberaEnvelope(t, privateKey, &envelope)
	require.Equal(t, 1, payload.Version)
	require.Equal(t, cfg.AdminEmail, payload.Admin.Email)
	require.Equal(t, cfg.AdminHandle, payload.Admin.Handle)
	require.Len(t, payload.Admin.Password, 32)
	require.Equal(t, "client_credentials", payload.OAuth.GrantType)
	require.Equal(t, "api", payload.OAuth.Scope)
	require.Equal(t, "/auth/oauth2/token", payload.OAuth.TokenURL)
	require.Equal(t, aberaMCPModeBasic, payload.OAuth.Mode)
	require.True(t, payload.OAuth.ExplicitApprovalRequired)
	require.NotEmpty(t, payload.OAuth.ClientSecret)
	require.NotContains(t, string(raw), payload.Admin.Password)
	require.NotContains(t, string(raw), payload.OAuth.ClientSecret)

	stateSetting, err := store.LookupSettingValueByNameOwnedBy(ctx, s, aberaBootstrapStateSetting, 0)
	require.NoError(t, err)
	require.NotContains(t, string(stateSetting.Value), payload.Admin.Password)
	require.NotContains(t, string(stateSetting.Value), payload.OAuth.ClientSecret)

	clientID, err := strconv.ParseUint(payload.OAuth.ClientID, 10, 64)
	require.NoError(t, err)
	client, err := store.LookupAuthClientByID(ctx, s, clientID)
	require.NoError(t, err)
	require.Equal(t, hashAberaOAuthSecret(payload.OAuth.ClientSecret), client.Secret)
	require.NotEqual(t, payload.OAuth.ClientSecret, client.Secret)

	userID, err := strconv.ParseUint(payload.Admin.UserID, 10, 64)
	require.NoError(t, err)
	credentials, _, err := store.SearchCredentials(ctx, s, systemTypes.CredentialFilter{OwnerID: userID})
	require.NoError(t, err)
	require.NotEmpty(t, credentials)
	for _, credential := range credentials {
		require.NotEqual(t, payload.Admin.Password, credential.Credentials)
		require.NotContains(t, credential.Credentials, payload.Admin.Password)
	}

	for _, entry := range observed.All() {
		serialized, marshalErr := json.Marshal(entry.ContextMap())
		require.NoError(t, marshalErr)
		require.NotContains(t, entry.Message+string(serialized), payload.Admin.Password)
		require.NotContains(t, entry.Message+string(serialized), payload.OAuth.ClientSecret)
	}

	if runtime.GOOS != "windows" {
		info, statErr := os.Stat(cfg.OutputFile)
		require.NoError(t, statErr)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}

	require.NoError(t, provisionAberaBootstrap(ctx, log, s, cfg))
	secondRaw, err := os.ReadFile(cfg.OutputFile)
	require.NoError(t, err)
	require.Equal(t, raw, secondRaw)

	users, _, err := store.SearchUsers(ctx, s, systemTypes.UserFilter{Email: cfg.AdminEmail})
	require.NoError(t, err)
	require.Len(t, users, 1)
	clients, _, err := store.SearchAuthClients(ctx, s, systemTypes.AuthClientFilter{Handle: aberaOAuthClientHandle})
	require.NoError(t, err)
	require.Len(t, clients, 1)
	require.Equal(t, client.Secret, clients[0].Secret)
}

func TestAberaBootstrapRecoversPendingDelivery(t *testing.T) {
	ctx, s := newAberaTestStore(t)
	privateKey, cfg := newAberaTestConfig(t, aberaMCPModeBasic, 3072)
	require.NoError(t, provisionAberaBootstrap(ctx, zap.NewNop(), s, cfg))

	initial, err := os.ReadFile(cfg.OutputFile)
	require.NoError(t, err)
	var initialEnvelope aberaEncryptedEnvelope
	require.NoError(t, json.Unmarshal(initial, &initialEnvelope))
	initialPayload := decryptAberaEnvelope(t, privateKey, &initialEnvelope)

	state, err := loadAberaBootstrapState(ctx, s)
	require.NoError(t, err)
	state.Status = "pending"
	require.NoError(t, saveAberaBootstrapState(ctx, s, state))
	require.NoError(t, provisionAberaBootstrap(ctx, zap.NewNop(), s, cfg))
	afterPublishedFile, err := os.ReadFile(cfg.OutputFile)
	require.NoError(t, err)
	require.Equal(t, initial, afterPublishedFile)
	state, err = loadAberaBootstrapState(ctx, s)
	require.NoError(t, err)
	require.Equal(t, "complete", state.Status)

	state.Status = "pending"
	require.NoError(t, saveAberaBootstrapState(ctx, s, state))
	require.NoError(t, os.Remove(cfg.OutputFile))

	require.NoError(t, provisionAberaBootstrap(ctx, zap.NewNop(), s, cfg))
	recovered, err := os.ReadFile(cfg.OutputFile)
	require.NoError(t, err)
	require.Equal(t, initial, recovered)
	var recoveredEnvelope aberaEncryptedEnvelope
	require.NoError(t, json.Unmarshal(recovered, &recoveredEnvelope))
	recoveredPayload := decryptAberaEnvelope(t, privateKey, &recoveredEnvelope)
	require.Equal(t, initialPayload.Admin.Password, recoveredPayload.Admin.Password)
	require.Equal(t, initialPayload.OAuth.ClientSecret, recoveredPayload.OAuth.ClientSecret)

	state, err = loadAberaBootstrapState(ctx, s)
	require.NoError(t, err)
	require.Equal(t, "complete", state.Status)
}

func TestAberaBootstrapRejectsChangedKeyAndManipulatedDelivery(t *testing.T) {
	ctx, s := newAberaTestStore(t)
	_, cfg := newAberaTestConfig(t, aberaMCPModeBasic, 3072)
	require.NoError(t, provisionAberaBootstrap(ctx, zap.NewNop(), s, cfg))

	_, changedKeyCfg := newAberaTestConfig(t, aberaMCPModeBasic, 3072)
	changedKeyCfg.AdminEmail = cfg.AdminEmail
	changedKeyCfg.AdminName = cfg.AdminName
	changedKeyCfg.AdminHandle = cfg.AdminHandle
	changedKeyCfg.OutputFile = cfg.OutputFile
	err := provisionAberaBootstrap(ctx, zap.NewNop(), s, changedKeyCfg)
	require.ErrorContains(t, err, "clave pública no coincide")

	tampered := []byte(`{"version":1,"algorithm":"manipulado"}`)
	require.NoError(t, os.WriteFile(cfg.OutputFile, tampered, 0o600))
	err = provisionAberaBootstrap(ctx, zap.NewNop(), s, cfg)
	require.ErrorContains(t, err, "manipulado")
	actual, readErr := os.ReadFile(cfg.OutputFile)
	require.NoError(t, readErr)
	require.Equal(t, tampered, actual)
}

func TestAberaBootstrapRejectsCoincidentUserWithoutMarker(t *testing.T) {
	ctx, s := newAberaTestStore(t)
	_, cfg := newAberaTestConfig(t, aberaMCPModeBasic, 3072)
	require.NoError(t, store.CreateUser(ctx, s, &systemTypes.User{
		ID:        nextID(),
		Email:     cfg.AdminEmail,
		Name:      "Cuenta preexistente",
		Handle:    "cuenta-preexistente",
		CreatedAt: *now(),
	}))

	err := provisionAberaBootstrap(ctx, zap.NewNop(), s, cfg)
	require.ErrorContains(t, err, "sin un marcador de bootstrap")
	_, statErr := os.Stat(cfg.OutputFile)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestAberaBootstrapModeChangeAndPermissionMatrix(t *testing.T) {
	ctx, s := newAberaTestStore(t)
	_, cfg := newAberaTestConfig(t, aberaMCPModeBasic, 3072)
	require.NoError(t, provisionAberaBootstrap(ctx, zap.NewNop(), s, cfg))

	cfg.Mode = aberaMCPModeAdmin
	require.NoError(t, provisionAberaBootstrap(ctx, zap.NewNop(), s, cfg))

	state, err := loadAberaBootstrapState(ctx, s)
	require.NoError(t, err)
	require.Equal(t, aberaMCPModeAdmin, state.Mode)

	roles, err := loadAberaRoles(ctx, s)
	require.NoError(t, err)
	adminRole := roles["abera-mcp-admin"]
	consultaRole := roles["abera-consulta"]
	client, err := store.LookupAuthClientByID(ctx, s, state.AuthClientID)
	require.NoError(t, err)
	require.Equal(t, []string{strconv.FormatUint(adminRole.ID, 10)}, client.Security.PermittedRoles)
	require.Equal(t, []string{strconv.FormatUint(adminRole.ID, 10)}, client.Security.ForcedRoles)

	rules, _, err := store.SearchRbacRules(ctx, s, rbac.RuleFilter{})
	require.NoError(t, err)
	require.True(t, hasAberaRule(rules, adminRole.ID, composeTypes.NamespaceRbacResource(0), "module.create", rbac.Allow))
	require.True(t, hasAberaRule(rules, adminRole.ID, composeTypes.ModuleRbacResource(0, 0), "update", rbac.Allow))
	require.False(t, hasAberaRule(rules, adminRole.ID, composeTypes.ModuleRbacResource(0, 0), "delete", rbac.Allow))
	require.True(t, hasAberaRule(rules, adminRole.ID, systemTypes.ComponentRbacResource(), "user.create", rbac.Allow))
	require.True(t, hasAberaRule(rules, adminRole.ID, systemTypes.ComponentRbacResource(), "role.create", rbac.Allow))
	require.False(t, hasAberaRule(rules, adminRole.ID, systemTypes.ComponentRbacResource(), "grant", rbac.Allow))
	require.False(t, hasAberaRule(rules, adminRole.ID, systemTypes.ComponentRbacResource(), "settings.manage", rbac.Allow))
	require.False(t, hasAberaRule(rules, adminRole.ID, systemTypes.ComponentRbacResource(), "auth-client.create", rbac.Allow))
	require.True(t, hasAberaRule(rules, adminRole.ID, systemTypes.ComponentRbacResource(), "auth-clients.search", rbac.Deny))
	require.True(t, hasAberaRule(rules, adminRole.ID, systemTypes.AuthClientRbacResource(0), "read", rbac.Deny))
	require.True(t, hasAberaRule(rules, adminRole.ID, composeTypes.ModuleRbacResource(0, 0), "delete", rbac.Deny))
	require.True(t, hasAberaRule(rules, adminRole.ID, systemTypes.RoleRbacResource(consultaRole.ID), "members.manage", rbac.Allow))
	require.True(t, hasAberaRule(rules, adminRole.ID, systemTypes.RoleRbacResource(0), "members.manage", rbac.Deny))

	actions, _, err := store.SearchActionlogs(ctx, s, actionlog.Filter{Action: "abera.mcp.mode.promote"})
	require.NoError(t, err)
	require.Len(t, actions, 1)
	require.Equal(t, actionlog.Notice, actions[0].Severity)
}

func TestAberaRoleMatrices(t *testing.T) {
	const (
		basicID    = uint64(101)
		proID      = uint64(102)
		adminID    = uint64(103)
		consultaID = uint64(104)
	)

	basic := aberaRulesForRole(basicID, "abera-mcp-basic", consultaID)
	require.True(t, hasAberaRule(basic, basicID, composeTypes.RecordRbacResource(0, 0, 0), "update", rbac.Allow))
	require.False(t, hasAberaRule(basic, basicID, composeTypes.RecordRbacResource(0, 0, 0), "delete", rbac.Allow))
	require.True(t, hasAberaRule(basic, basicID, automationTypes.ComponentRbacResource(), "workflow.create", rbac.Allow))
	require.True(t, hasAberaRule(basic, basicID, automationTypes.ComponentRbacResource(), "workflows.search", rbac.Allow))
	require.False(t, hasAberaRule(basic, basicID, automationTypes.WorkflowRbacResource(0), "update", rbac.Allow))
	require.False(t, hasAberaRule(basic, basicID, automationTypes.WorkflowRbacResource(0), "delete", rbac.Allow))
	require.False(t, hasAberaRule(basic, basicID, automationTypes.WorkflowRbacResource(0), "execute", rbac.Allow))
	require.True(t, hasAberaRule(basic, basicID, systemTypes.ComponentRbacResource(), "auth-clients.search", rbac.Deny))
	require.True(t, hasAberaRule(basic, basicID, systemTypes.ComponentRbacResource(), "users.search", rbac.Deny))
	require.False(t, hasAberaRule(basic, basicID, composeTypes.NamespaceRbacResource(0), "module.create", rbac.Allow))

	pro := aberaRulesForRole(proID, "abera-mcp-pro", consultaID)
	require.True(t, hasAberaRule(pro, proID, composeTypes.RecordRbacResource(0, 0, 0), "delete", rbac.Allow))
	require.True(t, hasAberaRule(pro, proID, automationTypes.ComponentRbacResource(), "workflow.create", rbac.Allow))
	require.True(t, hasAberaRule(pro, proID, automationTypes.ComponentRbacResource(), "workflows.search", rbac.Allow))
	require.True(t, hasAberaRule(pro, proID, automationTypes.WorkflowRbacResource(0), "update", rbac.Allow))
	require.False(t, hasAberaRule(pro, proID, automationTypes.WorkflowRbacResource(0), "delete", rbac.Allow))
	require.False(t, hasAberaRule(pro, proID, automationTypes.WorkflowRbacResource(0), "execute", rbac.Allow))
	require.False(t, hasAberaRule(pro, proID, composeTypes.NamespaceRbacResource(0), "module.create", rbac.Allow))

	admin := aberaRulesForRole(adminID, "abera-mcp-admin", consultaID)
	require.True(t, hasAberaRule(admin, adminID, composeTypes.NamespaceRbacResource(0), "module.create", rbac.Allow))
	require.True(t, hasAberaRule(admin, adminID, automationTypes.ComponentRbacResource(), "workflow.create", rbac.Allow))
	require.True(t, hasAberaRule(admin, adminID, automationTypes.ComponentRbacResource(), "workflows.search", rbac.Allow))
	require.True(t, hasAberaRule(admin, adminID, automationTypes.WorkflowRbacResource(0), "update", rbac.Allow))
	require.True(t, hasAberaRule(admin, adminID, automationTypes.WorkflowRbacResource(0), "delete", rbac.Allow))
	require.True(t, hasAberaRule(admin, adminID, automationTypes.WorkflowRbacResource(0), "execute", rbac.Allow))
	require.False(t, hasAberaRule(admin, adminID, composeTypes.ModuleRbacResource(0, 0), "delete", rbac.Allow))

	consulta := aberaRulesForRole(consultaID, "abera-consulta", consultaID)
	require.True(t, hasAberaRule(consulta, consultaID, composeTypes.RecordRbacResource(0, 0, 0), "read", rbac.Allow))
	require.False(t, hasAberaRule(consulta, consultaID, composeTypes.RecordRbacResource(0, 0, 0), "update", rbac.Allow))
	require.False(t, hasAberaRule(consulta, consultaID, composeTypes.ModuleRbacResource(0, 0), "record.create", rbac.Allow))
}

func TestAberaBootstrapRejectsUnsafeInputs(t *testing.T) {
	t.Run("small RSA key", func(t *testing.T) {
		_, cfg := newAberaTestConfig(t, aberaMCPModeBasic, 2048)
		_, _, err := loadAberaBootstrapPublicKey(cfg.PublicKeyFile)
		require.ErrorContains(t, err, "3072")
	})

	t.Run("invalid public key", func(t *testing.T) {
		filename := filepath.Join(t.TempDir(), "public.pem")
		require.NoError(t, os.WriteFile(filename, []byte("not a key"), 0o600))
		_, _, err := loadAberaBootstrapPublicKey(filename)
		require.ErrorContains(t, err, "PEM")
	})

	t.Run("unexpected output is never overwritten", func(t *testing.T) {
		filename := filepath.Join(t.TempDir(), "bootstrap.enc.json")
		require.NoError(t, os.WriteFile(filename, []byte("unexpected"), 0o600))
		err := writeAberaEnvelopeAtomic(filename, []byte("{}\n"))
		require.ErrorContains(t, err, "no coincide")
		actual, readErr := os.ReadFile(filename)
		require.NoError(t, readErr)
		require.Equal(t, []byte("unexpected"), actual)
	})

	t.Run("output symlink", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		link := filepath.Join(dir, "bootstrap.enc.json")
		require.NoError(t, os.WriteFile(target, []byte("target"), 0o600))
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("el sistema no permite crear symlinks para esta prueba: %v", err)
		}
		err := writeAberaEnvelopeAtomic(link, []byte("{}\n"))
		require.ErrorContains(t, err, "archivo regular")
	})
}

func TestAberaBootstrapProviderNeutrality(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	files := []string{
		filepath.Join(root, "server", "pkg", "provision", "abera_bootstrap.go"),
		filepath.Join(root, "server", "pkg", "provision", "provision.go"),
		filepath.Join(root, "server", "auth", "oauth2", "client.go"),
		filepath.Join(root, "Dockerfile"),
		filepath.Join(root, "BOOTSTRAP_OAUTH.md"),
	}
	forbidden := []string{
		"A" + "WS",
		"Cloud" + "Formation",
		"Secrets " + "Manager",
		"V" + "ault",
	}
	for _, filename := range files {
		raw, readErr := os.ReadFile(filename)
		require.NoError(t, readErr)
		lower := strings.ToLower(string(raw))
		for _, term := range forbidden {
			require.NotContains(t, lower, strings.ToLower(term), "%s contiene una referencia de proveedor", filename)
		}
	}
}

func newAberaTestStore(t *testing.T) (context.Context, store.Storer) {
	t.Helper()
	ctx := context.Background()
	aberaTestIDOnce.Do(func() { id.Init(ctx) })
	dsn := fmt.Sprintf("sqlite3+alt://file:abera_%d?mode=memory&cache=shared", nextID())
	s, err := sqlite.Connect(ctx, dsn)
	require.NoError(t, err)
	require.NoError(t, store.Upgrade(ctx, zap.NewNop(), s))
	_, err = SystemRoles(ctx, zap.NewNop(), s)
	require.NoError(t, err)
	return ctx, s
}

func newAberaTestConfig(t *testing.T, mode aberaMCPMode, bits int) (*rsa.PrivateKey, aberaBootstrapConfig) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(crand.Reader, bits)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)

	dir := t.TempDir()
	publicKeyFile := filepath.Join(dir, "public-key.pem")
	require.NoError(t, os.WriteFile(publicKeyFile, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0o600))

	return privateKey, aberaBootstrapConfig{
		Enabled:       true,
		AdminEmail:    "admin@example.test",
		AdminName:     "Administrador Cliente",
		AdminHandle:   "admin-cliente",
		Mode:          mode,
		PublicKeyFile: publicKeyFile,
		OutputFile:    filepath.Join(dir, "bootstrap.enc.json"),
	}
}

func decryptAberaEnvelope(t *testing.T, privateKey *rsa.PrivateKey, envelope *aberaEncryptedEnvelope) aberaBootstrapPayload {
	t.Helper()
	decode := func(value string) []byte {
		out, err := base64.RawURLEncoding.DecodeString(value)
		require.NoError(t, err)
		return out
	}

	key, err := rsa.DecryptOAEP(sha256.New(), crand.Reader, privateKey, decode(envelope.EncryptedKey), nil)
	require.NoError(t, err)
	block, err := aes.NewCipher(key)
	require.NoError(t, err)
	aead, err := cipher.NewGCM(block)
	require.NoError(t, err)
	plaintext, err := aead.Open(nil, decode(envelope.Nonce), decode(envelope.Ciphertext), []byte(aberaEnvelopeAAD))
	require.NoError(t, err)

	var payload aberaBootstrapPayload
	require.NoError(t, json.Unmarshal(plaintext, &payload))
	return payload
}

func hasAberaRule(
	rules rbac.RuleSet,
	roleID uint64,
	resource string,
	operation string,
	access rbac.Access,
) bool {
	for _, rule := range rules {
		if rule.RoleID == roleID &&
			rule.Resource == resource &&
			rule.Operation == operation &&
			rule.Access == access {
			return true
		}
	}
	return false
}

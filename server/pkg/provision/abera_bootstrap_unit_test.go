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
	"os"
	"path/filepath"
	"strings"
	"testing"

	automationTypes "github.com/cortezaproject/corteza/server/automation/types"
	composeTypes "github.com/cortezaproject/corteza/server/compose/types"
	"github.com/cortezaproject/corteza/server/pkg/options"
	"github.com/cortezaproject/corteza/server/pkg/rbac"
	systemTypes "github.com/cortezaproject/corteza/server/system/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLoadAberaBootstrapConfigUnit(t *testing.T) {
	cfg, err := loadAberaBootstrapConfig(func(string) string { return "" })
	require.NoError(t, err)
	require.False(t, cfg.Enabled)

	dir := t.TempDir()
	values := map[string]string{
		"ABERA_INITIAL_ADMIN_EMAIL":       "Admin.Cliente@example.test",
		"ABERA_INITIAL_ADMIN_NAME":        "Administrador Cliente",
		"ABERA_MCP_MODE":                  "BASIC",
		"ABERA_BOOTSTRAP_PUBLIC_KEY_FILE": filepath.Join(dir, "public.pem"),
		"ABERA_BOOTSTRAP_OUTPUT_FILE":     filepath.Join(dir, "bootstrap.enc.json"),
	}
	cfg, err = loadAberaBootstrapConfig(func(name string) string { return values[name] })
	require.NoError(t, err)
	require.True(t, cfg.Enabled)
	require.Equal(t, "admin-cliente", cfg.AdminHandle)
	require.Equal(t, aberaMCPModeBasic, cfg.Mode)

	for name, mutate := range map[string]func(map[string]string){
		"missing email": func(values map[string]string) { values["ABERA_INITIAL_ADMIN_EMAIL"] = "" },
		"missing name":  func(values map[string]string) { values["ABERA_INITIAL_ADMIN_NAME"] = "" },
		"invalid mode":  func(values map[string]string) { values["ABERA_MCP_MODE"] = "owner" },
		"relative key":  func(values map[string]string) { values["ABERA_BOOTSTRAP_PUBLIC_KEY_FILE"] = "public.pem" },
		"relative out":  func(values map[string]string) { values["ABERA_BOOTSTRAP_OUTPUT_FILE"] = "bootstrap.json" },
	} {
		t.Run(name, func(t *testing.T) {
			testValues := make(map[string]string, len(values))
			for key, value := range values {
				testValues[key] = value
			}
			mutate(testValues)
			_, configErr := loadAberaBootstrapConfig(func(key string) string { return testValues[key] })
			require.Error(t, configErr)
		})
	}
}

func TestAberaBootstrapMutualExclusionUnit(t *testing.T) {
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

func TestEncryptAberaBootstrapPayloadUnit(t *testing.T) {
	privateKey := generateAberaUnitKey(t, 3072)
	payload := aberaBootstrapPayload{
		Version: 1,
		Admin: aberaAdminPayload{
			UserID:   "123",
			Email:    "admin@example.test",
			Handle:   "admin",
			Password: "private-password",
		},
		OAuth: aberaOAuthPayload{
			ClientID:                 "456",
			ClientSecret:             "private-secret",
			GrantType:                "client_credentials",
			Scope:                    "api",
			TokenURL:                 "/auth/oauth2/token",
			Mode:                     aberaMCPModeBasic,
			ExplicitApprovalRequired: true,
		},
	}

	envelope, err := encryptAberaBootstrapPayload(&privateKey.PublicKey, &payload)
	require.NoError(t, err)
	raw, _, err := marshalAberaEnvelope(envelope)
	require.NoError(t, err)
	require.NotContains(t, string(raw), payload.Admin.Password)
	require.NotContains(t, string(raw), payload.OAuth.ClientSecret)

	decoded := decryptAberaEnvelopeUnit(t, privateKey, envelope)
	require.Equal(t, payload, decoded)
}

func TestAberaRoleMatricesUnit(t *testing.T) {
	const (
		basicID    = uint64(101)
		proID      = uint64(102)
		adminID    = uint64(103)
		consultaID = uint64(104)
	)

	basic := aberaRulesForRole(basicID, "abera-mcp-basic", consultaID)
	require.True(t, hasAberaRuleUnit(basic, basicID, composeTypes.RecordRbacResource(0, 0, 0), "update"))
	require.False(t, hasAberaRuleUnit(basic, basicID, composeTypes.RecordRbacResource(0, 0, 0), "delete"))
	require.True(t, hasAberaRuleUnit(basic, basicID, automationTypes.ComponentRbacResource(), "workflow.create"))
	require.True(t, hasAberaRuleUnit(basic, basicID, automationTypes.ComponentRbacResource(), "workflows.search"))
	require.False(t, hasAberaRuleUnit(basic, basicID, automationTypes.WorkflowRbacResource(0), "update"))
	require.False(t, hasAberaRuleUnit(basic, basicID, automationTypes.WorkflowRbacResource(0), "delete"))
	require.False(t, hasAberaRuleUnit(basic, basicID, automationTypes.WorkflowRbacResource(0), "execute"))
	require.True(t, hasAberaRuleUnitAccess(basic, basicID, systemTypes.ComponentRbacResource(), "auth-clients.search", rbac.Deny))
	require.True(t, hasAberaRuleUnitAccess(basic, basicID, systemTypes.ComponentRbacResource(), "users.search", rbac.Deny))
	require.False(t, hasAberaRuleUnit(basic, basicID, composeTypes.NamespaceRbacResource(0), "module.create"))

	pro := aberaRulesForRole(proID, "abera-mcp-pro", consultaID)
	require.True(t, hasAberaRuleUnit(pro, proID, composeTypes.RecordRbacResource(0, 0, 0), "delete"))
	require.True(t, hasAberaRuleUnit(pro, proID, automationTypes.ComponentRbacResource(), "workflow.create"))
	require.True(t, hasAberaRuleUnit(pro, proID, automationTypes.ComponentRbacResource(), "workflows.search"))
	require.True(t, hasAberaRuleUnit(pro, proID, automationTypes.WorkflowRbacResource(0), "update"))
	require.False(t, hasAberaRuleUnit(pro, proID, automationTypes.WorkflowRbacResource(0), "delete"))
	require.False(t, hasAberaRuleUnit(pro, proID, automationTypes.WorkflowRbacResource(0), "execute"))
	require.False(t, hasAberaRuleUnit(pro, proID, composeTypes.NamespaceRbacResource(0), "module.create"))

	admin := aberaRulesForRole(adminID, "abera-mcp-admin", consultaID)
	require.True(t, hasAberaRuleUnit(admin, adminID, composeTypes.NamespaceRbacResource(0), "module.create"))
	require.True(t, hasAberaRuleUnit(admin, adminID, automationTypes.ComponentRbacResource(), "workflow.create"))
	require.True(t, hasAberaRuleUnit(admin, adminID, automationTypes.ComponentRbacResource(), "workflows.search"))
	require.True(t, hasAberaRuleUnit(admin, adminID, automationTypes.WorkflowRbacResource(0), "update"))
	require.True(t, hasAberaRuleUnit(admin, adminID, automationTypes.WorkflowRbacResource(0), "delete"))
	require.True(t, hasAberaRuleUnit(admin, adminID, automationTypes.WorkflowRbacResource(0), "execute"))
	require.True(t, hasAberaRuleUnit(admin, adminID, systemTypes.ComponentRbacResource(), "user.create"))
	require.True(t, hasAberaRuleUnit(admin, adminID, systemTypes.RoleRbacResource(consultaID), "members.manage"))
	require.False(t, hasAberaRuleUnit(admin, adminID, composeTypes.ModuleRbacResource(0, 0), "delete"))
	require.False(t, hasAberaRuleUnit(admin, adminID, systemTypes.ComponentRbacResource(), "grant"))
	require.False(t, hasAberaRuleUnit(admin, adminID, systemTypes.ComponentRbacResource(), "settings.manage"))
	require.False(t, hasAberaRuleUnit(admin, adminID, systemTypes.ComponentRbacResource(), "auth-client.create"))
	require.True(t, hasAberaRuleUnitAccess(admin, adminID, systemTypes.ComponentRbacResource(), "auth-clients.search", rbac.Deny))
	require.True(t, hasAberaRuleUnitAccess(admin, adminID, systemTypes.AuthClientRbacResource(0), "read", rbac.Deny))
	require.True(t, hasAberaRuleUnitAccess(admin, adminID, composeTypes.ModuleRbacResource(0, 0), "delete", rbac.Deny))
	require.True(t, hasAberaRuleUnitAccess(admin, adminID, systemTypes.RoleRbacResource(0), "members.manage", rbac.Deny))

	consulta := aberaRulesForRole(consultaID, "abera-consulta", consultaID)
	require.True(t, hasAberaRuleUnit(consulta, consultaID, composeTypes.RecordRbacResource(0, 0, 0), "read"))
	require.False(t, hasAberaRuleUnit(consulta, consultaID, composeTypes.RecordRbacResource(0, 0, 0), "update"))
	require.False(t, hasAberaRuleUnit(consulta, consultaID, composeTypes.ModuleRbacResource(0, 0), "record.create"))
}

func TestAberaRoleDenialsOverrideAuthenticatedDefaultsUnit(t *testing.T) {
	const (
		userID          = uint64(900)
		basicID         = uint64(901)
		adminID         = uint64(902)
		consultaID      = uint64(903)
		authenticatedID = uint64(904)
		anotherRoleID   = uint64(905)
	)

	authenticatedRules := rbac.RuleSet{
		rbac.AllowRule(authenticatedID, systemTypes.ComponentRbacResource(), "auth-clients.search"),
		rbac.AllowRule(authenticatedID, systemTypes.UserRbacResource(0), "read"),
		rbac.AllowRule(authenticatedID, systemTypes.RoleRbacResource(0), "read"),
	}

	basicService := rbac.NewService(zap.NewNop(), nil)
	basicService.UpdateRoles(
		rbac.CommonRole.Make(basicID, "abera-mcp-basic"),
		rbac.AuthenticatedRole.Make(authenticatedID, "authenticated"),
	)
	require.NoError(t, basicService.Grant(
		context.Background(),
		append(aberaRulesForRole(basicID, "abera-mcp-basic", consultaID), authenticatedRules...)...,
	))
	basicSession := rbac.ParamsToSession(context.Background(), userID, basicID, authenticatedID)
	require.False(t, basicService.Can(basicSession, "auth-clients.search", rbac.NewResource(systemTypes.ComponentRbacResource())))
	require.False(t, basicService.Can(basicSession, "read", rbac.NewResource(systemTypes.UserRbacResource(42))))
	require.True(t, basicService.Can(basicSession, "namespaces.search", rbac.NewResource(composeTypes.ComponentRbacResource())))

	adminService := rbac.NewService(zap.NewNop(), nil)
	adminService.UpdateRoles(
		rbac.CommonRole.Make(adminID, "abera-mcp-admin"),
		rbac.AuthenticatedRole.Make(authenticatedID, "authenticated"),
	)
	require.NoError(t, adminService.Grant(
		context.Background(),
		append(aberaRulesForRole(adminID, "abera-mcp-admin", consultaID), authenticatedRules...)...,
	))
	adminSession := rbac.ParamsToSession(context.Background(), userID, adminID, authenticatedID)
	require.False(t, adminService.Can(adminSession, "auth-clients.search", rbac.NewResource(systemTypes.ComponentRbacResource())))
	require.True(t, adminService.Can(adminSession, "read", rbac.NewResource(systemTypes.UserRbacResource(42))))
	require.True(t, adminService.Can(adminSession, "members.manage", rbac.NewResource(systemTypes.RoleRbacResource(consultaID))))
	require.False(t, adminService.Can(adminSession, "members.manage", rbac.NewResource(systemTypes.RoleRbacResource(anotherRoleID))))
	require.False(t, adminService.Can(adminSession, "delete", rbac.NewResource(composeTypes.ModuleRbacResource(11, 22))))
}

func TestAberaBootstrapUnsafeInputsUnit(t *testing.T) {
	t.Run("small RSA key", func(t *testing.T) {
		privateKey := generateAberaUnitKey(t, 2048)
		filename := writeAberaUnitPublicKey(t, privateKey)
		_, _, err := loadAberaBootstrapPublicKey(filename)
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

func TestAberaBootstrapProviderNeutralityUnit(t *testing.T) {
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

func generateAberaUnitKey(t *testing.T, bits int) *rsa.PrivateKey {
	t.Helper()
	privateKey, err := rsa.GenerateKey(crand.Reader, bits)
	require.NoError(t, err)
	return privateKey
}

func writeAberaUnitPublicKey(t *testing.T, privateKey *rsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	filename := filepath.Join(t.TempDir(), "public-key.pem")
	require.NoError(t, os.WriteFile(filename, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0o600))
	return filename
}

func decryptAberaEnvelopeUnit(
	t *testing.T,
	privateKey *rsa.PrivateKey,
	envelope *aberaEncryptedEnvelope,
) aberaBootstrapPayload {
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

func hasAberaRuleUnit(rules rbac.RuleSet, roleID uint64, resource string, operation string) bool {
	return hasAberaRuleUnitAccess(rules, roleID, resource, operation, rbac.Allow)
}

func hasAberaRuleUnitAccess(rules rbac.RuleSet, roleID uint64, resource string, operation string, access rbac.Access) bool {
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

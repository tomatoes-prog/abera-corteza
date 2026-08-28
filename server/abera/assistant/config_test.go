// Copyright 2026 Abera/Corteza contributors
// Licensed under the Apache License, Version 2.0.

package assistant

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigDisabledDoesNotRequireSecrets(t *testing.T) {
	t.Setenv("ABERA_AI_ENABLED", "false")
	config, err := ConfigFromEnvironment()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Enabled {
		t.Fatal("expected the bridge to be disabled")
	}
}

func TestConfigLocalToken(t *testing.T) {
	directory := t.TempDir()
	tokenFile := filepath.Join(directory, "agent-token")
	if err := os.WriteFile(tokenFile, []byte(strings.Repeat("a", 48)), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ABERA_AI_ENABLED", "true")
	t.Setenv("ABERA_AI_AGENT_URL", "http://agent:8000")
	t.Setenv("ABERA_AI_TENANT_ID", "cliente-acme")
	t.Setenv("ABERA_AI_AUTH_MODE", "local_token")
	t.Setenv("ABERA_AI_LOCAL_TOKEN_FILE", tokenFile)
	t.Setenv("ABERA_AI_TIMEOUT", "90s")

	config, err := ConfigFromEnvironment()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.LocalToken != strings.Repeat("a", 48) || config.Timeout != 90*time.Second {
		t.Fatalf("unexpected config: %#v", config)
	}
}

func TestConfigRejectsRelativeSecretPath(t *testing.T) {
	t.Setenv("ABERA_AI_ENABLED", "true")
	t.Setenv("ABERA_AI_AGENT_URL", "http://agent:8000")
	t.Setenv("ABERA_AI_TENANT_ID", "cliente-acme")
	t.Setenv("ABERA_AI_AUTH_MODE", "local_token")
	t.Setenv("ABERA_AI_LOCAL_TOKEN_FILE", "relative-token")

	if _, err := ConfigFromEnvironment(); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("expected an absolute-path error, got %v", err)
	}
}

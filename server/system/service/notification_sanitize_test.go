package service

import (
	"strings"
	"testing"

	"github.com/cortezaproject/corteza/server/system/types"
)

func TestSanitizeNotificationConfig(t *testing.T) {
	cfg := types.NotificationConfig{
		Simple: types.SimpleNotificationConfig{
			Description: "<p>Hola</p><script>alert(\"xss\")</script><img src=\"x\" onerror=\"alert(1)\">",
		},
		Record: types.RecordNotificationConfig{
			Description: "<strong>Registro</strong><a href=\"javascript:alert(1)\">abrir</a>",
		},
	}

	sanitizeNotificationConfig(&cfg)

	for _, description := range []string{cfg.Simple.Description, cfg.Record.Description} {
		lower := strings.ToLower(description)
		if strings.Contains(lower, "<script") || strings.Contains(lower, "onerror") || strings.Contains(lower, "javascript:") {
			t.Fatalf("notification description was not sanitized: %q", description)
		}
	}

	if !strings.Contains(cfg.Simple.Description, "Hola") || !strings.Contains(cfg.Record.Description, "Registro") {
		t.Fatalf("sanitization removed safe content: %#v", cfg)
	}
}

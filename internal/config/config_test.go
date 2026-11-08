package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if !cfg.Limits.IsMIMEAllowed("image/png") {
		t.Errorf("expected image/png to be allowed")
	}
	if !cfg.Limits.IsMIMEAllowed("IMAGE/JPEG") {
		t.Errorf("expected IMAGE/JPEG to be allowed case-insensitively")
	}
	if cfg.Limits.IsMIMEAllowed("application/x-msdownload") {
		t.Errorf("expected .exe MIME to be rejected")
	}
	if cfg.Limits.PresignExpiry() != 1*time.Hour {
		t.Errorf("expected 1h presign expiry")
	}
}

func TestLoadConfigWithEnv(t *testing.T) {
	os.Setenv("UPLOADGATE_SERVER_PORT", "9999")
	defer os.Unsetenv("UPLOADGATE_SERVER_PORT")

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 9999 {
		t.Errorf("expected port 9999 from env, got %d", cfg.Server.Port)
	}
}

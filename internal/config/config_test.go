package config_test

import (
	"testing"
	"time"

	"telegram-audio-bot/internal/config"
)

func TestLoadRequiresBotToken(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("SHAZAM_SCRIPT_PATH", "")
	t.Setenv("AUDD_API_TOKEN", "")
	t.Setenv("ACR_HOST", "")
	t.Setenv("ACR_KEY", "")
	t.Setenv("ACR_SECRET", "")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected error for missing bot token")
	}
}

func TestLoadAcceptsShazamOnly(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("SHAZAM_SCRIPT_PATH", "/tmp/shazam.py")
	t.Setenv("AUDD_API_TOKEN", "")
	t.Setenv("ACR_HOST", "")
	t.Setenv("ACR_KEY", "")
	t.Setenv("ACR_SECRET", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.HasShazam() || cfg.HasACR() || cfg.HasAudD() {
		t.Fatalf("unexpected recognizer flags: %+v", cfg)
	}
}

func TestLoadEmptyShazamDisablesDefault(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("SHAZAM_SCRIPT_PATH", "")
	t.Setenv("AUDD_API_TOKEN", "audd")
	t.Setenv("ACR_HOST", "")
	t.Setenv("ACR_KEY", "")
	t.Setenv("ACR_SECRET", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HasShazam() {
		t.Fatal("empty SHAZAM_SCRIPT_PATH should disable Shazam")
	}
	if !cfg.HasAudD() {
		t.Fatal("expected AudD enabled")
	}
}

func TestLoadRejectsNoRecognizersWhenShazamDisabled(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("SHAZAM_SCRIPT_PATH", "")
	t.Setenv("AUDD_API_TOKEN", "")
	t.Setenv("ACR_HOST", "")
	t.Setenv("ACR_KEY", "")
	t.Setenv("ACR_SECRET", "")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected error when no recognizers configured")
	}
}

func TestLoadRejectsPartialACR(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("SHAZAM_SCRIPT_PATH", "/tmp/shazam.py")
	t.Setenv("ACR_HOST", "host")
	t.Setenv("ACR_KEY", "")
	t.Setenv("ACR_SECRET", "")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected error for partial ACR credentials")
	}
}

func TestLoadTimeoutOverrides(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("SHAZAM_SCRIPT_PATH", "/tmp/shazam.py")
	t.Setenv("ACR_HOST", "")
	t.Setenv("ACR_KEY", "")
	t.Setenv("ACR_SECRET", "")
	t.Setenv("EXTRACT_TIMEOUT", "15s")
	t.Setenv("MAX_UPLOAD_BYTES", "1048576")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ExtractTimeout != 15*time.Second {
		t.Fatalf("ExtractTimeout = %v", cfg.ExtractTimeout)
	}
	if cfg.MaxUploadBytes != 1048576 {
		t.Fatalf("MaxUploadBytes = %d", cfg.MaxUploadBytes)
	}
}

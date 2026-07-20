package config

import (
	"strings"
	"testing"
)

// setBaseEnv sets the minimum env required for Load() to reach security validation.
// strongSecret is a 64-char hex value (>= minGuestTokenSecretLen).
const strongSecret = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func setBaseEnv(t *testing.T, ginMode string) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("GIN_MODE", ginMode)
	// Default to safe values; individual tests override what they exercise.
	t.Setenv("GUEST_TOKEN_SECRET", strongSecret)
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")
	t.Setenv("MFA_ENCRYPTION_KEY", "")
}

func TestRelease_RejectsDevGuestSecret(t *testing.T) {
	setBaseEnv(t, "release")
	t.Setenv("GUEST_TOKEN_SECRET", devGuestTokenSecret)
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GUEST_TOKEN_SECRET") {
		t.Fatalf("expected dev guest secret to fail in release, got err=%v", err)
	}
}

func TestRelease_RejectsShortGuestSecret(t *testing.T) {
	setBaseEnv(t, "release")
	t.Setenv("GUEST_TOKEN_SECRET", "too-short-secret")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "too short") {
		t.Fatalf("expected short guest secret to fail in release, got err=%v", err)
	}
}

func TestRelease_RejectsEmptyCORS(t *testing.T) {
	setBaseEnv(t, "release")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "CORS_ALLOWED_ORIGINS") {
		t.Fatalf("expected empty CORS to fail in release, got err=%v", err)
	}
}

func TestRelease_RejectsShortMFAKey(t *testing.T) {
	setBaseEnv(t, "release")
	t.Setenv("MFA_ENCRYPTION_KEY", "short")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "MFA_ENCRYPTION_KEY") {
		t.Fatalf("expected short MFA key to fail in release, got err=%v", err)
	}
}

func TestRelease_RejectsShortWebhookSecret(t *testing.T) {
	setBaseEnv(t, "release")
	t.Setenv("PAYMENT_WEBHOOK_SECRET_RAZORPAY", "short")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "PAYMENT_WEBHOOK_SECRET_RAZORPAY") {
		t.Fatalf("expected short webhook secret to fail in release, got err=%v", err)
	}
}

func TestRelease_AcceptsSecureConfig(t *testing.T) {
	setBaseEnv(t, "release")
	t.Setenv("MFA_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected secure release config to boot, got err=%v", err)
	}
	if cfg.Auth.GuestTokenSecret != strongSecret {
		t.Fatalf("unexpected guest secret: %q", cfg.Auth.GuestTokenSecret)
	}
}

func TestDebug_AllowsDevDefaults(t *testing.T) {
	setBaseEnv(t, "debug")
	t.Setenv("GUEST_TOKEN_SECRET", devGuestTokenSecret)
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	if _, err := Load(); err != nil {
		t.Fatalf("debug mode should permit dev defaults, got err=%v", err)
	}
}

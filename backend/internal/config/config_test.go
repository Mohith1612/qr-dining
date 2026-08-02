package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	if cfg.Auth.GuestTokenTTL != 12*time.Hour {
		t.Fatalf("expected safe guest token TTL default, got %s", cfg.Auth.GuestTokenTTL)
	}
	if !cfg.FeatureFlags.AuthGuestCredentialsRequired ||
		!cfg.FeatureFlags.AuthStaffCodeRequired ||
		!cfg.FeatureFlags.AuthStaffSessionDBRequired ||
		!cfg.FeatureFlags.WSTicketAuthRequired {
		t.Fatal("expected R4, R5, and R6 authentication enforcement to default on")
	}
}

func TestRelease_AllowsExplicitTimeBoundAuthRollback(t *testing.T) {
	setBaseEnv(t, "release")
	t.Setenv("AUTH_GUEST_CREDENTIALS_REQUIRED", "false")
	t.Setenv("AUTH_STAFF_CODE_REQUIRED", "false")
	t.Setenv("AUTH_STAFF_SESSION_DB_REQUIRED", "false")
	t.Setenv("WS_TICKET_AUTH_REQUIRED", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected explicit rollback config to boot, got err=%v", err)
	}
	if cfg.FeatureFlags.AuthGuestCredentialsRequired ||
		cfg.FeatureFlags.AuthStaffCodeRequired ||
		cfg.FeatureFlags.AuthStaffSessionDBRequired ||
		cfg.FeatureFlags.WSTicketAuthRequired {
		t.Fatal("explicit false values must retain the supervised rollback path")
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

func TestRelease_DotenvCannotOverrideInjectedEnvironment(t *testing.T) {
	setBaseEnv(t, "release")
	tempDir := t.TempDir()
	dotenv := strings.Join([]string{
		"DATABASE_URL=postgres://attacker:pass@localhost:5432/other",
		"GUEST_TOKEN_SECRET=too-short",
		"AUTH_GUEST_CREDENTIALS_REQUIRED=false",
	}, "\n")
	if err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte(dotenv), 0o600); err != nil {
		t.Fatalf("write .env fixture: %v", err)
	}

	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("release config should ignore stray .env, got err=%v", err)
	}
	if cfg.DB.URL != "postgres://user:pass@localhost:5432/db" {
		t.Fatalf("stray .env overrode DATABASE_URL: %q", cfg.DB.URL)
	}
	if !cfg.FeatureFlags.AuthGuestCredentialsRequired {
		t.Fatal("stray .env disabled guest credential enforcement")
	}
}

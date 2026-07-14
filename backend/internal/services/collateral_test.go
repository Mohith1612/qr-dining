package services

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/domain"
)

func TestDecodeCollateralStrict_RejectsUnknownKeys(t *testing.T) {
	_, err := decodeCollateralStrict(json.RawMessage(`{"format":"sticker","customCss":"body{}"}`))
	if !errors.Is(err, domain.ErrInvalidCollateralConfig) {
		t.Fatalf("expected ErrInvalidCollateralConfig for unknown key, got %v", err)
	}
}

func TestDecodeCollateralStrict_EmptyYieldsDefaults(t *testing.T) {
	cfg, err := decodeCollateralStrict(json.RawMessage(``))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Format != defaultCollateralFormat || !cfg.ShowLogo || !cfg.ShowFooter {
		t.Fatalf("empty input should yield defaults, got %+v", cfg)
	}
}

func TestDecodeCollateralStrict_DefaultsPreservedForAbsentFields(t *testing.T) {
	// showLogo absent -> stays default true; showWifi explicit true -> respected.
	cfg, err := decodeCollateralStrict(json.RawMessage(`{"format":"standing_card","showWifi":true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.ShowLogo {
		t.Fatalf("absent showLogo should keep default true")
	}
	if !cfg.ShowWifi {
		t.Fatalf("explicit showWifi=true should be respected")
	}
}

func TestValidateCollateral_BadFormat(t *testing.T) {
	cfg := DefaultCollateralConfig()
	cfg.Format = "billboard"
	if err := validateCollateral(cfg); !errors.Is(err, domain.ErrInvalidCollateralConfig) {
		t.Fatalf("expected invalid-format error, got %v", err)
	}
}

func TestValidateCollateral_OversizedText(t *testing.T) {
	cfg := DefaultCollateralConfig()
	cfg.WelcomeMessage = strings.Repeat("a", collateralTextLimits["welcomeMessage"]+1)
	if err := validateCollateral(cfg); !errors.Is(err, domain.ErrInvalidCollateralConfig) {
		t.Fatalf("expected oversized-text error, got %v", err)
	}
}

func TestValidateCollateral_AllFormatsAllowed(t *testing.T) {
	for _, f := range AllowedCollateralFormats() {
		cfg := DefaultCollateralConfig()
		cfg.Format = f
		if err := validateCollateral(cfg); err != nil {
			t.Fatalf("format %q should be valid, got %v", f, err)
		}
	}
}

func TestNormalizeCollateral_TrimsWhitespace(t *testing.T) {
	cfg := DefaultCollateralConfig()
	cfg.WelcomeMessage = "  Scan to begin  "
	cfg.WifiName = "  Lounge-WiFi "
	got := normalizeCollateral(cfg)
	if got.WelcomeMessage != "Scan to begin" || got.WifiName != "Lounge-WiFi" {
		t.Fatalf("expected trimmed text, got %+v", got)
	}
}

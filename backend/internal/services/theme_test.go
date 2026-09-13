package services

import (
	"errors"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/domain"
)

func TestValidateThemeTokens(t *testing.T) {
	tests := []struct {
		name    string
		tokens  map[string]string
		wantErr bool
	}{
		{"empty ok", map[string]string{}, false},
		{"valid 6-hex", map[string]string{"accent": "#C9A876"}, false},
		{"valid 8-hex alpha", map[string]string{"bg-base": "#0E0C09FF"}, false},
		{"multiple valid", map[string]string{"accent": "#aabbcc", "ink-1": "#FFFFFF"}, false},
		{"unknown key rejected", map[string]string{"evil": "#FFFFFF"}, true},
		{"non-hex value rejected", map[string]string{"accent": "red"}, true},
		{"css injection rejected", map[string]string{"accent": "#fff; background: url(x)"}, true},
		{"short hex rejected", map[string]string{"accent": "#FFF"}, true},
		{"missing hash rejected", map[string]string{"accent": "C9A876"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateThemeTokens(tc.tokens)
			if tc.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantErr && err != nil && !errors.Is(err, domain.ErrInvalidThemeToken) {
				t.Errorf("error should wrap ErrInvalidThemeToken, got %v", err)
			}
		})
	}
}

func TestAllowedThemeTokenKeysSorted(t *testing.T) {
	keys := AllowedThemeTokenKeys()
	if len(keys) != len(allowedThemeTokens) {
		t.Fatalf("got %d keys, want %d", len(keys), len(allowedThemeTokens))
	}
	for i := 1; i < len(keys); i++ {
		if keys[i-1] > keys[i] {
			t.Errorf("keys not sorted: %q before %q", keys[i-1], keys[i])
		}
	}
}

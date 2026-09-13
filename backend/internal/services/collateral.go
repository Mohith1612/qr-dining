package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/jackc/pgx/v5"
)

// Premium QR collateral configuration. This is the *physical/content* concern only —
// chosen print format, content toggles, and text/WiFi/social content. It deliberately
// holds NO theme tokens: the structured theme (services/theme.go) stays the single
// source of truth for colour/typography, and the frontend renderer composes
// Theme + Branch metadata + this config. No arbitrary HTML/CSS is ever accepted —
// the format is allowlisted, unknown keys are rejected, and strings are length-capped.

const defaultCollateralFormat = "standing_card"

// allowedCollateralFormats are the supported physical print layouts. These mirror the
// frontend format registry (frontend/lib/collateral/formats.ts).
var allowedCollateralFormats = map[string]bool{
	"standing_card": true,
	"table_tent":    true,
	"sticker":       true,
	"square_card":   true,
	"bulk_sheet":    true,
}

// collateralTextLimits caps each free-text field (rune count). Generous enough for
// hospitality copy, tight enough to keep layouts clean and reject abuse.
var collateralTextLimits = map[string]int{
	"welcomeMessage": 120,
	"subtitle":       160,
	"footerNote":     160,
	"branchDisplay":  60,
	"tagline":        120,
	"wifiName":       64,
	"wifiPassword":   64,
	"instagram":      120,
	"website":        200,
}

// CollateralConfig is the structured per-branch collateral configuration. JSON tags are
// camelCase to match the frontend config object verbatim (no translation layer).
type CollateralConfig struct {
	Format         string `json:"format"`
	ShowLogo       bool   `json:"showLogo"`
	ShowBranch     bool   `json:"showBranch"`
	ShowWifi       bool   `json:"showWifi"`
	ShowFooter     bool   `json:"showFooter"`
	WelcomeMessage string `json:"welcomeMessage"`
	Subtitle       string `json:"subtitle"`
	FooterNote     string `json:"footerNote"`
	BranchDisplay  string `json:"branchDisplay"`
	Tagline        string `json:"tagline"`
	WifiName       string `json:"wifiName"`
	WifiPassword   string `json:"wifiPassword"`
	Instagram      string `json:"instagram"`
	Website        string `json:"website"`
}

// DefaultCollateralConfig is the starting point for a branch with no saved config.
func DefaultCollateralConfig() CollateralConfig {
	return CollateralConfig{
		Format:     defaultCollateralFormat,
		ShowLogo:   true,
		ShowBranch: true,
		ShowWifi:   false,
		ShowFooter: true,
	}
}

// AllowedCollateralFormats returns the sorted format allowlist (for API discovery).
func AllowedCollateralFormats() []string {
	keys := make([]string, 0, len(allowedCollateralFormats))
	for k := range allowedCollateralFormats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// CollateralService reads and writes the structured collateral config for a branch.
// It is the single validation/storage authority shared by the platform and staff
// trust domains (both write the same branch_collateral row).
type CollateralService struct {
	repos *repository.Repos
}

func NewCollateralService(repos *repository.Repos) *CollateralService {
	return &CollateralService{repos: repos}
}

// GetForBranch returns the stored config (merged over defaults), or the defaults when
// none has been saved. A persisted-but-unknown format degrades to the default format.
func (s *CollateralService) GetForBranch(ctx context.Context, branchID int64) (CollateralConfig, error) {
	row, err := s.repos.GetBranchCollateral(ctx, branchID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DefaultCollateralConfig(), nil
		}
		return CollateralConfig{}, err
	}
	cfg := DefaultCollateralConfig()
	_ = json.Unmarshal(row.ConfigJson, &cfg)
	if !allowedCollateralFormats[cfg.Format] {
		cfg.Format = defaultCollateralFormat
	}
	return cfg, nil
}

// SetForBranch strictly decodes raw config JSON (unknown keys rejected), validates the
// format and field lengths, normalizes (trims) text, and persists. actorID is the
// platform user id for platform writes, nil for staff writes.
func (s *CollateralService) SetForBranch(ctx context.Context, branchID int64, raw json.RawMessage, actorID *int64) (CollateralConfig, error) {
	cfg, err := decodeCollateralStrict(raw)
	if err != nil {
		return CollateralConfig{}, err
	}
	cfg = normalizeCollateral(cfg)
	if err := validateCollateral(cfg); err != nil {
		return CollateralConfig{}, err
	}
	out, err := json.Marshal(cfg)
	if err != nil {
		return CollateralConfig{}, err
	}
	row, err := s.repos.UpsertBranchCollateral(ctx, branchID, out, actorID)
	if err != nil {
		return CollateralConfig{}, err
	}
	stored := DefaultCollateralConfig()
	_ = json.Unmarshal(row.ConfigJson, &stored)
	return stored, nil
}

// decodeCollateralStrict decodes onto a defaults base so absent fields keep their
// defaults, while rejecting any unknown key (no arbitrary fields).
func decodeCollateralStrict(raw json.RawMessage) (CollateralConfig, error) {
	cfg := DefaultCollateralConfig()
	if len(bytes.TrimSpace(raw)) == 0 {
		return cfg, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return CollateralConfig{}, fmt.Errorf("%w: %v", domain.ErrInvalidCollateralConfig, err)
	}
	return cfg, nil
}

func normalizeCollateral(cfg CollateralConfig) CollateralConfig {
	cfg.Format = strings.TrimSpace(cfg.Format)
	cfg.WelcomeMessage = strings.TrimSpace(cfg.WelcomeMessage)
	cfg.Subtitle = strings.TrimSpace(cfg.Subtitle)
	cfg.FooterNote = strings.TrimSpace(cfg.FooterNote)
	cfg.BranchDisplay = strings.TrimSpace(cfg.BranchDisplay)
	cfg.Tagline = strings.TrimSpace(cfg.Tagline)
	cfg.WifiName = strings.TrimSpace(cfg.WifiName)
	cfg.WifiPassword = strings.TrimSpace(cfg.WifiPassword)
	cfg.Instagram = strings.TrimSpace(cfg.Instagram)
	cfg.Website = strings.TrimSpace(cfg.Website)
	return cfg
}

func validateCollateral(cfg CollateralConfig) error {
	if !allowedCollateralFormats[cfg.Format] {
		return fmt.Errorf("%w: unknown format %q", domain.ErrInvalidCollateralConfig, cfg.Format)
	}
	fields := map[string]string{
		"welcomeMessage": cfg.WelcomeMessage,
		"subtitle":       cfg.Subtitle,
		"footerNote":     cfg.FooterNote,
		"branchDisplay":  cfg.BranchDisplay,
		"tagline":        cfg.Tagline,
		"wifiName":       cfg.WifiName,
		"wifiPassword":   cfg.WifiPassword,
		"instagram":      cfg.Instagram,
		"website":        cfg.Website,
	}
	for name, val := range fields {
		if max := collateralTextLimits[name]; max > 0 && utf8.RuneCountInString(val) > max {
			return fmt.Errorf("%w: %s exceeds %d characters", domain.ErrInvalidCollateralConfig, name, max)
		}
	}
	return nil
}

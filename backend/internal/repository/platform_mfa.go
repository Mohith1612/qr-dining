package repository

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// MFA-related repository helpers. The sqlc-generated types stay internal to
// this file; callers see plain Go structs.

var ErrMFANotEnrolled = errors.New("mfa not enrolled")

type PlatformMFA struct {
	PlatformUserID   int64
	SecretEncrypted  string
	Status           string
	RecoveryCodes    []string
	EnrolledAtUnix   int64
}

func (r *Repos) GetPlatformMFA(ctx context.Context, userID int64) (PlatformMFA, error) {
	row, err := r.q.GetPlatformMFA(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlatformMFA{}, ErrMFANotEnrolled
	}
	if err != nil {
		return PlatformMFA{}, err
	}
	var codes []string
	if len(row.RecoveryCodes) > 0 {
		_ = json.Unmarshal(row.RecoveryCodes, &codes)
	}
	return PlatformMFA{
		PlatformUserID:  row.PlatformUserID,
		SecretEncrypted: row.SecretEncrypted,
		Status:          row.Status,
		RecoveryCodes:   codes,
		EnrolledAtUnix:  row.EnrolledAt.Time.Unix(),
	}, nil
}

func (r *Repos) UpsertPendingPlatformMFA(ctx context.Context, userID int64, encryptedSecret string) error {
	_, err := r.q.UpsertPlatformMFAPending(ctx, sqlc.UpsertPlatformMFAPendingParams{
		PlatformUserID:  userID,
		SecretEncrypted: encryptedSecret,
	})
	return err
}

func (r *Repos) ActivatePlatformMFA(ctx context.Context, userID int64, recoveryHashes []string) error {
	codes, err := json.Marshal(recoveryHashes)
	if err != nil {
		return err
	}
	_, err = r.q.ActivatePlatformMFA(ctx, sqlc.ActivatePlatformMFAParams{
		PlatformUserID: userID,
		RecoveryCodes:  codes,
	})
	return err
}

func (r *Repos) DisablePlatformMFA(ctx context.Context, userID int64) error {
	return r.q.DisablePlatformMFA(ctx, userID)
}

func (r *Repos) TouchPlatformMFAUse(ctx context.Context, userID int64) error {
	return r.q.TouchPlatformMFAUse(ctx, userID)
}

func (r *Repos) ConsumeRecoveryCodes(ctx context.Context, userID int64, remaining []string) error {
	codes, err := json.Marshal(remaining)
	if err != nil {
		return err
	}
	return r.q.ConsumeRecoveryCodes(ctx, sqlc.ConsumeRecoveryCodesParams{
		PlatformUserID: userID,
		RecoveryCodes:  codes,
	})
}

func (r *Repos) CreatePlatformMFAChallenge(ctx context.Context, userID int64, challengeHash string, expiresAt time.Time, ip string, userAgent string) error {
	var addr *netip.Addr
	if parsed, err := netip.ParseAddr(ip); err == nil {
		addr = &parsed
	}
	_, err := r.q.CreatePlatformMFAChallenge(ctx, sqlc.CreatePlatformMFAChallengeParams{
		PlatformUserID: userID,
		ChallengeHash:  challengeHash,
		ExpiresAt:      expiresAt,
		Ip:             addr,
		UserAgent:      pgtype.Text{String: userAgent, Valid: userAgent != ""},
	})
	return err
}

func (r *Repos) ResolvePlatformMFAChallenge(ctx context.Context, challengeHash string) (int64, error) {
	row, err := r.q.GetPlatformMFAChallengeByHash(ctx, challengeHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrMFANotEnrolled
	}
	if err != nil {
		return 0, err
	}
	return row.PlatformUserID, nil
}

func (r *Repos) ConsumePlatformMFAChallenge(ctx context.Context, challengeHash string) error {
	return r.q.ConsumePlatformMFAChallenge(ctx, challengeHash)
}

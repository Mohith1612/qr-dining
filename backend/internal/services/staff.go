package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

// staffTokenSetKey returns the Redis Set key that tracks all active token keys for a staff member.
func staffTokenSetKey(staffID int64) string {
	return fmt.Sprintf(staffTokenSetFmt, staffID)
}

const (
	bcryptCost       = 12
	staffTokenTTL    = 8 * time.Hour
	staffTokenPrefix = "staff:token:"
	staffTokenSetFmt = "staff:tokens:%d" // set of active token keys per staff ID
)

type StaffService struct {
	repos        *repository.Repos
	cache        *redisPkg.Cache
	logger       zerolog.Logger
	requireDBRow bool
}

func NewStaffService(repos *repository.Repos, cache *redisPkg.Cache, logger zerolog.Logger) *StaffService {
	return &StaffService{repos: repos, cache: cache, logger: logger}
}

// SetRequireSessionDBRow toggles strict DB-backed staff session validation.
// When true, ValidateToken rejects any Redis-only hit whose corresponding
// staff_sessions row is missing or stale. Wired from
// AUTH_STAFF_SESSION_DB_REQUIRED at bootstrap.
func (s *StaffService) SetRequireSessionDBRow(require bool) { s.requireDBRow = require }

type StaffSession struct {
	Token          string         `json:"token"`
	SessionToken   string         `json:"session_token"`
	SessionID      uuid.UUID      `json:"session_id"`
	StaffID        int64          `json:"staff_id"`
	BranchID       int64          `json:"branch_id"`
	OrganizationID int64          `json:"organization_id"`
	Role           sqlc.StaffRole `json:"role"`
	StaffCode      string         `json:"staff_code"`
	TokenVersion   int32          `json:"token_version"`
	PinVersion     int32          `json:"pin_version"`
}

// Authenticate verifies a staff PIN against stored bcrypt hashes for the branch.
// Returns a session token stored in Redis on success.
func (s *StaffService) Authenticate(ctx context.Context, branchID int64, pin string) (StaffSession, error) {
	staff, err := s.repos.ListActiveStaffForBranch(ctx, branchID)
	if err != nil {
		return StaffSession{}, err
	}

	matches := make([]sqlc.Staff, 0, 1)
	for i := range staff {
		if err := bcrypt.CompareHashAndPassword([]byte(staff[i].PinHash), []byte(pin)); err == nil {
			matches = append(matches, staff[i])
		}
	}
	if len(matches) != 1 {
		return StaffSession{}, domain.ErrParticipantUnauthorized
	}
	return s.createSession(ctx, matches[0], "")
}

func (s *StaffService) AuthenticateWithCode(ctx context.Context, branchCode, staffCode, pin, deviceName string) (StaffSession, error) {
	branch, err := s.repos.GetBranchByCode(ctx, normalizeCode(branchCode))
	if err != nil {
		return StaffSession{}, domain.ErrParticipantUnauthorized
	}
	staff, err := s.repos.GetStaffByBranchAndCode(ctx, branch.ID, normalizeCode(staffCode))
	if err != nil {
		return StaffSession{}, domain.ErrParticipantUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(staff.PinHash), []byte(pin)); err != nil {
		return StaffSession{}, domain.ErrParticipantUnauthorized
	}
	return s.createSession(ctx, staff, deviceName)
}

func (s *StaffService) createSession(ctx context.Context, staff sqlc.Staff, deviceName string) (StaffSession, error) {
	token := uuid.NewString()
	expiresAt := time.Now().UTC().Add(staffTokenTTL)
	branch, err := s.repos.GetBranchByID(ctx, staff.BranchID)
	if err != nil {
		return StaffSession{}, fmt.Errorf("resolve staff branch: %w", err)
	}
	if branch.Status != "active" {
		return StaffSession{}, domain.ErrParticipantUnauthorized
	}
	organization, err := s.repos.GetOrganizationByBranchID(ctx, staff.BranchID)
	if err != nil {
		return StaffSession{}, fmt.Errorf("resolve staff organization: %w", err)
	}
	if organization.Status != "active" {
		return StaffSession{}, domain.ErrParticipantUnauthorized
	}
	dbSession, err := s.repos.CreateStaffSession(
		ctx,
		staff.ID,
		staff.BranchID,
		hashToken(token),
		deviceName,
		staff.TokenVersion,
		staff.PinVersion,
		expiresAt,
	)
	if err != nil {
		return StaffSession{}, fmt.Errorf("create staff session: %w", err)
	}
	session := StaffSession{
		Token:          token,
		SessionToken:   token,
		SessionID:      dbSession.ID,
		StaffID:        staff.ID,
		BranchID:       staff.BranchID,
		OrganizationID: organization.ID,
		Role:           staff.Role,
		StaffCode:      staff.StaffCode,
		TokenVersion:   staff.TokenVersion,
		PinVersion:     staff.PinVersion,
	}

	tokenKey := staffTokenPrefix + token
	if err := s.cache.Set(ctx, tokenKey, session, staffTokenTTL); err != nil {
		return StaffSession{}, fmt.Errorf("store staff token: %w", err)
	}
	// Track this token key in a per-staff Set for O(1) batch invalidation on deactivation.
	if err := s.cache.SAdd(ctx, staffTokenSetKey(staff.ID), tokenKey, staffTokenTTL+time.Minute); err != nil {
		s.logger.Warn().Err(err).Int64("staff_id", staff.ID).Msg("failed to track staff token in set; deactivation will not invalidate this token")
	}
	return session, nil
}

// ValidateToken retrieves the staff session for a given token from Redis.
func (s *StaffService) ValidateToken(ctx context.Context, token string) (StaffSession, error) {
	var session StaffSession
	hit, err := s.cache.Get(ctx, staffTokenPrefix+token, &session)
	if err != nil {
		return StaffSession{}, err
	}
	if !hit {
		return StaffSession{}, errors.New("invalid or expired staff token")
	}
	if err := s.validateSessionState(ctx, token, session); err != nil {
		return StaffSession{}, err
	}
	return session, nil
}

func (s *StaffService) validateSessionState(ctx context.Context, token string, session StaffSession) error {
	staff, err := s.repos.GetStaffByID(ctx, session.StaffID)
	if err != nil {
		return err
	}
	if !staff.IsActive || staff.TokenVersion != session.TokenVersion || staff.PinVersion != session.PinVersion {
		return errors.New("invalid or expired staff token")
	}
	if session.SessionID == uuid.Nil {
		// Token was issued before staff_sessions existed. Strict mode rejects;
		// shadow mode allows so the rollout can proceed without invalidating
		// every active shift on flag flip.
		if s.requireDBRow {
			return errors.New("invalid or expired staff token")
		}
		return nil
	}
	dbSession, err := s.repos.GetActiveStaffSessionByTokenHash(ctx, hashToken(token))
	if err != nil {
		if s.requireDBRow {
			return errors.New("invalid or expired staff token")
		}
		// Permissive mode: missing DB row is treated as advisory. The Redis
		// session is still bound to staff_id/token_version/pin_version which
		// just passed validation above.
		return nil
	}
	if dbSession.ID != session.SessionID || dbSession.TokenVersion != session.TokenVersion || dbSession.PinVersion != session.PinVersion {
		return errors.New("invalid or expired staff token")
	}
	_ = s.repos.TouchStaffSession(ctx, dbSession.ID)
	return nil
}

// CreateStaff creates a new staff member with a bcrypt-hashed PIN.
// Only owners may create staff (enforced in handler via middleware).
func (s *StaffService) CreateStaff(ctx context.Context, branchID int64, role sqlc.StaffRole, name, staffCode, pin string) (sqlc.Staff, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcryptCost)
	if err != nil {
		return sqlc.Staff{}, fmt.Errorf("hash PIN: %w", err)
	}
	if staffCode == "" {
		staffCode = defaultStaffCode(name)
	}
	return s.repos.CreateStaff(ctx, sqlc.CreateStaffParams{
		BranchID:  branchID,
		Name:      name,
		Role:      role,
		PinHash:   string(hash),
		StaffCode: normalizeCode(staffCode),
	})
}

// RotatePIN verifies the current PIN and replaces it with the new one.
func (s *StaffService) RotatePIN(ctx context.Context, staffID int64, currentPIN, newPIN string) error {
	staff, err := s.repos.GetStaffByID(ctx, staffID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(staff.PinHash), []byte(currentPIN)); err != nil {
		return domain.ErrUnauthorized
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPIN), bcryptCost)
	if err != nil {
		return fmt.Errorf("hash new PIN: %w", err)
	}
	return s.repos.UpdateStaffPIN(ctx, staffID, string(newHash))
}

func (s *StaffService) RotatePINScoped(ctx context.Context, staffID, branchID int64, currentPIN, newPIN string) error {
	staff, err := s.repos.GetStaffByID(ctx, staffID)
	if err != nil {
		return err
	}
	if staff.BranchID != branchID {
		return domain.ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(staff.PinHash), []byte(currentPIN)); err != nil {
		return domain.ErrUnauthorized
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPIN), bcryptCost)
	if err != nil {
		return fmt.Errorf("hash new PIN: %w", err)
	}
	return s.repos.UpdateStaffPINScoped(ctx, staffID, branchID, string(newHash))
}

// Deactivate marks a staff member inactive and invalidates all their active Redis tokens.
func (s *StaffService) Deactivate(ctx context.Context, staffID int64) error {
	if err := s.repos.DeactivateStaff(ctx, staffID); err != nil {
		return err
	}
	if err := s.repos.RevokeStaffSessionsForStaff(ctx, staffID); err != nil {
		s.logger.Warn().Err(err).Int64("staff_id", staffID).Msg("failed to revoke durable staff sessions")
	}
	// Invalidate all active tokens for this staff member.
	// Redis is ephemeral so failures are non-fatal, but must be observable.
	setKey := staffTokenSetKey(staffID)
	tokenKeys, err := s.cache.SMembers(ctx, setKey)
	if err != nil {
		s.logger.Warn().Err(err).Int64("staff_id", staffID).Msg("failed to fetch token set for deactivated staff; tokens may persist until TTL")
	} else if len(tokenKeys) > 0 {
		keysToDelete := append(tokenKeys, setKey)
		if err := s.cache.DeleteMany(ctx, keysToDelete...); err != nil {
			s.logger.Warn().Err(err).Int64("staff_id", staffID).Msg("failed to batch-delete tokens for deactivated staff; tokens may persist until TTL")
		}
	}
	return nil
}

func (s *StaffService) DeactivateScoped(ctx context.Context, staffID, branchID int64) error {
	if err := s.repos.DeactivateStaffScoped(ctx, staffID, branchID); err != nil {
		return err
	}
	if err := s.repos.RevokeStaffSessionsForStaff(ctx, staffID); err != nil {
		s.logger.Warn().Err(err).Int64("staff_id", staffID).Msg("failed to revoke durable staff sessions")
	}
	setKey := staffTokenSetKey(staffID)
	tokenKeys, err := s.cache.SMembers(ctx, setKey)
	if err != nil {
		s.logger.Warn().Err(err).Int64("staff_id", staffID).Msg("failed to fetch token set for deactivated staff; tokens may persist until TTL")
	} else if len(tokenKeys) > 0 {
		keysToDelete := append(tokenKeys, setKey)
		if err := s.cache.DeleteMany(ctx, keysToDelete...); err != nil {
			s.logger.Warn().Err(err).Int64("staff_id", staffID).Msg("failed to batch-delete tokens for deactivated staff; tokens may persist until TTL")
		}
	}
	return nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func normalizeCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func defaultStaffCode(name string) string {
	code := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r - ('a' - 'A')
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		default:
			return -1
		}
	}, name)
	if code == "" {
		return "STAFF"
	}
	if len(code) > 24 {
		code = code[:24]
	}
	return code
}

// HashPIN hashes a PIN with bcrypt cost 12. Used by the seed script.
func HashPIN(pin string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

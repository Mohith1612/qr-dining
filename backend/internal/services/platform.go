package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
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

const (
	PlatformRoleSuperAdmin      = "super_admin"
	PlatformRoleSupportAdmin    = "support_admin"
	PlatformRoleBillingAdmin    = "billing_admin"
	PlatformRoleReadOnlyAuditor = "read_only_auditor"
	platformTokenTTL            = 8 * time.Hour
	platformTokenPrefix         = "platform:token:"
	platformBcryptCost          = 12
)

type PlatformService struct {
	repos      *repository.Repos
	cache      *redisPkg.Cache
	logger     zerolog.Logger
	lockout    *redisPkg.LockoutStore
	lockPolicy redisPkg.LockoutPolicy
	mfaEncKey  string
}

func NewPlatformService(repos *repository.Repos, cache *redisPkg.Cache, logger zerolog.Logger) *PlatformService {
	return &PlatformService{
		repos:  repos,
		cache:  cache,
		logger: logger,
		// Platform admin auth is tighter than branch staff PIN: fewer attempts
		// allowed, longer lockout, longer lookback window. Manual operator
		// unlock is the recovery path.
		lockPolicy: redisPkg.LockoutPolicy{
			Window:         5 * time.Minute,
			MaxFailures:    5,
			LockoutTTL:     30 * time.Minute,
			FailOpenOnLoss: false,
		},
	}
}

// SetLockoutStore wires the brute-force protection store.
func (s *PlatformService) SetLockoutStore(store *redisPkg.LockoutStore) {
	s.lockout = store
}

type PlatformSession struct {
	Token          string    `json:"token"`
	SessionID      uuid.UUID `json:"session_id"`
	PlatformUserID int64     `json:"platform_user_id"`
	Email          string    `json:"email"`
	DisplayName    string    `json:"display_name"`
	Roles          []string  `json:"roles"`
	ExpiresAt      time.Time `json:"expires_at"`
}

// PlatformAuthResult bundles the two possible outcomes of a successful
// password check: either a fully-issued session, or an MFA challenge that
// must be completed before a session is issued. Exactly one of the two
// embedded values is populated.
type PlatformAuthResult struct {
	Session   *PlatformSession      `json:"session,omitempty"`
	Challenge *PlatformMFAChallenge `json:"mfa_challenge,omitempty"`
}

func (s *PlatformService) Authenticate(ctx context.Context, email, password, deviceName string) (PlatformSession, error) {
	identity := NormalizePlatformEmail(email)
	if err := s.preAuthCheckLockout(ctx, identity); err != nil {
		return PlatformSession{}, err
	}

	user, err := s.repos.GetPlatformUserByEmail(ctx, identity)
	if err != nil {
		s.recordAuthFailure(ctx, identity)
		return PlatformSession{}, domain.ErrUnauthorized
	}
	if user.Status != "active" {
		s.recordAuthFailure(ctx, identity)
		return PlatformSession{}, domain.ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		s.recordAuthFailure(ctx, identity)
		return PlatformSession{}, domain.ErrUnauthorized
	}
	roles, err := s.repos.ListPlatformRolesForUser(ctx, user.ID)
	if err != nil {
		return PlatformSession{}, err
	}
	if len(roles) == 0 {
		s.recordAuthFailure(ctx, identity)
		return PlatformSession{}, domain.ErrUnauthorized
	}
	s.resetAuthFailure(ctx, identity)
	return s.createSession(ctx, user, roles, deviceName)
}

// AuthenticateWithMFA returns either a full session or an MFA challenge,
// depending on whether the user has active MFA enrolled. Brute-force lockout
// applies to the password check; subsequent MFA verification is gated by the
// challenge expiry.
func (s *PlatformService) AuthenticateWithMFA(ctx context.Context, email, password, deviceName, ip, userAgent string) (PlatformAuthResult, error) {
	session, err := s.Authenticate(ctx, email, password, deviceName)
	if err != nil {
		return PlatformAuthResult{}, err
	}
	active, mfaErr := s.MFAStatus(ctx, session.PlatformUserID)
	if mfaErr != nil {
		return PlatformAuthResult{}, mfaErr
	}
	if !active {
		// User is enrolled in MFA-required policy but has not completed enrollment yet,
		// OR MFA is not required and not enrolled. In either case, issue the session
		// directly — the platform.users.mfa_required flag is enforced at enrollment
		// time, not at every login (otherwise no one could ever do their first login
		// to set up MFA).
		return PlatformAuthResult{Session: &session}, nil
	}
	// MFA is active. Revoke the just-issued session and return a challenge —
	// the session is only released after the user completes the second factor.
	_ = s.Logout(ctx, session, session.Token)
	challenge, err := s.IssueMFAChallenge(ctx, session.PlatformUserID, ip, userAgent)
	if err != nil {
		return PlatformAuthResult{}, err
	}
	return PlatformAuthResult{Challenge: &challenge}, nil
}

// LockoutRetryAfter reports remaining lockout seconds for a platform email.
func (s *PlatformService) LockoutRetryAfter(ctx context.Context, identity string) int {
	if s.lockout == nil {
		return 0
	}
	return s.lockout.RemainingLockoutSeconds(ctx, "platform_auth", NormalizePlatformEmail(identity))
}

func (s *PlatformService) preAuthCheckLockout(ctx context.Context, identity string) error {
	if s.lockout == nil {
		return nil
	}
	if err := s.lockout.CheckLocked(ctx, "platform_auth", identity, s.lockPolicy); err != nil {
		if errors.Is(err, redisPkg.ErrAuthLockedOut) {
			return domain.ErrAuthLockedOut
		}
		s.logger.Warn().Err(err).Str("identity", identity).Msg("platform auth lockout backend unavailable")
		return domain.ErrUnauthorized
	}
	return nil
}

func (s *PlatformService) recordAuthFailure(ctx context.Context, identity string) {
	if s.lockout == nil {
		return
	}
	_, _ = s.lockout.RecordFailure(ctx, "platform_auth", identity, s.lockPolicy)
}

func (s *PlatformService) resetAuthFailure(ctx context.Context, identity string) {
	if s.lockout == nil {
		return
	}
	s.lockout.Reset(ctx, "platform_auth", identity)
}

func (s *PlatformService) createSession(ctx context.Context, user sqlc.PlatformUser, roles []string, deviceName string) (PlatformSession, error) {
	token := uuid.NewString()
	expiresAt := time.Now().UTC().Add(platformTokenTTL)
	dbSession, err := s.repos.CreatePlatformSession(ctx, sqlc.CreatePlatformSessionParams{
		PlatformUserID: user.ID,
		TokenHash:      hashPlatformToken(token),
		DeviceName:     deviceName,
		ExpiresAt:      expiresAt,
	})
	if err != nil {
		return PlatformSession{}, fmt.Errorf("create platform session: %w", err)
	}

	session := PlatformSession{
		Token:          token,
		SessionID:      dbSession.ID,
		PlatformUserID: user.ID,
		Email:          user.Email,
		DisplayName:    user.DisplayName,
		Roles:          roles,
		ExpiresAt:      dbSession.ExpiresAt,
	}
	if err := s.cache.Set(ctx, platformTokenPrefix+token, session, platformTokenTTL); err != nil {
		return PlatformSession{}, fmt.Errorf("store platform token: %w", err)
	}
	return session, nil
}

func (s *PlatformService) ValidateToken(ctx context.Context, token string) (PlatformSession, error) {
	var session PlatformSession
	hit, err := s.cache.Get(ctx, platformTokenPrefix+token, &session)
	if err != nil {
		return PlatformSession{}, err
	}
	if !hit {
		return PlatformSession{}, errors.New("invalid or expired platform token")
	}
	if err := s.validateSessionState(ctx, token, session); err != nil {
		return PlatformSession{}, err
	}
	return session, nil
}

func (s *PlatformService) validateSessionState(ctx context.Context, token string, session PlatformSession) error {
	user, err := s.repos.GetPlatformUserByID(ctx, session.PlatformUserID)
	if err != nil {
		return err
	}
	if user.Status != "active" {
		return errors.New("invalid or expired platform token")
	}
	roles, err := s.repos.ListPlatformRolesForUser(ctx, user.ID)
	if err != nil {
		return err
	}
	if len(roles) == 0 {
		return errors.New("invalid or expired platform token")
	}
	dbSession, err := s.repos.GetActivePlatformSessionByTokenHash(ctx, hashPlatformToken(token))
	if err != nil {
		return err
	}
	if dbSession.ID != session.SessionID || dbSession.PlatformUserID != session.PlatformUserID {
		return errors.New("invalid or expired platform token")
	}
	_ = s.repos.TouchPlatformSession(ctx, dbSession.ID)
	return nil
}

func (s *PlatformService) Logout(ctx context.Context, session PlatformSession, token string) error {
	if err := s.repos.RevokePlatformSession(ctx, session.SessionID, session.PlatformUserID); err != nil {
		return err
	}
	return s.cache.Invalidate(ctx, platformTokenPrefix+token)
}

func PlatformHasRole(session PlatformSession, roles ...string) bool {
	if slices.Contains(session.Roles, PlatformRoleSuperAdmin) {
		return true
	}
	for _, role := range roles {
		if slices.Contains(session.Roles, role) {
			return true
		}
	}
	return false
}

func NormalizePlatformEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func NormalizePlatformCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func HashPlatformPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), platformBcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func hashPlatformToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

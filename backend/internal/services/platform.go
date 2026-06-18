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
	repos  *repository.Repos
	cache  *redisPkg.Cache
	logger zerolog.Logger
}

func NewPlatformService(repos *repository.Repos, cache *redisPkg.Cache, logger zerolog.Logger) *PlatformService {
	return &PlatformService{repos: repos, cache: cache, logger: logger}
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

func (s *PlatformService) Authenticate(ctx context.Context, email, password, deviceName string) (PlatformSession, error) {
	user, err := s.repos.GetPlatformUserByEmail(ctx, NormalizePlatformEmail(email))
	if err != nil {
		return PlatformSession{}, domain.ErrUnauthorized
	}
	if user.Status != "active" {
		return PlatformSession{}, domain.ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return PlatformSession{}, domain.ErrUnauthorized
	}
	roles, err := s.repos.ListPlatformRolesForUser(ctx, user.ID)
	if err != nil {
		return PlatformSession{}, err
	}
	if len(roles) == 0 {
		return PlatformSession{}, domain.ErrUnauthorized
	}
	return s.createSession(ctx, user, roles, deviceName)
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

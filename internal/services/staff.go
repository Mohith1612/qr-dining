package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/repository"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/google/uuid"
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
	repos *repository.Repos
	cache *redisPkg.Cache
}

func NewStaffService(repos *repository.Repos, cache *redisPkg.Cache) *StaffService {
	return &StaffService{repos: repos, cache: cache}
}

type StaffSession struct {
	Token    string        `json:"token"`
	StaffID  int64         `json:"staff_id"`
	BranchID int64         `json:"branch_id"`
	Role     sqlc.StaffRole `json:"role"`
}

// Authenticate verifies a staff PIN against stored bcrypt hashes for the branch.
// Returns a session token stored in Redis on success.
func (s *StaffService) Authenticate(ctx context.Context, branchID int64, pin string) (StaffSession, error) {
	staff, err := s.repos.ListStaffForBranch(ctx, branchID)
	if err != nil {
		return StaffSession{}, err
	}

	var matched *sqlc.Staff
	for i := range staff {
		if err := bcrypt.CompareHashAndPassword([]byte(staff[i].PinHash), []byte(pin)); err == nil {
			matched = &staff[i]
			break
		}
	}
	if matched == nil {
		return StaffSession{}, domain.ErrParticipantUnauthorized
	}

	token := uuid.NewString()
	session := StaffSession{
		Token:    token,
		StaffID:  matched.ID,
		BranchID: matched.BranchID,
		Role:     matched.Role,
	}

	tokenKey := staffTokenPrefix + token
	if err := s.cache.Set(ctx, tokenKey, session, staffTokenTTL); err != nil {
		return StaffSession{}, fmt.Errorf("store staff token: %w", err)
	}
	// Track this token key in a per-staff Set for O(1) batch invalidation on deactivation.
	_ = s.cache.SAdd(ctx, staffTokenSetKey(matched.ID), tokenKey, staffTokenTTL+time.Minute)
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
	return session, nil
}

// CreateStaff creates a new staff member with a bcrypt-hashed PIN.
// Only owners may create staff (enforced in handler via middleware).
func (s *StaffService) CreateStaff(ctx context.Context, branchID int64, role sqlc.StaffRole, name, pin string) (sqlc.Staff, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcryptCost)
	if err != nil {
		return sqlc.Staff{}, fmt.Errorf("hash PIN: %w", err)
	}
	return s.repos.CreateStaff(ctx, sqlc.CreateStaffParams{
		BranchID: branchID,
		Name:     name,
		Role:     role,
		PinHash:  string(hash),
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

// Deactivate marks a staff member inactive and invalidates all their active Redis tokens.
func (s *StaffService) Deactivate(ctx context.Context, staffID int64) error {
	if err := s.repos.DeactivateStaff(ctx, staffID); err != nil {
		return err
	}
	// Invalidate all active tokens for this staff member.
	setKey := staffTokenSetKey(staffID)
	tokenKeys, err := s.cache.SMembers(ctx, setKey)
	if err == nil && len(tokenKeys) > 0 {
		keysToDelete := append(tokenKeys, setKey)
		_ = s.cache.DeleteMany(ctx, keysToDelete...)
	}
	return nil
}

// HashPIN hashes a PIN with bcrypt cost 12. Used by the seed script.
func HashPIN(pin string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

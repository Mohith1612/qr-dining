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

const (
	bcryptCost      = 12
	staffTokenTTL   = 8 * time.Hour
	staffTokenPrefix = "staff:token:"
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

	if err := s.cache.Set(ctx, staffTokenPrefix+token, session, staffTokenTTL); err != nil {
		return StaffSession{}, fmt.Errorf("store staff token: %w", err)
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
	return session, nil
}

// HashPIN hashes a PIN with bcrypt cost 12. Used by the seed script.
func HashPIN(pin string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

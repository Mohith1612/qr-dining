package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Mohith1612/qr-dining/internal/crypto"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	mfaChallengeTTL        = 5 * time.Minute
	mfaRecoveryCodeCount   = 10
	mfaRecoveryCodeBytes   = 10 // 16 base32 chars
	mfaIssuerLabel         = "QR Dining"
	mfaEncryptionKeyMinLen = 16
)

// PlatformMFASetup is the response from BeginEnrollment containing everything
// an authenticator app needs to provision a new code.
type PlatformMFASetup struct {
	Secret     string `json:"secret"`
	OTPAuthURI string `json:"otpauth_uri"`
}

// PlatformMFAEnrollmentResult is returned by ConfirmEnrollment with the
// recovery codes generated for the user. These are shown ONCE and never again
// — only their bcrypt hashes are stored.
type PlatformMFAEnrollmentResult struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

// PlatformMFAChallenge is the ephemeral token returned to the caller during a
// password-success-but-MFA-required login.
type PlatformMFAChallenge struct {
	Challenge string    `json:"challenge"`
	ExpiresAt time.Time `json:"expires_at"`
}

// MFAEncryptionKey returns the configured key; helpers fail closed if missing
// rather than encrypting with a weak default — TOTP secrets must never be
// stored unencrypted.
func (s *PlatformService) mfaKey() (string, error) {
	if len(s.mfaEncKey) < mfaEncryptionKeyMinLen {
		return "", domain.ErrMFANotConfigured
	}
	return s.mfaEncKey, nil
}

// SetMFAEncryptionKey wires the AES-GCM key used for TOTP secret storage.
func (s *PlatformService) SetMFAEncryptionKey(key string) { s.mfaEncKey = key }

// BeginMFAEnrollment generates a new TOTP secret, stores it pending, and
// returns the secret + provisioning URI. The caller MUST confirm by passing a
// valid code back through ConfirmMFAEnrollment within a reasonable window.
func (s *PlatformService) BeginMFAEnrollment(ctx context.Context, userID int64) (PlatformMFASetup, error) {
	key, err := s.mfaKey()
	if err != nil {
		return PlatformMFASetup{}, err
	}
	user, err := s.repos.GetPlatformUserByID(ctx, userID)
	if err != nil {
		return PlatformMFASetup{}, err
	}
	secret, err := crypto.GenerateTOTPSecret()
	if err != nil {
		return PlatformMFASetup{}, err
	}
	encrypted, err := crypto.EncryptSecret(secret, key)
	if err != nil {
		return PlatformMFASetup{}, err
	}
	if err := s.repos.UpsertPendingPlatformMFA(ctx, userID, encrypted); err != nil {
		return PlatformMFASetup{}, err
	}
	return PlatformMFASetup{
		Secret:     secret,
		OTPAuthURI: crypto.BuildOTPAuthURI(mfaIssuerLabel, user.Email, secret),
	}, nil
}

// ConfirmMFAEnrollment verifies the first TOTP code, generates recovery
// codes, persists their bcrypt hashes, and activates MFA on the account.
func (s *PlatformService) ConfirmMFAEnrollment(ctx context.Context, userID int64, code string) (PlatformMFAEnrollmentResult, error) {
	key, err := s.mfaKey()
	if err != nil {
		return PlatformMFAEnrollmentResult{}, err
	}
	mfa, err := s.repos.GetPlatformMFA(ctx, userID)
	if err != nil {
		return PlatformMFAEnrollmentResult{}, err
	}
	if mfa.Status != "pending" {
		return PlatformMFAEnrollmentResult{}, errors.New("mfa enrollment not pending")
	}
	secret, err := crypto.DecryptSecret(mfa.SecretEncrypted, key)
	if err != nil {
		return PlatformMFAEnrollmentResult{}, fmt.Errorf("decrypt totp secret: %w", err)
	}
	if !crypto.VerifyTOTP(secret, code) {
		return PlatformMFAEnrollmentResult{}, domain.ErrMFAInvalidCode
	}
	codes, hashes, err := generateRecoveryCodes()
	if err != nil {
		return PlatformMFAEnrollmentResult{}, err
	}
	if err := s.repos.ActivatePlatformMFA(ctx, userID, hashes); err != nil {
		return PlatformMFAEnrollmentResult{}, err
	}
	return PlatformMFAEnrollmentResult{RecoveryCodes: codes}, nil
}

// DisableMFA removes MFA from the account after verifying a valid code (or
// recovery code). The caller's role and the platform_users.mfa_required
// policy decide whether disabling is permitted at all.
func (s *PlatformService) DisableMFA(ctx context.Context, userID int64, code string) error {
	if !s.verifyMFACode(ctx, userID, code) {
		return domain.ErrMFAInvalidCode
	}
	return s.repos.DisablePlatformMFA(ctx, userID)
}

// IssueMFAChallenge returns an ephemeral challenge token after a successful
// password check, when the user has active MFA enrolled. The caller then
// completes login by POSTing the code + challenge to /platform/auth/mfa.
func (s *PlatformService) IssueMFAChallenge(ctx context.Context, userID int64, ip, userAgent string) (PlatformMFAChallenge, error) {
	challenge := uuid.NewString()
	hash := sha256Hex(challenge)
	expires := time.Now().UTC().Add(mfaChallengeTTL)
	if err := s.repos.CreatePlatformMFAChallenge(ctx, userID, hash, expires, ip, userAgent); err != nil {
		return PlatformMFAChallenge{}, err
	}
	return PlatformMFAChallenge{Challenge: challenge, ExpiresAt: expires}, nil
}

// CompleteMFAChallenge consumes the challenge, verifies the supplied code or
// recovery code, and returns the platform session that would have been issued
// at the end of password authentication.
func (s *PlatformService) CompleteMFAChallenge(ctx context.Context, challenge, code, deviceName string) (PlatformSession, error) {
	hash := sha256Hex(challenge)
	userID, err := s.repos.ResolvePlatformMFAChallenge(ctx, hash)
	if err != nil {
		return PlatformSession{}, domain.ErrUnauthorized
	}
	if !s.verifyMFACode(ctx, userID, code) {
		return PlatformSession{}, domain.ErrMFAInvalidCode
	}
	if err := s.repos.ConsumePlatformMFAChallenge(ctx, hash); err != nil {
		return PlatformSession{}, err
	}
	user, err := s.repos.GetPlatformUserByID(ctx, userID)
	if err != nil {
		return PlatformSession{}, err
	}
	roles, err := s.repos.ListPlatformRolesForUser(ctx, userID)
	if err != nil {
		return PlatformSession{}, err
	}
	return s.createSession(ctx, user, roles, deviceName)
}

// verifyMFACode accepts either a fresh TOTP code or a single-use recovery
// code. Recovery code matches are removed from the pool atomically.
func (s *PlatformService) verifyMFACode(ctx context.Context, userID int64, code string) bool {
	key, err := s.mfaKey()
	if err != nil {
		return false
	}
	mfa, err := s.repos.GetPlatformMFA(ctx, userID)
	if err != nil || mfa.Status != "active" {
		return false
	}
	secret, err := crypto.DecryptSecret(mfa.SecretEncrypted, key)
	if err != nil {
		return false
	}
	cleaned := normalizeMFACode(code)
	if crypto.VerifyTOTP(secret, cleaned) {
		_ = s.repos.TouchPlatformMFAUse(ctx, userID)
		return true
	}
	// Recovery code path: walk hashes, bcrypt-compare, drop matched code.
	for i, h := range mfa.RecoveryCodes {
		if bcrypt.CompareHashAndPassword([]byte(h), []byte(cleaned)) == nil {
			remaining := append([]string{}, mfa.RecoveryCodes[:i]...)
			remaining = append(remaining, mfa.RecoveryCodes[i+1:]...)
			_ = s.repos.ConsumeRecoveryCodes(ctx, userID, remaining)
			return true
		}
	}
	return false
}

func generateRecoveryCodes() ([]string, []string, error) {
	plain := make([]string, 0, mfaRecoveryCodeCount)
	hashes := make([]string, 0, mfaRecoveryCodeCount)
	for i := 0; i < mfaRecoveryCodeCount; i++ {
		b := make([]byte, mfaRecoveryCodeBytes)
		if _, err := rand.Read(b); err != nil {
			return nil, nil, err
		}
		// Base32 (Crockford-style, no padding) is easy to type and unambiguous.
		code := strings.ToUpper(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b))
		// Hyphenate every 4 chars for readability.
		code = insertEvery(code, 4, "-")
		plain = append(plain, code)
		hash, err := bcrypt.GenerateFromPassword([]byte(code), 12)
		if err != nil {
			return nil, nil, err
		}
		hashes = append(hashes, string(hash))
	}
	return plain, hashes, nil
}

func normalizeMFACode(code string) string {
	return strings.ToUpper(strings.TrimSpace(strings.ReplaceAll(code, " ", "")))
}

func insertEvery(s string, n int, sep string) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	var b strings.Builder
	for i, r := range s {
		if i > 0 && i%n == 0 {
			b.WriteString(sep)
		}
		b.WriteRune(r)
	}
	return b.String()
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// MFAStatus reports whether MFA is active for a platform user. Used by the
// login handler to decide whether to short-circuit into the MFA challenge.
func (s *PlatformService) MFAStatus(ctx context.Context, userID int64) (active bool, _ error) {
	mfa, err := s.repos.GetPlatformMFA(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrMFANotEnrolled) {
			return false, nil
		}
		return false, err
	}
	return mfa.Status == "active", nil
}

// IsMFARequired reports whether MFA is mandatory for the user. Combines the
// per-user mfa_required column and a future global policy hook.
func (s *PlatformService) IsMFARequired(user struct {
	MfaRequired bool
}) bool {
	return user.MfaRequired
}

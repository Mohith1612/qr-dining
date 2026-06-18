package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const GuestAudience = "qr-dining-guest"

var (
	ErrGuestTokenMalformed = errors.New("guest token malformed")
	ErrGuestTokenInvalid   = errors.New("guest token invalid")
	ErrGuestTokenExpired   = errors.New("guest token expired")
)

type GuestClaims struct {
	Subject           string    `json:"sub"`
	SessionID         uuid.UUID `json:"sid"`
	BranchID          int64     `json:"bid"`
	TableID           int64     `json:"tid"`
	OrganizationID    int64     `json:"org"`
	Role              string    `json:"role"`
	ParticipantID     int64     `json:"participant_id"`
	CredentialVersion int32     `json:"credential_version"`
	IssuedAtUnix      int64     `json:"iat"`
	ExpiresAtUnix     int64     `json:"exp"`
	JTI               string    `json:"jti"`
	Audience          string    `json:"aud"`
}

type GuestTokenService struct {
	secret []byte
	ttl    time.Duration
}

func NewGuestTokenService(secret string, ttl time.Duration) *GuestTokenService {
	return &GuestTokenService{secret: []byte(secret), ttl: ttl}
}

func (s *GuestTokenService) Issue(claims GuestClaims) (string, error) {
	now := time.Now().UTC()
	if claims.Subject == "" {
		claims.Subject = fmt.Sprintf("participant:%d", claims.ParticipantID)
	}
	if claims.Audience == "" {
		claims.Audience = GuestAudience
	}
	if claims.IssuedAtUnix == 0 {
		claims.IssuedAtUnix = now.Unix()
	}
	if claims.ExpiresAtUnix == 0 {
		claims.ExpiresAtUnix = now.Add(s.ttl).Unix()
	}
	if claims.JTI == "" {
		jti, err := randomJTI()
		if err != nil {
			return "", err
		}
		claims.JTI = jti
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal guest claims: %w", err)
	}

	payloadPart := base64.RawURLEncoding.EncodeToString(payload)
	signaturePart := base64.RawURLEncoding.EncodeToString(s.sign(payloadPart))
	return payloadPart + "." + signaturePart, nil
}

func (s *GuestTokenService) Validate(token string) (GuestClaims, error) {
	payloadPart, signaturePart, ok := strings.Cut(token, ".")
	if !ok || payloadPart == "" || signaturePart == "" {
		return GuestClaims{}, ErrGuestTokenMalformed
	}

	gotSig, err := base64.RawURLEncoding.DecodeString(signaturePart)
	if err != nil {
		return GuestClaims{}, ErrGuestTokenMalformed
	}
	if !hmac.Equal(gotSig, s.sign(payloadPart)) {
		return GuestClaims{}, ErrGuestTokenInvalid
	}

	payload, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil {
		return GuestClaims{}, ErrGuestTokenMalformed
	}

	var claims GuestClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return GuestClaims{}, ErrGuestTokenMalformed
	}
	if claims.Audience != GuestAudience {
		return GuestClaims{}, ErrGuestTokenInvalid
	}
	if claims.ExpiresAtUnix <= time.Now().UTC().Unix() {
		return GuestClaims{}, ErrGuestTokenExpired
	}
	return claims, nil
}

func (s *GuestTokenService) sign(payloadPart string) []byte {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payloadPart))
	return mac.Sum(nil)
}

func randomJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate guest token id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

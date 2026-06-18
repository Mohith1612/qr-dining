package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGuestTokenRoundTrip(t *testing.T) {
	svc := NewGuestTokenService("secret", time.Hour)
	sessionID := uuid.New()

	token, err := svc.Issue(GuestClaims{
		SessionID:         sessionID,
		BranchID:          10,
		TableID:           20,
		OrganizationID:    30,
		Role:              "host",
		ParticipantID:     40,
		CredentialVersion: 1,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	claims, err := svc.Validate(token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if claims.SessionID != sessionID || claims.ParticipantID != 40 || claims.Audience != GuestAudience {
		t.Fatalf("claims mismatch: %+v", claims)
	}
}

func TestGuestTokenRejectsExpired(t *testing.T) {
	svc := NewGuestTokenService("secret", -time.Second)

	token, err := svc.Issue(GuestClaims{
		SessionID:         uuid.New(),
		BranchID:          10,
		TableID:           20,
		OrganizationID:    30,
		Role:              "guest",
		ParticipantID:     40,
		CredentialVersion: 1,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	_, err = svc.Validate(token)
	if !errors.Is(err, ErrGuestTokenExpired) {
		t.Fatalf("Validate error = %v, want %v", err, ErrGuestTokenExpired)
	}
}

func TestGuestTokenRejectsTampering(t *testing.T) {
	svc := NewGuestTokenService("secret", time.Hour)

	token, err := svc.Issue(GuestClaims{
		SessionID:         uuid.New(),
		BranchID:          10,
		TableID:           20,
		OrganizationID:    30,
		Role:              "guest",
		ParticipantID:     40,
		CredentialVersion: 1,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	token = "A" + token[1:]
	_, err = svc.Validate(token)
	if !errors.Is(err, ErrGuestTokenInvalid) && !errors.Is(err, ErrGuestTokenMalformed) {
		t.Fatalf("Validate error = %v, want invalid or malformed", err)
	}
}

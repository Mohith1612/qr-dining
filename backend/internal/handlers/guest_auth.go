package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Mohith1612/qr-dining/internal/auth"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// guestSafeSession returns a copy of the session with the guest credential
// (session_token) cleared. Guests authenticate with their HMAC guest_access_token
// and never need session_token, so it must never be serialized to a guest. This is
// permanent and independent of the AUTH_GUEST_CREDENTIALS_REQUIRED (R6) rollout flag.
// sqlc.Session is a value type, so the cleared copy never touches the DB row.
func guestSafeSession(s sqlc.Session) sqlc.Session {
	s.SessionToken = ""
	return s
}

func guestTokenFromHeader(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	token, ok := strings.CutPrefix(authHeader, "Bearer ")
	if !ok {
		return ""
	}
	return token
}

func guestParticipantID(
	c *gin.Context,
	tokens *auth.GuestTokenService,
	repos *repository.Repos,
	sessionID uuid.UUID,
	legacyParticipantID int64,
	required bool,
) (int64, bool) {
	token := guestTokenFromHeader(c)
	if token == "" {
		if legacyParticipantID == 0 {
			if headerParticipantID, ok := participantIDFromHeader(c); ok {
				legacyParticipantID = headerParticipantID
			}
		}
		if required {
			respondError(c, http.StatusUnauthorized, CodeUnauthorized, "guest credential required")
			return 0, false
		}
		// The legacy header is not authentication, but while the rollback path
		// exists it must never allow a participant from another session to cross
		// the trust boundary. Secure production config requires a signed token.
		if legacyParticipantID != 0 {
			participant, err := repos.GetSessionParticipantByID(c.Request.Context(), legacyParticipantID)
			if err != nil || participant.SessionID != sessionID {
				recordGuestTokenFailure("legacy_participant_mismatch")
				respondError(c, http.StatusForbidden, CodeForbidden, "participant does not belong to this session")
				return 0, false
			}
		}
		return legacyParticipantID, true
	}

	claims, err := tokens.Validate(token)
	if err != nil {
		// A bearer token was presented but is not a valid guest credential.
		// Reject it — never fail open. Failing open here let a staff/foreign JWT
		// (malformed-as-guest) pass through to participant 0, leaking cross-org
		// snapshots (T-01) and 500ing guest routes (X-03). The legacy/anonymous
		// path is only the token == "" branch above.
		recordGuestTokenFailure(guestTokenReasonSlug(err))
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "invalid guest credential")
		return 0, false
	}
	if claims.SessionID != sessionID {
		recordGuestTokenFailure("session_mismatch")
		respondError(c, http.StatusForbidden, CodeForbidden, "guest credential does not belong to this session")
		return 0, false
	}
	if legacyParticipantID != 0 && claims.ParticipantID != legacyParticipantID {
		recordGuestTokenFailure("participant_mismatch")
		respondError(c, http.StatusForbidden, CodeForbidden, "guest credential does not match participant")
		return 0, false
	}

	participant, err := repos.GetSessionParticipantByID(c.Request.Context(), claims.ParticipantID)
	if err != nil || participant.SessionID != sessionID {
		// The token names a participant that isn't a member of this session.
		recordGuestTokenFailure("stale_credential")
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "guest credential is no longer valid")
		return 0, false
	}
	if participant.CredentialVersion != claims.CredentialVersion {
		// The credential was rotated out from under this token. Closing a
		// session rotates every participant's version, so this — not a 410 — is
		// what a returning guest's stored token hits after the table is closed.
		// Flagged terminal so the client shows the ended screen instead of
		// looping on reconnect (L-09 / F-22). The code stays UNAUTHORIZED: it is
		// part of the published contract.
		recordGuestTokenFailure("stale_credential")
		respondErrorWithReason(c, http.StatusUnauthorized, CodeUnauthorized, ReasonCredentialRevoked,
			"guest credential is no longer valid")
		return 0, false
	}
	if participant.RevokedAt.Valid {
		recordGuestTokenFailure("revoked")
		respondErrorWithReason(c, http.StatusUnauthorized, CodeUnauthorized, ReasonCredentialRevoked,
			"guest credential has been revoked")
		return 0, false
	}

	return claims.ParticipantID, true
}

// guestTokenReasonSlug maps a guest token validation error to a bounded label.
// ErrGuestTokenInvalid collapses signature + audience mismatch, so they share "invalid".
func guestTokenReasonSlug(err error) string {
	switch {
	case errors.Is(err, auth.ErrGuestTokenMalformed):
		return "malformed"
	case errors.Is(err, auth.ErrGuestTokenExpired):
		return "expired"
	case errors.Is(err, auth.ErrGuestTokenInvalid):
		return "invalid"
	default:
		return "other"
	}
}

func recordGuestTokenFailure(reason string) {
	if handlerMetrics != nil && handlerMetrics.GuestTokenValidationFailedTotal != nil {
		handlerMetrics.GuestTokenValidationFailedTotal.WithLabelValues(reason).Inc()
	}
}

func requireGuestSession(
	c *gin.Context,
	tokens *auth.GuestTokenService,
	repos *repository.Repos,
	sessionID uuid.UUID,
	required bool,
) bool {
	_, ok := guestParticipantID(c, tokens, repos, sessionID, 0, required)
	return ok
}

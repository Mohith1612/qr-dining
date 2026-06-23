package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Mohith1612/qr-dining/internal/auth"
	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

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
		return legacyParticipantID, true
	}

	claims, err := tokens.Validate(token)
	if err != nil {
		if required || !errors.Is(err, auth.ErrGuestTokenMalformed) {
			respondError(c, http.StatusUnauthorized, CodeUnauthorized, "invalid guest credential")
			return 0, false
		}
		return legacyParticipantID, true
	}
	if claims.SessionID != sessionID {
		respondError(c, http.StatusForbidden, CodeForbidden, "guest credential does not belong to this session")
		return 0, false
	}
	if legacyParticipantID != 0 && claims.ParticipantID != legacyParticipantID {
		respondError(c, http.StatusForbidden, CodeForbidden, "guest credential does not match participant")
		return 0, false
	}

	participant, err := repos.GetSessionParticipantByID(c.Request.Context(), claims.ParticipantID)
	if err != nil || participant.SessionID != sessionID || participant.CredentialVersion != claims.CredentialVersion {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "guest credential is no longer valid")
		return 0, false
	}
	if participant.RevokedAt.Valid {
		respondError(c, http.StatusUnauthorized, CodeUnauthorized, "guest credential has been revoked")
		return 0, false
	}

	return claims.ParticipantID, true
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

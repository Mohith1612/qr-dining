package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/auth"
	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestIssueWSTicketRejectsSessionMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokenSvc := auth.NewGuestTokenService("test-secret", time.Hour)
	token, err := tokenSvc.Issue(auth.GuestClaims{
		SessionID:         uuid.New(),
		BranchID:          1,
		TableID:           1,
		OrganizationID:    1,
		ParticipantID:     1,
		CredentialVersion: 1,
	})
	if err != nil {
		t.Fatalf("Issue guest token: %v", err)
	}

	handler := NewSessionHandler(nil, nil, nil, tokenSvc, nil, config.FeatureFlags{}, nil)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Params = gin.Params{{Key: "id", Value: uuid.NewString()}}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/sessions/id/ws-ticket", nil)
	ctx.Request.Header.Set("Authorization", "Bearer "+token)

	handler.IssueWSTicket(ctx)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusForbidden)
	}
}

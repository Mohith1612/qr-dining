package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/gin-gonic/gin"
)

func TestWSUpgradeRequiresTicketWhenFlagEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewWSHandler(nil, nil, nil, nil, nil, config.FeatureFlags{WSTicketAuthRequired: true})

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/ws?session_id=00000000-0000-0000-0000-000000000000&participant_id=1", nil)

	handler.Upgrade(ctx)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

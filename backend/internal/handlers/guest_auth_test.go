package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// A present-but-invalid bearer token (e.g. a staff JWT, which is malformed as a
// guest token) must be REJECTED with 401 — never failed open to participant 0.
// Failing open here was the root cause of T-01 (cross-org snapshot read) and
// X-03 (500 on guest routes). The reject path returns before any repo access, so
// repos can be nil.
func TestGuestParticipantID_PresentInvalidTokenRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens := auth.NewGuestTokenService("test-secret", time.Hour)

	// Looks like a JWT (3 dot-separated parts) — a staff token shape. Not a valid
	// guest credential. Tested with required=false to prove non-strict mode still rejects.
	for _, required := range []bool{false, true} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/sessions/x/snapshot", nil)
		c.Request.Header.Set("Authorization", "Bearer aaaa.bbbb.cccc")

		pid, ok := guestParticipantID(c, tokens, nil, uuid.New(), 0, required)
		if ok {
			t.Fatalf("required=%v: expected rejection, got ok=true pid=%d", required, pid)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("required=%v: status got %d, want 401", required, rec.Code)
		}
	}
}

// When no bearer token is presented in non-strict mode, the legacy/anonymous path
// is preserved (no 401) — the fix must not break legacy reconnects.
func TestGuestParticipantID_NoTokenNonStrictPreserved(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens := auth.NewGuestTokenService("test-secret", time.Hour)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/sessions/x/snapshot", nil)

	_, ok := guestParticipantID(c, tokens, nil, uuid.New(), 0, false)
	if !ok {
		t.Fatalf("no-token non-strict should pass through, got ok=false (status %d)", rec.Code)
	}
}

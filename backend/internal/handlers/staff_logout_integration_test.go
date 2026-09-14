//go:build integration

package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/Mohith1612/qr-dining/internal/audit"
	"github.com/Mohith1612/qr-dining/internal/authz"
	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/middleware"
	"github.com/Mohith1612/qr-dining/internal/observability"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/services"
	"github.com/Mohith1612/qr-dining/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// POST /staff/logout cleared a cookie and returned 204 while leaving every
// piece of session state intact: the Redis token (8h TTL), its entry in the
// per-staff token set, and the durable staff_sessions row. A bearer client
// that ignored the response kept full access for the rest of the shift, which
// on a shared restaurant tablet means "log out" did nothing.
//
// These run the real middleware and the real handler over a real engine, so a
// pass means the wire actually rejects the token — not merely that a service
// method was reachable.

type staffLogoutFixture struct {
	engine    *gin.Engine
	pool      *pgxpool.Pool
	staffSvc  *services.StaffService
	redis     *goredis.Client
	staffID   int64
	tabletTok string
	termTok   string
	tabletSID uuid.UUID
	termSID   uuid.UUID
}

func newStaffLogoutFixture(t *testing.T) staffLogoutFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)

	pool := testutil.OpenTestDB(t)
	f := testutil.SeedFixtures(t, pool)
	repos := testutil.NewTestRepos(pool)
	ctx := context.Background()

	t.Cleanup(func() {
		testutil.TruncateTables(t, pool, "staff_sessions", "staff", "audit_log")
	})

	redisClient := openStaffLogoutRedis(t)
	cache := redisPkg.NewCache(redisClient, nil, nil)

	staffSvc := services.NewStaffService(repos, cache, zerolog.Nop())
	// Strict DB-backed validation: AUTH_STAFF_SESSION_DB_REQUIRED defaults true
	// in config.go, so this is the shipped posture. Revoking only Redis would
	// pass a permissive-mode test and still be wrong here.
	staffSvc.SetRequireSessionDBRow(true)

	// SeedFixtures writes a lower-case branch_code, but GetBranchByCode upper-cases
	// whatever it is given, so the seeded value cannot be resolved by code. Give
	// this branch a code the login path can actually find.
	branchCode := "LOGOUT" + strings.ToUpper(uuid.NewString()[:6])
	if _, err := pool.Exec(ctx, `UPDATE branches SET branch_code = $1 WHERE id = $2`, branchCode, f.BranchID); err != nil {
		t.Fatalf("set branch code: %v", err)
	}

	const pin = "4821"
	staffCode := "WAIT" + strings.ToUpper(uuid.NewString()[:4])
	if _, err := staffSvc.CreateStaff(ctx, f.BranchID, sqlc.StaffRoleWaiter, "Priya", staffCode, pin); err != nil {
		t.Fatalf("CreateStaff: %v", err)
	}

	// Two concurrent shifts for one staff member: the floor tablet and the
	// front-of-house terminal.
	tablet, err := staffSvc.AuthenticateWithCode(ctx, branchCode, staffCode, pin, "floor-tablet")
	if err != nil {
		t.Fatalf("authenticate tablet: %v", err)
	}
	terminal, err := staffSvc.AuthenticateWithCode(ctx, branchCode, staffCode, pin, "front-terminal")
	if err != nil {
		t.Fatalf("authenticate terminal: %v", err)
	}

	staffH := NewStaffHandler(
		staffSvc, repos, observability.NewMetrics(), config.FeatureFlags{},
		authz.NewEnforcingAuthorizer(false),
		audit.NewWriter(sqlc.New(pool), true, zerolog.Nop(), nil),
	)

	engine := gin.New()
	auth := middleware.StaffAuth(staffSvc, zerolog.Nop())
	engine.POST("/staff/logout", auth, staffH.Logout)
	// Stands in for every staff route the token still opens.
	engine.GET("/staff/probe", auth, func(c *gin.Context) { c.Status(http.StatusOK) })

	return staffLogoutFixture{
		engine:    engine,
		pool:      pool,
		staffSvc:  staffSvc,
		redis:     redisClient,
		staffID:   tablet.StaffID,
		tabletTok: tablet.Token,
		termTok:   terminal.Token,
		tabletSID: tablet.SessionID,
		termSID:   terminal.SessionID,
	}
}

func openStaffLogoutRedis(t *testing.T) *goredis.Client {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL not set")
	}
	opts, err := goredis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse TEST_REDIS_URL: %v", err)
	}
	client := goredis.NewClient(opts)
	if err := client.Ping(context.Background()).Err(); err != nil {
		_ = client.Close()
		t.Fatalf("ping test Redis: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func (f staffLogoutFixture) do(t *testing.T, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	f.engine.ServeHTTP(rec, req)
	return rec
}

func (f staffLogoutFixture) revokedAt(t *testing.T, sessionID uuid.UUID) (bool, bool) {
	t.Helper()
	var revoked *string
	var found bool
	err := f.pool.QueryRow(context.Background(),
		`SELECT revoked_at::text, TRUE FROM staff_sessions WHERE id = $1`, sessionID,
	).Scan(&revoked, &found)
	if err != nil {
		t.Fatalf("read staff_sessions row %s: %v", sessionID, err)
	}
	return revoked != nil, found
}

func TestStaffLogout_RevokesTheTokenItWasCalledWith(t *testing.T) {
	f := newStaffLogoutFixture(t)

	if rec := f.do(t, http.MethodGet, "/staff/probe", f.tabletTok); rec.Code != http.StatusOK {
		t.Fatalf("probe before logout: got %d, want 200", rec.Code)
	}

	if rec := f.do(t, http.MethodPost, "/staff/logout", f.tabletTok); rec.Code != http.StatusNoContent {
		t.Fatalf("logout: got %d, want 204", rec.Code)
	}

	// The claim under test: the token stops working on the wire.
	if rec := f.do(t, http.MethodGet, "/staff/probe", f.tabletTok); rec.Code != http.StatusUnauthorized {
		t.Errorf("token still accepted after logout: probe returned %d, want 401", rec.Code)
	}

	// Durable revocation, so the token stays dead across a Redis flush and the
	// operator-facing session list reflects reality.
	if revoked, _ := f.revokedAt(t, f.tabletSID); !revoked {
		t.Errorf("staff_sessions.revoked_at is still NULL after logout")
	}

	// Redis state: both the token key and its entry in the per-staff set.
	ctx := context.Background()
	tokenKey := "staff:token:" + f.tabletTok
	if n, err := f.redis.Exists(ctx, tokenKey).Result(); err != nil {
		t.Fatalf("redis exists: %v", err)
	} else if n != 0 {
		t.Errorf("redis key %s survived logout", tokenKey)
	}
	members, err := f.redis.SMembers(ctx, "staff:tokens:"+strconv.FormatInt(f.staffID, 10)).Result()
	if err != nil {
		t.Fatalf("redis smembers: %v", err)
	}
	for _, m := range members {
		if m == tokenKey {
			t.Errorf("per-staff token set still tracks the logged-out token")
		}
	}
}

func TestStaffLogout_LeavesTheSameStaffMembersOtherSessionAlive(t *testing.T) {
	f := newStaffLogoutFixture(t)

	if rec := f.do(t, http.MethodPost, "/staff/logout", f.tabletTok); rec.Code != http.StatusNoContent {
		t.Fatalf("logout tablet: got %d, want 204", rec.Code)
	}

	// Logging out on the tablet must not log the same person out on the
	// terminal — the reason this needed a new single-session revoke rather than
	// the existing RevokeStaffSessionsForStaff.
	if rec := f.do(t, http.MethodGet, "/staff/probe", f.termTok); rec.Code != http.StatusOK {
		t.Errorf("terminal session killed by tablet logout: probe returned %d, want 200", rec.Code)
	}
	if revoked, _ := f.revokedAt(t, f.termSID); revoked {
		t.Errorf("terminal staff_sessions row was revoked by the tablet's logout")
	}
	if _, err := f.staffSvc.ValidateToken(context.Background(), f.termTok); err != nil {
		t.Errorf("terminal token no longer validates: %v", err)
	}
}

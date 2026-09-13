# Security hardening checklist

Last verified against code: 2026-09-13. This is a review checklist, not a claim of
certification. Mark an item complete only with the cited implementation and a
relevant test.

- [x] Guest tokens are signed, audience-bound, expiring, and scoped to participant,
  session, branch, table, organization, and credential version
  (`backend/internal/auth/guest.go:18-39,46-111`).
- [x] Guest requests recheck participant membership, version, and revocation
  (`backend/internal/handlers/guest_auth.go:67-115`).
- [x] WebSocket tickets are single-use and recheck all tenant/session/participant
  scope before upgrade (`backend/internal/handlers/ws.go:109-149`).
- [x] Staff auth uses bcrypt PINs, fail-closed lockout, durable hashed sessions,
  Redis caching, and token/PIN version checks
  (`backend/internal/services/staff.go:26-55,115-176,179-280`).
- [ ] Staff logout revokes the server session. It currently clears only the cookie
  (`backend/internal/handlers/staff.go:124-130`).
- [x] Platform auth uses bcrypt, fail-closed lockout, role checks, durable hashed
  sessions, Redis caching, logout revocation, and an MFA challenge when active
  (`backend/internal/services/platform.go:22-30,41-55,82-142,180-265`).
- [x] Tenant scope violations are enforced regardless of the central-role flag
  (`backend/internal/handlers/authz.go:48-103`).
- [~] Central role-policy enforcement is environment-dependent: code default false,
  manual-testing true (`backend/internal/config/config.go:253-263`,
  `scripts/manual-testing-up.sh:62-75`). Verify the deployed environment; do not
  mark this globally on or off.
- [x] Raw `session_token` is removed from guest create/snapshot, event/log, staff
  active-session, and force-close serialization paths
  (`backend/internal/handlers/session.go:59-81`,
  `backend/internal/handlers/snapshot.go:50-65`,
  `backend/internal/services/session.go:68-96,204-230`,
  `backend/internal/services/session_close.go:17-31,73-80`).
- [x] Release mode validates CORS and credential configuration before serving
  (`backend/internal/config/config.go:291-335`).
- [x] Nginx restricts `/metrics` to internal address ranges
  (`deploy/nginx/qr-dining.conf:65-72`).
- [ ] Alert delivery is configured. Checked-in Alertmanager still targets a
  placeholder URL (`deploy/observability/alertmanager.yml:17-38`).
- [ ] `db_errors_total` has a verified increment site. It is declared and
  registered, but this rebuild found no increment in backend source
  (`backend/internal/observability/metrics.go:169-174,338-360`).

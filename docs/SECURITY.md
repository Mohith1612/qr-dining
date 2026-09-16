# Security model

Last verified against code: 2026-09-13.

The application has three separate authenticated identities: a guest participant,
a restaurant staff member, and a platform operator. They have different token
formats, stores, middleware, and scopes; a token from one class is not a token for
another (`backend/internal/handlers/guest_auth.go:25-32`,
`backend/internal/middleware/staff_auth.go:18-52`,
`backend/internal/middleware/platform_auth.go:15-47`).

## Guest participant

Create and join return an HMAC-SHA-256 guest access token whose claims bind the
participant to session, branch, table, organization, role, credential version,
audience, expiry, and a random token ID
(`backend/internal/auth/guest.go:18-39,46-80,114-125`). Validation checks encoding,
signature, audience, and expiry (`backend/internal/auth/guest.go:82-111`). Request
authorization additionally checks session binding, participant binding, the
current database credential version, and revocation
(`backend/internal/handlers/guest_auth.go:67-115`).

When `AUTH_GUEST_CREDENTIALS_REQUIRED=false`, the legacy participant identifier
path remains available, but it checks that the participant belongs to the named
session. When the flag is true, absence of the signed credential is rejected
(`backend/internal/handlers/guest_auth.go:34-65`). The code default is true
(`backend/internal/config/config.go:253-261`).

Closing a session revokes its participants and increments their credential
versions, invalidating earlier guest tokens without a token denylist
(`backend/internal/repository/session.go:103-118`). A WebSocket ticket is consumed
once from Redis and its tenant, session, participant, credential-version, and
revocation claims are rechecked before upgrade
(`backend/internal/handlers/ws.go:109-149`).

## Restaurant staff

Staff authentication supports the branch-code/staff-code/PIN flow. It resolves
the branch and staff record, compares the stored bcrypt PIN hash, and applies a
Redis-backed failure/lockout policy that fails closed when the lockout store is
unavailable (`backend/internal/services/staff.go:26-55,115-176`). Successful auth
requires active branch and organization, creates an eight-hour hashed-token row
in PostgreSQL, and caches the session in Redis
(`backend/internal/services/staff.go:179-230`).

The middleware accepts the same opaque token in an Authorization bearer header
or the HttpOnly `qrd_staff_session` cookie
(`backend/internal/middleware/staff_auth.go:12-22,65-77`). Each use validates the
staff active state and token/PIN versions; strict mode additionally requires the
matching active database session row
(`backend/internal/services/staff.go:233-280`). The code default for strict DB
session validation is true (`backend/internal/config/config.go:255-258`). PIN
reset and staff deactivation revoke durable sessions and best-effort clear all
cached tokens (`backend/internal/services/staff.go:336-395`).

Known gap: `POST /staff/logout` only expires the response cookie and does not
revoke the durable or cached session, so the same bearer token remains usable
until another revocation path or expiry (`backend/internal/handlers/staff.go:124-130`).

## Platform operator

Platform users use email/password, four platform roles, bcrypt cost 12, and
eight-hour opaque tokens (`backend/internal/services/platform.go:22-30,82-110`).
The lockout permits five failures per five minutes and locks for 30 minutes; a
lockout-backend failure does not fail open
(`backend/internal/services/platform.go:41-55,152-162`). If active MFA exists, a
password-authenticated session is immediately revoked and replaced with an MFA
challenge; only a completed second factor yields a usable session
(`backend/internal/services/platform.go:113-142`).

Platform sessions are stored hashed in PostgreSQL and cached in Redis. Validation
rechecks the active user, current roles, and database session; logout revokes both
stores (`backend/internal/services/platform.go:180-253`). Platform middleware
accepts only bearer transport (`backend/internal/middleware/platform_auth.go:15-46`).
`super_admin` satisfies every platform role check
(`backend/internal/services/platform.go:256-265`).

## Authorization and environment-dependent enforcement

The central staff policy checks tenant scope before role. Same-branch actions,
same-organization actions, and role permissions are enumerated in one policy
(`backend/internal/authz/policy.go:48-79,82-159`). Scope violations are always
blocked. Role denials are blocked only when `AUTHZ_CENTRAL_POLICY_ENFORCE=true`;
otherwise they are audited and metered but the central decision allows the
request to continue (`backend/internal/handlers/authz.go:48-103`). Individual
handlers and services may still enforce roles independently.

There is no universal value for this flag:

- the application default is `false`
  (`backend/internal/config/config.go:253-263`);
- the manual-testing stack explicitly starts both backends with `true`
  (`scripts/manual-testing-up.sh:62-75`); and
- deployment behavior is whatever its environment sets. The production example
  currently shows `false` (`deploy/vm/.env.production.example:46-58`).

Do not describe the policy as globally “enabled” or “disabled” without naming
the environment.

## Safe projections and credential stripping

`sessions.session_token` is a stored guest credential and must not be serialized.
The handler projection clears it from guest responses
(`backend/internal/handlers/guest_auth.go:15-22`). Session creation returns that
safe session beside the separate guest access token
(`backend/internal/handlers/session.go:59-81`). The service uses a second safe
projection before `SESSION_CREATED` is published or persisted
(`backend/internal/services/session.go:68-96`). Snapshot responses clear it at
the last serialization boundary (`backend/internal/handlers/snapshot.go:50-65`).

Staff active-session results and force-close results also pass through safe
projections; integration tests assert that neither body contains the stored token
(`backend/internal/handlers/session_credential_leak_integration_test.go:40-89,91-132`,
`backend/internal/services/session_close.go:17-31,73-80`).

The pattern is deliberate: generated database structs contain storage-only
fields, so a response, event, or log must map to an explicit safe projection at
the boundary. Do not serialize a raw `sqlc.Session` from a new surface; the
existing regression test explains why raw embedding leaked credentials
(`backend/internal/handlers/session_credential_leak_integration_test.go:16-28`).

## Secrets and exposure boundaries

Release-mode configuration requires guest and webhook secrets and validates
minimum secret length (`backend/internal/config/config.go:286-319`). The metrics
endpoint is public in the application router, while the production nginx config
permits it only from internal Docker networks
(`backend/internal/server/server.go:187-197`,
`deploy/nginx/qr-dining.conf:65-72`). WebSocket origins are allowlisted when
configured; an empty allowlist permits all origins and logs a warning
(`backend/internal/websocket/hub.go:53-83`).

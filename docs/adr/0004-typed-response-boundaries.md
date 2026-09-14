# ADR 0004 — Type high-risk response boundaries incrementally

**Status:** Proposed · **Date:** 2026-09-14 · **Recommendation:** keep the existing typed error envelope, replace `gin.H` on public success responses when those endpoints are next changed, and do not fund a blanket response/event-payload conversion

> **Current verification, 2026-09-14.** A `go/types` inventory of every
> `(*gin.Context).JSON` call found 157 response sites: 97 have the static type
> `gin.H` and 60 pass a declared struct, pointer, or slice. The existing credential
> guard already performs the same kind of call-site type analysis
> (`backend/internal/handlers/credential_guard_typewalk_test.go:1-31`).

## Context

`gin.H` is `map[string]any`. It is convenient, but the compiler cannot prove which
JSON keys or value types a handler emits. That is the same unchecked seam that let
response casing and embedded credential fields drift from the API contract.

The contract did not become stale during this audit. The audit exposed defects that
were already present:

- three of the four reachable `POST /sessions/{id}/orders` conflict codes were not
  documented before `IDEMPOTENCY_IN_PROGRESS` was added
  (`backend/internal/handlers/order.go:104-113`);
- the logout summary claimed that the current staff token was invalidated while the
  baseline handler only cleared its cookie (`backend/internal/handlers/staff.go:124-130`);
- the order request schema required `branch_id` and `placed_by_participant_id` even
  though the handler rejects clients that supply either field
  (`backend/internal/handlers/order.go:68-71`).

A gate limited to drift introduced by the current diff would have accepted every one
of those defects. The value of whole-surface contract checks is therefore not merely
keeping pace with the rate of change; it is making an incorrect contract discoverable
at all. This argues for exhaustive automated checks where they are tractable. It does
not, by itself, make a blanket conversion of every response map cost-effective.

The often-quoted count of 82 `gin.H` responses is syntactic: those calls contain a
literal `gin.H{...}` argument. Fifteen more pass a variable whose static type is
`gin.H`, making the semantic count 97:

| Response expression | Sites | Share |
|---|---:|---:|
| `gin.H`, including variables | 97 | 62% |
| Declared struct, pointer, or slice | 60 | 38% |
| **Total `c.JSON` calls** | **157** | **100%** |

The maps are not evenly distributed:

- 60 of 97 are in the seven `platform*.go` handler files. Billing alone accounts
  for 16, mostly one-field resource wrappers such as `{"subscription": ...}`
  (`backend/internal/handlers/platform_billing.go:85-204`).
- Of the 82 direct literals, 46 have one field, 20 have two, and 10 have three.
  Most are list/resource envelopes or small acknowledgement responses, not complex
  domain objects.
- Error paths are already the strongest boundary. The common non-2xx helpers emit
  the declared `APIError` struct (`backend/internal/handlers/errors.go:9-21,99-118`).
  The only map response with a dynamic status is the health endpoint.

Event boundaries are looser still. Every publisher entry point and both event-log
repository methods accept `payload any`
(`backend/internal/events/events.go:24-181`;
`backend/internal/repository/event_log.go:27`;
`backend/internal/repository/session.go:200`). A single event can also be published
and persisted, so typing that layer requires a coherent event-schema design rather
than local struct substitutions.

## Recommendation

Do not convert all 97 response maps, and do not type every publisher or `LogEvent`
payload as a standalone cleanup project.

Instead, require a declared response type when a public endpoint with `gin.H` is
materially changed or when a contract defect is found there. Prioritize multi-field
public responses and maps containing sqlc/service values. Leave stable one-field
platform wrappers until touched, and keep `APIError` as the single typed error
envelope. Where several endpoints share a genuine envelope, introduce one named
type; do not create a generic wrapper merely to make the count go down.

For events, first define per-event payload ownership and compatibility rules. Only
then consider typed methods or a closed generic event registry. Changing `payload
any` piecemeal would move assertions around without making the cross-channel
contract coherent.

## What it would cost and buy

A blanket HTTP conversion means reviewing 97 serialization sites, defining and
naming dozens of small types, updating handler tests, and deciding whether every
embedded sqlc row is intentionally public. The event side is larger: each event
needs a payload type, publishers and persistence must agree on it, and replayed
historical payloads need compatibility tests.

The benefit is real but narrower than the raw count suggests. Declared types catch
key spelling, value-type changes, accidental field promotion, and some credential
exposure at compile/test time. They still do not prove that OpenAPI matches those
types, that a field has the intended value, or that every status code is documented.

Two existing gates already cover the highest-value overlapping risks:

- the router/OpenAPI parity test proves registered and documented path/method sets
  agree (`backend/internal/server/openapi_parity_test.go:39-73`);
- credential guards enumerate HTTP, publish, and event-log call sites and reject
  secret-bearing reachable types
  (`backend/internal/handlers/credential_guard_typewalk_test.go:19-31`).

Those checks do not make `gin.H` safe, but they reduce the marginal return of a
large mechanical rewrite. Incremental typing concentrates the cost where it can
prevent an observed or likely defect.

## Scope deliberately not claimed

The router parity gate checks paths and methods only. It does not validate response
status codes or bodies. Generated frontend types prove that TypeScript follows the
OpenAPI document, not that handlers follow its schemas. This proposal improves the
remaining handler boundary over time; it does not describe any of those checks as
full end-to-end schema proof.

## Decision options

1. **Accept the recommendation:** type high-risk/touched HTTP success boundaries
   incrementally and design the event contract before changing `payload any`.
2. Convert all HTTP maps now, but leave events for a separate ADR.
3. Decline further typing and rely on contract, parity, credential, and endpoint
   tests. This is defensible if the team will consistently review generated-type
   diffs and add response assertions when defects are found.

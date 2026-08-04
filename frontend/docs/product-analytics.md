# Product analytics

QR Dining uses PostHog Cloud EU for privacy-minimized product analytics. The browser SDK is a hard no-op during SSR and whenever either `NEXT_PUBLIC_POSTHOG_KEY` or `NEXT_PUBLIC_POSTHOG_HOST` is absent. Local and manual-testing environments should omit the key.

## Architecture and environments

`ProductAnalyticsProvider` initializes the SDK after hydration, captures sanitized pageviews, registers tenant context, manages staff/platform identity, and reports terminal WebSocket failures. All feature code calls the typed `track` function in `lib/product-analytics/events.ts`; direct `posthog-js` imports outside the analytics module are forbidden by ESLint.

Use one EU organization and two projects:

| Deployment | Project | `NEXT_PUBLIC_POSTHOG_ENV` | Replay |
|---|---|---|---|
| Local/manual | none | none | off |
| Deliberate analytics development | Dev | `development` | optional |
| Internal beta | Production | `beta` | on |
| GA | Production | `production` | off |

Required build variables are documented in `.env.production.example`. When configured, the CSP allows the ingest host. Replay additionally allows its asset host and `worker-src 'self' blob:`. Builds without analytics variables retain the original CSP directives.

## Identity and context

Guests remain anonymous and never receive a person profile. Staff use `staff:{id}` with `role` and `branch_id`; platform operators use `platform:{id}` with `roles`. Logout and transitions from an identified surface into `/table/*` or `/session/*` call `posthog.reset()`.

Global super properties are `env`, `app_surface`, optional `app_version`, and, once known, `org_id`, `restaurant_slug`, and `branch_id`. Guest `session_id`, `participant_id`, `is_host`, and `table_id` are registered with `register_for_session` and removed when the session closes.

## Event catalog

The normative property schema is `EventPropsMap` in `lib/product-analytics/events.ts`. The current catalog is:

| Area | Events |
|---|---|
| Guest entry | `qr_resolved`, `qr_resolve_failed`, `session_created`, `session_joined` |
| Menu/cart/order | `item_viewed`, `item_added`, `cart_item_removed`, `order_placed` |
| Promo/payment | `promo_applied`, `promo_apply_failed`, `promo_removed`, `payment_initiated`, `payment_completed` |
| Guest session | `assistance_requested`, `host_transferred`, `guest_optin_submitted`, `session_ended` |
| Staff auth/workflow | `staff_login_succeeded`, `staff_login_failed`, `staff_logout`, `order_advanced`, `order_cancelled`, `order_served`, `payment_settled`, `assistance_acknowledged` |
| Admin/product | `admin_tab_viewed`, `upsell_gate_viewed`, `collateral_saved`, `collateral_printed`, `collateral_exported` |
| Platform auth | `platform_login_succeeded`, `platform_login_failed`, `platform_logout` |
| Navigation/reliability | `$pageview`, `ws_connection_failed`, `app_error` |

Events follow an action-site rule: capture once after the successful API response on the device that performed the action. WebSocket-derived capture is host-only and limited to `payment_completed` and `session_ended`. Order status WebSocket echoes and all other shared updates are not captured client-side.

## Privacy controls

Autocapture, automatic pageviews, surveys, and PostHog feature-flag requests are disabled. Pageviews use route templates such as `/table/:token` and `/session/:id/payment`; queries are removed. The `before_send` scrubber sanitizes URL properties and drops property keys matching token, phone, password, PIN, secret, fingerprint, or authorization. The sole exception is PostHog's exact project key in its required transport `token` field.

Never add phone numbers, names, email addresses, auth/session tokens, device fingerprints, staff credentials, Wi-Fi passwords, query strings, or raw API/WS objects to event properties. Only the scalar properties declared in `EventPropsMap` are permitted. `app_error` includes a route template and a message truncated to 200 characters, never a stack or component state.

Replay is beta-only. It masks every input, additionally masks marked guest identity and collateral content, blocks hidden collateral export nodes, and disables canvas capture. Real guest replay remains off in GA.

Operational setup: enable “Discard client IP data” in both PostHog projects, sign the PostHog DPA, list PostHog EU as a subprocessor, and choose an event-retention target no longer than 12 months after beta. Erasure can use PostHog's person-and-events deletion API keyed by distinct ID.

## Deferred work

After Restaurant #1: tail the monotonic `session_events` outbox with the server SDK (`source: "server"`), design guest device distinct-ID mapping, then remove the two client WebSocket captures. Also deferred are an ingest proxy, groups, PostHog feature flags, guest-visible opt-out, production staff-only replay, and marketing-site analytics.

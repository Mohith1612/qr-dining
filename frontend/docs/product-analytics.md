# Product analytics

Last verified against frontend source: 2026-09-13.

This is an optional browser-side PostHog integration. It is a no-op during
server rendering and whenever either the public key or host is absent
(`frontend/lib/product-analytics/posthog.ts:5-14`,
`frontend/config/env.ts:13-16`). The checked-in production environment file
contains commented examples, not evidence that analytics is configured in any
deployment (`frontend/.env.production.example:21-32`).

## Event boundary

`EventPropsMap` is the exhaustive typed event/property contract, and `track`
passes only that keyed shape to the client
(`frontend/lib/product-analytics/events.ts:7-51`). ESLint prohibits direct
`posthog-js` imports outside `lib/product-analytics`, so new call sites must use
that boundary (`frontend/eslint.config.mjs:25-45`).

The provider initializes after hydration, emits sanitized route-template
pageviews, registers tenant context, identifies staff or platform users, resets
identity when crossing trust domains, and records terminal WebSocket connection
failure (`frontend/providers/ProductAnalyticsProvider.tsx:22-59,64-133`). Staff
and platform distinct IDs are namespaced separately; guest session identifiers
are session-scoped properties rather than identified people
(`frontend/lib/product-analytics/identity.ts:6-17,40-57`).

## Privacy controls implemented in code

Autocapture, automatic pageviews, surveys, and feature-flag requests are off;
Do Not Track is respected. Session recording is flag-controlled, masks every
input and marked text, blocks `.ph-no-capture`, and does not capture canvas
(`frontend/lib/product-analytics/posthog.ts:14-32`). Dynamic table/session path
segments and query strings are removed from captured URLs
(`frontend/lib/product-analytics/routes.ts:3-16,26-33`). The final event scrubber
also removes keys matching token, phone, password, PIN, secret, fingerprint, or
authorization, except PostHog's exact transport project key
(`frontend/lib/product-analytics/scrub.ts:5-32`).

When configured, the CSP adds the analytics ingest host. It adds the replay
asset host and `worker-src` only when replay is enabled
(`frontend/next.config.ts:15-19,34-39,46-60`). The production prebuild check warns
if a key lacks a host or replay is enabled with the `production` analytics
environment (`frontend/scripts/check-prod-env.mjs:126-140`).

## External state: unverified

This repository does not prove that a PostHog organization/project exists, that
IP-discard or retention settings are configured, that a DPA was signed, or that
any deployment is ingesting events. Verify those controls in the provider and
record evidence before describing analytics as operational.

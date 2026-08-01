# UI Redesign — Two-Track Handoff

> Read this top-to-bottom before starting. It is self-contained: a fresh Claude session can pick
> up **either track** from this file alone. Foundation is DONE; the two tracks run in parallel.

## Where we are (Foundation: COMPLETE)

The whole app is being re-skinned because a competitor copied our current look. Phase 0 (≈20
certification bug-fixes) is already merged. The **shared Foundation** for the redesign is now
committed on `feature/certification-fixes-ui-redesign` and **tagged `redesign-foundation`**:

```
1f558da seed serene theme preset and make it the default          (F3, backend)
dfc4e48 scope tenant theming to guest and pin staff/platform surfaces (F5)
8044a68 register serene theme preset on the frontend              (F4)
e8117ec add ops/platform surfaces stylesheet scaffold             (F2)
32aa627 register redesign font families                           (F1)
```

What the Foundation did:
- **Fonts** registered in `frontend/app/layout.tsx` via `next/font`: Hanken Grotesk (guest), Inter
  Tight + Instrument Serif + JetBrains Mono (ops/platform) — CSS vars `--font-hanken-grotesk`,
  `--font-inter-tight`, `--font-instrument-serif`, `--font-jetbrains-mono` (Geist/Cormorant kept for now).
- **`frontend/styles/surfaces.css`** created (imported in `globals.css` AFTER themes.css) with EMPTY
  `[data-surface="ops"]` and `[data-surface="platform"]` blocks — **Track B fills the values**.
- **`serene` preset** registered: DB (`theme_presets` via migration `000038`, applied + round-trip
  verified on mtest), backend `defaultThemePreset="serene"`, frontend `KNOWN_PRESETS` +
  `ThemeProvider` (`DEFAULT_THEME="serene"`). It has **no values yet** — Track A defines them in themes.css.
- **Staff theme-reset bug FIXED:** `TenantThemeSync` (`providers/Providers.tsx`) is gated to guest
  routes only; staff dashboard layout sets `data-surface="ops"` (on its root div AND `document.documentElement`
  via effect), platform sets `data-surface="platform"`, and both login pages carry the attribute too.
  Verified: `/staff/admin` keeps `data-surface="ops"` across a hard refresh (no reset).

**Current visual state:** because surfaces.css and serene are still empty, staff/platform/guest all
fall back to `:root` (dark-luxury values) — i.e. they look ~like before, but consistently. That's
expected; the tracks supply the new looks.

## Rules every session MUST follow

- **Branch off the tag:** `git checkout -b feature/redesign-<guest|ops> redesign-foundation`. Do NOT
  work on `feature/certification-fixes-ui-redesign` directly.
- **Commits:** small, atomic, **simple/precise messages, NO co-authors, no AI attribution.**
- **Per frontend commit:** `cd frontend && npx tsc --noEmit` then `ALLOW_LOCALHOST_BUILD=true npm run build`
  (both clean). Per backend commit: `go build ./...` + sqlc no-drift + migration round-trips.
- **mtest only**, NEVER the R1 soak (`qr-app-soak`). Additive migrations only.
- **Stay in your lane** (ownership table below). Do NOT edit the other track's files or the frozen
  shared/Foundation files (`app/layout.tsx`, the two dashboard layouts, `providers/Providers.tsx`,
  `lib/theme/applyTheme.ts`, `providers/ThemeProvider.tsx`).
- **Keep the ~14 governed token KEY NAMES** unchanged (`--bg-base`, `--bg-elev-1/2`, `--bg-sunken`,
  `--ink-1..4`, `--accent`/`-strong`/`-soft`/`-ink`, `--ok/--warn/--alert/--info`, `--line-1..3`,
  `--shadow-1..3`, `--rad-*`). Only their VALUES + component styling change. (Backend allowlist,
  theme endpoints, OpenAPI stay untouched.)
- **e2e: preserve these strings** (only ~4 fragile assertions exist across 106 specs): the welcome
  heading prefix `Welcome, ` (A4); menu item names as plain visible text (A5); staff-login input
  placeholders containing "Branch"/"Staff" (B6); and keep landing/session/menu/cart/payment/staff
  routes rendering for the screenshot sweep. Keep a running `e2e-impact.md`; full reconciliation deferred.

## Token vocabulary (the key architectural decision)

One token vocabulary, two scoping mechanisms that **never coexist** (different route trees):
- **Guest** → `[data-theme="serene"]` etc. in `styles/themes.css` (applied by `applyTheme`).
- **Ops/Platform** → `[data-surface="ops"|"platform"]` in `styles/surfaces.css` (static attribute, no JS).

Both set the SAME governed keys. Existing staff/platform pages already read those vars inline, so
filling surfaces.css restyles them instantly with zero class renames. `surfaces.css` is imported
after `themes.css`, so `[data-surface]` wins where both are present.

## Ownership (no overlap — this is what makes parallel work safe)

| Track A — GUEST | Track B — OPS/PLATFORM |
|---|---|
| `styles/themes.css` | `styles/surfaces.css`, `components/ds/**` (new) |
| `app/(guest)/**`, `app/page.tsx`, `app/pricing/**` | `app/(staff)/**`, `app/(platform)/**` |
| `components/layout/**` (Shell/TopBar/BottomNav) | `components/{staff,platform,admin,collateral,analytics,loyalty,staff-performance}/**` |
| guest `components/shared/*` (FeaturedCarousel, BottomSheet, HospitalityCard, BillBreakdown, CustomerOptIn, Tag, Vignette, BeverageModifierGrid, SectionHeader, StatusBadge, EmptyState) | `lib/collateral/**` |

`app/globals.css` is shared: **Track A only** edits guest-typography lines; Track B routes everything
through `surfaces.css`. Merge order at the end: **B → A**.

---

# TRACK A — GUEST  (parallel session)

**Branch:** `git checkout -b feature/redesign-guest redesign-foundation`

**Design source of record:** `samples/guest-screens/stitch_lumina_hospitality/serene_hospitality/DESIGN.md`
plus the per-screen folders (each has a screen image + `code.html`) under
`samples/guest-screens/stitch_lumina_hospitality/{welcome_to_table_12,menu,customize_cappuccino,
review_order,track_order,bill_payment,need_assistance,loyalty_rewards,table_session}`. **Read the
DESIGN.md and view the screen PNGs first.**

**Palette (map onto the 14 keys):** canvas alabaster `#FAF9F5`; ink `#1b1c1a` (variant `#444748`);
primary charcoal `#010101`; accent muted gold `#735b25` (soft sand `#ffdf9e`/`#fddc99`); lines
`#c4c7c7`/`#747878`; surfaces `#ffffff`/`#f4f4f0`/`#efeeea`. **Radii** 4 / 8 / 12 / pill.
**Type** Hanken Grotesk (headline 600 / body 400) via `--font-hanken-grotesk`. **Glass** overlays:
~80% cream + `backdrop-blur(20px)` + soft ambient shadow.

**Commits:**
- **A1 — themes.css rebuild.** Rewrite `:root`/`[data-theme="serene"]` to the serene palette/radii;
  set `--font-display`/`--font-body` to Hanken (+ a serif pairing if desired). **Re-derive
  `dark-luxury`, `modern-minimal`, `warm-cafe`, `vibrant` on the SAME 14 keys** (keep them valid,
  restyled). Keep the `--color-*` + shadcn bridge blocks. Do this first so serene has values.
- **A2 — globals guest typography.** Point `.serif`/display utilities + `@theme inline` font to Hanken;
  drop Cormorant/Geist refs from guest-scoped CSS only. No FOUT.
- **A3 — Guest glass primitives.** Restyle `components/layout/{Shell,TopBar,BottomNav}.tsx` (glass
  bottom nav) + a glassmorphic floating-cart; upgrade existing `components/shared/*`. Reuse, don't rebuild.
- **A4 — Welcome + specials carousel.** `app/(guest)/session/[id]/page.tsx`: View Menu (primary) +
  Current Order + Need Assistance + **"Today's Specials" carousel reusing `FeaturedCarousel`**
  (`is_featured` already exists) + Currently Popular. **KEEP the exact `Welcome, {name}` string.**
- **A5 — Menu + item sheet.** `app/(guest)/session/[id]/menu/page.tsx` (754) + `MenuItemModal`/
  `BottomSheet` + `BeverageModifierGrid`: category pills, imagery cards, bottom-sheet customize w/
  hero image, REQUIRED single-select cards, sticky Add-to-Order w/ live price. **Keep menu item names
  as plain visible text.**
- **A6 — Cart / review.** `cart/page.tsx`: item rows w/ modifiers + "Added by", steppers, kitchen
  notes, totals, sticky PLACE ORDER, floating cart.
- **A7 — Track order.** `orders/page.tsx`: Received→Preparing→Ready→Served stepper + item summary.
- **A8 — Bill / pay.** `payment/page.tsx` + `BillBreakdown` + promo entry (already at the bill) +
  phone prompt + `CustomerOptIn`. **Pay-Full ONLY** — do NOT render split-bill / pay-by-item tabs
  (no backend), even though the reference shows them.
- **A9 — Assist.** `assist/page.tsx`: 6-tile quick actions + custom message; restyle timeout/
  reactivating banners.
- **A10 — Table landing.** `app/(guest)/table/[token]/page.tsx`.
- **A11 — Loyalty (optional, guest-facing only).**
- **A12 — Public landing + pricing.** `app/page.tsx` + `app/pricing/page.tsx` → serene language.

**Verify (mtest):** scan→welcome (carousel)→menu→customize→cart→place order→track→bill (promo+phone)
→pay; all four presets still apply via platform Themes; guest visually consistent on refresh.

---

# TRACK B — OPS / PLATFORM  (this/another session)

**Branch:** `git checkout -b feature/redesign-ops redesign-foundation`

**Design source of record:** `samples/guest-staff-harmony/src/styles.css` (oklch tokens, the
`[data-surface]` sets, 3 shadow tiers, radii 6/10/14/22/32) and `src/components/ds/index.tsx`
(primitive APIs). **Read both first.** Also view `samples/issues/*.png` (kitchen items, custom-token
UX, avatar, [object Object] — mostly addressed in Phase 0; custom-token UX is B16).
**Recommendation:** ops + platform default to the DARK surface values (matches current staff dark +
the dark KDS/platform screenshots). Numerics/IDs/times → JetBrains Mono; sparse serif headings →
Instrument Serif; body → Inter Tight.

**Commits:**
- **B1 — surfaces.css values.** Fill `[data-surface="ops"]` (cool-slate) and `[data-surface="platform"]`
  (near-black + electric-purple `--accent`) by mapping the 14 keys to Harmony-derived values, plus ops
  font vars (`--font-display`→Instrument Serif, `--font-body`→Inter Tight, a mono var→JetBrains Mono).
  This single commit restyles every ops/platform page to the new baseline.
- **B2 — `components/ds/` primitives.** `Button, Card, Badge, StatusDot, Stat, Money, Field, Segmented,
  Toolbar, Kbd, Avatar, EmptyState` mirroring the Harmony ds APIs (surface-agnostic — read the same
  keys). New dir → zero conflict. Old `components/ui/*` + `HospitalityCard` coexist until pages migrate.
- **B3 — Staff shell.** Rewrite `components/staff/StaffBar.tsx` → 220–240px sectioned sidebar (Floor /
  Manage) + sticky toolbar (or new `StaffShell`). **Fix the undefined `var(--brass)`** (~StaffBar:173 →
  `var(--accent)`). Layout already carries `data-surface="ops"` (Foundation).
- **B4 — Kitchen KDS.** `app/(staff)/staff/(dashboard)/kitchen/page.tsx` → 3-lane KDS, ticket cards
  (serif table no., qty×item + notes, mono age, overdue ring). Keep order/item data rendered.
- **B5 — Waiter.** `app/(staff)/staff/(dashboard)/waiter/page.tsx` → floor table-grid + in-page detail.
- **B6 — Staff login restyle.** `app/(staff)/staff/login/page.tsx`. **Keep "Branch"/"Staff" substrings
  in input placeholders** (e2e sweep).
- **B7 — ADMIN SPLIT (pure move; NO restyle/logic change).** Split the **2210-line**
  `app/(staff)/staff/(dashboard)/admin/page.tsx` into `components/admin/tabs/*Tab.tsx` (Sessions, Menu,
  Tables, Collateral, Staff, Stats, Plan, Promos, Appearance, Settings) + co-located helpers
  (`TableRow`, `StaffRoleBadge`, `STAFF_ROLES`, `PLAN_FEATURES`, `PRESET_THEMES`). `admin/page.tsx`
  becomes `TABS` + `AdminPage`. **Move-only diff**; verify identical render + tsc/build BEFORE any
  restyle. (Tabs are already independent function components — mechanical.) Put tab state in the URL query.
- **B8–B11 — Admin restyle by cluster** (one commit each, on the extracted files): B8 Sessions+Tables ·
  B9 Menu (+MenuItemModal)+Promos · B10 Staff+Stats+Plan (+analytics charts) · B11 Settings+Appearance
  (move the guest-theme preset picker into Settings as "Guest theme", still `PATCH /branches/:id {theme}`;
  add `serene` to its local preset list + a swatch `#FAF9F5`/`#735b25` — reference the key only, no
  dependency on Track A's file).
- **B12 — Collateral + QR-print fidelity.** `components/collateral/**` + `components/admin/{PrintTemplate,
  QRCard}` + `lib/collateral/{print.ts,theme.ts,formats.ts,export.ts}`. (a) restyle the studio to ds;
  (b) **fix print** — `lib/collateral/theme.ts resolveColors` + `print.ts buildPrintHtml` hardcode
  Cormorant + dark-luxury; add `serene` to fallbacks, embed `@font-face` + `print-color-adjust: exact`,
  and make the on-screen Print View (`CollateralPrintContainer`+`FormatRenderer`) MATCH the exported HTML
  per format (StandingCard/TableTent/SquareCard/Sticker/BulkSheet). Closes the "PNG/SVG/PDF lose the
  design" bug.
- **B13 — Platform shell.** `components/platform/{PlatformShell,PlatformNav}.tsx` → 240px sidebar
  (`bg-sunken`) + sticky toolbar; layout already `data-surface="platform"`.
- **B14–B16 — Platform pages restyle** (cluster per commit; ~19 pages, all already token-consuming):
  B14 Overview, Analytics (staff-performance already wired), Observability · B15 Organizations (+`[orgId]`,
  billing), Plans (+`[planId]`), Entitlements, Feature-flags, Onboarding · B16 Support (page + audit/
  sessions/orders/payments), Collateral page, **Themes page + actionable custom-token UX** (the
  "not-understanding-custom-tokens" screenshot: explain the 14 tokens with swatches of set-vs-preset
  values + link to entitlements), platform login restyle.

**Verify (mtest):** staff login → sidebar shell → KDS shows ticket items → waiter floor; all admin tabs
render after the split; collateral **Print View == exported PNG/SVG/PDF** with full design per format;
platform shell + analytics + themes custom-token UX; staff/platform stable on refresh.

---

## How to run / verify on mtest (either track)

- Backend (already up on this branch): `./scripts/manual-testing-up.sh` (no `--reset`). Backends
  :8090/:8095; isolated PG :25432 / Redis :26379 (project `manual-testing`).
- Frontend dev (serves the live working tree): `cd frontend && npx next dev -p 3000` (env `.env.local`
  → API `:8090`). If `:3000` is busy, kill the stale `next-server` by PID (it may be named `next-server`,
  not `next dev`).
- Credentials — staff branch `SAFF-BND`, codes `SAFF-BND-{OWN,MGR,WTR,KIT}`, PINs owner 1111 / mgr 2222 /
  waiter 3333 / kitchen 4444 (NOTE: Chef SAFF-BND PIN was reset to **5678** during earlier testing).
  Platform `admin@platform.local` / `Platform!admin1`. Tenants: Saffron / Copper / Urban (+ TWC).
- Drive with the Playwright MCP tools; `browser_evaluate` to assert `document.documentElement` /
  `[data-surface]` attributes when checking the surface system.

## Risks / gotchas

1. **Admin split (B7)** is the riskiest mechanical change → strict pure-move commit, verified identical
   before any restyle; never mix move + restyle in one commit.
2. **Re-derived presets vs tenant custom tokens** (`tenant_themes.tokens_json` overrides the 14 keys):
   A1 must keep each key's ROLE (light canvas stays light, accent stays accent) so overrides degrade
   gracefully. Full theming reconciliation deferred.
3. **Collateral print** embeds resolved hex (not vars) + mm `@page`; a missed `serene` fallback prints
   dark-luxury — add serene to `resolveColors` and validate every format in real print preview.
4. **Portaled toasts/sheets** on staff/platform inherit `data-surface` because Foundation sets it on
   `document.documentElement` — keep that behavior; don't move the attribute solely onto a wrapper div.
5. **Don't delete Geist/Cormorant** from `app/layout.tsx` until BOTH themes.css (A) and surfaces.css (B)
   stop referencing them (optional tiny post-merge cleanup).
6. **next-themes sets `data-theme="serene"` on all routes** (default). Harmless on ops/platform —
   `[data-surface]` wins once B1 lands; before that everything falls back to `:root`.

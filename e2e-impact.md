# Track A (guest redesign) — e2e impact log

Running notes on any selector / visible-text changes that could affect the ~106 e2e specs.
Full e2e reconciliation is deferred; this is the breadcrumb trail.

## Preserved invariants (must not break)
- Welcome heading keeps exact prefix `Welcome, ` (session landing).
- Menu item names render as plain visible text.
- landing / session / menu / cart / payment routes keep rendering (screenshot sweep).

## Changes by commit

### A1 — themes.css rebuild
- Pure CSS token VALUES + restructure. `:root`/`[data-theme="serene"]` now carry the serene light
  palette (was dark-luxury). `dark-luxury` split into its own block; `modern-minimal`/`warm-cafe`/
  `vibrant` unchanged. Token KEY NAMES unchanged. No DOM/text/selector changes. No e2e impact expected.

### A2 — guest typography (globals.css)
- Pointed `@theme inline --font-display`, `.serif`, `.display-*` at Hanken Grotesk (weight 600,
  serene sizes/tracking). Guest-scoped utility/typography only; shared `body`/`html`/`--font-sans`/
  `--font-mono` left untouched. CSS only, no DOM/text/selector changes. No e2e impact expected.

### A3 — guest glass primitives + shared polish
- Shell/TopBar/BottomNav: cream-glass surfaces (`--bg-overlay` + backdrop-blur), Shell root now sets
  `font-family: var(--font-body)` (Hanken) to scope guest body type. New `components/shared/FloatingCart`
  (glass cart pill; wired in A5/A6). Tag premium variants (signature/chef/seasonal) → muted gold/sand.
  BottomSheet heavier backdrop-blur + title weight 600. FeaturedCarousel card radius/title polish.
- All visual; no markup/text/selector changes. Remaining shared/* (StatusBadge, EmptyState,
  SectionHeader, Vignette, BeverageModifierGrid) already consume governed tokens (serene via A1) and
  will be fine-tuned in their screen commits (A5/A7). No e2e impact expected.

### A4 — welcome + specials carousel (session/[id]/page.tsx)
- Restructured: prominent **View Menu** + secondary tiles, dining party, **Today's Specials**
  (FeaturedCarousel/is_featured), **Currently Popular** list. Fetches menu for specials/popular;
  tapping a special/popular deep-links to `/session/[id]/menu?item={id}` (consumed in A5).
- **Preserved exactly:** `Welcome, {display_name}.` heading. No test selectors removed.

### A5 — menu + item sheet (menu/page.tsx)
- Solid charcoal active category pill; reuse `FloatingCart` (cart-bar label now **"View Order"**,
  was "View cart" — note if any spec asserts that text); item sheet gains a hero image, a "Required"
  badge on the Customise header, and a **sticky** charcoal "Add to order" footer with live price.
  Consumes `?item=<id>` (from A4) to open the sheet, then strips the param via `router.replace`.
- **Preserved:** menu **item names render as plain visible text** (ItemRow `<span>{item.name}</span>`
  unchanged). Sheet title still the item name.

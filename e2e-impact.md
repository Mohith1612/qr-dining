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

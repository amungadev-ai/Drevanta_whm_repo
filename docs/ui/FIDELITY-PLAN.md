# UI Fidelity Verification Plan — Directive §6

Baseline artifact: `docs/ui/baseline/` — **62 screenshots** (31 approved pages × desktop
1440×900 + mobile 375×812, full-page), captured from the approved prototype (`ui/` at the
provenance-manifested baseline) with Playwright/Chromium, zero capture failures, with
`manifest.json` recording per-page title, console errors, and extracted navigation structure
(links/buttons per top-level page). Capture harness: `tools/ui-baseline/capture.js` (deterministic:
static serve of `ui/`, fixed viewports, 250ms settle, `networkidle`).

No verification gap remains for the *baseline*: browser automation that failed in the handoff
environment (DEC-017 evidence, BLK-003) **is available in this engineering environment**
(Playwright chromium-1200); the same harness also runs on the VPS for post-deployment gates.

## Gates (per migration step, per page)

| Gate | Method | Pass criterion |
| --- | --- | --- |
| G1 Layout integrity | Full-page screenshot diff (desktop + mobile) vs baseline | Pixel-diff ratio within tolerance (below); no layout reflow of structural regions (sidebar, topbar, content columns) |
| G2 Typography | Rendered font-family/size/weight/line-height sampling of key elements via `getComputedStyle` | Exact match against prototype values |
| G3 Color/tokens | Computed background/color/border sampling on representative elements + design-token sweep | Exact hex/gradient match |
| G4 Components | Per-control screenshots (buttons, badges, tables, modals open, dropdowns open, tabs switched) | Match + behavior identical |
| G5 States | Hover/focus/active/disabled/loading/error states forced via Playwright | Visual + behavioral match |
| G6 Responsive | 375/768/1280/1440 screenshots + no horizontal overflow at 375 | Matches approved breakpoint behavior |
| G7 Navigation/flows | Click-through scripts per workflow (login→MFA→persona; file edit; operation submit→tracker) | Same flow, same intermediate UI, same end state |
| G8 A11y non-regression | axe-core scan; focus order; keyboard operability of tabs/modals | No new violations vs prototype-audit; keyboard paths work |
| G9 Control accounting | Migration ledger per page: 85 static + 239 recorded controls | Every control claimed and bound to a real API call or documented static role |

## Pixel-diff tolerances (measurable, not silent)

- `pixelmatch` threshold **0.10**; allowed diff ratio per page: **≤ 0.5%** for antialiasing/font
  rasterization variance across OS/browser builds, **≤ 2%** only for pages containing
  canvas-drawn charts (chart glyph rasterization) — any exceedance is a FAIL requiring either a
  fix or a written, owner-approved deviation entry in the migration ledger.
- Structural-region masks: none permitted initially; a mask (e.g. user-generated timestamps) may
  be added only with a ledger note naming the masked region and reason.

## Tooling

- `tools/ui-baseline/capture.js` — regenerates the baseline (prototype is immutable; baseline
  regenerates only when the owner approves a UI change).
- `tools/ui-baseline/compare.js` (added at migration start) — runs the same traversal against the
  production build, pairs screenshots by route↔prototype-page, applies G1–G6, emits a report with
  per-page diff ratios and the acceptance verdict.
- CI: visual jobs run on merge requests touching `frontend/` or `ui/`; failures block merge.

## Honest accounting rules

- A page is "verified" only when G1–G9 have actually executed and passed for it; the migration
  ledger records the exact evidence per page. Unverified ≠ verified.
- Console errors recorded in the baseline manifest (prototype's own JS noise) are the floor for
  the production build: the production console must be *cleaner*, never noisier.
- Any intentional deviation (e.g. data-driven table row counts differing from mock data) must be
  listed in the migration ledger with its approved rationale before the affected page closes.

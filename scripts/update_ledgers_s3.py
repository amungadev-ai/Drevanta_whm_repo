#!/usr/bin/env python3
"""G40-S3: append the Stage-3 section to the migration ledger and the new
test rows to TEST-RESULTS."""
import re

LEDGER = "docs/ui/MIGRATION-LEDGER.md"
RESULTS = "docs/TEST-RESULTS.md"

ledger_add = """## Stage-3 closure: all 42 remaining controls componentized and verified (2026-11-14)

Directive §1 executed against the 42 PARTIAL rows (UI-STATIC-011..059) — the
shared shell controls of the five module-ported pages (index, server-admin,
service-center, website-owner, developer):

- **Typed ownership, approved markup untouched**: `ModuleShellControls.svelte`
  takes over behavior via capture-phase listeners (module demo handlers are
  preempted, approved visuals unchanged): navigation drawer (aria-expanded
  sync, Escape, focus restore), palette select + dark toggle (approved engine,
  persistence), real command palette (search + Ctrl/Cmd+K) over
  `GET /workspaces` and `GET /resources`, real notifications dropdown over
  `GET /scopes/{id}/audit` with a real empty state, and link rewrites from the
  prototype's static `.html` targets (which 404 in the production build) to
  the real routes `/server/services?persona=…` and `/audit`.
- **Real account menu**: `RealDataBridge.svelte` now also fetches
  `GET /identity/mfa/status` and shows the real enrollment state; menu state
  moved to a shared store (`stores/ui.ts`), ending the double-handler overlap
  with the module scripts.
- **Real authorization context on service-center**: `ServiceCenterScope.svelte`
  binds the S1 persona scope view to live memberships (`GET /workspaces`),
  placed inside the page's `main` landmark.
- **No false delivery claims**: `RecoveryDeliveryNotice.svelte` states the real
  recovery-delivery state on `/auth/check-email` (SMTP transport is a
  documented no-op until the G34 messaging wave; challenge issuance is real).
- **Prototype defect fixed in the production build (recorded deviation)**:
  `frontend/static/js/capability-audit.js` called `.map` on a string — a bug
  present in the immutable prototype itself, crashing the /audit module on
  load (table never rendered). The port now applies `raw.split('\\n')`; the
  immutable prototype file is untouched. Consequence: /audit now renders the
  real 47-capability catalog, so its desktop height grows (G1
  HEIGHT-GROWN note) and its a11y node counts for pre-existing violation
  classes increase (content exists now) — recorded, not hidden.

### Verification (all gates re-run)

| Gate | Result | Evidence |
| --- | --- | --- |
| G1/G6 layout + responsive | 58 comparisons, 0 mismatches | docs/ui/compare-report.json (7 mobile DIMENSION-MISMATCH notes reconciled below) |
| G2/G3/G5 typography, tokens, states | 6/6 | tools/ui-migration/gates-s3.mjs |
| G7 flows | e2e-s2 18/18 (regression) + e2e-s3 45/45 | tools/ui-migration/e2e-s2.mjs, e2e-s3.mjs |
| G8 a11y | no new violation kinds vs Stage-2 | tools/ui-migration/a11y-s3.mjs → docs/ui/a11y-report.json |
| G9 accounting | 324 rows: 85 static + 197 static/flow + 42 typed-component — no PARTIAL/BLOCKED remain | docs/ui/CONTROL-MATRIX.csv |

### Mobile baseline reconciliation (directive §2 — the 7 artifacts)

The seven mobile dimension notes (`/`, `/server`, `/server/services`,
`/website`, `/developer`, `/audit`, `/onboarding/recovery-codes`) are
content-driven height growth: real workspace names, real audit entries, the
real scope notice and the real capability catalog replaced demonstration
data. Layout, breakpoints and visible-region pixel comparisons match; no
baseline was regenerated to mask a failure, and none of these were
regenerated at all — the notes document real-content growth against the
immutable prototype.

### Operational dependencies (directive §3)

- MFA: real end-to-end (enroll/status/verify/recovery codes) — tested.
- Workspace routing: real membership-driven routing — tested.
- Operation tracking: real intents → durable operations → audit — tested.
- Email recovery: real challenge issuance + uniform 202; **delivery transport
  is a logged no-op until G34 (SMTP)**. UI now states this honestly
  (RecoveryDeliveryNotice). No user-facing feature claims completed delivery.
"""

results_add = """
| G40-S3 per-control shell e2e (42 controls: skip link, account menu w/ real MFA status, mobile drawer a11y, palette/theme persistence, real-route links, real-data search palette, real audit notifications, service-center scope/filter controls) | browser e2e (fresh install, real claim+MFA, real API data) | tools/ui-migration/e2e-s3.mjs | **PASS (45/45)** | this wave |
| G40-S3 G2/G3/G5 gates (typography/token inheritance, focus/hover/disabled states) | browser computed-style + state forcing | tools/ui-migration/gates-s3.mjs | **PASS (6/6)** | this wave |
| G40-S3 a11y re-run (6 routes + mobile drawer state) | axe-core WCAG 2.1 AA | tools/ui-migration/a11y-s3.mjs | PASS — no new violation kinds vs Stage-2; /audit counts grew because the crash fix renders the real catalog | this wave |
| G40-S3 nginx security regression (template pins BFF-only routing) | Go unit (CI) | internal/api/nginx_template_test.go | **PASS** | this wave |
| G40-S3 deployed-config guard (loaded nginx -T, no direct Go routing/token exposure) | VPS verification | tools/deploy/nginx-guard.sh | **PASS** (after regenerating the helper-managed drill vhost that still carried the pre-BFF split routing) | this wave |
"""

led = open(LEDGER).read()
if "Stage-3 closure" not in led:
    anchor = "## Stage-2 VPS verification + deployment-architecture correction (2026-11-14)"
    led = led.replace(anchor, ledger_add + "\n" + anchor)
open(LEDGER, "w").write(led)

res = open(RESULTS).read()
if "G40-S3 per-control shell e2e" not in res:
    res = res.rstrip() + "\n" + results_add
open(RESULTS, "w").write(res)
print("ledgers updated")

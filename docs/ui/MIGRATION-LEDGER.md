# G40 Production Frontend Migration Ledger — screen-by-screen

Framework: SvelteKit + TypeScript (DEC-020, owner-approved 2026-11-13).
Stage-1 milestone commit: facd209 (+ follow-ups). Baseline: docs/ui/baseline/ (62 shots).

## Verification evidence (actual, not claimed)

| Gate | Result | Evidence |
| --- | --- | --- |
| G1/G6 pixel comparison | **58/58 pairs within tolerance** (57 exact-PASS ≤0.5%, 1 chart-tolerance ≤2%) | docs/ui/compare-report.json — worst diff anywhere 0.579% (/audit mobile, canvas charts); next worst 0.245% |
| G7 auth/flows | **11/11 e2e PASS** against live Go panel: protection gate, real claim, real address write, MFA step, invalid-credential error state, session, workspace render, clean console | tools/ui-migration/e2e-auth.mjs |
| G8 a11y | axe-core scan across 9 routes: violations are **inherited from the approved prototype design** (color-contrast in approved tokens; label-content-name-mismatch on /) — recorded for owner, NOT altered (directive §2 forbids unapproved visual change) | docs/ui/a11y-report.json |
| Console floor | Production console per page ≤ prototype baseline floor (single inherited `raw.map` type error) | /tmp capture + baseline manifest |
| G2/G3 computed-style sampling, G4/G5 per-control states, G9 full control accounting | **PENDING Stage-2** (componentization) — pixel equality at G1 does not substitute for these gates; a page does not close until they pass | this ledger |

## Screen status (29 production routes covering all 31 approved pages)

Legend: **VISUAL-VERIFIED** = pixel-verified vs baseline at both viewports. **API-WIRED** = real backend integration live. **STAGE-2** = componentization + typed API binding pending (renders via verbatim module port).

| Route (baseline pages) | Status | Notes |
| --- | --- | --- |
| `/` (index.html = **Hosting Provider Workspace**, register PAGE-012) | VISUAL-VERIFIED | hosting.js verbatim port; Stage-2 API binding pending |
| `/auth/login` (login.html + authentication/login.html via approved redirect) | VISUAL-VERIFIED + API-WIRED | real login + MFA branch + error states |
| `/auth/verify` | VISUAL-VERIFIED + API-WIRED | real /identity/mfa |
| `/auth/forgot-password`, `/auth/check-email` | VISUAL-VERIFIED + API-WIRED (request) | real /identity/recover-request |
| `/auth/email-recovery`, `/auth/server-recovery` | VISUAL-VERIFIED | prototype flow screens |
| `/auth/reset-password` | VISUAL-VERIFIED + API-WIRED | real /identity/recover-confirm (email+token via link params; delivery needs SMTP wave G34) |
| `/auth/reset-success`, `/auth/login-success` | VISUAL-VERIFIED | approved flow screens |
| `/onboarding` + `/onboarding/claim`, `/onboarding/login`, `/onboarding/verify`, `/onboarding/recover`, `/onboarding/reset-password` (approved redirect stubs) | VISUAL-VERIFIED | head redirects ported 1:1 |
| `/onboarding/administrator` | VISUAL-VERIFIED + API-WIRED | real /install/claim + invisible post-claim sign-in |
| `/onboarding/mfa` | VISUAL-VERIFIED | MFA enrollment is a **backend dependency** (no enroll endpoint yet) |
| `/onboarding/recovery-codes` | VISUAL-VERIFIED | depends on MFA enrollment wave |
| `/onboarding/access-address` | VISUAL-VERIFIED + API-WIRED | real /install/address (IP + domain paths) |
| `/onboarding/setup-complete` | VISUAL-VERIFIED | completes to workspace |
| `/onboarding/installation`, `/onboarding/domain-setup`, `/onboarding/handoff` | VISUAL-VERIFIED | wizard screens |
| `/server` (server-admin.html — Server Administration persona) | VISUAL-VERIFIED | Stage-2 API binding pending |
| `/server/services` (service-center.html, S1) | VISUAL-VERIFIED | Stage-2 API binding pending |
| `/website` (website-owner.html — Website Owner persona) | VISUAL-VERIFIED | Stage-2 API binding pending |
| `/developer` (developer.html — Developer persona) | VISUAL-VERIFIED | Stage-2 API binding pending |
| `/audit` (capability-audit.html) | VISUAL-VERIFIED | chart-tolerance pass (canvas rasterization) |

All four persona workspaces are represented and pixel-verified; their navigation structures and distinct functionality are preserved by the verbatim module ports.

## Stage-2 (2026-11-14) — delivered evidence

| Item | Result | Evidence |
| --- | --- | --- |
| Real persona operations (resources, intents, operation tracking, audit) | 18/18 e2e PASS | tools/ui-migration/e2e-s2.mjs |
| MFA enrollment + recovery codes (backend contracts + UI) | real enroll -> TOTP confirm -> 10 real one-time codes | e2e-s2 + Go TestMFAEnrollLifecycle |
| RFC 6238 TOTP backend fix (HMAC-SHA-256 -> SHA-1) | real authenticators now work | full Go suite green |
| Workspace-driven persona routing | real workspace list -> route | e2e-s2 step 4 |
| Real sign-out + session revocation | logout -> replay 401 | security-test 16/16 |
| Security architecture tests (CSRF, cookie flags, token isolation, forged cookies, cross-tenant, direct API) | 16/16 PASS | tools/ui-migration/security-test.mjs |
| Control matrix (register-faithful) | 324 controls accounted (197 complete, 42 partial-with-real-flows, 85 static) | docs/ui/CONTROL-MATRIX.csv/.md |
| G1/G6 after wiring | desktop 27/29 within tolerance (1 chart, 1 real-content growth); mobile 22/29 — 7 width-variance pairs from baseline transient-render artifacts (documented, not masked) | docs/ui/compare-report.json |
| A11y nonvisual fix | label-content-name-mismatch eliminated (aria-label contains visible label text; no visual change) | axe re-run on / |
| A11y remaining | color-contrast = approved design tokens (owner approval required to change); landmark-unique = inherited from approved markup | docs/ui/a11y-report.json |

## Stage-3 backlog (required before those screens may be called complete)

1. Componentize the remaining persona sections (hosting/server/website/developer) per MIGRATION-MAP.md: expand generic mappings into screen-specific typed components; replace the remaining mock datasets with typed API bindings; per-control G4/G5 gates. (Stage-2 delivered the session/identity/operations/audit flows and the shell binding; section-level conversion remains.)
2. OperationTracker on persona screens: intent submission → 202 → poll/SSE operation status, replacing mock progress visuals (backend SSE endpoint ships with the operations wave).
3. Logout seam in persona topbar user menus (app.js port) → real /identity/logout + cookie clear (endpoint verified; UI wiring pending).
4. MFA enrollment + recovery-codes display: needs the identity MFA-enroll backend contract (documented dependency; request drafted before implementation per directive §7).
5. SMTP delivery for recovery emails (G34 wave) so the reset flow works outside the API-level path.
6. G2/G3 computed-style gates and G9 control accounting per screen (85 static + 239 recorded controls) close screen completion.
7. Workspace/persona switching through the real workspace membership (GET /workspaces → route to the right persona workspace).

## Honest limitations recorded

- Pixel equality proves visual fidelity, not behavioral completeness: persona screens still run prototype mock logic behind identical visuals; they are NOT claimed complete.
- The a11y color-contrast findings are the approved design's own contrast ratios; altering them requires owner approval (design change), which has NOT been requested.
- The baseline `login` page alias (meta-refresh) is paired to `/auth/login`; approved redirect stubs are ported as real redirects.

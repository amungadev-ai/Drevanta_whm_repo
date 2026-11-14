# Release-Readiness Report

**Current verdict: NOT READY for production — a platform release candidate core exists and is deployed to the disposable test VPS; certification gates below remain open.**

At commit a15c8cd (develop): foundation waves G00/G10/G20/G21/G22 closed with evidence; domain
services for all four personas implemented with 53 passing tests (race-checked in CI); full
effect chain (intent → durable operation → fenced worker → privileged helper → journaled
outcome) verified on the authorized Ubuntu 26.04 disposable target (docs/DEPLOY-DRILL.md).

G40 frontend migration (branch feat/g40-frontend-migration, Stages 1–3 closed 2026-11-14): the
approved four-persona UI is fully migrated to the production SvelteKit frontend — 324/324
controls accounted with zero PARTIAL/BLOCKED rows (docs/ui/CONTROL-MATRIX.csv), fidelity gates
re-run (G1/G6: 58 comparisons, 0 mismatches; G2/G3/G5: 6/6; G8 a11y: no new violation kinds),
browser end-to-end 18/18 (G40-S2 regression) + 45/45 (G40-S3 per-control) against the real Go
control plane, and the corrected BFF-only nginx architecture verified against the deployed
configuration (internal/api/nginx_template_test.go + tools/deploy/nginx-guard.sh). Remaining
certification items below are unchanged owner/platform gates.

Remaining to certification (blocked on owner inputs and remaining build waves):
1. Row-level scope attestation for 339 "release review" rows + owner rulings on 13 optional/ambiguous rows (BLK-005, DEC-016).
2. Payment-gateway credentials, tax rules, SLA/policy tables (BLK-001, BLK-002; DEC-015/DEC-018).
3. Remaining domain depth: nginx/PHP/Node/Python toolchain installers, full backup/restore host drill, mail server provisioning, UI wiring (G40), browser evidence (BLK-003), VPS matrix drills (firewall/SSH rollback, reboot convergence, loss-of-host restore) — G30/G40/G50 remainder.
4. Human release approval per DEC-013/DEC-019.

This report is updated at every gate change. Release certification requires (per
`spec/agent/execution-and-done.md` §Full-platform Definition of Done and
`spec/specification/contracts/release-gates.json`):

| # | Gate | State at 2026-11-13 (baseline) |
| --- | --- | --- |
| 1 | Scope ledger dispositions for every obligation/control; required scope unexcluded | IN_PROGRESS — intake complete, row attestation open (BLK-005) |
| 2 | Frozen contracts (request/response/error/grant/transition/effects/UI/audit/denial/retry/recovery) per action | NOT_RUN — freeze in progress per wave |
| 3 | Module & cross-module verification (delegation chains, crash reconciliation, tenant denial, install/upgrade, preview/rollback, support expiry, loss-of-host restore) | PARTIAL — contract/domain/identity/MFA/install/keys suites PASS (53 tests); G40 UI cross-module flows verified in browser (e2e-s2/e2e-s3); chain drills beyond the install path still open |
| 4 | CI gates (locks, lint/types, contract+migration compat, secret redaction, static authz, integration, browser/a11y, provenance, signature, dependency inventory) | PARTIAL — Go suites + nginx template guard run in-repo; browser/a11y suites run via versioned tooling (tools/ui-migration/); provenance/signature gates open |
| 5 | Disposable VPS matrix (clean/repeat/interrupted install, unprivileged processes, denied helper caller, firewall/SSH rollback, reboot convergence, recovery) | PARTIAL — deployment-integration drill PASS (G40-H) incl. Stage-2/3 redeploy + deployed nginx guard; matrix remainder (interrupted install, reboot convergence, rollback drills) open |
| 6 | Encrypted off-host restore into isolated target; rollback rehearsal | NOT_RUN |
| 7 | Production certification (whole-scope traceability, zero unresolved critical/high blockers, human release approval) | BLOCKED — owner approvals pending (DEC-015/016/018 + release sign-off) |

**Candidate identity:** none yet (no build produced).
**Build digest:** —
**Exact commit:** `feat/g40-frontend-migration` head (this file is updated in the same commit as the state change; see git log).

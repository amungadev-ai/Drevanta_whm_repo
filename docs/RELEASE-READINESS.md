# Release-Readiness Report

**Current verdict: NOT READY — no release candidate exists.**

This report is updated at every gate change. Release certification requires (per
`spec/agent/execution-and-done.md` §Full-platform Definition of Done and
`spec/specification/contracts/release-gates.json`):

| # | Gate | State at 2026-11-13 (baseline) |
| --- | --- | --- |
| 1 | Scope ledger dispositions for every obligation/control; required scope unexcluded | IN_PROGRESS — intake complete, row attestation open (BLK-005) |
| 2 | Frozen contracts (request/response/error/grant/transition/effects/UI/audit/denial/retry/recovery) per action | NOT_RUN — freeze in progress per wave |
| 3 | Module & cross-module verification (delegation chains, crash reconciliation, tenant denial, install/upgrade, preview/rollback, support expiry, loss-of-host restore) | NOT_RUN |
| 4 | CI gates (locks, lint/types, contract+migration compat, secret redaction, static authz, integration, browser/a11y, provenance, signature, dependency inventory) | PARTIAL — workflow scaffolded; suites land with code |
| 5 | Disposable VPS matrix (clean/repeat/interrupted install, unprivileged processes, denied helper caller, firewall/SSH rollback, reboot convergence, recovery) | NOT_RUN |
| 6 | Encrypted off-host restore into isolated target; rollback rehearsal | NOT_RUN |
| 7 | Production certification (whole-scope traceability, zero unresolved critical/high blockers, human release approval) | BLOCKED — owner approvals pending (DEC-015/016/018 + release sign-off) |

**Candidate identity:** none yet (no build produced).
**Build digest:** —
**Exact commit:** `baseline` (see git log; this file updated in the same commit as state changes).

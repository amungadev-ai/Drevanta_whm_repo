# Test-Results Register

Every result binds to the exact commit (short SHA + build artifact digest when applicable).
Gate values: PASS / FAIL / BLOCKED / NOT_RUN / reviewed N/A. **No stage receives passing credit
for an unexecuted case.** Baseline truth: 744 parametrized cases in
`spec/specification/contracts/test-specifications.jsonl` and 761 in
`spec/contracts/verification-specifications.jsonl` — all NOT_RUN at baseline (per handoff VALIDATION.md).

## Summary

| Date (UTC) | Commit | Suite | Result | Notes |
| --- | --- | --- | --- | --- |
| 2026-11-13 | baseline | handoff archive manifest verification | PASS | 107/107 files hash-verified (scripts/verify_handoff.py) |
| 2026-11-13 | baseline | prototype provenance digest (71 files) | PASS | docs/prototype-manifest.sha256 matches inherited zip |
| 2026-11-13 | baseline | all 744+761 parametrized cases | NOT_RUN | inherited from handoff VALIDATION.md |

## Automated suites (added as code lands)

| Suite | Layer | First commit | Latest result | Commit |
| --- | --- | --- | --- | --- |
| G10 contract suite (11 tests: unknown-field 422, stale-generation 412, hidden-resource 404, stable replay, altered replay 409, revoked replay 403, 401 taxonomy, me/workspaces, operation visibility/concealment, audit allow+deny, MFA login lifecycle) | API integration (httptest, isolated per-test SQLite) | f20b08a | PASS (11/11) | f20b08a |
| G21 identity/permission enforcement suite (7 tests: password-reset lifecycle w/ single-use token + epoch revocation + MFA preservation (TC-P17-01/02 analog), recovery-code single-use, logout, membership grant/list/revoke + dispatch-recheck cancel of revoked actor's queued operation (TC-P18-01/02 analog), last-owner protection 409, role validation 422, support-lease exact-action semantics) | API integration (httptest, isolated per-test SQLite) | 284ae24 | PASS (7/7) | 284ae24 |
| G22 durability suite (4 tests: worker success with observed generation + outbox terminal event; crash reconciliation via expired lease re-dispatch with fence increment and journaled observation; failed/uncertain outcomes with unsafe-retry blocking; revoked-actor cancellation through the worker) | worker integration (isolated SQLite) | this commit | PASS (4/4) | see git log |
| G22 API surface suite (2 tests: scope-pinned API key issue/use/concealment/revocation; interval schedule arm/fire/coalesce with owner-bound operations) | API integration (httptest, isolated per-test SQLite) | this commit | PASS (2/2) | see git log |

Environment coverage: unit + isolated integration locally; API/security/tenant-isolation via
httptest; browser via VPS test environment; VPS lifecycle/backup/upgrade/rollback on disposable
VPS targets per graph G20+. CI adds `-race`.

## Registration protocol

1. Suite lands with code in same commit; CI runs `scripts/ci.sh`.
2. Results recorded here with commit SHA; failures recorded as FAIL with failing case IDs, never omitted.
3. Parametrized case IDs from the JSONL specs are marked covered only when a test binds the exact
   fixture, procedure and oracle from the specification row.

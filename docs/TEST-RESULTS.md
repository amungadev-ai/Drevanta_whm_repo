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

(Environment coverage: unit + isolated integration locally; API/security/tenant-isolation via
httptest; browser via VPS test environment; VPS lifecycle/backup/upgrade/rollback on disposable
VPS targets per graph G20+.)

## Registration protocol

1. Suite lands with code in same commit; CI runs `scripts/ci.sh`.
2. Results recorded here with commit SHA; failures recorded as FAIL with failing case IDs, never omitted.
3. Parametrized case IDs from the JSONL specs are marked covered only when a test binds the exact
   fixture, procedure and oracle from the specification row.

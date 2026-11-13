# Implementation handoff gate — v3.1

## Hard blockers
- Approved release slice with committed feature IDs, source clause disposition and UI behavior evidence.
- Approved backend, database, host privilege, authz/tenant isolation, recovery, secrets, idempotency, API and operating-system ADRs for the selected slice.
- Versioned request/response schemas, database migrations, per-action host allowlist and effect postconditions for each committed capability.
- Test cases must have concrete fixtures and expected outcomes, including denied cross-tenant requests, lost acknowledgements, duplicate operations, secret handling, backup restore and rollback.
- Every critical/high gap relevant to the release slice has evidence-backed closure or explicit scoped exclusion approved by its owner.
- Exact revision, manifest, dependency graph and a traceable human approval record.

## Distinct verdicts
- Documentation structural checks can pass without implementation readiness.
- Implementation handoff remains BLOCKED until above requirements are approved and verified.
- Production readiness cannot pass without build-specific executed tests, deployment evidence, restore/rollback drills and separate release authorization.

## Future AI-agent boundaries
- Credentials via scoped secret mechanism, never in specifications.
- Agent may propose changes and run tests on disposable infrastructure within approved limits.
- Production destructive operations and final promotion require an independent human release authorization.
- No implicit authorization from a documentation approval.

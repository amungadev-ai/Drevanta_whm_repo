# Evidence-driven closure workplan

1. Establish source-of-truth hierarchy: approved decisions, source PRD clauses, prototype behavior, proposed contracts. Resolve contradictions without rewriting history.
2. Select and approve a minimal end-to-end implementation slice; record deferred feature IDs.
3. Execute isolated browser audit of each in-scope control with exact prototype digest, state, event, role, expected result and evidence.
4. Approve trust boundaries, host agent protocol, tenant permissions, recovery, credential custody and threat-model mitigations.
5. Approve backend topology, storage engine, migration/restore, durable jobs and API conventions.
6. Freeze exact supported host tuple and hosting adapter matrix; reject unsupported targets before side effects.
7. Derive executable API schema, migration, permission, host operation and fixture artifacts from the approved slice.
8. Re-evaluate all 744 tests: close only applicable inputs with actual approved values; mark deferred tests explicitly rather than fabricating defaults.
9. Run documentation consistency and provenance validation, then issue a separate implementation-handoff verdict.

## Required evidence per decision
ADR ID; owner; explicit choice; rationale and rejected alternatives; affected feature IDs and phases; approved schema/policy values; test IDs; source links; approval date; version.

## Remaining open source gaps
The v3.0 gap register contains 18 open gaps; this v3.1 review packet does not claim they have been resolved.

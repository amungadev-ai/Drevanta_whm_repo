# v3.0 agent execution and release gates

Required verdict vocabulary is PASS, FAIL, BLOCKED, NOT_RUN and NOT_APPLICABLE. Evaluate in order and fail closed. A missing field, unknown verdict, stale approval, contradictory source or unverifiable evidence prevents promotion. NOT_APPLICABLE requires a reviewer-signed scope rationale. Revoked authority invalidates the action gate even if evidence remains valid for review.

| Gate | Entry conditions | Pass condition |
| --- | --- | --- |
| D0 Documentation | Sealed inputs and phase roadmap | Every expected item has a disposition. Structural references, provenance, checksums and schemas validate. Semantic exceptions remain listed. |
| I0 Handoff | D0 and recorded scope | Exact committed feature IDs, owners, applicable test inventory, accepted ADR revisions, named authority and approved environment. Zero critical/high blockers for that slice. |
| C1 Pull request | I0 and exact authorized branch scope | Reviewed changes, locked dependencies, static analysis, secret scan, docs trace and applicable unit/contract/authorization checks. |
| C2 Build | C1 protected reviewed commit | Immutable artifact, provenance, toolchain and dependency inventory; attest candidate digest and check signatures. |
| C3 Integration | C2 and isolated fixture revision | Exact candidate passes relevant migration, adversarial, fault, browser, API, isolation and data integrity cases. |
| C4 Authorized disposable target | C3, allowlisted environment and compatibility tuple | Install/reboot/upgrade, least privilege, network rollback, backup/key recovery and scoped workloads verified. |
| C5 Staging | Same digest and approved policy thresholds | Verify smoke, schema, data, HTTPS, recovery and rollback under target-like conditions. Measure RPO/RTO. |
| R1 Production authorization | Exact release digest, allowed actions, target and validity window | Independent approval verified against exact commit, digest, target, environment, window and rollback plan. |
| R2 Release and recovery | R1 live and C5 current | Run approved rollout only. Observe exact policy thresholds. Stop on breach, apply approved rollback or declare incident, verify data and service after recovery. |

## Agent task record

Mandatory inputs: task_id, requirement_ids, contract_revision, decision_refs, tests, repository, branch, exact commit or expected base, allowed environment and host IDs, allowed verbs, expiry, reviewer, credential references, baseline probes, rollback steps, and evidence destination. Verify allowed scope before each protected action. Recheck digest, scope and approval when inputs drift. Use short-lived environment-scoped credentials from authorized custody. Reject raw passwords, keys, reset links and authorization headers in evidence. Use independent reviewer for protected merge and release. No authorization can be inferred from connectivity.

## Release evidence and failure handling

Record exact build digest, source commit, environment tuple, contract revision, approved policy digest, test and fixture IDs, expected and actual signals, UTC duration, verdict, redacted evidence hash/URI and approver. For restore, record recovered data timestamp, verified service time, source/target manifest and independent key access. For rollback, record previous and failed artifact/config/schema versions, approved rollback trigger, observed deployed digest and post-rollback smoke/data/API checks. Incompatible data rollback follows isolated restore and reviewed cutover. Stop queued promotion after breach. Never declare an ambiguous host or provider effect a verified rollback. Record exception ID, scope, impact, unknowns and authorized next action.

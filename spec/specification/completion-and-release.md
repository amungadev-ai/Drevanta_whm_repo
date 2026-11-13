# Completion and release verification

Status: proposed governance, incorporating the source evidence rules. This is the checklist a future engineering task uses after approval. Blocked, not run, failed, and not applicable are distinct states. A not-applicable row requires a scoped human rationale and exact candidate binding.

| Level | Required conditions for PASS | Required evidence | Failure disposition |
| --- | --- | --- | --- |
| Feature | Approved scope and requirement revision; exact contract for each applicable UI/API/data/permission/job/helper surface; committed code bindings; all applicable positive, denial, invalid input, concurrency, failure, retry/rollback and recovery tests pass | Review ID, source refs, exact commit and candidate digest, test IDs and logs, before/after assertions | Block feature enablement; record failed or missing predicate and owner |
| Module | All committed features pass; installation, upgrade and disable tested; schema compatibility, quotas, permission isolation, module failure containment and rollback exercised; zero unresolved critical/high blocker on affected slice | Immutable artifact, SBOM, compatibility matrix, module integration/security evidence, restore and disable results | Withhold module; if core depends on it, also block platform candidate |
| Phase | Proposed contracts and confirmed decisions clearly identified; inbound/outbound dependencies reconciled; no orphan committed requirement or applicable gate; phase-specific runtime evidence attached when claiming engineering completion | Phase review, source crosswalk, decision closure or explicit scoped deferral, exact execution evidence | Documentation may advance with open evidence. Engineering phase does not pass |
| Platform | Every committed module and cross-module journey passes on every supported tuple; protected branches and evidence provenance verified; backup/key restore, observability, release health and rollback tested; exact-candidate human production approval recorded | Release certificate, signed artifact, environment/host inventory, verified restore data, change window, rollback triggers and named approvers | No certification or rollout. On failed release observation, apply approved rollback or declare incident |

## Future CI/CD sequence

1. Pull request gate checks required human review, scoped work item, locked dependencies, formatting, static analysis, schema and docs/trace consistency. Run unit, contract and authorization tests with synthetic fixtures. Detect secrets before accepting evidence output. Reject live credentials in PR artifacts.
2. Build a candidate once from the protected reviewed commit. Capture commit, build identity, artifact hashes, dependency inventory, toolchain versions and provenance. Verify candidate signature and provenance at each later stage. Report vulnerabilities by severity, impacted scope and reviewed dispositions.
3. Isolated integration runs apply migrations to seeded prior versions, test tenant isolation, API compatibility, idempotency and job fault injection, and run UI browser keyboard/state checks. Use explicit leases and cleanup markers. Prove cleanup of test-owned resources and preserve evidence.
4. Disposable VPS verification runs exact supported tuples on an authorized host allowlist and records install, operating privileges, agent allowlist, networking rollback, upgrade/reboot, workload smoke, backup/key restore and pressure tests. Bind to named image digest and inspected host tuple.
5. Staging promotion uses the same immutable digest. Rehearse recovery with approved backup, restore into isolation and compare application records and known health probes. Measure RPO/RTO under the approved policy. Validate rollback compatibility before rollout.
6. Protected merge and production deployment require the applicable human-controlled authorization for exact repo, branch, commit/artifact, environment, target, operations and validity window. Approval drift or missing threshold blocks execution. Release stages report healthy, halted, rolled back or incident with proof.
7. During release, compare approved health signals and thresholds over approved observation intervals. If threshold is violated, stop promotion and use approved rollback or incident plan. Replay smoke, schema/data checks and external health probes after rollback. Record if recovery requires restoration to an isolated copy before cutover.

## Evidence and credential custody

Evidence rows require test ID, acceptance ID, requirement or gate ID, commit, artifact digest, environment/image/OS tuple, fixture/procedure revision, start/end UTC, expected, actual, verdict, redacted artifact URI and digest, reviewer, and exception reference if any. Record policy constants used in the test. Release evidence is immutable to the agent after review.

Use separate scoped CI, staging and release identities. Supply credentials through the approved secret provider, by reference only, scoped to operation/environment with limited lifetime. No tokens, private keys, passwords, reset links, MFA material, or raw environment dumps in Git, prompts, structured output or evidence. Review redaction with synthetic canaries. Restrict evidence reads by scope. Revoke and rotate if exposed and invalidate related evidence or approvals as appropriate.

## Release certificate checks

The release reviewer verifies every committed scope ID maps to exact implementation and test evidence; applicable test counts equal the approved planned counts; no required verdict is missing, failed or stale; zero unresolved release-blocking critical/high findings affect the candidate; compatibility and operational thresholds are approved; recovery and rollback evidence match the candidate; and live authorization is current. Certificate fields include candidate digest, source revision, scope matrix revision, gate verdicts and evidence digests, review identity, timestamp, and bounded authorization. A certificate cannot grant new operational privileges.


Use the [v3.0 staged gate profile](execution-and-release-gates.md) for exact entry and exit evidence.

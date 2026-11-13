# Shared engineering contracts

All contracts in this document are proposed. An implementation task requires recorded authorization, closed blocking decisions and an approved contract revision.

## Operation workflow

1. Authenticate principal and resolve installation, owner and resource from server-controlled data.
2. Authorize the action and target, including capability installation, scope, role, revision and requested risk. Redact the target from foreign-scope results.
3. Resolve valid idempotent replays under the same scope. For new operations validate schema, preconditions, limits, reservations and expected generation.
4. Commit intent, desired generation, operation and outbox atomically. Return operation reference and audit correlation.
5. Worker fences the operation, revalidates authorization and ownership, and dispatches only the typed action. The adapter reports observed state and failure class.
6. Close on verified postcondition. On uncertainty, reconcile; on confirmed failure, follow defined compensation or record operator action. Never compensate into another owner’s resource.
7. Project scoped UI progress, audit and any notifications. Preserve terminal outcome and artifact references for the approved retention period.

## Logical interface envelope

| Contract | Required fields and behavior | Owner |
| --- | --- | --- |
| Resource reference | installation_id, type, opaque id, owner scope, revision, desired and observed generations | [Phase 04](phases/04.md) |
| Permission check | principal_id, action ID, resource reference, role or grant edge, current policy revision and any required step-up or approval | [Phase 18](phases/18.md) |
| Mutation request | action-specific arguments, stable resource reference, expected generation if applicable, idempotency key for async operations | [Phase 23](phases/23.md) |
| Operation status | operation_id, target scope, state, current step, safe reason, correlation_id, created/updated UTC timestamps and generation | [Phase 22](phases/22.md) |
| Host operation | typed action/version, installation/host/resource IDs, owner revision, deadline, fence, operation_id, typed arguments | [Phase 07](phases/07.md) |
| Observation | source, scope, target, generation, time, outcome, normalized reason and evidence reference | [Phase 21](phases/21.md) |
| Release evidence | test_id, requirement/acceptance IDs, commit, build digest, environment tuple, expected, actual, verdict, UTC time, reviewer and evidence digest | [Phase 24](phases/24.md) |

## Failure and concurrency vocabulary

- **Invalid** means reject before dispatch. Record explicit validation without reporting a host effect.
- **Conflict** means a submitted intent is inconsistent with current resource state, idempotency binding, quota or capability readiness.
- **Transient** means eligible for bounded retry under the appropriate policy and lease. Uncertain external outcomes must first be observed.
- **Uncertain** means call timed out, reply was lost or state is incomplete. Record observed evidence, keep fencing, and reconcile before reapplying.
- **Denied** means zero accepted effects and a scoped response. Resource existence is hidden where authorization requires.
- **Degraded** means a known expected postcondition failed. **Unknown** means observations cannot establish current health.
- Test fixtures pin policy and timeout values even when approved production values are open. Report fixture values in evidence.

## Permission and effect binding

The capability register binds each source ID to a proposed action, object, acceptance and test. For each approved action, the future route/schema review must specify concrete argument fields and validation, allowed grants, resource owner derivation, idempotency namespace, transactional writes and adapter behavior. Scope applies independently to reads, state changes, audit, access to test artifacts, worker claims and host dispatch. Confirm account lifecycle, resource policy, actual thresholds and error schemas in the relevant phase before use.

## Documentation and future execution gates

| Gate | Evidence | Outcome in this package |
| --- | --- | --- |
| Per-phase structural check | Canonical phase ID, earlier dependencies, criteria, test and gate IDs, references and explicit open decisions | Passed |
| Final bundle integrity | Archive read and SHA-256 comparison for payload entries | See validation report and manifests |
| Inventory sign-off | Atomic PRD dispositions, control reachability and approved scope matrix | Open |
| Implementation handoff | Approved scope, architecture, permission, environment and detailed contract closure | Open |
| Runtime verification and release | Evidence and authorization for exact release candidate | Pending |

## Audit precedence

Approved product direction is preserved by explicit source citation. Proposed contracts require decision closure. The control plane owns human authorization; a privileged helper enforces independent host policy. A compromised allowed peer can exercise the scope available to that peer, and OS root is outside application tenant isolation. Rollback is a distinct deployment or recovery operation bound to an approved compatibility and data plan. See [completion criteria](completion-and-release.md).


## v3.0 closure profile

See the [candidate wire and storage profile](contracts/profile.md). Source authority and engineering proposals retain separate status.

# Drevanta v3.0 candidate specification contracts

Profile ID DREVANTA-CONTRACT-3.0. All wire and storage definitions below are proposed design inputs. Approved source facts remain in the source decision register. Profile adoption and compatibility promises require the listed decisions. Contract values prefixed with fixture are synthetic test values.

## API envelope · DEC-004, DEC-009, DEC-012

Candidate prefix `/api/v1`. JSON request bodies use UTF-8. Accept mutation intent only with an authenticated principal, a current scoped grant, and CSRF protection for browser-cookie calls. Resolve target ownership server-side. The recommended order is authenticate → visibility/permission → idempotency replay check → body/precondition → atomic desired state/job/outbox → 202. Replay on the same actor and capability returns the original result only if readable under current policy. Changed canonical intent under the same key yields 409; concealed objects yield 404 without resource identifiers. Use 401 for absent/invalid auth, 403 for denied action on a visible resource, 412 for stale precondition, 422 for invalid payload, and 503 for unavailable dependencies before acceptance. Keep HTTP 409 for conflicting intent, state or unavailable feature flag. The audit catalog retains every historic method/path without committing it for publication.

| Candidate route | Request | Success | Authorization |
| --- | --- | --- | --- |
| GET /api/v1/me | no body | 200 principal_id and memberships the subject can read | authenticated self |
| GET /api/v1/workspaces | optional opaque cursor | 200 items and next_cursor or null | active scoped membership for each item |
| GET /api/v1/operations/{operation_id} | path opaque ID | 200 OperationStatus | owner scope plus operation.read |
| POST /api/v1/resources/{resource_id}/intents | MutationIntent plus Idempotency-Key | 202 OperationAccepted | explicit action grant on resource; additional approval if required |

Routes are proposed. The generic mutation route is a design option for first-slice orchestration only. Concrete action schemas, unsafe combinations, and permission rules must freeze before exposure. Until then no generic execute behavior is permitted. Reject unknown request properties. List cursor integrity binds actor, tenant, filters, ordering and cutoff. No credential or protected data in error detail, cursor or operation ID.

## Persistence · DEC-002, DEC-004, DEC-009, DEC-011

Opaque primary IDs and UTC timestamps. Choose engine-specific migrations after ADR closure. This is a relational contract, with columns in the storage register. Scope constraints use authoritative owner references rather than caller-supplied tenant. Enforce composite owner/target lookups and generation checks transactionally. Idempotency unique namespace is actor + scope + action + key. Persist the canonical intent digest before scheduling, alongside desired generation, operation, and outbox in one transaction. Side effects, human grants, and provider events are observed and journaled across boundaries rather than implied transactionally atomic. Never retry if outcome is uncertain and no safe observation or provider deduplication exists.

## Authorization and tenant isolation · DEC-003, DEC-004

A persona is a workspace projection. Grants require a valid principal, active membership or approved support lease, explicit action, matching resource ownership and current revision. Decisions, dispatches, streaming frames, artifact dereferences, jobs and callbacks each enforce scope independently. Role catalogue and owner actions are reviewed per committed capability. Platform administration is distinct from ordinary payload access. Local OS root has technical access and is outside the application isolation claim. Stale ownership or policy revision blocks dispatch and commit. Helper trusts authenticated peer scope plus root-owned policy and permitted target only. Human membership is enforced on the API and dispatch side.

## VPS protocol · DEC-003, DEC-005, DEC-009

Candidate v1 is a local restricted socket. No inbound remote exposure by default in this proposal. Match peer against approved policy before decoding a bounded message, then validate version, host/installation, verb, typed arguments, owner revision, expiry, fence and canonical digest. Reject unknown verbs or extra fields before dispatch. Host-local registry config and journals must be readable/writable only by authorized host principals. Each verb requires a typed argument schema, declared effects, timeout, required privilege, target proof and postcondition. Replay same operation and digest may return prior recorded result or observe. Reused ID with different digest rejects. Reserve/fence on the host before side effects and compare-and-set terminal state in control plane. If an effect cannot be safely fenced or observed, stop automated retry and record uncertain state and recovery instructions.

Outcome taxonomy: rejected before effects; accepted with no terminal evidence; verified success with observed generation; verified failure with known postcondition; uncertain with explicitly unknown effects. API and worker journals correlate via operation_id. Never pass arbitrary process arguments, path strings, unit names or shell text to a generic privileged endpoint. Adapter resolves stable target IDs using root-owned bindings.

## Changes and compatibility

Revision v3.0 changes documentation only. Contract owners must register proposed → approved → implemented → verified states separately. Any incompatible schema, role, command or behavior change updates affected tests and decision owners. Historic milestone numbers and S0–S4 remain provenance. Decline unsupported targets before dispatch.

## Reference fixtures for contract reviewers

These values are test data. They define how to verify the candidate envelopes independently of an implementation. They do not configure a server or publish a route.

| Scenario | Exact fixture stimulus | Required result under the candidate |
| --- | --- | --- |
| Unknown mutation field | Valid intent with extra top-level `run_shell` field | 422 invalid_input; operation/outbox/host dispatch counts unchanged |
| Stale desired generation | Resource at generation 8; intent says 7; fresh key | 412 stale_precondition; generation stays 8; dispatch count unchanged |
| Hidden resource | Actor A uses valid B resource ID in path | 404 not_found; body carries correlation only and approved safe detail; no B identifier |
| Stable replay | Actor A uses the same key and same canonical intent before and after accepted response is lost | Both accepted responses identify the same operation; one accepted intent and one persisted outbox event |
| Altered replay | Same actor, scope, action and key, but changed arguments | 409 conflict; existing operation remains unchanged; no additional intent |
| Revoked replay | Revoke actor A grant before replay of previous key | Denial under current visibility; no operation ID returned |
| Wrong host | Send host request with host fixture-host-b to fixture-host-a | Rejected before effects; no target mutation; safe reason correlated |
| Stale fence | Host has accepted fence 2; worker submits fence 1 | Rejected before effects and finalization; effect counter unchanged |
| Lost host reply | Host completes accepted effect and response disappears; worker repeats same ID/digest | One logical effect; journal returns prior observation or reconciling state |
| Changed host digest | Same operation ID and different canonical request digest | Rejected before dispatch; previous journal entry retained |

Candidate HTTP intent fields include exact generation and owner revision. Define canonical serialization during DEC-009/012, including key order, string normalization and number handling. Until frozen, a wire digest interoperability test remains blocked. Trusted network transport does not substitute for authorization. Endpoint-specific responses, request bounds and side effects must still be mapped before exposing the endpoint.

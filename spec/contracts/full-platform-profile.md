# Full-platform contract profile v4.0 · proposed

This profile extends the preserved v3.0 envelopes. Domain contracts name governed records, candidate actions, security invariants and fault outcomes. Candidate route catalog retains source method/path without enabling it. The domain and source indexes must be reconciled to exact public schemas and approved grants before implementation. Where a row identifies missing input, fail closed.

## Global API and state

Use one versioned API and point historic aliases to an explicit publish/defer/replace decision. Authenticate, authorize owner visibility and current revision, then validate exact schema and safe generation. For a replay, reauthorize before returning an existing result. Canonical intent digest is over a versioned operation and canonical payload. Namespace key by actor, tenant and action. Reuse same key/body returns same operation, different body conflicts. Commit intent, state and outbox in one transaction. 401 invalid principal, 403 visible denied, 404 concealed/nonexistent, 409 conflicting intent/state, 412 stale generation, 422 invalid fields, 503 dependency unavailable before acceptance. Candidate async acceptance uses 202, operation ID and safe correlation ID. Test schemas reject extra fields. No credentials in URLs, errors, logs or evidence.

## Data and ownership

Anchor installations, hosts, workspaces, owners, subjects and typed resources using opaque IDs. Cross-object references constrain both ID and owning scope. Reserve scarce capacity and names transactionally; proof of authenticated owner precedes effect. Freeze engine ADR, concrete migrations and column-level constraints before handoff. Store revisions and current desired/observed generations. Operation attempts have distinct monotonic fences, durable checkpoints and terminal compare-and-set. Maintain append-only application audit and bounded outbox projections. Secret references resolve through scoped custody; test key rotation and independent backup recovery.

## Permission and privileged effects

Effective allow requires active principal, explicit target action and relationship/grant, matching resource scope and current revision, plus approval or step-up where applicable. Role or persona names and historic registry codes do not grant authority. Apply these predicates independently to UI projections, API, dispatch, streams, artifact retrieval, provider callbacks and host bindings. Revoked grants and transferred ownership block delayed effects. Root control is outside application isolation; a compromised allowed process can invoke its full allowed helper policy.

Local restricted socket is the proposed initial transport. Each allowlisted action needs independent peer, host and target proof, typed arguments, expiry, fencing, audit and observable postcondition. No arbitrary shell or generic privileged command. Same operation/digest can return journaled observation; changed digest or stale fence rejects. Unknown effect with no safe observation blocks retry. Multi-node/remote transport, optional adapter allowlists and engine enforcement require scoped ADRs.

## Deployment and recovery

Represent source commit, verified artifact, immutable digest, target, config revision, approval, previous version and compatibility. Deployment and rollback each create an operation bound to an approved target generation. Restore is a separate staged operation: verify digest/key, restore into isolation, check authorized inventory/application state, then exact authorized cutover. Capture prior version, policy window and after-state recovery. Host-lockout rollback must be independent of panel and existing management connection. No delivery, backup, recovery or rollout status without supporting observation.

## What still needs to freeze

Cross-check every endpoint payload and status, storage column/ownership constraint, role/verb/target grant, named supported tuple, key/issuer/provider configuration, environment threshold and per-action fixture. The attached source contains illustrative or conflicting examples. The registers preserve those facts and require approval before enabling any ambiguous operation.

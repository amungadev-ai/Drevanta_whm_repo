# Drevanta v3.1 · Human approval dossier

All 19 decisions await either a recorded human choice or evidence attestation. This dossier proposes choices and preserves the approved unified-console direction. Baseline contract paths and selectors resolve against the sealed v3.0 ZIP cited in the provenance record.

## Review order

1. **Scope and evidence** · DEC-016, DEC-017. Name the slice and resolve its source evidence first.

2. **Trust and authority** · DEC-003, DEC-004, DEC-008, DEC-011, DEC-019. Review together with the service, OS, mail and durability decisions where coupled.

3. **Architecture and interfaces** · DEC-001, DEC-002, DEC-009, DEC-012. Freeze the coupled persistence, job and replay semantics together.

4. **Host, recovery and optional features** · DEC-005, DEC-006, DEC-007, DEC-010, DEC-014, DEC-015. Document exclusions for the selected slice.

5. **Policy and release** · DEC-013, DEC-018. Use measured evidence. Final production authorization is separate.

Review batches are agendas. Dependencies below name design inputs or coupled review items, so reciprocal references are expected. The highlighted tests target the decision directly or serve as representative regressions. The CSV crosswalk separately lists every test that already declares the decision as an input.

### DEC-016 · Capability commitment and source release crosswalk

**Critical · Product owner**. Pending Human Decision.

**Recommendation.** Approve one end-to-end slice with explicit committed IDs and deferred items; keep S0–S4 as source codes. **Alternative.** Approve a broad initial feature set only with additional dependencies and validation capacity.

**Rationale.** Explicit scope prevents registry staging codes or prototype visibility from committing product behavior. **Security.** Uncommitted actions remain unavailable; scope changes re-open impacted authorization and threat review.

**Operational effect and closure.** Approve a candidate core journey from install and claim to sign-in, scoped observation, and one safe reconciled mutation. Name exact feature IDs, exclusions and release owner. Unified console is approved product direction (P1 OD-01 and ADR-020). S0–S4 are source labels.

**Dependencies.** No upstream decision dependency in this review.

**Contract bindings.** contracts/release-gates.json#I0; registers/capability-traceability.csv; registers/requirements.csv. **Highlighted tests.** TC-P02-02,TC-P24-01,TC-OPS-01. The baseline declares 455 case links.

### DEC-017 · Atomic source and runtime UI reconciliation

**High · Documentation and UX leads**. Pending Evidence Attestation.

**Recommendation.** Review atomic text and exercise the existing static prototype in isolation when authorized. Collect versioned state/control evidence. **Alternative.** Static-only intake is reproducible and leaves reachability, handlers and hidden controls unresolved.

**Rationale.** Atomic source reconciliation and rendered evidence answer different questions; preserve source anchors while testing conditions and transitions. **Security.** Static visibility cannot establish permission. Use isolated synthetic browser evidence and record denied paths.

**Operational effect and closure.** Classify each in-scope clause and control; attach exact source/browser revision, actor, selector, state, action and outcome. Escalate actual product conflicts only. Evidence workflow and attestation, with product approval only for conflicting interpretations.

**Dependencies.** No upstream decision dependency in this review.

**Contract bindings.** registers/prd-id-statements.csv; registers/ui-controls.csv; registers/prototype-pages.csv. **Highlighted tests.** TC-P01-01,TC-P01-02,TC-P19-01. The baseline declares 354 case links.

### DEC-003 · VPS agent privileged execution protocol

**Critical · Host and security owners**. Pending Human Decision.

**Recommendation.** Start with a local authenticated Unix socket, root-owned typed registry and host journal. **Alternative.** Remote agents require mutual authentication, rotation, reachability and a compromised-control-plane model.

**Rationale.** A narrow helper protocol keeps general web parsing and workflow state outside the privileged boundary. **Security.** Peer checks and root-owned allowlists constrain allowed verbs. A compromised allowed peer can invoke its whole allowlist; host root remains outside tenant isolation.

**Operational effect and closure.** Freeze per-verb schemas, privilege, stable target proof, fencing, journal outcomes and recovery. Disable remote agent and unobservable automatic retry unless approved. P1 §§35–36 assigns human authorization to API/dispatcher and host enforcement to helper.

**Dependencies.** DEC-001,DEC-004,DEC-005,DEC-009,DEC-011

**Contract bindings.** contracts/envelopes.schema.json#HostRequest; contracts/envelopes.schema.json#HostResult; registers/vps-operations.csv. **Highlighted tests.** TC-P07-01,TC-AUD-05,TC-P09-01,TC-P22-01. The baseline declares 140 case links.

### DEC-004 · Tenant ownership and authorization semantics

**Critical · Product and security owners**. Pending Human Decision.

**Recommendation.** Use explicit per-workspace membership, separate ownership edges and default deny at HTTP, jobs and artifacts. **Alternative.** Broader inherited delegation reduces admin effort but enlarges blast radius and needs explicit relationship semantics.

**Rationale.** Authoritative owner edges and current grants let scope be recomputed at every access and effect boundary. **Security.** Deny unknown or foreign scope, stale grants and unauthorized artifacts. Keep support leases explicit, time-limited and auditable.

**Operational effect and closure.** Approve role/action/resource tuples, transfer revision, revocation during work and streams, last-owner rule and support policy. Source persona codes and workspace switching do not grant authority.

**Dependencies.** DEC-016

**Contract bindings.** registers/permission-matrix.csv; registers/storage-schema.csv#membership,grant,resource,support_lease; contracts/envelopes.schema.json#MutationIntent. **Highlighted tests.** TC-P18-01,TC-CORE-02-SCOPE,TC-CORE-03-SCOPE,TC-INT-01-SCOPE. The baseline declares 340 case links.

### DEC-008 · Recovery through SSH/console

**Critical · Identity and security owners**. Pending Human Decision.

**Recommendation.** Review an installation-local, purpose-bound, one-use recovery grant that preserves MFA. **Alternative.** Email-only recovery reduces host privilege exposure but leaves users without recovery when delivery is unavailable.

**Rationale.** Single-use local recovery covers delivery failure while preserving normal authentication boundaries. **Security.** Bind proof, purpose, installation and expiry; consume atomically; revoke old sessions and preserve enrolled MFA. Avoid exposing raw recovery material.

**Operational effect and closure.** Approve local proof and console flow, no-bypass semantics, recovery alternatives, TTL, audit and safe denial. Prototype recovery is simulated; source requires no MFA bypass.

**Dependencies.** DEC-003,DEC-004,DEC-007,DEC-011,DEC-018

**Contract bindings.** registers/storage-schema.csv#challenge,session; contracts/profile.md#Authorization and tenant isolation. **Highlighted tests.** TC-P06-01,TC-P17-01,TC-P17-02,TC-CORE-01. The baseline declares 41 case links.

### DEC-011 · Secrets and credential storage

**Critical · Security owner**. Pending Human Decision.

**Recommendation.** Keep only secret references in ordinary data. Use scoped broker or envelope encryption with separate key custody. **Alternative.** A fully external vault improves separation and adds availability and bootstrap dependencies.

**Rationale.** Stable secret references support rotation and restrict plaintext distribution across panel, helper and workers. **Security.** Separate custody, scope provider access, redact logs and preserve recovery when key infrastructure is unavailable.

**Operational effect and closure.** Choose custody per credential class; define rotation, revoke, backup key recovery, incident handling and operator access. Credential decisions apply to future execution only.

**Dependencies.** DEC-001,DEC-004

**Contract bindings.** registers/storage-schema.csv#secret_ref; contracts/envelopes.schema.json#Evidence; execution-and-release-gates.md. **Highlighted tests.** TC-P05-01,TC-P13-01,TC-P15-01,TC-P21-01. The baseline declares 149 case links.

### DEC-019 · Future autonomous agent authority

**Critical · Repository, infrastructure and release owners**. Pending Human Decision.

**Recommendation.** Grant future tasks narrow repo/branch and disposable target scope using time-bounded credentials. Separate release approver. **Alternative.** Broader standing access increases potential impact, review burden and recovery exposure.

**Rationale.** Per-task authority binds access to approved repo, branch, environment, target and operation. **Security.** Time-bound separate credentials and independent release approval limit unauthorized effects. Documentation approval grants no infrastructure access.

**Operational effect and closure.** Name authorized scopes, expiry, permitted operations, protected merges, approvers, credential references and revocation. No repository or VPS authority was provided for this review.

**Dependencies.** DEC-016,DEC-013

**Contract bindings.** execution-and-release-gates.md#Agent task record; contracts/release-gates.json#I0,R1. **Highlighted tests.** TC-P25-01,TC-AUD-25,TC-P24-01. The baseline declares 3 case links.

### DEC-001 · Backend language and service architecture

**High · Architecture board**. Pending Human Decision.

**Recommendation.** Keep an unprivileged Go modular monolith with static UI and a separately constrained local helper for the first slice. **Alternative.** Split service deployment adds network identity, operational overhead and independent scaling.

**Rationale.** Modular single process reduces deployment coordination while retaining a distinct trust boundary for host execution. **Security.** Keep the web/API process unprivileged; isolate untrusted builds and credential access from helper and control store.

**Operational effect and closure.** Freeze process placement, IPC, static UI packaging, resource limits and failure domains. Historic Go/static UI is an inherited direction, not v3.1 adoption.

**Dependencies.** DEC-016

**Contract bindings.** contracts/profile.md#VPS protocol; shared-contracts.md; phases/03.md. **Highlighted tests.** TC-P03-01,TC-P03-02,TC-P07-01. The baseline declares 3 case links.

### DEC-002 · Database engine and migration strategy

**High · Data architecture owner**. Pending Human Decision.

**Recommendation.** Review SQLite WAL for single-host metadata with transactional outbox and explicit migration backup. **Alternative.** External database offers a different scaling/availability model and adds provisioning and recovery dependencies.

**Rationale.** A local transactional store fits single-host intent/outbox atomicity. Verify concurrency and durability before promising limits. **Security.** Ownership relations and secret references must be enforced transactionally; migration backups need separate custody and restore proof.

**Operational effect and closure.** Freeze engine, schema constraints/indexes, durability settings, migration order and recovery from interrupted upgrade. SQLite WAL remains a candidate.

**Dependencies.** DEC-001,DEC-004,DEC-009,DEC-010

**Contract bindings.** registers/storage-schema.csv; contracts/profile.md#Persistence. **Highlighted tests.** TC-P04-01,TC-P04-02,TC-P14-01,TC-P22-01. The baseline declares 6 case links.

### DEC-009 · Jobs, idempotency and reconciliation

**High · Orchestration owner**. Pending Human Decision.

**Recommendation.** Durable transactional jobs and outbox with leases, monotonically fenced effects and observation before replay. **Alternative.** External broker supplies queue tooling but does not by itself solve durable intent or host deduplication.

**Rationale.** Durable intent and transactional outbox avoid losing accepted work. Lease and effect fencing solve separate stale-worker risks. **Security.** Recheck grants before dispatch; ambiguity blocks unsafe replay. A queue cannot guarantee exactly-once external effects.

**Operational effect and closure.** Freeze operation states, canonical intent, retry and lease limits, journal interaction, cancellation, observation and terminal CAS. DB-engine choice and job semantics must be co-designed.

**Dependencies.** DEC-003,DEC-004

**Contract bindings.** contracts/envelopes.schema.json#OperationStatus,HostRequest,HostResult; registers/storage-schema.csv#operation,operation_attempt,idempotency,outbox. **Highlighted tests.** TC-P03-01,TC-P07-01,TC-P22-01,TC-CORE-04. The baseline declares 9 case links.

### DEC-012 · API versioning and error conventions

**High · API owner**. Pending Human Decision.

**Recommendation.** Use a versioned namespace with one canonical schema and stable errors; preserve source routes as candidates. **Alternative.** Publishing historic aliases increases compatibility, authorization and deprecation burden.

**Rationale.** A canonical versioned interface prevents historic path aliases from acquiring unreviewed behavior. **Security.** Check visibility before replay lookup; bind pagination and idempotency to scope. Protect browser mutations and validate callbacks independently.

**Operational effect and closure.** Approve each selected method/path and request/response/error fixture, compatibility window and disabled route behavior. Historic route catalog is candidate evidence, not publication.

**Dependencies.** DEC-004,DEC-009,DEC-016

**Contract bindings.** contracts/envelopes.schema.json#Error,MutationIntent,OperationAccepted,OperationStatus; registers/source-api-catalog.csv. **Highlighted tests.** TC-P23-01,TC-P23-02,TC-AUD-23. The baseline declares 187 case links.

### DEC-005 · Supported Linux distributions and versions

**High · Host engineering owner**. Pending Human Decision.

**Recommendation.** Pick one verified apt/systemd host tuple per initial support tier after install, upgrade and restore proof. **Alternative.** A broad Ubuntu/Debian claim increases ongoing matrix and support cost.

**Rationale.** One exact qualified host tuple makes install/upgrade claims reproducible and narrows adapter variation. **Security.** Unsupported systems fail preflight before mutation. Package rollback and host recovery require distinct evidence.

**Operational effect and closure.** Name distro release, architecture, image/kernel/init/package backend and verified install/upgrade/restore paths. Ubuntu/Debian apt/systemd are candidate direction, not an approved matrix.

**Dependencies.** DEC-003,DEC-013

**Contract bindings.** phases/08.md; registers/vps-operations.csv; contracts/release-gates.json#C4. **Highlighted tests.** TC-P06-02,TC-P08-01,TC-P08-02,TC-P09-01. The baseline declares 27 case links.

### DEC-006 · Web server and runtime support matrix

**High · Web/runtime owner**. Pending Human Decision.

**Recommendation.** Start with a reviewed static website profile and only separately approved runtime adapters. **Alternative.** Adding PHP, Node, Python and OCI early increases isolation, compatibility and update burden.

**Rationale.** Static hosting limits early runtime/supervisor dependencies; adapter commitments can grow from measured compatibility. **Security.** Protect unit identities, per-site principals and filesystem roots. Optional CLI, OCI and terminals need explicit privilege review.

**Operational effect and closure.** Publish feature/version/host eligibility and unavailable behavior; verify isolated workload restart and safe rollback. Candidate static scope needs DEC-016.

**Dependencies.** DEC-003,DEC-005,DEC-016

**Contract bindings.** phases/10.md; phases/11.md; registers/vps-operations.csv; registers/capability-traceability.csv. **Highlighted tests.** TC-P11-01,TC-P11-02,TC-WEB-01,TC-APP-08. The baseline declares 133 case links.

### DEC-007 · Mail transport and optional mailbox hosting

**High · Integrations owner**. Pending Human Decision.

**Recommendation.** Use transactional SMTP for recovery. Defer mailbox hosting pending separate service commitment. **Alternative.** Operating mailboxes adds abuse, delivery and data retention duties.

**Rationale.** SMTP isolates transactional recovery from mail server operations; acceptance and actual delivery differ. **Security.** Protect reset material and avoid account enumeration. Reconcile ambiguous send outcomes before retry.

**Operational effect and closure.** Name adapter, transport/security policy, delivery observability, retries and optional mailbox deferral. Delivery is unknown without trusted provider evidence.

**Dependencies.** DEC-008,DEC-011,DEC-016

**Contract bindings.** phases/16.md; phases/17.md; registers/storage-schema.csv#secret_ref,challenge. **Highlighted tests.** TC-P16-01,TC-P16-02,TC-MAIL-01,TC-MAIL-02. The baseline declares 9 case links.

### DEC-010 · Backup encryption, retention and restore guarantees

**Critical · Reliability and security owners**. Pending Human Decision.

**Recommendation.** Require encrypted independently restorable backups, plus an off-host recoverable copy for disaster recovery where committed. **Alternative.** Local-only snapshots ease operation but cannot survive complete VPS loss.

**Rationale.** Independent recovery requires backup integrity plus access to keys and a usable application-level restore. **Security.** Encrypt and scope backup retrieval and cutover; keep destructive recovery separate from verification. Local copies cannot cover host loss.

**Operational effect and closure.** Approve consistency, key and retention policy, separate recovery location if in scope, measured RPO/RTO and cutover approval. Remote backup commitment remains open.

**Dependencies.** DEC-002,DEC-005,DEC-011,DEC-018

**Contract bindings.** registers/storage-schema.csv#backup_manifest; contracts/release-gates.json#C4,C5; phases/14.md. **Highlighted tests.** TC-P14-01,TC-P14-02,TC-AUD-14,TC-WEB-09. The baseline declares 6 case links.

### DEC-014 · Custom domain DNS/TLS verification

**High · Network and web owners**. Pending Human Decision.

**Recommendation.** Require separate domain control proof, DNS target observation and independent HTTPS probe before activation. **Alternative.** Manual certificate/DNS workflow reduces automation and leaves ongoing operator duties.

**Rationale.** DNS control, DNS target alignment and live HTTPS rules catch different activation failures. **Security.** Prevent hostname takeover and preserve management fallback. Wrong SAN, expiry and wrong target must not become serving-ready.

**Operational effect and closure.** Approve proof and issuer strategy, validity/renewal, rollback, IPv4/IPv6 checks as applicable and access fallback. Prototype verification has no live production evidence.

**Dependencies.** DEC-003,DEC-005,DEC-006,DEC-011

**Contract bindings.** phases/06.md; phases/09.md; phases/10.md; registers/vps-operations.csv. **Highlighted tests.** TC-P09-01,TC-P10-01,TC-P10-02,TC-WEB-02. The baseline declares 58 case links.

### DEC-015 · Scope and billing module status

**High · Hosting product owner**. Pending Human Decision.

**Recommendation.** Commit customer and account primitives independently; defer commercial payments and invoices until approved. **Alternative.** Early billing adds ledgers, reconciliation, privacy and support commitments.

**Rationale.** Core account provisioning and upstream allocation can operate independently of payments and tax rules. **Security.** Commercial modules introduce payment callback, refund, sensitive-data and delegation controls.

**Operational effect and closure.** Record scope of accounts, billing, portal, branding, support and SLA. Show planned versus enforced quotas regardless of commerce. P2 H-008 requires distinct planned/reserved/measured/enforced semantics.

**Dependencies.** DEC-004,DEC-016

**Contract bindings.** phases/20.md; registers/capability-traceability.csv; registers/permission-matrix.csv. **Highlighted tests.** TC-P20-01,TC-BIZ-01,TC-BIZ-02,TC-BIZ-03. The baseline declares 60 case links.

### DEC-013 · Production readiness and support boundaries

**High · Release and operations owners**. Pending Human Decision.

**Recommendation.** Require exact supported tuples, immutable promotion, verified rollback and named on-call/incident handoff. **Alternative.** Limited pilot support reduces initial breadth but must be stated and enforced as narrower scope.

**Rationale.** Artifact-bound promotion and support limits prevent evidence from drifting across builds and targets. **Security.** Require independent promotion authorization, exact-candidate recovery and controls against stale/overbroad approval.

**Operational effect and closure.** Approve support tiers, incident ownership, staging gate and candidate-bound rollback and approval rules. Production readiness requires separately executed evidence and authorization.

**Dependencies.** DEC-005,DEC-010,DEC-016,DEC-018

**Contract bindings.** contracts/release-gates.json#C2,C4,C5,R1,R2; execution-and-release-gates.md. **Highlighted tests.** TC-P08-02,TC-P24-01,TC-P24-02,TC-P25-01. The baseline declares 33 case links.

### DEC-018 · Measurable policy targets

**High · Reliability and security owners**. Pending Human Decision.

**Recommendation.** Measure supported workloads and approve a per-environment policy table. **Alternative.** Reusing illustrative test values as service targets creates unsupported commitments.

**Rationale.** Measure on named tuples, then set policy; test fixture values are not operating limits. **Security.** Overbroad leases, stale alerts and lax retention change exposure. Tight limits can reduce availability.

**Operational effect and closure.** Approve values and units for TTL, lease/retry, retention, resource limits, rollback thresholds and RPO/RTO. Bind exact policy digest to tests. No production thresholds are approved by fixture examples.

**Dependencies.** DEC-005,DEC-016

**Contract bindings.** contracts/fixture-defaults.json; contracts/envelopes.schema.json#Evidence; contracts/release-gates.json#C5,R2. **Highlighted tests.** TC-P14-02,TC-P17-01,TC-P21-01,TC-P22-01. The baseline declares 60 case links.

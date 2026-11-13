# Unresolved human decisions · v4.0

Full-platform delivery is directed. DEC-016 covers detailed interpretation. DEC-017 is evidence work. Paths in contract references are relative to the preserved specification.

## DEC-016 · Capability commitment and source release crosswalk

**Authority.** Product owner. **State.** Authorized objective; precise disputed scope pending.

**Recommendation.** Full platform is directed. Attest exact row-level obligations, duplicates, and optional or conflicting scope. Preserve every required feature in final release. **Alternative.** Choose native or approved integration only where sources permit. Build waves may change implementation order while keeping the final obligations.

**Technical basis.** Explicit scope prevents registry staging codes or prototype visibility from committing product behavior. **Security.** Uncommitted actions remain unavailable; scope changes re-open impacted authorization and threat review.

**Closure evidence.** Sign scope ledger and map each enabled capability to approved contract, owner and release. **Coupled reviews.** None recorded.

**Affected contracts.** contracts/release-gates.json#I0; registers/capability-traceability.csv; registers/requirements.csv. **Representative regressions.** TC-P02-02,TC-P24-01,TC-OPS-01.

## DEC-003 · VPS agent privileged execution protocol

**Authority.** Host and security owners. **State.** Pending human decision.

**Recommendation.** Start with a local authenticated Unix socket, root-owned typed registry and host journal. **Alternative.** Remote agents require mutual authentication, rotation, reachability and a compromised-control-plane model.

**Technical basis.** A narrow helper protocol keeps general web parsing and workflow state outside the privileged boundary. **Security.** Peer checks and root-owned allowlists constrain allowed verbs. A compromised allowed peer can invoke its whole allowlist; host root remains outside tenant isolation.

**Closure evidence.** Approve peer IDs, root-owned allowlist, target identity checks, replay and fencing behavior. **Coupled reviews.** DEC-001,DEC-004,DEC-005,DEC-009,DEC-011.

**Affected contracts.** contracts/envelopes.schema.json#HostRequest; contracts/envelopes.schema.json#HostResult; registers/vps-operations.csv. **Representative regressions.** TC-P07-01,TC-AUD-05,TC-P09-01,TC-P22-01.

## DEC-004 · Tenant ownership and authorization semantics

**Authority.** Product and security owners. **State.** Pending human decision.

**Recommendation.** Use explicit per-workspace membership, separate ownership edges and default deny at HTTP, jobs and artifacts. **Alternative.** Broader inherited delegation reduces admin effort but enlarges blast radius and needs explicit relationship semantics.

**Technical basis.** Authoritative owner edges and current grants let scope be recomputed at every access and effect boundary. **Security.** Deny unknown or foreign scope, stale grants and unauthorized artifacts. Keep support leases explicit, time-limited and auditable.

**Closure evidence.** Approve all grants, revocation boundaries, transfer, last owner and support policy. **Coupled reviews.** DEC-016.

**Affected contracts.** registers/permission-matrix.csv; registers/storage-schema.csv#membership,grant,resource,support_lease; contracts/envelopes.schema.json#MutationIntent. **Representative regressions.** TC-P18-01,TC-CORE-02-SCOPE,TC-CORE-03-SCOPE,TC-INT-01-SCOPE.

## DEC-008 · Recovery through SSH/console

**Authority.** Identity and security owners. **State.** Pending human decision.

**Recommendation.** Review an installation-local, purpose-bound, one-use recovery grant that preserves MFA. **Alternative.** Email-only recovery reduces host privilege exposure but leaves users without recovery when delivery is unavailable.

**Technical basis.** Single-use local recovery covers delivery failure while preserving normal authentication boundaries. **Security.** Bind proof, purpose, installation and expiry; consume atomically; revoke old sessions and preserve enrolled MFA. Avoid exposing raw recovery material.

**Closure evidence.** Approve proof of authority, TTL, session effects, audit, MFA continuity and fallback. **Coupled reviews.** DEC-003,DEC-004,DEC-007,DEC-011,DEC-018.

**Affected contracts.** registers/storage-schema.csv#challenge,session; contracts/profile.md#Authorization and tenant isolation. **Representative regressions.** TC-P06-01,TC-P17-01,TC-P17-02,TC-CORE-01.

## DEC-011 · Secrets and credential storage

**Authority.** Security owner. **State.** Pending human decision.

**Recommendation.** Keep only secret references in ordinary data. Use scoped broker or envelope encryption with separate key custody. **Alternative.** A fully external vault improves separation and adds availability and bootstrap dependencies.

**Technical basis.** Stable secret references support rotation and restrict plaintext distribution across panel, helper and workers. **Security.** Separate custody, scope provider access, redact logs and preserve recovery when key infrastructure is unavailable.

**Closure evidence.** Approve secret classes, rotation, revocation, operator access, audit and recovery. **Coupled reviews.** DEC-001,DEC-004.

**Affected contracts.** registers/storage-schema.csv#secret_ref; contracts/envelopes.schema.json#Evidence; execution-and-release-gates.md. **Representative regressions.** TC-P05-01,TC-P13-01,TC-P15-01,TC-P21-01.

## DEC-019 · Future autonomous agent authority

**Authority.** Repository, infrastructure and release owners. **State.** Pending human decision.

**Recommendation.** Grant future tasks narrow repo/branch and disposable target scope using time-bounded credentials. Separate release approver. **Alternative.** Broader standing access increases potential impact, review burden and recovery exposure.

**Technical basis.** Per-task authority binds access to approved repo, branch, environment, target and operation. **Security.** Time-bound separate credentials and independent release approval limit unauthorized effects. Documentation approval grants no infrastructure access.

**Closure evidence.** Record named owners, target IDs, protected actions, evidence access, expiry and revocation. **Coupled reviews.** DEC-016,DEC-013.

**Affected contracts.** execution-and-release-gates.md#Agent task record; contracts/release-gates.json#I0,R1. **Representative regressions.** TC-P25-01,TC-AUD-25,TC-P24-01.

## DEC-001 · Backend language and service architecture

**Authority.** Architecture board. **State.** Pending human decision.

**Recommendation.** Review the unprivileged modular control plane and separate host/build trust boundaries. Qualify the topology for the full platform, including approved multi-node scope. **Alternative.** Use independently deployed services where scaling or isolation warrants them; this adds service identity and recovery complexity.

**Technical basis.** Modular single process reduces deployment coordination while retaining a distinct trust boundary for host execution. **Security.** Keep the web/API process unprivileged; isolate untrusted builds and credential access from helper and control store.

**Closure evidence.** Freeze process, transport and privilege boundaries plus minimum supported resource profile. **Coupled reviews.** DEC-016.

**Affected contracts.** contracts/profile.md#VPS protocol; shared-contracts.md; phases/03.md. **Representative regressions.** TC-P03-01,TC-P03-02,TC-P07-01.

## DEC-002 · Database engine and migration strategy

**Authority.** Data architecture owner. **State.** Pending human decision.

**Recommendation.** Decide storage using durability, concurrency and restore evidence for all approved topologies. SQLite WAL is a single-host candidate. **Alternative.** External database offers a different scaling/availability model and adds provisioning and recovery dependencies.

**Technical basis.** A local transactional store fits single-host intent/outbox atomicity. Verify concurrency and durability before promising limits. **Security.** Ownership relations and secret references must be enforced transactionally; migration backups need separate custody and restore proof.

**Closure evidence.** Select engine, migration compatibility matrix, durability settings and restore drill. **Coupled reviews.** DEC-001,DEC-004,DEC-009,DEC-010.

**Affected contracts.** registers/storage-schema.csv; contracts/profile.md#Persistence. **Representative regressions.** TC-P04-01,TC-P04-02,TC-P14-01,TC-P22-01.

## DEC-009 · Jobs, idempotency and reconciliation

**Authority.** Orchestration owner. **State.** Pending human decision.

**Recommendation.** Durable transactional jobs and outbox with leases, monotonically fenced effects and observation before replay. **Alternative.** External broker supplies queue tooling but does not by itself solve durable intent or host deduplication.

**Technical basis.** Durable intent and transactional outbox avoid losing accepted work. Lease and effect fencing solve separate stale-worker risks. **Security.** Recheck grants before dispatch; ambiguity blocks unsafe replay. A queue cannot guarantee exactly-once external effects.

**Closure evidence.** Approve retries, lease policy, state transitions, cancel semantics and ambiguity handling. **Coupled reviews.** DEC-003,DEC-004.

**Affected contracts.** contracts/envelopes.schema.json#OperationStatus,HostRequest,HostResult; registers/storage-schema.csv#operation,operation_attempt,idempotency,outbox. **Representative regressions.** TC-P03-01,TC-P07-01,TC-P22-01,TC-CORE-04.

## DEC-012 · API versioning and error conventions

**Authority.** API owner. **State.** Pending human decision.

**Recommendation.** Use a versioned namespace with one canonical schema and stable errors; preserve source routes as candidates. **Alternative.** Publishing historic aliases increases compatibility, authorization and deprecation burden.

**Technical basis.** A canonical versioned interface prevents historic path aliases from acquiring unreviewed behavior. **Security.** Check visibility before replay lookup; bind pagination and idempotency to scope. Protect browser mutations and validate callbacks independently.

**Closure evidence.** Approve route-by-route schema, statuses, idempotency and compatibility. **Coupled reviews.** DEC-004,DEC-009,DEC-016.

**Affected contracts.** contracts/envelopes.schema.json#Error,MutationIntent,OperationAccepted,OperationStatus; registers/source-api-catalog.csv. **Representative regressions.** TC-P23-01,TC-P23-02,TC-AUD-23.

## DEC-005 · Supported Linux distributions and versions

**Authority.** Host engineering owner. **State.** Pending human decision.

**Recommendation.** Pick one verified apt/systemd host tuple per initial support tier after install, upgrade and restore proof. **Alternative.** A broad Ubuntu/Debian claim increases ongoing matrix and support cost.

**Technical basis.** One exact qualified host tuple makes install/upgrade claims reproducible and narrows adapter variation. **Security.** Unsupported systems fail preflight before mutation. Package rollback and host recovery require distinct evidence.

**Closure evidence.** Name supported releases, architectures, images and exclusions. **Coupled reviews.** DEC-003,DEC-013.

**Affected contracts.** phases/08.md; registers/vps-operations.csv; contracts/release-gates.json#C4. **Representative regressions.** TC-P06-02,TC-P08-01,TC-P08-02,TC-P09-01.

## DEC-006 · Web server and runtime support matrix

**Authority.** Web/runtime owner. **State.** Pending human decision.

**Recommendation.** Approve lifecycle and compatibility profiles for every required web and runtime capability. Static hosting is a dependency milestone. **Alternative.** Develop adapters concurrently for earlier integration feedback; this increases compatibility and isolation verification load.

**Technical basis.** Static hosting limits early runtime/supervisor dependencies; adapter commitments can grow from measured compatibility. **Security.** Protect unit identities, per-site principals and filesystem roots. Optional CLI, OCI and terminals need explicit privilege review.

**Closure evidence.** Publish feature-to-runtime and host eligibility matrix. **Coupled reviews.** DEC-003,DEC-005,DEC-016.

**Affected contracts.** phases/10.md; phases/11.md; registers/vps-operations.csv; registers/capability-traceability.csv. **Representative regressions.** TC-P11-01,TC-P11-02,TC-WEB-01,TC-APP-08.

## DEC-007 · Mail transport and optional mailbox hosting

**Authority.** Integrations owner. **State.** Pending human decision.

**Recommendation.** Define separate SMTP recovery and mailbox contracts and agree native versus integrated provider coverage. **Alternative.** Use an external mailbox provider where accepted; support, tenant identity, abuse and data custody boundaries change.

**Technical basis.** SMTP isolates transactional recovery from mail server operations; acceptance and actual delivery differ. **Security.** Protect reset material and avoid account enumeration. Reconcile ambiguous send outcomes before retry.

**Closure evidence.** Approve SMTP provider interface, security and failure policies; record mailbox disposition. **Coupled reviews.** DEC-008,DEC-011,DEC-016.

**Affected contracts.** phases/16.md; phases/17.md; registers/storage-schema.csv#secret_ref,challenge. **Representative regressions.** TC-P16-01,TC-P16-02,TC-MAIL-01,TC-MAIL-02.

## DEC-010 · Backup encryption, retention and restore guarantees

**Authority.** Reliability and security owners. **State.** Pending human decision.

**Recommendation.** Require encrypted independently restorable backups, plus an off-host recoverable copy for disaster recovery where committed. **Alternative.** Local-only snapshots ease operation but cannot survive complete VPS loss.

**Technical basis.** Independent recovery requires backup integrity plus access to keys and a usable application-level restore. **Security.** Encrypt and scope backup retrieval and cutover; keep destructive recovery separate from verification. Local copies cannot cover host loss.

**Closure evidence.** Approve consistency class, destinations, key custody, retention, RPO/RTO and cutover rehearsal. **Coupled reviews.** DEC-002,DEC-005,DEC-011,DEC-018.

**Affected contracts.** registers/storage-schema.csv#backup_manifest; contracts/release-gates.json#C4,C5; phases/14.md. **Representative regressions.** TC-P14-01,TC-P14-02,TC-AUD-14,TC-WEB-09.

## DEC-014 · Custom domain DNS/TLS verification

**Authority.** Network and web owners. **State.** Pending human decision.

**Recommendation.** Require separate domain control proof, DNS target observation and independent HTTPS probe before activation. **Alternative.** Manual certificate/DNS workflow reduces automation and leaves ongoing operator duties.

**Technical basis.** DNS control, DNS target alignment and live HTTPS rules catch different activation failures. **Security.** Prevent hostname takeover and preserve management fallback. Wrong SAN, expiry and wrong target must not become serving-ready.

**Closure evidence.** Approve proof method, DNS validation rules, certificate issuer/challenge, retries and access fallback. **Coupled reviews.** DEC-003,DEC-005,DEC-006,DEC-011.

**Affected contracts.** phases/06.md; phases/09.md; phases/10.md; registers/vps-operations.csv. **Representative regressions.** TC-P09-01,TC-P10-01,TC-P10-02,TC-WEB-02.

## DEC-015 · Scope and billing module status

**Authority.** Hosting product owner. **State.** Pending human decision.

**Recommendation.** Resolve native, integrated or optional status for commercial and portal features while retaining required account and delegation behavior. **Alternative.** Choose native commerce for direct control or an approved integrated ledger/payment path for lower operational burden.

**Technical basis.** Core account provisioning and upstream allocation can operate independently of payments and tax rules. **Security.** Commercial modules introduce payment callback, refund, sensitive-data and delegation controls.

**Closure evidence.** Record inclusion/exclusion for portal, branding, payments, invoicing, SLA and support. **Coupled reviews.** DEC-004,DEC-016.

**Affected contracts.** phases/20.md; registers/capability-traceability.csv; registers/permission-matrix.csv. **Representative regressions.** TC-P20-01,TC-BIZ-01,TC-BIZ-02,TC-BIZ-03.

## DEC-013 · Production readiness and support boundaries

**Authority.** Release and operations owners. **State.** Pending human decision.

**Recommendation.** Require exact supported tuples, immutable promotion, verified rollback and named on-call/incident handoff. **Alternative.** Limited pilot support reduces initial breadth but must be stated and enforced as narrower scope.

**Technical basis.** Artifact-bound promotion and support limits prevent evidence from drifting across builds and targets. **Security.** Require independent promotion authorization, exact-candidate recovery and controls against stale/overbroad approval.

**Closure evidence.** Approve supported release channels, environment gates and operational ownership. **Coupled reviews.** DEC-005,DEC-010,DEC-016,DEC-018.

**Affected contracts.** contracts/release-gates.json#C2,C4,C5,R1,R2; execution-and-release-gates.md. **Representative regressions.** TC-P08-02,TC-P24-01,TC-P24-02,TC-P25-01.

## DEC-018 · Measurable policy targets

**Authority.** Reliability and security owners. **State.** Pending human decision.

**Recommendation.** Measure supported workloads and approve a per-environment policy table. **Alternative.** Reusing illustrative test values as service targets creates unsupported commitments.

**Technical basis.** Measure on named tuples, then set policy; test fixture values are not operating limits. **Security.** Overbroad leases, stale alerts and lax retention change exposure. Tight limits can reduce availability.

**Closure evidence.** Approve values and units for limits, TTLs, rate controls, retention, RPO/RTO, alerting and rollout. **Coupled reviews.** DEC-005,DEC-016.

**Affected contracts.** contracts/fixture-defaults.json; contracts/envelopes.schema.json#Evidence; contracts/release-gates.json#C5,R2. **Representative regressions.** TC-P14-02,TC-P17-01,TC-P21-01,TC-P22-01.

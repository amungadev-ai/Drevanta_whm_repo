# Drevanta v3.1 — Architecture and Scope Decision Review

**Status: review package; no decision is approved by this document.**

Source: user-provided Specification Closure v3.0 archive. Recommendations and alternatives below are transcribed from its decision register, not newly approved architecture.

## Review sequence

First: scope and source reconciliation (DEC-016, DEC-017). Second: trust boundaries and host operations (DEC-003, DEC-004, DEC-008, DEC-011, DEC-019). Third: core service and durability (DEC-001, DEC-002, DEC-009, DEC-012). Fourth: host compatibility and hosting capabilities (DEC-005, DEC-006, DEC-007, DEC-010, DEC-014, DEC-015). Fifth: release and measurable targets (DEC-013, DEC-018).

A reviewer must record APPROVE / MODIFY / DEFER / REJECT, approver identity, dated ADR, supported evidence, and resulting tests. Silence is not approval.

## DEC-001 — Backend language and service architecture
**Review priority:** High; **owner:** Architecture board; **affected phases:** 03

**Candidate recommendation:** Keep an unprivileged Go modular monolith with static UI and a separately constrained local helper for the first slice.

**Alternative / consequence:** Split service deployment adds network identity, operational overhead and independent scaling.

**Required closure evidence:** Freeze process, transport and privilege boundaries plus minimum supported resource profile.

**Approval:** PENDING. **Do not promote candidate contracts to production contracts until approved.**

## DEC-002 — Database engine and migration strategy
**Review priority:** High; **owner:** Data architecture owner; **affected phases:** 04

**Candidate recommendation:** Review SQLite WAL for single-host metadata with transactional outbox and explicit migration backup.

**Alternative / consequence:** External database offers a different scaling/availability model and adds provisioning and recovery dependencies.

**Required closure evidence:** Select engine, migration compatibility matrix, durability settings and restore drill.

**Approval:** PENDING. **Do not promote candidate contracts to production contracts until approved.**

## DEC-003 — VPS agent privileged execution protocol
**Review priority:** Critical; **owner:** Host and security owners; **affected phases:** 03,05,06,07,09,11,25

**Candidate recommendation:** Start with a local authenticated Unix socket, root-owned typed registry and host journal.

**Alternative / consequence:** Remote agents require mutual authentication, rotation, reachability and a compromised-control-plane model.

**Required closure evidence:** Approve peer IDs, root-owned allowlist, target identity checks, replay and fencing behavior.

**Approval:** PENDING. **Do not promote candidate contracts to production contracts until approved.**

## DEC-004 — Tenant ownership and authorization semantics
**Review priority:** Critical; **owner:** Product and security owners; **affected phases:** 12,13,18,19,20,23

**Candidate recommendation:** Use explicit per-workspace membership, separate ownership edges and default deny at HTTP, jobs and artifacts.

**Alternative / consequence:** Broader inherited delegation reduces admin effort but enlarges blast radius and needs explicit relationship semantics.

**Required closure evidence:** Approve all grants, revocation boundaries, transfer, last owner and support policy.

**Approval:** PENDING. **Do not promote candidate contracts to production contracts until approved.**

## DEC-005 — Supported Linux distributions and versions
**Review priority:** High; **owner:** Host engineering owner; **affected phases:** 06,07,08,09

**Candidate recommendation:** Pick one verified apt/systemd host tuple per initial support tier after install, upgrade and restore proof.

**Alternative / consequence:** A broad Ubuntu/Debian claim increases ongoing matrix and support cost.

**Required closure evidence:** Name supported releases, architectures, images and exclusions.

**Approval:** PENDING. **Do not promote candidate contracts to production contracts until approved.**

## DEC-006 — Web server and runtime support matrix
**Review priority:** High; **owner:** Web/runtime owner; **affected phases:** 10,11,12,13,15

**Candidate recommendation:** Start with a reviewed static website profile and only separately approved runtime adapters.

**Alternative / consequence:** Adding PHP, Node, Python and OCI early increases isolation, compatibility and update burden.

**Required closure evidence:** Publish feature-to-runtime and host eligibility matrix.

**Approval:** PENDING. **Do not promote candidate contracts to production contracts until approved.**

## DEC-007 — Mail transport and optional mailbox hosting
**Review priority:** High; **owner:** Integrations owner; **affected phases:** 16

**Candidate recommendation:** Use transactional SMTP for recovery. Defer mailbox hosting pending separate service commitment.

**Alternative / consequence:** Operating mailboxes adds abuse, delivery and data retention duties.

**Required closure evidence:** Approve SMTP provider interface, security and failure policies; record mailbox disposition.

**Approval:** PENDING. **Do not promote candidate contracts to production contracts until approved.**

## DEC-008 — Recovery through SSH/console
**Review priority:** Critical; **owner:** Identity and security owners; **affected phases:** 05,06,17,18

**Candidate recommendation:** Review an installation-local, purpose-bound, one-use recovery grant that preserves MFA.

**Alternative / consequence:** Email-only recovery reduces host privilege exposure but leaves users without recovery when delivery is unavailable.

**Required closure evidence:** Approve proof of authority, TTL, session effects, audit, MFA continuity and fallback.

**Approval:** PENDING. **Do not promote candidate contracts to production contracts until approved.**

## DEC-009 — Jobs, idempotency and reconciliation
**Review priority:** High; **owner:** Orchestration owner; **affected phases:** 22

**Candidate recommendation:** Durable transactional jobs and outbox with leases, monotonically fenced effects and observation before replay.

**Alternative / consequence:** External broker supplies queue tooling but does not by itself solve durable intent or host deduplication.

**Required closure evidence:** Approve retries, lease policy, state transitions, cancel semantics and ambiguity handling.

**Approval:** PENDING. **Do not promote candidate contracts to production contracts until approved.**

## DEC-010 — Backup encryption, retention and restore guarantees
**Review priority:** Critical; **owner:** Reliability and security owners; **affected phases:** 14

**Candidate recommendation:** Require encrypted independently restorable backups, plus an off-host recoverable copy for disaster recovery where committed.

**Alternative / consequence:** Local-only snapshots ease operation but cannot survive complete VPS loss.

**Required closure evidence:** Approve consistency class, destinations, key custody, retention, RPO/RTO and cutover rehearsal.

**Approval:** PENDING. **Do not promote candidate contracts to production contracts until approved.**

## DEC-011 — Secrets and credential storage
**Review priority:** Critical; **owner:** Security owner; **affected phases:** 04,05,07,13,14,15,16,17,21,22,23

**Candidate recommendation:** Keep only secret references in ordinary data. Use scoped broker or envelope encryption with separate key custody.

**Alternative / consequence:** A fully external vault improves separation and adds availability and bootstrap dependencies.

**Required closure evidence:** Approve secret classes, rotation, revocation, operator access, audit and recovery.

**Approval:** PENDING. **Do not promote candidate contracts to production contracts until approved.**

## DEC-012 — API versioning and error conventions
**Review priority:** High; **owner:** API owner; **affected phases:** 23

**Candidate recommendation:** Use a versioned namespace with one canonical schema and stable errors; preserve source routes as candidates.

**Alternative / consequence:** Publishing historic aliases increases compatibility, authorization and deprecation burden.

**Required closure evidence:** Approve route-by-route schema, statuses, idempotency and compatibility.

**Approval:** PENDING. **Do not promote candidate contracts to production contracts until approved.**

## DEC-013 — Production readiness and support boundaries
**Review priority:** High; **owner:** Release and operations owners; **affected phases:** 08,21,24,25

**Candidate recommendation:** Require exact supported tuples, immutable promotion, verified rollback and named on-call/incident handoff.

**Alternative / consequence:** Limited pilot support reduces initial breadth but must be stated and enforced as narrower scope.

**Required closure evidence:** Approve supported release channels, environment gates and operational ownership.

**Approval:** PENDING. **Do not promote candidate contracts to production contracts until approved.**

## DEC-014 — Custom domain DNS/TLS verification
**Review priority:** High; **owner:** Network and web owners; **affected phases:** 06,09,10,12

**Candidate recommendation:** Require separate domain control proof, DNS target observation and independent HTTPS probe before activation.

**Alternative / consequence:** Manual certificate/DNS workflow reduces automation and leaves ongoing operator duties.

**Required closure evidence:** Approve proof method, DNS validation rules, certificate issuer/challenge, retries and access fallback.

**Approval:** PENDING. **Do not promote candidate contracts to production contracts until approved.**

## DEC-015 — Scope and billing module status
**Review priority:** High; **owner:** Hosting product owner; **affected phases:** 02,20

**Candidate recommendation:** Commit customer and account primitives independently; defer commercial payments and invoices until approved.

**Alternative / consequence:** Early billing adds ledgers, reconciliation, privacy and support commitments.

**Required closure evidence:** Record inclusion/exclusion for portal, branding, payments, invoicing, SLA and support.

**Approval:** PENDING. **Do not promote candidate contracts to production contracts until approved.**

## DEC-016 — Capability commitment and source release crosswalk
**Review priority:** Critical; **owner:** Product owner; **affected phases:** 01,02,12,15,16,18,19,20,24

**Candidate recommendation:** Approve one end-to-end slice with explicit committed IDs and deferred items; keep S0–S4 as source codes.

**Alternative / consequence:** Approve a broad initial feature set only with additional dependencies and validation capacity.

**Required closure evidence:** Sign scope ledger and map each enabled capability to approved contract, owner and release.

**Approval:** PENDING. **Do not promote candidate contracts to production contracts until approved.**

## DEC-017 — Atomic source and runtime UI reconciliation
**Review priority:** High; **owner:** Documentation and UX leads; **affected phases:** 01,19

**Candidate recommendation:** Review atomic text and exercise the existing static prototype in isolation when authorized. Collect versioned state/control evidence.

**Alternative / consequence:** Static-only intake is reproducible and leaves reachability, handlers and hidden controls unresolved.

**Required closure evidence:** Reviewer attests exhaustive source and rendered-state disposition. This is primarily an evidence closure, with human decisions needed for actual conflicts.

**Approval:** PENDING. **Do not promote candidate contracts to production contracts until approved.**

## DEC-018 — Measurable policy targets
**Review priority:** High; **owner:** Reliability and security owners; **affected phases:** 02,04,08,14,17,21,22,24

**Candidate recommendation:** Measure supported workloads and approve a per-environment policy table.

**Alternative / consequence:** Reusing illustrative test values as service targets creates unsupported commitments.

**Required closure evidence:** Approve values and units for limits, TTLs, rate controls, retention, RPO/RTO, alerting and rollout.

**Approval:** PENDING. **Do not promote candidate contracts to production contracts until approved.**

## DEC-019 — Future autonomous agent authority
**Review priority:** Critical; **owner:** Repository, infrastructure and release owners; **affected phases:** 25

**Candidate recommendation:** Grant future tasks narrow repo/branch and disposable target scope using time-bounded credentials. Separate release approver.

**Alternative / consequence:** Broader standing access increases potential impact, review burden and recovery exposure.

**Required closure evidence:** Record named owners, target IDs, protected actions, evidence access, expiry and revocation.

**Approval:** PENDING. **Do not promote candidate contracts to production contracts until approved.**

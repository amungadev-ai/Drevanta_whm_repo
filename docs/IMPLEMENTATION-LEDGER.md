# Drevanta Implementation Ledger (living document)

Baseline: Drevanta Full-Platform Implementation Handoff Closure v4.0
(input SHA-256 `ee27e3813542c35f0540b66dd2c8793a9cbe5811befd25a038f810e8d705e13d`; 107/107 files
hash-verified against `spec/MANIFEST.json` on 2026-11-13 — see `scripts/verify_handoff.py` output in
commit history).

Execution graph: `spec/agent/execution-graph.json` — 14 nodes G00→G60. The graph's global
precondition (DEC-019 scoped authority) is satisfied for **development scope only** by the owner's
written assignment (2026-11-13): designated repository `amungadev-ai/Drevanta_whm_repo`, branches,
code, migrations, tests, CI, PR preparation, and isolated test environments. **Not granted**:
merging protected branches, production infrastructure changes, production customer data access,
destructive host operations, production deployment. Recorded in `docs/DECISIONS.md` as DEC-019-D1.

Status vocabulary: `NOT_STARTED` / `IN_PROGRESS` / `IMPLEMENTED` (code exists, tests pending) /
`VERIFIED` (applicable automated tests pass at a recorded commit) / `BLOCKED` (with reason) /
`PARTIAL` (explicitly listed remainder). No status above `VERIFIED` is claimed without evidence
in `docs/TEST-RESULTS.md`. Release-level completion additionally requires the G50/G60 gates.

## Wave map (execution graph)

| Node | Name | Phases | Status | Ledger section |
| --- | --- | --- | --- | --- |
| G00 | Scope, source, UX and decision freeze | 01, 02 | IN_PROGRESS | W-G00 |
| G10 | Architecture, data and baseline security | 03, 04, 05 | IN_PROGRESS | W-G10 |
| G20 | Installer, host privilege and compatibility | 06, 07, 08 | IN_PROGRESS | W-G20 |
| G21 | Identity and permission enforcement | 17, 18 | IN_PROGRESS | W-G21 |
| G22 | Durability, contracts and integrations | 22, 23 | IN_PROGRESS | W-G22 |
| G30 | Networking, web server and runtimes | 09, 10, 11 | IN_PROGRESS | W-G30 |
| G31 | Files, data services and recovery | 13, 14 | IN_PROGRESS | W-G31 |
| G32 | Website and application lifecycle | 12 | IN_PROGRESS | W-G32 |
| G33 | Developer workflows | 15 | IN_PROGRESS | W-G33 |
| G34 | Messaging and commercial hosting | 16, 20 | IN_PROGRESS | W-G34 |
| G35 | Observability and audit | 21 | IN_PROGRESS | W-G35 |
| G40 | Four persona runtime UX acceptance | 19 | NOT_STARTED | W-G40 |
| G50 | Full integration, release and recovery certification | 24 | NOT_STARTED | W-G50 |
| G60 | Agent evidence and protected-release sign-off | 25 | NOT_STARTED | W-G60 |

## W-G00 · Scope, source and decision freeze

| Item | Disposition | Evidence |
| --- | --- | --- |
| Handoff archive integrity (TC-P01-01 analog) | VERIFIED | 107/107 manifest hashes match; `spec/PROVENANCE.md` |
| Prototype provenance (71 files, TC-P01-02 fixture) | VERIFIED | `docs/prototype-manifest.sha256` committed byte-identical at baseline; `scripts/verify-prototype.sh` enforces |
| Scope register intake (404 rows) | VERIFIED | Rows imported to ledger tracking; 339 "release review" + 34 backlog + 18 evidence-pending + 13 owner-ruling rows preserved (no discards) |
| 47 capability records | VERIFIED | Enumerated in this ledger under capability workstreams |
| 18 DEC decisions dispositioned | IN_PROGRESS | `docs/DECISIONS.md` — recommended defaults adopted for development; owner ratification pending |
| Source reconciliation items REC-01…REC-12 | PARTIAL | Dispositions applied per `spec/specification/registers/reconciliation.csv`; conflicts preserved, none silently discarded |
| 476 handoff blocker rows | IN_PROGRESS | Carried into `docs/BLOCKERS.md`; closed only with linked evidence |

## W-G10 · Architecture, data and baseline security

Adopted stack (development, pending owner ADR ratification): Go control plane + SQLite WAL +
inherited static UI + Go root-owned helper. Basis: `spec/specification/registers/reconciliation.csv`
REC-02 ("Go/SQLite/static UI as strong candidate"), DEC-001 (modular unprivileged control plane,
separate host trust boundary), DEC-002 (SQLite WAL single-host candidate), DEC-003 (local
authenticated Unix socket, root-owned typed registry, host journal), DEC-009 (durable transactional
jobs + outbox with leases/fencing), DEC-012 (versioned namespace, canonical schemas, stable errors).

| Item | Disposition | Code | Tests | Evidence |
| --- | --- | --- | --- | --- |
| Migration framework + schema v1 (17 logical tables + credential + support_lease_grant) | VERIFIED | `internal/store`, `migrations/0001_core_schema.sql`, `migrations/0002_role_grants_seed.sql` | exercised via contract suite | TEST-RESULTS G10 row |
| Canonical JSON + strict field validation | VERIFIED | `internal/api/errors.go` (decodeStrict) | TestContractUnknownField422 | TEST-RESULTS G10 row |
| AuthN: principals, Argon2id credentials, sessions, TOTP MFA (RFC 6238), challenges | VERIFIED | `internal/authn` | TestLoginMFALifecycle, TestUnauthorizedRequests401 | TEST-RESULTS G10 row |
| AuthZ: workspaces, memberships, kind-qualified roles, grant rules, support leases, deny-by-default | VERIFIED | `internal/authz`, `migrations/0002` | TestContractRevokedReplay, TestAuditTrailRecordsDenyAndAllow | TEST-RESULTS G10 row |
| Resource graph + generations/owner revisions | VERIFIED | `internal/store/entities.go` | exercised via contract suite | TEST-RESULTS G10 row |
| Operations: intent digest, idempotency (stable/altered/revoked replay), CAS transitions, fences | VERIFIED | `internal/operations` | TestContractStableReplay, TestContractAlteredReplay409, TestContractRevokedReplay | TEST-RESULTS G10 row |
| API envelope `/api/v1` (me, workspaces, resources, operations, intents, scope audit, login, mfa) + error taxonomy | VERIFIED | `internal/api` | full G10 contract suite | TEST-RESULTS G10 row |
| Append-only audit | VERIFIED | `internal/audit` | TestAuditTrailRecordsDenyAndAllow | TEST-RESULTS G10 row |
| Contract fixtures from `profile.md` (6 of 10 scenarios; host-fixture scenarios 7–10 need the G20 helper) | PARTIAL | `internal/api`, `internal/operations` | 6 scenario tests | remaining 4 scenarios bound to G20 host-verb work |

Semantics decisions recorded (pending owner ratification alongside DEC-004/DEC-012):
- Roles are kind-qualified IDs (`website.owner`); role table carries workspace_kind.
- Visibility rule: any active membership or any support-lease row → visible (denied actions → 403);
  no relationship → concealed (404, no identifiers). Matches profile fixtures ("denial under
  current visibility"; "hidden resource → 404").
- `operation.read` enforced as (creator ‖ active member of operation scope) until the permission
  vocabulary freeze adds an explicit action row.
- Idempotency TTL 24h; session TTL 24h; challenge TTL 30m — fixture defaults (DEC-018), not
  operating limits.

## W-G20 · Installer, host privilege and compatibility

| Item | Disposition | Code | Tests | Evidence |
| --- | --- | --- | --- | --- |
| Installation claim (token scoped to installation, expiring, hashed storage, atomic consume with owner creation) | VERIFIED | `internal/install`, `internal/api/handlers_install.go` | TestInstallerLifecycle | TEST-RESULTS G20 rows |
| Interrupted-setup checkpoints + resume (status checkpoint; claim replay denied) | VERIFIED | `internal/install` | TestInstallerLifecycle | TEST-RESULTS G20 rows |
| Access address setup + verification checkpoint | VERIFIED (write path; DNS/HTTPS probe = DEC-014 work) | `internal/install` | TestInstallerLifecycle | TEST-RESULTS G20 rows |
| Privileged helper: local Unix socket, peer-credential check, root-owned policy file (runtime reload), typed verbs, strict request schema | VERIFIED | `internal/helper`, `cmd/helper` | helper suite | TEST-RESULTS G20 rows |
| Host fixtures 7–10 (wrong host, stale fence, lost reply, changed digest) | VERIFIED | `internal/helper` | TestWrongHost/StaleFence/LostReply/ChangedDigest | TEST-RESULTS G20 rows |
| Built-in verbs: host.inspect, host.preflight (fail-closed OS tuple check), unit.apply (root-owned bindings; no raw unit names/shell) | VERIFIED (inspect/preflight full; unit.apply logic tested via typed-arg discipline) | `internal/helper/verbs.go` | helper suite | TEST-RESULTS G20 rows |
| Panel↔helper wire integration (real socket, replay identity) | VERIFIED | `internal/hostagent`, `internal/helper` | TestHostAgentSocketIntegration | TEST-RESULTS G20 rows |
| Intent→dispatch→helper bridge (host verbs through operations engine) | VERIFIED | `internal/api`, `internal/operations` | TestIntentToHelperFlow | TEST-RESULTS G20 rows |
| VPS matrix drills (clean/repeat/interrupted install, firewall/SSH rollback, reboot convergence) | NOT_STARTED | — | — | G50; requires disposable VPS targets + systemd service units |
| Package/service management verbs breadth (package.apply, vhost.apply, …) | NOT_STARTED | — | — | G30 domain waves extend the allowlist |

Semantics decisions recorded (pending owner ratification alongside DEC-003/DEC-005):
- Replay-before-fence ordering with fence-aware replay: a retry repeats the same fence and
  receives the journaled observation; an older fence rejects as stale (profile fixtures 8/9).
- Helper policy supports target binding per verb ("*" in dev policy; production binds host IDs).
- Supported tuples (dev default): ubuntu 24.04/26.04/26.10, debian 12/13 — fails closed otherwise.

## W-G21 · Identity and permission enforcement

| Item | Disposition | Code | Tests | Evidence |
| --- | --- | --- | --- | --- |
| Password reset (single-use challenge, session revocation via epoch, MFA preserved, no enumeration) | VERIFIED | `internal/authn`, `internal/api/handlers_identity.go` | TestIdentityPasswordResetLifecycle | TEST-RESULTS G21 row |
| Recovery codes (single-use, MFA second factor, hashed storage) | VERIFIED | `internal/authn` | TestIdentityRecoveryCodes | TEST-RESULTS G21 row |
| Logout / session revocation | VERIFIED | `internal/api` | TestIdentityLogout | TEST-RESULTS G21 row |
| Membership grant/list/revoke API (kind-qualified roles, 422 cross-kind, idempotent grant) | VERIFIED | `internal/api/handlers_identity.go` | TestMembership*, TestMembershipRoleValidation | TEST-RESULTS G21 row |
| Last-owner protection (409) | VERIFIED | `internal/api/handlers_identity.go` | TestMembershipLastOwnerProtection | TEST-RESULTS G21 row |
| Dispatch-time reauthorization (revoked actor's queued op cancels, never dispatches) | VERIFIED (control-plane; helper-dispatch-count evidence binds to G20) | `internal/operations.DispatchGuard` | TestMembershipGrantRevokeAndDispatchRecheck | TEST-RESULTS G21 row |
| Support leases: exact per-lease action grants, expiry, revocation | VERIFIED | `internal/authz` | TestSupportLeaseExactActions | TEST-RESULTS G21 row |
| Invitation acceptance bound to intended principal (TC-CORE-02) | NOT_STARTED | — | — | requires signup-bound principals (installer/commercial wave) |
| Browser cookie sessions + CSRF double-submit | NOT_STARTED | — | — | G40 UI wiring |
| Mailer SMTP transport for recovery delivery | PARTIAL | LogMailer no-op seam | — | G34 messaging wave |

Semantics decisions recorded (pending owner ratification alongside DEC-004/DEC-008):
- Last-owner protection adopted as unconditional policy (workspace retains ≥1 active owner).
- Recovery token delivery via Mailer seam; LogMailer default (challenge issued, no delivery)
  until SMTP config (G34). Tokens never appear in logs/responses/audit.

## W-G22 · Durability, contracts and integrations

| Item | Disposition | Code | Tests | Evidence |
| --- | --- | --- | --- | --- |
| Durable job worker (claim, lease, dispatch, observe, terminal CAS) | VERIFIED | `internal/jobs` | TestWorkerSucceedsWithObservation | TEST-RESULTS G22 row |
| Crash reconciliation (expired lease re-dispatch, monotonic fences, journaled observation) | VERIFIED | `internal/jobs` | TestWorkerCrashReconciliationReleasesLease | TEST-RESULTS G22 row |
| Outcome taxonomy (failed; uncertain blocks unsafe retry) | VERIFIED | `internal/jobs` | TestWorkerFailedAndUncertainOutcomes | TEST-RESULTS G22 row |
| Dispatch recheck through worker (revoked actor cancels) | VERIFIED | `internal/jobs` | TestWorkerRevokedActorCancelled | TEST-RESULTS G22 row |
| API keys (issue-once, scope-pinned, revoke-deny) | VERIFIED | `internal/jobs/apikeys.go`, `internal/api/handlers_keys.go` | TestAPIKeyLifecycleScopePinning | TEST-RESULTS G22 row |
| Interval schedules (arm, fire-once, coalesce, owner-bound ops) | VERIFIED (core) | `internal/jobs/apikeys.go`, `internal/api/handlers_keys.go` | TestScheduleArmsFiresAndCoalesces | TEST-RESULTS G22 row |
| Outbox delivery to external endpoints (webhooks) | NOT_STARTED | — | — | INT-01 external contracts need owner ruling (DEC-016 scope rows) |
| Cron-expression schedules | NOT_STARTED | — | — | WEB-08 contract freeze pending |
| Envelope schemas as published JSON Schema | PARTIAL | Go types are the frozen wire contract | contract suite | publish step in G50 |

Semantics decisions recorded (pending owner ratification alongside DEC-009):
- Schedule ticks advance the row before accepting the operation: missed ticks coalesce, never
  double-fire; scheduled operations run as the schedule's owning principal with full predicates.
- Interval (every_seconds) schedules implemented as the WEB-08 core; cron expressions are a
  follow-up increment under the same registry.

## W-G30/G31/G32/G33/G34/G35 (planned)

Domain waves per execution graph: networking/web/runtimes (nginx vhosts, TLS, PHP/Node/Python
toolchains, process supervision), files/databases/backups (SFTP, MariaDB/PostgreSQL/Redis,
encrypted backup/restore), site lifecycle + app templates, developer pipelines (git, builds,
promotion), messaging/commercial (mail, customers/orders/invoices/gateways/tickets — scope note
DEC-015), observability (metrics, logs, status pages, audit trails).

## W-G40 · Four persona runtime UX (planned)

Wire `ui/` prototype to `/api/v1`; render-state traversal evidence per DEC-017 (route, actor,
state, selector, interaction, result).

## W-G50/G60 · Release and certification (planned)

Only when all required scope and verification gates are satisfied; otherwise the
release-readiness report states exactly which gates remain FAIL/BLOCKED/NOT_RUN.

## Domain wave status (a15c8cd)

| Domain | Implemented | Remaining for certification |
| --- | --- | --- |
| Files (WEB-05) | file.apply verb: site-root confinement, traversal rejection, bounds; intent→worker→helper verified | editor API, archive ops, quota accounting |
| SFTP (WEB-06) | sftp.apply verb: account create/delete/key with validation | sshd config integration, host drill |
| Databases (DB-01/02/03) | database.apply verb: provision/grant/revoke/drop via engine bindings; engine absence fails safe | MariaDB/PostgreSQL host drill, Redis optional |
| Vhosts/TLS (WEB-01/02) | vhost.apply verb + **live VPS drill PASSED** (intent → helper → managed nginx conf → reload → HTTP serving) | TLS issuance (DEC-014), nginx service-reload hardening |
| Runtimes (APP-02/03/04/08) | toolchain.apply mapping registered | per-runtime adapters + matrix (DEC-006) |
| Site lifecycle (APP-01, MKT-01/02) | template catalog, site installs bound to application resources | template.apply host effect, install drill |
| Developer (APP-05/06/07) | source connections, builds (digest-bound), environments, CAS promotion + outbox | build isolation executor, release.activate host drill |
| Commercial (BIZ-01/02/03) | plans/customers/accounts/subscriptions, invoices, payments (settle-once), credits | gateway credentials + tax rules (BLK-001) |
| Support (SUP-01/02) | tickets + SLA + state machine, KB | notifications delivery, SLA policy ratification |
| Mail (MAIL-01/02) | domains (verify flow), mailboxes, aliases | mailbox.apply host drill, SMTP transport (DEC-007) |
| Observability (OBS-01) | observations, latest-per-name, notifications | metrics sampler worker, status pages |
| Backups (WEB-09) | manifests (digest+key+schema), verify+key restore gates | encrypted backup creation + isolated restore drill (G50) |

## Frontend directive wave (2026-11-13, DEC-020/DEC-023)

| Directive deliverable | Disposition | Artifact |
| --- | --- | --- |
| §4.1 Prototype inventory | VERIFIED | docs/ui/PROTOTYPE-INVENTORY.md + inventory.json (31 pages = 31 register screens, 13 CSS, 10 JS, 71 digests, mock-data/control accounting) |
| §10.1 Framework comparison + recommendation | SUBMITTED FOR APPROVAL | docs/ui/FRAMEWORK-EVALUATION.md → SvelteKit+TS (DEC-020) |
| §10.3 Migration map | DONE (framework-agnostic) | docs/ui/MIGRATION-MAP.md (31 routes, shared components, sequencing, control coverage) |
| §4.3 Screenshot baseline | VERIFIED — 62 captures, 0 failures | docs/ui/baseline/ (desktop 1440×900 + mobile 375×812, full-page, manifest with console errors + navigation structure); harness tools/ui-baseline/capture.js; BLK-003 gap closed for baseline capture |
| §10.4 Fidelity verification plan | DONE | docs/ui/FIDELITY-PLAN.md (gates G1–G9, pixelmatch tolerances, honest-accounting rules) |
| §10.5/§10.6 Architecture + deployment plan | DONE (pending framework approval) | docs/ui/FRONTEND-ARCHITECTURE.md (typed API client, cookie sessions, intent pipeline + operation tracker, nginx-integrated single-installation deployment, upgrade/rollback) |
| §7 Backend protection | RESPECTED — zero backend changes made for the frontend directive; Go/SQLite/helper/tests unchanged | git diff scope |
| §9 Independent work continues | see domain waves (a15c8cd) + web-server drill below | ledger |

Migration implementation: **Stage-1 EXECUTED** (facd209..5651d8c) and **Stage-2 EXECUTED** (084b05c..8d5c8db): real MFA enrollment + recovery codes (incl. the RFC 6238 TOTP backend fix), real persona operations (resources/intents/operations/audit) behind Go permission enforcement, workspace-driven routing, real logout, security suite 16/16, real-flow e2e 18/18, control matrix 324/324 accounted, nonvisual a11y fix. Remaining: Stage-3 section-level componentization (42 partial controls), SMTP wave (G34), VPS re-verification + push (credentials expired — blockers recorded in TEST-RESULTS.md).

## Requirement → implementation traceability

Ledger rows for all 72 requirements (`spec/specification/registers/requirements.csv`) and 47
capabilities are tracked in `docs/traceability.csv` (machine-readable mirror) and updated in the
same commit as the code they bind. The authoritative normative rows remain in `spec/`; this ledger
never narrows them, only records implementation state and evidence pointers.

## G30/G50 wave (2026-11-14)

All ten host-operation and disaster-recovery drills executed on the disposable VPS
with REAL evidence (DEPLOY-DRILL-G30-G50.md): TLS lifecycle, mail provisioning +
end-to-end delivery, toolchain provisioning (real apt), ISOLATED build execution
(fail-closed sandbox, deterministic digests), full encrypted backup/DR chain incl.
sqlite WAL recovery into a clean nspawn environment, service reload/restart/revert,
interrupted-operation recovery (DispatchGuard reauthorization demonstrated as a
security property), failed-upgrade rollback, reboot convergence (6 services),
firewall + SSH safety rollback. Release reconciliation produced for all 404 scope
rows (release-reconciliation.csv).

## G31 wave (2026-11-14, DEC-016/021/022 rulings)

Mail lifecycle completed and client-verified on the VPS (SSHA512 dovecot-order
digests with round-trip test — plaintext never at rest; generated passwords
returned once; suspension revokes auth while preserving mail; alias forwarding
delivered end-to-end; cross-tenant maildir isolation; IMAP LOGIN verified).
DEC-016 infrastructure services implemented + drilled: Redis/Valkey
(verb-driven install, isolated port, PING PONG), authoritative DNS zones
(nsd-checkzone ok), OCI containers (hardened lifecycle via verb). WEB-07
scoped terminal sessions (sandboxed, audited, interpreter allowlist).
Commercial core: DEC-026 validated configuration + tax math; sandbox payment
adapter (idempotency, webhook HMAC + replay). Remaining engineering is
explicitly owned by engineering (BLK-006b/009); owner input only for the
second clean-VPS DR environment (BLK-010).


## G32 wave (2026-11-14)
FP-P3-052 + FP-P4-026 closed with real implementations (reconciliation 241
IMPLEMENTED-TESTED / 0 IMPLEMENTED-UNTESTED); decision register dedupe
(DEC-023/024/025/026, DEC-REG-1) and corrected nginx/BFF architecture text;
mail client operations fully verified (IMAP/POP3/isolation); Redis AUTH,
container idempotency, interactive PTY sessions; REAL build->deploy->
health->rollback chain; full-stack cookie-to-host-effect chain with 401/422
negatives and audit verification. Roadmap expanded to every outstanding
scope (A1-A11, B1-B5); G31/G32 VPS report + gateway options register added
to the evidence set.

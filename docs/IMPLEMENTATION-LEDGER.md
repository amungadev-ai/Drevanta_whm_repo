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
| G20 | Installer, host privilege and compatibility | 06, 07, 08 | NOT_STARTED | W-G20 |
| G21 | Identity and permission enforcement | 17, 18 | NOT_STARTED | W-G21 |
| G22 | Durability, contracts and integrations | 22, 23 | NOT_STARTED | W-G22 |
| G30 | Networking, web server and runtimes | 09, 10, 11 | NOT_STARTED | W-G30 |
| G31 | Files, data services and recovery | 13, 14 | NOT_STARTED | W-G31 |
| G32 | Website and application lifecycle | 12 | NOT_STARTED | W-G32 |
| G33 | Developer workflows | 15 | NOT_STARTED | W-G33 |
| G34 | Messaging and commercial hosting | 16, 20 | NOT_STARTED | W-G34 |
| G35 | Observability and audit | 21 | NOT_STARTED | W-G35 |
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

## W-G20 · Installer, host privilege and compatibility (planned)

Installer (claim token, bootstrap, administrator enrollment, recovery codes, access address),
helper protocol implementation (peer proof, typed allowlist, fences, journals), OS compatibility
preflight (single apt/systemd tuple: Debian-family), host health/packages/services verbs.

## W-G21 · Identity and permission enforcement (planned)

Session lifecycle enforcement, credential epochs, step-up, membership grants/revocation/transfer,
tenant isolation across API/jobs/artifacts, support-lease expiry, browser CSRF.

## W-G22 · Durability, contracts and integrations (planned)

Worker crash reconciliation, outbox delivery, API key auth, external SSO hooks, webhooks,
cron/schedules, envelope schemas frozen as JSON Schema + Go types.

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

## Requirement → implementation traceability

Ledger rows for all 72 requirements (`spec/specification/registers/requirements.csv`) and 47
capabilities are tracked in `docs/traceability.csv` (machine-readable mirror) and updated in the
same commit as the code they bind. The authoritative normative rows remain in `spec/`; this ledger
never narrows them, only records implementation state and evidence pointers.

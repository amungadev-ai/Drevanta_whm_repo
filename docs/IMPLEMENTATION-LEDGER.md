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
| Migration framework + schema v1 (17 logical tables) | IN_PROGRESS | `internal/store`, `migrations/0001_core_schema.sql` | `internal/store` | |
| Canonical JSON + strict field validation | IN_PROGRESS | `internal/api/jsonutil` | `internal/api` | |
| AuthN: principals, Argon2id credentials, sessions, TOTP MFA, challenges | IN_PROGRESS | `internal/authn` | `internal/authn` | |
| AuthZ: workspaces, memberships, roles, grants, support leases, deny-by-default | IN_PROGRESS | `internal/authz` | `internal/authz` | |
| Resource graph + generations/owner revisions | IN_PROGRESS | `internal/store` | `internal/store` | |
| Operations: intent digest, idempotency, CAS transitions, outbox | IN_PROGRESS | `internal/operations` | `internal/operations` | |
| API envelope `/api/v1` (me, workspaces, operations, intents) + error taxonomy | IN_PROGRESS | `internal/api` | `internal/api` | |
| Append-only audit | IN_PROGRESS | `internal/audit` | `internal/audit` | |
| Contract fixtures from `profile.md` (10 scenarios) | IN_PROGRESS | `internal/api` | `internal/api/contract_test.go` | |

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

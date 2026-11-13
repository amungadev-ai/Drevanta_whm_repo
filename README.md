# Drevanta Hosting Management Platform

Full-platform hosting management system spanning four personas:

1. **Server & Platform Administration** — installation, host privilege, OS/services, networking, storage
2. **Hosting Provider, Reseller & Agency** — customers, plans, orders, billing, support, delegation
3. **Website Owner** — sites, files, databases, backups, domains, TLS
4. **Developer** — source, builds, releases, environments, secrets, CLI/API access

## Engineering baseline

The engineering baseline is the **Drevanta Full-Platform Implementation Handoff Closure v4.0**
(archived under [`spec/`](spec/START-HERE.md), input SHA-256
`ee27e3813542c35f0540b66dd2c8793a9cbe5811befd25a038f810e8d705e13d`, all 107 files verified
against the embedded manifest). The 25-phase specification, scope register (404 rows),
47 capability records, 188-row permission matrix, 33 VPS operation bindings, 744 parametrized
test specifications and 18 cross-persona workflows in `spec/` are the normative inputs.

The inherited static UI prototype (71 files, byte-identical to handoff provenance) lives in
[`ui/`](ui/index.html) with its digest manifest at `docs/prototype-manifest.sha256`.

## Architecture (adopted for development; see docs/DECISIONS.md)

Per handoff DEC-001/DEC-002 recommendations and reconciliation record REC-02:

- **Control plane** — single modular unprivileged process (`cmd/panel`), Go, versioned JSON API at `/api/v1`
- **Persistence** — SQLite in WAL mode, forward-only migrations, 17-table logical schema per `spec/specification/registers/storage-schema.csv`
- **Privileged execution** — separate root-owned helper binary (`cmd/helper`) over a local authenticated
  Unix socket with typed per-verb allowlists, peer-credential checks, fencing and host journals (DEC-003)
- **Durability** — transactional intent + operation + outbox commits, idempotency keys with canonical digests,
  leased/fenced job workers (DEC-009)
- **Authorization** — explicit per-workspace membership, ownership edges, default deny at every boundary (DEC-004)
- **UI** — the static prototype wired to the real API

## Repository layout

| Path | Purpose |
| --- | --- |
| `spec/` | Engineering baseline handoff v4.0 (normative registers, 25 phases, test specs) |
| `ui/` | Inherited prototype UI (71 files) |
| `cmd/` | Binaries: `panel` (control plane), `helper` (privileged host agent) |
| `internal/` | Control-plane packages: store, authn, authz, api, operations, jobs, audit, hostagent |
| `internal/helper/` | Helper-side verb registry and typed handlers |
| `migrations/` | Forward-only SQL migrations |
| `docs/` | Governance: implementation ledger, decision/blocker/test registers, release readiness |
| `infra/` | Installer, systemd units, deployment assets |
| `scripts/` | CI and evidence tooling |

## Governance

- [`docs/IMPLEMENTATION-LEDGER.md`](docs/IMPLEMENTATION-LEDGER.md) — living requirement→code→test→evidence map
- [`docs/DECISIONS.md`](docs/DECISIONS.md) — decision register (DEC-001…DEC-019 dispositions)
- [`docs/BLOCKERS.md`](docs/BLOCKERS.md) — blocker register
- [`docs/TEST-RESULTS.md`](docs/TEST-RESULTS.md) — test-results register bound to exact commits
- [`docs/RELEASE-READINESS.md`](docs/RELEASE-READINESS.md) — release-readiness report

No capability is "complete" without its contract, implementation, authorization, persistence,
operational effects, error handling, recovery behavior, documentation and applicable tests.
Unexecuted tests are recorded `NOT_RUN`, never `PASS`.

## Development

```
go build ./...
go test ./...
scripts/ci.sh        # fmt, vet, build, tests, prototype digest check
```

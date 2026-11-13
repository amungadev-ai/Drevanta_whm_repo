# Deployment drill — disposable VPS target (2026-11-13)

Target: owner-authorized disposable VPS (Ubuntu 26.04.1 LTS, 4 vCPU, 5.9GB RAM, 93GB free —
the exact supported tuple; DEC-005). Scope: isolated test environment per owner authorization;
no firewall/SSH modification, no production exposure (panel bound to 127.0.0.1:8080).

## Install procedure (repeatable via infra/)

1. Static binaries cross-compiled (CGO off): `panel` (9662616 B),
   `helper` (6856856 B); SHA-256 recorded in `dist/SHA256SUMS` and **verified on target**
   (`sha256sum -c` → OK both).
2. Layout: `/opt/drevanta/bin` (root-owned binaries), `/opt/drevanta/migrations`,
   `/var/lib/drevanta` (drevanta:drevanta 0750), `/etc/drevanta` (root-owned policy +
   unit bindings), `/run/drevanta` (root:drevanta 0750 — socket dir reachable by panel group).
3. systemd: `drevanta-panel.service` (User=drevanta, hardened: NoNewPrivileges,
   ProtectSystem=strict, ReadWritePaths=/var/lib/drevanta) and `drevanta-helper.service`
   (root, bounded by typed allowlist + peer-credential checks; socket group drevanta).
4. Bootstrap: `panel bootstrap` → installation created, claim token printed once (single-use,
   expiring). Helper bound to the installation ID via systemd override.
5. Claim via `POST /api/v1/install/claim` → 201; installation state **active**; replay of the
   same claim → 401 (token consumed atomically).
6. Drill flow: admin login → `POST /install/address` (checkpoint address_configured) →
   `POST /api/v1/scopes/{ws}/resources` (host snapshot) → `POST /api/v1/resources/{id}/intents`
   (host.inspect, Idempotency-Key) → worker (1s loop) dispatch → helper over Unix socket →
   helper journal row (`host.inspect succeeded fence=1`) → operation **succeeded**, outbox
   `operation.succeeded`.

## Failure-path evidence (recorded honestly)

- Pre-fix ambiguous attempt (socket perms): operation **uncertain** — blocked from unsafe
  automated retry per DEC-009; retained in the operation history as evidence.
- Unreachable-socket attempt after the executor fix: **failed** with safe reason
  ("helper unreachable; no effect delivered") — dial failure cannot have delivered an effect.
- Root causes fixed in this commit: socket file+directory group ownership (helper now chowns
  both to the panel group), dial failure classified as safe failure (no effect possible).

## Recovery notes

- Panel restart: `ReconcileExpired` requeues dispatched-with-expired-lease operations
  (verified in G22 suite).
- Helper restart: socket recreated with correct ownership by ExecStartPre + Serve.
- Claim token rotation: `panel bootstrap` reports the existing installation; token is
  single-use — a lost unconsumed token requires manual DB intervention (documented).

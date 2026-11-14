# G31/G32 VPS Execution Report (sanitized)

Host: disposable Ubuntu 26.04 VPS · branch feat/g40-frontend-migration
G31 checkpoint a07005b->9a81cbd, G32 through f7e4c3d. All operations journal-
traced via the privileged helper (journal ids in the helper journal).

## Mail infrastructure (BLK-006 -> closed except webmail)
- Dovecot 2.4 activation: passwd-file passdb+userdb, SSHA512 digests in
  dovecot order (password-first; verified with doveadm pw -t), extended
  lines user:hash:uid:gid::home, mail_inbox_path=%{home}/Maildir.
- Client verification: doveadm auth test OK; IMAP LOGIN/SELECT (4 real
  delivered msgs)/FETCH body/STORE flags/CREATE Archive/LIST; POP3
  USER/PASS/STAT (4, 2312 bytes); tenant-2 sees zero (per-domain namespaces).
- Lifecycle via mail.apply: password change (generated, returned once,
  old rejected), suspension revokes auth preserving mail, resume restores,
  alias info@->postmaster@ delivered end-to-end via postfix virtual map.
- Password hygiene: no plaintext at rest anywhere (passdb hashed; helper
  journals never carried the secret).

## DEC-016 infrastructure services
- cache.apply: Redis 8.0.5 installed via the verb (apt), config synthesis
  (127.0.0.1, isolated port 16379, maxmemory), PING->PONG; per-instance
  AUTH generated once and enforced (NOAUTH without, PONG with).
- dns.apply: zone synthesis A/CNAME/MX via verb; nsd-checkzone OK.
- container.apply: pull/stop/remove via verb; run idempotent
  (running->already-active; interrupted-create->recovered-existing);
  hardened flags (read-only, cap-drop=all, no-new-privileges, limits).
- WEB-07: exec.session audited sandboxed exec; exec.open/write/close
  interactive PTY (script-allocated PTY under systemd-run --pipe,
  non-root, no-network, caps); streamed output (echo of typed command +
  output + prompt); revocation enforced; terminal INPUT never journaled.

## Real application builds and deployments (G32 §7)
- build.execute: fetch (file:// drill repo) -> isolated checkout ->
  platform build command (sh build.sh) -> artifact tar.gz (artifact_dir)
  + tree digest; two builds (v1, v2).
- deploy.apply: traversal-safe release into /var/www/drill/releases/<id>,
  atomic current-symlink swap, web-servable modes, retention 5.
- Health: loopback GET via nginx :8081 -> 200 + body TRUE for v1, TRUE
  for v2, and TRUE after rollback to v1. Traversal artifact rejected
  (no escape file created).

## Full-stack chain (G32 §9)
Cookie session (minted via the production session schema) -> SvelteKit BFF
(:3100) -> loopback Go API (:8080) -> authz -> durable operation
(host.inspect, 202 accepted with operation_id/correlation_id) -> worker ->
helper -> REAL host observation (Ubuntu 26.04 host data) -> succeeded ->
audit events returned via GET /scopes/{id}/audit (200). Negative: invalid
cookie 401; unknown fields 422.

## Not covered here (external inputs)
Second-VPS restore drill (BLK-010/011), public-domain ACME (DEC-024),
production gateway credentials (DEC-025).

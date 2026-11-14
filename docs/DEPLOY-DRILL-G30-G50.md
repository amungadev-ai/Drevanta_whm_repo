# G30/G50 Host-Operations & Disaster-Recovery Drill — REAL VPS Evidence

Date: 2026-11-14 · Target: disposable VM `VM-97057ab9-6a82-4554-969f-e3ab5db01a39`
(Ubuntu 26.04.1 LTS, nginx 1.28.3, systemd running, 4 Drevanta services + mail stack)
Client: `tools/drill/verbctl` speaking the exact helper socket protocol (digest, fence,
journal) as root. Every effect below is journaled in the helper's `host_journal`
(journal IDs cited per drill). Repository head during drills: see git log (G30 waves).

## Environment prepared

- Deployed helper/panel binaries built from the G30 waves; policy allowlist extended
  (`backup.apply`, `tls.apply`, `mail.apply`, `toolchain.apply`, `build.execute`);
  root-owned bindings installed (`backup-`, `tls-`, `mail-`, `build-bindings.json`).
- Panel build boundary configured (`DREVANTA_BUILD_SCRATCH=/srv/drevanta/builds`) —
  the panel routes builds through the privileged isolated executor.
- Mail stack installed for real: postfix 3.10.6, dovecot 2.4.2 (+imapd/lmtpd), opendkim.

## D1 — TLS certificate lifecycle: PASS

| Step | Result | Journal |
| --- | --- | --- |
| issue (self-signed, SAN `drill.drevanta.local` + www) | active, fingerprint `24d91b59…db3c`, NotAfter 2027-02-12, key 0600 | `hj_452c3d67…` |
| renew at 90d validity / 30d threshold | `still-valid` (no-op — correct) | `hj_20921b6d…` |
| recover after corrupting the cert file | fresh pair, fingerprint `10c687fa…`, corrupt file preserved `.invalid` | `hj_b64c928c…` |

`openssl x509` on the issued cert: `subject=O=Drevanta, CN=drill.drevanta.local`,
SANs present, validity as requested. ACME path fails closed (no issuer registered —
public domain mapping is an owner decision); explicit fallback is reported as such.

## D2 — Mail-server provisioning + real delivery: PASS

1. `mail.apply configure_domain` — postfix virtual-domain file written, **real
   RSA-2048 DKIM keypair** generated (`mail.private` 0600 + DKIM TXT record with
   `v=DKIM1; k=rsa; p=…`), postfix/opendkim reloaded. Journal `hj_c512cbf0…`.
2. `mail.apply provision_mailbox` (password via root-owned 0600 file ref; value never
   journaled) — passdb entry `postmaster@mail.drevanta.local` in 0600 users file,
   Maildir cur/new/tmp created, dovecot reloaded. Journal `hj_9357b685…`.
3. Activation wiring (documented runbook `docs/MAIL-ACTIVATION.md`): postfix virtual
   delivery agent (`virtual_mailbox_domains/maps`, uid/gid 65534), dovecot 2.4
   flattened grammar (`passdb_driver = passwd-file`, `passwd_file_path`, lmtp
   listener socket). **Real mail delivered end-to-end**: message routed through the
   provisioned mailbox landed in `Maildir/new/` with
   `Subject: Drevanta G30 mail drill PASS`.
   Mailbox removal is data-preserving (unit-tested); IMAP auth consumption of the
   passdb file is the remaining mail item (see blockers).

## D3 — Runtime toolchain provisioning: PASS

- `toolchain.apply probe php8.5` → present=false (honest probe).
- `toolchain.apply ensure php8.5` → **real apt install through the registered
  runner** → `installed, version "PHP 8.5.4 (cli)"`, verified by the system binary.
  Node22/python3.12/go1.23 share the same pinned catalog; installer absence fails
  closed (unit-tested).

## D4 — Isolated developer build execution: PASS

Untrusted repository content ran **only** inside the verified sandbox:

- Two-phase design: privileged bare fetch (hooks/filters disabled by config, no repo
  content executes) → isolated checkout+digest under `systemd-run --wait` with
  `PrivateNetwork=yes`, `NoNewPrivileges=yes`, `--uid=nobody`,
  `CPUQuota=100% / MemoryMax=1G / TasksMax=256`, allowlisted minimal env (no host
  secrets), concurrency semaphore (2), timeout kill.
- Isolation PROBE before every build; probe failure refuses the build (CI failure
  mode is unit-tested: no systemd → refuse, never run unsandboxed).
- Drill: `bld-drill-14` succeeded, artifact `sha256:bf32cb46…f496`, size 40;
  `bld-drill-15` (identical source) → **identical digest** (determinism).
  `log_ref` is a redacted shape (no URLs). Journals `hj_7b9b283b…`, `hj_…`.
- Unsafe inputs rejected: `ssh://` scheme, traversal, embedded newline (unit-tested).

## D5 — Backup & disaster recovery: PASS (full chain)

| Step | Result |
| --- | --- |
| create `bk-drill-g50-2` (panel+helper DBs incl. **WAL/SHM**, migrations, mail data) | `complete`, envelope sha256 recorded, 87,027 B |
| verify (envelope + plaintext digests) | `verified` |
| off-host push (rsync to `/srv/offhost-store`) | `pushed`; `.enc` + `.manifest.json` present |
| pull into a fresh store | `pulled` |
| restore into a CLEAN environment (empty dir inside a debootstrap rootfs at `/var/lib/machines/drill-restore`) | `restored`, 1,917,213 B |
| in-container verification (systemd-nspawn `--private-network`) | mail domain file, 6 migration files, maildirs present inside the isolated rootfs |
| DB recovery | `panel.db` integrity **ok**, 46 tables; `helper.db` integrity **ok**, 44 journal rows (WAL recovered) |

Earlier artifact `bk-drill-g50-1` also passed create→verify→push→pull→restore
(50,149 B) and exposed the WAL pitfall that `bk-drill-g50-2` corrects. Tampering,
wrong-key, unverified-manifest, non-empty-target and traversal cases are all refusal
paths unit-tested against the same code that ran here.

## D6 — Service reload / restart / rollback: PASS

`unit.apply` on the live nginx binding: `reload` → applied, `restart` → applied,
`revert` (systemd-level configuration rollback) → applied; nginx `active` after all
three. Journals `hj_3a242c82…`, `hj_aac86fd2…`, `hj_5f30c8b9…`.

## D7 — Interrupted-operation recovery: PASS

- Seeded the exact post-crash state (operation `dispatched`, lease expired) for the
  REAL scope/resource/principal, restarted the panel: the reconcile loop re-claimed
  the operation and **completed it through the helper** —
  `D7 RESULT: succeeded`, observation `Ubuntu 26.04.1 LTS`, journal `hj_78a76089…`.
- Security property demonstrated live: the same seed with a NON-existent actor was
  **cancelled** by DispatchGuard — crash recovery never bypasses reauthorization.

## D8 — Failed-upgrade rollback: PASS

Broken binary installed after stop → `drevanta-panel` fails (`Failed with result
'exit-code'`, crash-loop) → known-good binary restored (sha256 verified against the
build) → `active`, `listening on 127.0.0.1:8080`. Rollback artifact retained
(`panel.bak-g30`).

## D9 — Reboot convergence: PASS

Real `systemctl reboot`. After return: helper, panel, web, nginx, postfix, dovecot
all `active`; panel "build execution worker started … listening on 127.0.0.1:8080";
helper serving with the installation binding. No manual intervention.

## D10 — Firewall & SSH safety rollback: PASS

- Pre-armed a timed self-revert (`systemd-run --on-active=120s ufw disable`) BEFORE
  any change — lockout prevention is procedural, not hopeful.
- `ufw default deny incoming`, `allow 22/tcp`, `enable` → **SSH from the sandbox
  succeeded through the active firewall** (new connection).
- After the timer: `Status: inactive` — verified rollback.

## Honest limits

- ACME/Let's Encrypt issuance is implemented as a fail-closed seam; real issuance
  needs an owner-provided public domain (DEC-024).
- IMAP/POP3 authentication against the provisioned passdb file (dovecot 2.4
  userdb wiring) is the remaining mail item — delivery itself is complete.
- Backup off-host target here is an rsync path on the drill host acting as the
  remote store; production targets (S3/object storage or a second site) plug into
  the same `push/pull` verbs.
- The restore-verification ran on the host against the restored artifact and inside
  the isolated rootfs via nspawn reads; a full application boot inside the container
  was not attempted (no runtime installed in minbase).

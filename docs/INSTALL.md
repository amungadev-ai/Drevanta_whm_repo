# Drevanta Manual Installation Guide

Version format: `1.0.0-owneraudit.1+git.<sha>` · Install package:
`drevanta-install-<version>-<commit>.tar.gz` (checksums in
`<package>.tar.gz.sha256` and the internal `SHA256SUMS`).

## Requirements

| Item | Minimum | Recommended |
| --- | --- | --- |
| OS | Ubuntu 22.04/24.04/26.04 (amd64) | Ubuntu 26.04 LTS |
| CPU | 2 vCPU | 4 vCPU |
| RAM | 4 GB | 8 GB |
| Disk | 40 GB | 80 GB SSD |
| Software | systemd, curl, python3, tar, Node.js ≥ 20, nginx (auto-installed if missing and declared) | — |

**Ports (loopback unless noted):** API `127.0.0.1:8080`, console `127.0.0.1:3100`,
nginx `127.0.0.1:80` (or `--nginx-port`). Optional services when activated:
mail 25/587/143/993/110/995, DNS 53, cache 16379 (loopback), payment-callback
`0.0.0.0:<port>` (callback path only). SSH 22 for administration/tunnels.

**Dependencies:** declared deps are auto-installed by the installer (nginx,
Node.js 20 via NodeSource). Independently activatable services (installed on
demand through the platform when used): mail stack (postfix/dovecot), DNS
(nsd), Redis/Valkey, OCI containers (podman), toolchains. External
configuration (not installable by Drevanta): publicly trusted TLS certificate
(domain required), payment gateway credentials, SMTP relay if used.

## Install

```bash
sha256sum -c drevanta-install-*.tar.gz.sha256
tar -xzf drevanta-install-*.tar.gz && cd drevanta-install-*
sudo ./install.sh                       # default instance
# isolated coexisting instance:
sudo ./install.sh --instance test --nginx-port 8443 --web-port 3101 --api-port 8082
```

The installer prints `[OK]/[FAIL]` per step and **exits non-zero listing the
failed steps if anything fails** — it never reports success for a failed
operation. On success it prints the access URLs and the claim-token file
path. What it does: creates an isolated system user and directories,
installs binaries + console bundle + migrations, runs the one-shot
bootstrap (creates the installation record and a **single-use claim token,
24 h TTL**, stored root-only at `$ETC/claim-token`), installs and starts
`drevanta[-test]-{helper,panel,web}` systemd units, and adds a loopback-only
nginx vhost. Remote access is by SSH tunnel; no admin authentication is
exposed over plain HTTP on public interfaces.

## First-run onboarding (performed by YOU)

1. `ssh -L 8443:127.0.0.1:8443 <user>@<host>` then browse
   `http://localhost:8443/`.
2. `sudo cat /etc/drevanta-test/claim-token` on the host (single-use;
   `sudo shred -u` the file after claiming).
3. Follow `/onboarding`: claim (create the initial administrator — this
   account has legitimate platform-wide administrative authority through the
   platform's authorization model), recovery codes, access address,
   sign-in, MFA/security setup, dashboard.

## Upgrade

```bash
sudo systemctl stop drevanta-test-panel drevanta-test-web drevanta-test-helper
tar -xzf drevanta-install-<new>.tar.gz && cd drevanta-install-<new>*
sudo ./install.sh --instance test --nginx-port 8443 --web-port 3101 --api-port 8082
```

The installer is idempotent: it re-deploys binaries/bundles and restarts
units; database data in `/var/lib/drevanta-test` is preserved (migrations
are forward-only).

## Uninstall

```bash
sudo systemctl disable --now drevanta-test-{panel,web,helper}
sudo rm -f /etc/systemd/system/drevanta-test-* /etc/nginx/sites-{available,enabled}/drevanta-test*.conf
sudo rm -rf /opt/drevanta-test /etc/drevanta-test /var/lib/drevanta-test /run/drevanta-test /var/log/drevanta-test
sudo userdel drevanta-test && sudo systemctl daemon-reload
```

## Backup & recovery

- Files: `/var/lib/drevanta-<inst>/panel.db` + `helper.db` (SQLite, WAL) —
  back up with the instance stopped or using SQLite online-backup semantics.
- Platform-managed backups: the platform's own backup subsystem covers
  websites/databases/configurations (see the in-app Backups area); host
  artifacts live under `/srv/drevanta` (on hosts where the helper manages
  them).
- Recovery: reinstall the same version, stop units, restore the DB files
  into `/var/lib/drevanta-<inst>/`, start units. The installation record
  (and claim state) lives inside `panel.db`.

## Troubleshooting

- Service state: `systemctl status drevanta-<inst>-{helper,panel,web}`;
  logs: `journalctl -u <unit>`.
- Panel not healthy: check migrations ran (journal), DB path permissions
  (owned by the instance user).
- Helper rejected request: policy file `$ETC/helper-policy.json` (root-owned
  allowlist; verbs/targets), socket group membership, installation-id match.
- Console down: `journalctl -u drevanta-<inst>-web`; Node ≥ 20 required.
- Claim token lost: wipe `panel.db` and re-run the installer (fresh install;
  do not attempt to mint tokens by hand).
- Port conflicts: rerun with `--nginx-port/--web-port/--api-port`.

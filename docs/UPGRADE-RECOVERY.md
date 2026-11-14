# Upgrade, Backup & Recovery

## Method E — version-aware in-place upgrade

```bash
tar -xzf drevanta-install-<new-version>-<commit>.tar.gz
cd drevanta-install-<new-version>-<commit>
sudo ./install.sh --upgrade                       # default instance
sudo ./install.sh --instance test --upgrade       # named instance
```

What the engine does, in order:

1. **Detection & separation** — an existing installation without `--upgrade`
   is refused (`refusing to modify`); fresh install and upgrade are explicit,
   separate paths. A claimed installation is never reset and never receives a
   new owner claim token.
2. **Backup (after services are stopped — consistent SQLite state)** —
   `/var/backups/drevanta/upgrade-<UTC-timestamp>/` (0700) receives: data +
   configuration (`data-etc.tar.gz`), **binaries (`bin.tar.gz`)**, **frontend
   bundle (`web.tar.gz`)**, **migrations (`migrations.tar.gz`)** and
   **systemd/nginx state + VERSION (`system-nginx.tar.gz`)** — all 0600.
   The rollback restores every archive to its correct target (bin/web/
   migrations into the installation directory) and re-runs health checks;
   rollback success is reported only when the previous version is actually
   restored and healthy — otherwise `rollback INCOMPLETE` is printed and
   the exit stays non-zero.
3. **Replace** — services stopped; binaries, console bundle, migrations and
   policy templates replaced.
4. **Migrate & start** — services restarted; the panel applies forward-only
   migrations at startup.
5. **Health gates** — control plane (`/api/v1/install/status`), console,
   unit activation (`systemctl is-active`) and the helper socket are
   polled; on any failed gate the previous version is **restored from the
   backups and restarted** automatically, post-rollback health is verified,
   and the upgrade exits non-zero with the failing steps listed.

Idempotent and retry-safe: a failed upgrade leaves either the new version
healthy or the previous version running — never an undefined state.
Compatibility: the installed and target versions must share the same major
version; a major-version change is refused with a pointer to the documented
migration path. An exclusive installation lock (`/var/lock/<svc>-install.lock`)
prevents concurrent installs/upgrades. Customized policy files
(`/etc/drevanta[-<inst>]/*.json*`) are preserved — never overwritten — by
upgrades.

## Backups

| What | Where | Notes |
| --- | --- | --- |
| Control-plane DB | `/var/lib/drevanta[-<inst>]/panel.db` | SQLite WAL; back up with units stopped or via SQLite online-backup semantics |
| Helper DB | `/var/lib/drevanta[-<inst>]/helper.db` | same |
| Configuration/policy/TLS | `/etc/drevanta[-<inst>]/` | includes helper policy, claim-token file, TLS key/cert (all root-only) |
| Upgrade archives | `/var/backups/drevanta/upgrade-<ts>/` | created automatically by `--upgrade` |
| Platform-managed website/database backups | in-app Backups area | host artifacts under `/srv/drevanta` where the helper manages them |

Recommended schedule: nightly copy of `panel.db` + `helper.db` +
`/etc/drevanta` to off-host storage (e.g. `sqlite3 panel.db ".backup ..."`
from a cron/systemd timer as root).

## Recovery

- **Bad upgrade (auto-rollback failed):** stop units, extract the upgrade
  archive over `/`, restart units:
  ```bash
  sudo systemctl stop drevanta-{panel,web,helper}
  sudo tar -xzf /var/backups/drevanta/upgrade-<ts>/data-etc.tar.gz -C /
  sudo systemctl start drevanta-{panel,web,helper}
  ```
- **Lost databases:** reinstall the same version, stop units, restore the
  DB files into `/var/lib/drevanta[-<inst>]/`, chown to the instance user,
  start. Installation record and claim state live inside `panel.db`.
- **Lost claim token (installation still pending):**
  `sudo drevanta-claim-token --regenerate` — issues a fresh single-use
  token (hash-only storage, 24 h window restarts). A claimed installation
  refuses regeneration by design.
- **Lost TLS key/cert:** rerun the engine with `--upgrade`; the certificate
  is re-issued if missing (fingerprint changes — verify the new one).
- **Full host loss:** provision a fresh host, install (Method A/B), then
  restore DBs/config as above; the installation record restores the claim
  state, so no re-claim of an already-claimed installation occurs.

## Uninstall

```bash
sudo systemctl disable --now drevanta[-<inst>]-{panel,web,helper}
sudo rm -f /etc/systemd/system/drevanta[-<inst>]-* \
          /etc/nginx/sites-{available,enabled}/drevanta[-<inst>]*.conf \
          /usr/local/bin/drevanta-claim-token
sudo rm -rf /opt/drevanta[-<inst>] /etc/drevanta[-<inst>] \
            /var/lib/drevanta[-<inst>] /run/drevanta[-<inst>] \
            /var/log/drevanta[-<inst>]
sudo userdel drevanta[-<inst>] && sudo systemctl daemon-reload
```

Uninstall destroys data — back up first. The installer itself never removes
data during upgrades or reruns.

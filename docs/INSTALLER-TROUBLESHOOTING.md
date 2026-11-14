# Installer Troubleshooting

Symptom-ordered, matching actual engine behavior. Every gate prints
`[INFO]/[OK]/[WARN]/[FAIL]`; failed mandatory steps make the installer exit
non-zero with the failing steps listed — the list below maps the most common
failures to causes and fixes.

## Bootstrap (Method A/B retrieval stage)

| Symptom | Cause | Fix |
| --- | --- | --- |
| `release <tag> not reachable` / token prompt | private repository | provide a GitHub token with Contents: Read (hidden prompt, or `export DREVANTA_GH_TOKEN`); never embed it in the URL |
| `unattended mode: export DREVANTA_GH_TOKEN` | unattended run without credentials | set `DREVANTA_GH_TOKEN` in the environment; the bootstrap never prompts noninteractively |
| `SHA-256 MISMATCH against trusted release manifest` | corrupted/tampered artifact or broken mirror | rerun (safe: nothing was modified); if it repeats, re-cut or re-mirror the release |
| `artifact is corrupted (tar extraction failed)` | truncated download | rerun; the bootstrap retries downloads twice automatically |
| `no drevanta-install-*.tar.gz asset found` | wrong `--version` tag or incomplete release | check the tag exists (`gh release view <tag>`) and assets are uploaded |
| `architecture … not supported` | non-amd64 host | artifacts are linux/amd64; arm64 builds are not produced yet |
| `apt-get not found` | non-Debian family | supported: Ubuntu 20.04+, Debian 11+ |

## Engine (installation stage)

| Symptom | Cause | Fix |
| --- | --- | --- |
| `run as root (sudo)` | installer run unprivileged | `sudo ./install.sh …` |
| `existing installation detected … refusing to modify` | rerun over an existing install | add `--upgrade` (data preserved) or use `--instance <name>` for a coexisting install |
| `available memory … below the 400MB floor` | undersized VPS | use a ≥1 GB instance |
| `disk: … below … 3000MB required` | insufficient `/var` space | free space or extend the volume |
| `port N already in use` | port conflict | choose other ports: `--mgmt-port/--nginx-port/--web-port/--api-port` |
| `control plane did not become healthy` | panel failed to start/migrate | `journalctl -u drevanta[-<inst>]-panel`; check DB permissions |
| `console not healthy` | Node missing/old or bundle issue | `journalctl -u drevanta[-<inst>]-web`; engine auto-installs Node ≥ 20 (apt → NodeSource) |
| `management endpoint not healthy locally` | nginx config/TLS issue | `nginx -t && journalctl -u nginx`; check `$ETC/tls/mgmt.{crt,key}` |
| `no active ufw detected` (warning) | ufw inactive | not an error: ensure the provider firewall/security group allows TCP 8443; the installer never enables ufw |
| `could not determine the server access address` | no default route / odd networking | pass `--access-ip <public-ip>` explicitly |
| bootstrap: `did not produce installation + claim token` | panel bootstrap failed | `journalctl -u drevanta[-<inst>]-panel`; DB path owned by the instance user |

## Post-install

| Symptom | Cause | Fix |
| --- | --- | --- |
| browser cannot reach `https://<ip>:8443` | provider firewall/security group, or NAT | open TCP 8443; verify with `ss -ltn sport = :8443`; the installer cannot verify external reachability from inside the host |
| certificate warning in browser | expected (installation-issued cert) | verify the SHA-256 fingerprint against the summary before proceeding |
| onboarding URL expired | 24 h TTL passed | `sudo drevanta-claim-token --regenerate` (only before claiming), then `--url` |
| `no outstanding claim token` from the CLI | installation already claimed | correct: the token is single-use and consumed |
| lost token before claiming | — | `sudo drevanta-claim-token --regenerate`; a claimed installation refuses regeneration |
| must redo access from a private network | — | rerun engine with `--private-mgmt` via `--upgrade`, or bind only the tunnel vhost |

## Diagnostics bundle

```bash
sudo systemctl status drevanta-{helper,panel,web}
sudo journalctl -u drevanta-panel -n 200 --no-pager
sudo cat /etc/drevanta/install-report.json          # token-free machine report
sudo tail -n 200 /var/log/drevanta/install-*.log    # claim tokens redacted
sudo nginx -t && sudo ss -ltn | grep -E ':(80|8443|3100|8080)'
```

Never paste claim tokens into bug reports; the installation log is safe to
share (tokens are redacted), the claim-token file is not.

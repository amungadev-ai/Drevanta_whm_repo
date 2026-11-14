# Drevanta Installation Guide

This guide covers every supported installation method. All methods share the
same underlying installation engine (`engine/install.sh` in the repository,
shipped as `install.sh` inside every release artifact) — there is one
installer implementation, not one per method.

## Simplified public installation (customer channel)

A short public-channel installation is prepared:
`curl -fsSL https://github.com/amungadev-ai/drevanta-distribution/releases/latest/download/bootstrap.sh | bash` —
see `docs/DISTRIBUTION.md` (architecture, trust chain, publication runbook)
and `docs/INSTALL-CLIENT.md` (the customer-facing guide, including the
download-and-verify alternative). The private-repo methods below remain the
engineering path and are unchanged.

## Installation methods (permanent priority order)

| Method | Use when | Entry point | Status |
| --- | --- | --- | --- |
| **A — Direct GitHub installation** | Standard customer journey (primary) | `install.sh` at the repository root, executed in one command | Implemented; verified in isolated harness (see `scripts/verify_installer.sh`, 90 checks incl. negative tests) |
| **B — Downloadable installation package** | No direct GitHub retrieval desired | release artifact `drevanta-install-*.tar.gz` + `sha256sum -c` + `./install.sh` | Implemented; verified on a prior fresh Ubuntu 26.04 host and in the harness |
| **C — Offline installation** | Air-gapped/restricted host | same artifact, engine `--offline` skips network stages | Implemented; dependency auto-install skipped, preinstalled deps required |
| **D — Developer installation** | Building from source | build the artifact with `scripts/build_install_package.sh`, install it (Method B flow) | Implemented; used for every release |
| **E — Upgrade** | Existing installation | engine `--upgrade` (backup → replace → migrate → health → rollback) | Implemented; verified in the harness; independent-VPS upgrade run still outstanding |

Outstanding engineering (disclosed, not claimed): automated ACME certificate
issuance/renewal for custom management domains; artifact code-signing
beyond SHA-256 manifest verification (needs a signing-key ceremony);
independent fresh-VPS verification of the full Method A journey (the owner
acceptance test performs this).

## Requirements

| Item | Minimum | Recommended |
| --- | --- | --- |
| OS | Debian family: Ubuntu 20.04+, Debian 11+ (linux/amd64) | Ubuntu 26.04 LTS |
| CPU | 2 vCPU | 4 vCPU |
| RAM | 1 GB (installer refuses below 400 MB available) | 4 GB |
| Disk | 3 GB free under `/var` | 40 GB SSD |
| Network | Outbound HTTPS for release retrieval (Method A/B) and apt | — |
| Software | systemd; curl, tar, python3 (bootstrap retrieves them via apt if missing); nginx and Node.js ≥ 20 are auto-installed by the engine | — |

**Ports:** management `0.0.0.0:8443/tcp` HTTPS (configurable `--mgmt-port`),
loopback HTTP `127.0.0.1:80` (SSH-tunnel target, configurable
`--nginx-port`), API `127.0.0.1:8080`, console `127.0.0.1:3100` (both
loopback-only, never exposed). Optional payment-callback vhost
`0.0.0.0:<port>` (callback path only). SSH 22 for administration. Ports 80
and 443 remain free for hosted websites and domain routing.

## Method A — direct GitHub installation (primary)

```bash
curl -fsSL https://raw.githubusercontent.com/amungadev-ai/Drevanta_whm_repo/<pinned-release-ref>/install.sh -o install.sh && sudo bash install.sh
```

While the repository is **private**, retrieval needs a GitHub token with
**Contents: Read**. The token is never embedded in URLs, arguments, shell
history or logs: the bootstrap reads it silently if it cannot fetch the
release, and passes it to curl via a root-only header file that is deleted
immediately after use. Export `DREVANTA_GH_TOKEN` beforehand (unattended
runs require this), or paste it at the hidden prompt.

What the bootstrap does, in order: platform validation → release resolution
(`--version <tag>`, default: the pinned development release, bound to its
exact full commit) → manifest retrieval **from the exact selected release's
asset collection** → manifest validation (schema, product, channel, tag,
commit, version, artifact identity — **mandatory, there is no fallback
verification path**) → artifact download → **exact SHA-256 match against the
trusted manifest** → archive-safety validation (`tar -tvz`: absolute paths,
path traversal, link targets escaping the extraction root and special
entries are rejected) → extraction into a private 0700 mktemp workspace →
internal `SHA256SUMS` verification **with complete file coverage** →
credential scrub from the environment → the shared engine runs as a child
process (the bootstrap's cleanup always fires) → workspace removal
(`--keep` preserves the artifact but never credentials). The customer never
compiles from source. Retrieval is HTTPS-only; the token lives in a 0600
header file deleted on success, failure and interruption, and is unset
before the engine (and everything it starts) inherits the environment.

For the stable channel (after owner approval), the same command form will
point at the official stable release reference; explicit version selection
and release provenance stay identical.

## Methods B/C — package and offline

```bash
sha256sum -c drevanta-install-<version>-<commit>.tar.gz.sha256   # must print OK
tar -xzf drevanta-install-<version>-<commit>.tar.gz && cd drevanta-install-<version>-<commit>
sudo ./install.sh                 # fresh install, management HTTPS on :8443
sudo ./install.sh --offline      # air-gapped host (deps must be preinstalled)
```

Package integrity (Method B/C): verify the sidecar
(`sha256sum -c <package>.tar.gz.sha256`) before extraction; the engine then
self-verifies its own tree at startup (internal `SHA256SUMS` **including
complete file coverage**) as a mandatory gate — the same gate the bootstrap
enforces for Method A. Self-hosted mirrors can substitute the
API base with `DREVANTA_API_BASE` for verified mirror scenarios.

## Method D — developer installation

```bash
bash scripts/build_install_package.sh          # gates: gofmt, go vet, go test
# then install the produced artifact exactly as Method B
```

## Engine options

```
--mgmt-port <port>      public HTTPS management port (default 8443)
--access-ip <ip>        override the detected server access address
--mgmt-domain <name>    additional DNS SAN on the management certificate
--nginx-port <port>     loopback HTTP tunnel target (default 80)
--private-mgmt          no public management listener (tunnel/VPN access only)
--public-callback-port  payment-provider callback vhost (callback path only)
--unattended            suppress token material in output (also DREVANTA_UNATTENDED=1)
--offline               no network stages
--upgrade               version-aware in-place upgrade (Method E)
--instance <name>       coexisting isolated installation
```

Every gate prints `[INFO]/[OK]/[WARN]/[FAIL]`; a failed mandatory gate makes
the installer exit non-zero and list the failed steps. It never reports
success for a failed or unexecuted step. A persistent installation log is
written to `/var/log/drevanta[-<instance>]/install-<timestamp>.log` with
claim tokens redacted, and a machine-readable (token-free) report to
`/etc/drevanta[-<instance>]/install-report.json`.

## What the engine configures

System user and directories (`/opt/drevanta`, `/etc/drevanta`,
`/var/lib/drevanta`, `/run/drevanta`, `/var/log/drevanta`; per-instance
variants), binaries (helper privileged + panel control plane), SvelteKit
console bundle, SQLite migrations (run at panel start and by the one-shot
bootstrap), systemd units `drevanta[-<inst>]-{helper,panel,web}`
(hardened: `NoNewPrivileges`, `ProtectSystem=strict`, private tmp), nginx
loopback vhost (tunnel target) and the public HTTPS management vhost
(TLSv1.2/1.3, security headers, `no-store`, self-origin CSP), an
installation-specific TLS certificate (RSA-2048, 825 days, the server IP as
subjectAltName; key root-only 0600), narrowly scoped firewall handling (ufw
is allowed only if it is ALREADY active — the installer never enables a
firewall or touches SSH), port-conflict detection, the root-only claim-token
retrieval command, and final health verification of panel, web, helper and
the management endpoint (local verification only — external reachability is
explicitly NOT claimed and must be confirmed from the browser).

## First-run onboarding (performed by the owner)

The completion summary prints the primary onboarding URL — **only to an
authorized interactive terminal; any non-interactive run suppresses token
material automatically** (`sudo drevanta-claim-token --url` retrieves it):

```
https://<server-ip>:8443/#<single-use-claim-token>
```

Ctrl+Click it (or type it manually). The approved onboarding interface reads
the claim token from the URL fragment, validates its format, auto-populates
the existing claim-token field, and immediately removes the token from the
address bar (`history.replaceState`) — no reload, no referrer, no storage.
The owner then completes onboarding personally: review the populated field →
continue → create the initial administrator → save recovery codes offline →
access address → sign-in → MFA/security setup → dashboard. Opening the link
never submits the token or creates an account; the token is consumed only by
the explicit claim step (`POST /api/v1/install/claim`, atomic, single-use,
24 h TTL). Manual claim-token entry remains available: the field accepts
`clm_…` tokens or a full onboarding link.

SSH-tunnel alternative (Priority 2):

```bash
ssh -N -L 8080:127.0.0.1:80 root@<server-ip>     # from your local computer
# then open http://localhost:8080/#<single-use-claim-token>
```

## Claim-token security and lifecycle

- Cryptographically random (crypto/rand), issued once per installation,
  stored **hash-only** in the database; the plaintext exists only in the
  root-only file `/etc/drevanta[-<inst>]/claim-token` (0600) and the
  authorized interactive terminal.
- Single-use, atomic consumption, 24 h validity; expired/malformed/consumed
  tokens are rejected (HTTP 401).
- Retrieval: `sudo drevanta-claim-token` (token + guidance),
  `--url` (full onboarding URL), `--regenerate` (authorized recovery before
  claiming; a claimed installation refuses regeneration — the owner claim is
  final).
- Token-bearing URLs are confidential: never share them; beware terminal
  scrollback, screenshots, clipboard history and browser extensions. They
  never appear in persistent logs (redacted), journals, diagnostics,
  CI output or analytics.
- An already claimed installation never receives a new owner claim token
  during an installer rerun; fresh installs on an existing database are
  refused without `--upgrade`.

## Certificate verification

The management certificate is issued by this installation, not a public CA —
the browser warning is expected. Before entering credentials or the claim
token, compare the certificate SHA-256 fingerprint shown by the browser with
the `TLS fingerprint` line in the installation summary (they must match
exactly). Do not bypass the warning blindly. Replacement with a publicly
trusted certificate (custom management domain) is covered in
[MANAGEMENT-ACCESS-TLS.md](MANAGEMENT-ACCESS-TLS.md).

## Upgrade, uninstall, backup & recovery

See [UPGRADE-RECOVERY.md](UPGRADE-RECOVERY.md) (Method E details, backups,
rollback, recovery) and the uninstall/backup commands there.

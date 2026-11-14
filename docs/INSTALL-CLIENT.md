# Installing Drevanta

## The standard installation (recommended)

1. SSH to your server as root:

   ```bash
   ssh root@your-server-ip
   ```

2. Enter your password when prompted.

3. Run **one command**:

   ```bash
   curl -fsSL https://github.com/amungadev-ai/drevanta-distribution/releases/latest/download/bootstrap.sh | bash
   ```

4. Wait for the automated installation. The installer validates your system,
   installs and hardens every component, verifies release authenticity and
   integrity cryptographically, and starts the services. You will see
   `INSTALLATION COMPLETE` when it is done.

5. Open the printed onboarding URL in your browser — the server address in it
   is clickable, and the single-use claim token is already embedded in the
   link (the token field fills automatically and the token is removed from
   the address bar immediately; you only choose your administrator email and
   password, review the pre-filled value, and submit).

6. You are on the dashboard. Setup takes about two minutes.

You never need a GitHub account, an access token, a checksum, a compiler or
a manual package download. Everything mandatory — dependency installation,
artifact verification, preflight validation, service hardening and health
checks — is automated.

## Requirements

- Ubuntu 20.04+ or Debian 11+, root access, amd64.
- At least 3 GB free disk space and 1 GB RAM.
- Outbound HTTPS. The installer itself needs no credentials of any kind.

## Options

Append options after the command when running the downloaded file, or use
`curl ... | bash -s -- <options>`:

| Option | Purpose |
| --- | --- |
| `--version <tag>` | Install an explicit release version instead of the latest stable |
| `--private-mgmt` | Management interface on loopback only (SSH tunnel access; strongest posture) |
| `--web-port/--api-port/--nginx-port/--mgmt-port <n>` | Custom ports |
| `--instance <name>` | Additional isolated installation on one server |
| `--upgrade` | Upgrade an existing installation (automatic verified backup + rollback) |
| `--artifact <path>` | Offline installation from a locally downloaded package |

## Upgrading

```bash
curl -fsSL https://github.com/amungadev-ai/drevanta-distribution/releases/latest/download/bootstrap.sh | bash -s -- --upgrade
```

The upgrade stops services without suppressing failures, verifies database
integrity, takes complete verified backups, replaces the software only when
every gate passes, and rolls back automatically (including nginx reload and
service verification) if anything fails.

## Offline installation

On a machine with internet, download from the release page:
`bootstrap.sh`, `SHA256SUMS`, `SHA256SUMS.sig`, the
`drevanta-install-*.tar.gz` artifact and `MANIFEST.json`. Transfer them to
the server into one directory, then run:

```bash
bash bootstrap.sh --artifact /path/to/drevanta-install-*.tar.gz
```

The same cryptographic verification runs; no internet connection is needed
on the server.

## Management access priorities

1. Direct server IP over HTTPS on port 8443 (default): the onboarding URL
   uses it. The TLS certificate is self-signed and its SHA-256 fingerprint
   is printed at installation — compare it in your browser once.
2. SSH tunnel: `ssh -N -L 8080:127.0.0.1:80 root@your-server-ip`, then
   `http://localhost:8080/`.
3. Optional custom management domain (advanced; see the main documentation).

## Claim token and recovery

The single-use claim token is valid for 24 hours and shown once. If it
expires or is lost before onboarding, regenerate it on the server:

```bash
sudo drevanta-claim-token --regenerate
```

This works even when the previous token expired or its record was lost.
Retrieval of the current token:

```bash
sudo drevanta-claim-token        # token + guidance
sudo drevanta-claim-token --url  # the full clickable onboarding URL
```

Token material is only ever displayed in an authorized interactive terminal —
never in logs, diagnostics or non-interactive output.

# DREVANTA — AGENT HANDOFF (2026-11-14, post-dev.6)

Read this fully before touching anything. It is the complete operating state
of the project as transferred. Nothing here requires asking the owner —
every open question has a documented decision or a default.

## 1. What the project is

Drevanta WHM — a hosting/infrastructure management platform:

- **Go control plane** (`cmd/panel`, `cmd/helper`, `internal/...`): panel
  (loopback API :8080, SQLite via migrations, build-execution worker), a
  privileged root helper (unix socket, verb registry: build.execute with a
  systemd-run isolation sandbox, toolchain, mail, DNS, containers, backups).
- **SvelteKit console** (adapter-node, `ui/`), served by Node on :3100 behind
  nginx (:80 loopback tunnel vhost, :8443 TLS management vhost).
- **Installation engine** (`engine/install.sh`) + **distribution pipeline**
  (`scripts/`): deterministic bootstrap generation, signing, publication to a
  public distribution repo, extensive verification suites.
- **Browser one-click onboarding**: a pending owner claim token + onboarding
  URL (confidential; shown once in an interactive terminal, retrievable via
  `sudo drevanta-claim-token`).

## 2. Repositories and current heads

| Repo | Role | State |
| --- | --- | --- |
| `amungadev-ai/Drevanta_whm_repo` | private/working repo | branch `feat/g40-frontend-migration`, HEAD `88e26c5` (dev.6 round complete) |
| `amungadev-ai/drevanta-distribution` | public distribution repo | main @ `936f0de` (README with dev.6 + recovery), release `v1.0.0-owneraudit.1-dev.6` (9 assets), release `v1.0.0-owneraudit.1-dev.5` flagged **prerelease + "FAILED — do not install"** |

CI on the private repo: `.github/workflows/distribute.yml` — green on all
current commits.

## 3. Published release (source of truth is GitHub, not any sandbox)

- **Current**: `v1.0.0-owneraudit.1-dev.6` @ commit `2777f9567c41`
  - bootstrap.sh digest `a63a98db873318e29d9e149864d9856f1684d3922790ab60adf467dbb9e14c81`
  - artifact `drevanta-install-1.0.0-owneraudit.1-2777f95.tar.gz`, sha256
    `b991b53939d5d4bef2a4a2390af8ac14ab4b0bacaeebfc78329a759bef0bda36`
  - 9 assets: bootstrap(+sha256), SHA256SUMS(.sig), MANIFEST.json, artifact(+sha256), INSTALL-CLIENT.md, VERIFICATION.md
- **Signing key fingerprint** (public key in the repo `keys/` and embedded in
  the bootstrap): `91eb6e3b73c57eb252214f319fb9b6e9a816c8f515bdb463a59d552d46bfa582`
- **dev.5 is recorded FAILED** (`docs/OWNER-INSTALL-DEV5-FAILED.md`): its
  installer had NO dependency provisioning (fresh Ubuntu 26.04 VPS: nginx/
  node/npm missing, web unit 68× 203/EXEC loop, build boundary unconfigured).
  Its release is retained for provenance only.

The `/releases/latest/download/...` path resolves to dev.6 (verified). The
redirect is CDN-cached for some minutes after a new publication — the
explicitly versioned URL is always authoritative.

## 4. Credentials and key management (procedures, never values)

- **GitHub access**: the owner supplies a token in-session. Do NOT print,
  store, or commit tokens (a PAT was used this round; it lives only in the
  conversation, and as a write-only Actions secret).
- **DIST_TOKEN** Actions secret on the private repo: write-only, stored via
  `scripts/store_dist_token_secret.py`. CI uses it to publish.
- **Signing key**: generated per-round by `scripts/gen_signing_key.sh`; the
  public key is committed to the distribution repo `keys/`; the PRIVATE key
  lives (a) in `/tmp/distkeys-v2/` (sandbox — EPHEMERAL) and (b) write-only
  in the `DREVANTA_DIST_SIGNING_KEY` / `DREVANTA_DIST_PUBLIC_KEY` Actions
  secrets, stored via `scripts/store_signing_secrets.py`.
- **Rotation rule (binding)**: if the sandbox-local key is gone when you need
  to SIGN a new release, ROTATE (generate + store new secrets + commit the
  new public key) BEFORE publishing. Rotation before any publication has been
  the accepted pattern; never publish with an unverified key.
- The CI pipeline itself re-stores the key from secrets per run, so CI
  publication works without any local key.

## 5. What the previous round (dev.6) delivered — all verified

1. Engine corrections (`engine/install.sh` @ `46ff1b1`):
   - Stage 1.5 mandatory dependency provisioning (nginx nodejs npm curl git)
     via apt in EVERY online mode; a local `--artifact` never implies
     offline; explicit `--offline` strictly verifies and aborts before any
     deployment; node >= 20 gate; container/userns apt-sandbox fallback
     (disclosed at install time).
   - systemd units rendered from functions (upgrades/recovery re-render the
     current templates), `StartLimitBurst=5` on all three units,
     `ConditionPathExists=/usr/bin/node` on the web unit.
   - Build isolation boundary wired: panel `DREVANTA_BUILD_SCRATCH`,
     root-allowlisted `build-bindings.json`, systemd-run probe (fail-closed).
   - `--recover` mode: preserves data/TLS/identity/claim; installs missing
     deps; re-creates nginx config; restarts frontend; verifies every
     management access path (`RECOVERY COMPLETE`).
2. Distribution pipeline hardening (`cfcfaf3`): uploads.github.com asset
   host, release-reuse on 422, dev-channel latest policy (prerelease flag
   OFF + make_latest), credential-helper exec bit.
3. Verification suites: `verify_installer.sh` (131 checks, green),
   `verify_real_provisioning.sh` (T7: real apt fetch, real nginx/panel/node
   processes, real HTTP 200/302/302, engine recovery verification),
   `verify_release_anon.sh` (literal customer commands, fresh flow 51 gates,
   recovery flow RECOVERY COMPLETE, token-free output).
4. Docs: failure register, client recovery + provisioning sections, dist
   README, `docs/DISTRIBUTION.md` publication record + latest policy,
   KNOWN-LIMITATIONS v6 (items 25–34 — READ IT; it discloses every simulated
   boundary: systemd supervision stubbed, root-run identity in the userns,
   dpkg unpack not runnable in the userns, upgrade health-gate race,
   latest-redirect cache, logger early-line race).

## 6. Owner's standing directives (BINDING)

- No protected-branch merges; no stable release; no production M-Pesa.
- No unrelated engineering work; the approved frontend design is frozen.
- The owner personally runs installs/recovery on their VPS — never access or
  modify their VPS.
- Preserve the short public install command + one-click browser onboarding.
- Development releases only until the owner declares production.
- Immediate objective: successful automated install, browser access, manual
  owner onboarding (the owner is running the dev.6 recovery command on their
  real VPS — their acceptance result is the next input).

## 7. Fresh-agent first-hour checklist

```bash
# 1. Clone + establish baseline truth (all suites are self-contained)
git clone https://github.com/amungadev-ai/Drevanta_whm_repo.git && cd Drevanta_whm_repo
git checkout feat/g40-frontend-migration
bash scripts/verify_installer.sh          # expect: 131 passed, 0 failed
bash scripts/verify_distribution.sh       # release-channel gates
# 2. Verify the published release anonymously (no token)
curl -fsSL https://github.com/amungadev-ai/drevanta-distribution/releases/latest/download/bootstrap.sh | sha256sum
#    expect a63a98db873318e29d9e149864d9856f1684d3922790ab60adf467dbb9e14c81
# 3. Check for a local signing key (ephemeral)
ls /tmp/distkeys-v2/ || echo "rotate per Section 4 before any signing"
# 4. Read docs/: KNOWN-LIMITATIONS (v6), OWNER-INSTALL-DEV5-FAILED.md,
#    DISTRIBUTION.md, OWNER-AUDIT-FINDINGS.md, DECISIONS.md, BLOCKERS.md
```

## 8. Sandbox quirks and engineering footguns (learned the hard way)

- **Sandbox recycles between sessions**: local commits, /tmp (signing keys,
  evidence logs, delivery zips) can vanish. PUSH after every meaningful
  commit; durable state = GitHub only. (This handoff was written after a
  recycle that cost the local checkout and the dev.6 delivery zip — both
  re-derived from remotes.)
- **bash heredoc-functions called directly inside the engine's upgrade
  branch crash with SIGSEGV** (interpreter-level; worked around by wrapping
  the unit-rendering calls in an explicit subshell — see the NOTE comment in
  `engine/install.sh`). Do not remove that subshell.
- **`grep -q` + `set -o pipefail` SIGPIPE race**: `cmd | grep -q` can report
  failure on a MATCH (early exit → SIGPIPE → 141). Capture output first
  (`OUT="$(cmd)"` + `[[ "$OUT" == *pat* ]]`). This bit the recovery listener
  checks and the upgrade backup step.
- **Suite workspaces eat disk**: `/tmp/drevanta-verify.*` accumulate
  ~222 MB each; disk-full mid-build produces CORRUPT artifacts that pass
  `bash -n`. Clean old `/tmp/drevanta-verify.*` before big runs; always run
  `build_install_package.sh` + verify the artifact self-check after.
- **Artifact is the test input**: suites extract `drevanta-install-*.tar.gz`
  from the repo root — REBUILD it after engine edits or you test a stale
  engine.
- **pkill self-match**: `pkill -f "<pattern>"` from a shell whose own
  command line contains the pattern kills your session.
- **Userns limits in verification harnesses**: one uid mapped (identity
  stubs), dpkg unpack impossible for libc packages (extract real .debs),
  real-root-owned dirs unwritable (tmpfs-mount /etc /opt /run /var).
- **GitHub `/releases/latest` redirect cache** lags minutes behind a new
  release; the API's `/releases/latest` is immediate.
- Engine logger starts before its log file exists: the first lines of a run
  may miss the persistent install log (pre-existing, cosmetic).

## 9. Where everything lives (doc index)

`docs/` in the private repo: `INSTALL` (client-facing), `INSTALL-CLIENT.md`,
`OWNER-ACCESS`, `OWNER-ACCEPTANCE-TESTING`, `OWNER-AUDIT-FINDINGS`,
`OWNER-INSTALL-DEV5-FAILED` (the FAILED record), `INTEGRATION-READINESS`,
`MPESA-SANDBOX`, `MANAGEMENT-ACCESS-TLS`, `UPGRADE-RECOVERY`,
`RELEASE-DISTRIBUTION`, `DISTRIBUTION` (publication register + policy),
`INSTALLER-TROUBLESHOOTING`, `BLOCKERS`, `DECISIONS`, `DEPLOY-DRILL`,
`IMPLEMENTATION-LEDGER`, `RELEASE-READINESS`, `TEST-RESULTS`.
Scripts: `scripts/` (build/cut/publish/verify chain; see Section 7).
CI: `.github/workflows/distribute.yml`.

## 10. Open items

1. **Owner acceptance of dev.6 recovery on their real VPS** — pending; their
   report is the next input. If it fails, the T7 harness + T6 suite are the
   reproduction tools.
2. A previous dev.6 evidence package (`drevanta-dev6-correction.zip`, 62
   files) was lost to a sandbox recycle; every byte in it is re-derivable
   (published release assets + repo + re-running the three suites —
   Section 7) if the owner wants it again.
3. Stable release, production M-Pesa, protected-branch merges: NOT authorized
   — do not initiate.

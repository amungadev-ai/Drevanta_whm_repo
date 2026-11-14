# Distribution Architecture and Publication Runbook

The customer-facing installation is one short command against the PUBLIC
distribution repository. All verification and installation engineering lives
in the PRIVATE development repository and ships inside the signed artifact.

## Repositories

| Repository | Role | Contents |
| --- | --- | --- |
| `amungadev-ai/Drevanta_whm_repo` (private) | Development + release engineering | Full source, CI, dev prereleases, this runbook |
| `amungadev-ai/drevanta-distribution` (public) | Distribution ONLY | bootstrap.sh, signed SHA256SUMS, artifacts, MANIFEST.json, customer docs, public key — no credentials, no private source, no engineering records |

**Authorization status (corrected 2026-11-14):** distribution targets
`amungadev-ai/drevanta-distribution` (public, same account as this
repository). An earlier external repository (`backupamunga-ops/
drevanta_server_manager`) was identified as a different, same-brand product;
all distribution work involving it is DISCONTINUED and that repository is
left untouched. Publication to the corrected repository is authorized for
development prereleases; stable publication still requires the exact owner
authorization phrase.

## Customer installation commands

| Channel | Command |
| --- | --- |
| Stable (permanent, once a stable release is approved) | `curl -fsSL https://github.com/amungadev-ai/drevanta-distribution/releases/latest/download/bootstrap.sh \| bash` |
| Development prerelease (explicit version, honest labeling) | `curl -fsSL https://github.com/amungadev-ai/drevanta-distribution/releases/download/<dev-tag>/bootstrap.sh \| bash` |
| Explicit stable version | stable command with `--version <tag>` (run the downloaded file) |
| Offline | download the release package, then `bash bootstrap.sh --artifact <tarball>` |

GitHub's `/releases/latest/download/` resolves the latest NON-prerelease
release. On this DEDICATED distribution repository the owner has directed
that the permanent customer command use `/releases/latest`: during the
development phase the current dev prerelease is published with the
prerelease flag OFF so the permanent command resolves, while the release
title, the bootstrap banner
(`DEVELOPMENT PRERELEASE … not a production release`) and the pinned
commit inside keep the development status unambiguous. When an approved
stable release is published it supersedes the dev release as `latest`
automatically (newest non-prerelease by creation time).

## Trust chain (what the bootstrap verifies, in order)

1. `SHA256SUMS.sig` — detached ECDSA P-256 signature over `SHA256SUMS`,
   verified against the public key EMBEDDED in the bootstrap.
2. Artifact + `MANIFEST.json` against the signed digests.
3. Release identity chain: tag / commit (dev: pinned) / version-carrys-commit
   / artifact filename / digest agreement.
4. Artifact-internal `SHA256SUMS` with complete file coverage + `VERSION`.
5. Only then: the installation engine (shipped inside the verified artifact)
   runs — mandatory preflight, installation lock, health gates, credential
   isolation, upgrade recovery, claim-token security all intact.

The customer never handles credentials: public releases need no GitHub token,
and the claim token is generated server-side and printed once at completion
(interactive-terminal gated, as before).

## curl-pipe-bash honesty

The bootstrap is the initial trust anchor and cannot self-verify its first
download. Documented mitigation: `docs/VERIFICATION.md` (published to the
distribution repository) gives the download-and-verify alternative — verify
`bootstrap.sh.sha256` and the signature over `SHA256SUMS` with the published
public key BEFORE executing — plus the key-fingerprint pinning guidance.

## Publication pipeline

- `scripts/gen_signing_key.sh` — one-time: distribution signing keypair
  (private key NEVER committed; goes to Actions secret
  `DREVANTA_DIST_SIGNING_KEY` + owner storage; public key is published and
  embedded in every bootstrap).
- `scripts/sign_release.sh` — builds + signs SHA256SUMS for a staged release.
- `scripts/make_dist_bootstrap.sh` — generates channel-aware bootstraps
  (stable latest / dev pinned / --version / --artifact offline /
  `DREVANTA_DIST_BASE` test override, always signature-enforced).
- `scripts/publish_distribution.sh` — CI gate (green checks on the release
  commit) → download + RE-VERIFY the private release → generate + sign →
  stage → publish release assets → sync repo docs/keys. `--dry-run` stages
  without publishing; `--channel stable` requires the exact owner phrase
  `STABLE-RELEASE-AUTHORIZED:<tag>`.
- `.github/workflows/distribute.yml` — the same pipeline in CI
  (workflow_dispatch: tag, channel, dry_run, stable_authorization; secrets:
  `DIST_TOKEN`, `DREVANTA_DIST_SIGNING_KEY`, `DREVANTA_DIST_PUBLIC_KEY`;
  mandatory local gates: go build/test, artifact build, 99-check suite).

## Verification of the distribution layer itself

`scripts/verify_distribution.sh` — 14 checks, all passing:
curl|bash end to end (signature → digests → identity → internal checksums →
engine → INSTALLATION COMPLETE, zero token material), offline `--artifact`
install, tampered-artifact refusal (stale signature), impostor-signing-key
refusal. Platform endpoints are answered by real local servers inside an
isolated namespace; downloads use REAL curl end to end.

## Current distribution signing key

- Algorithm: ECDSA P-256 (detached signature over SHA256SUMS).
- **Public-key fingerprint (SHA-256 of the DER-encoded public key,
  `openssl ec -pubin -in drevanta-dist-public.pem -outform DER | sha256sum`):
  `91eb6e3b73c57eb252214f319fb9b6e9a816c8f515bdb463a59d552d46bfa582`**
- Rotated twice on 2026-11-14, both times BEFORE any publication (no
  signature from either earlier key was ever published; no revocation
  required). Second rotation: the first rotated key's local copy did not
  survive a sandbox recycle by design (the durable copy lives only in the
  write-only Actions secret store), so a fresh keypair was generated and
  re-stored.
- Latest-resolution policy (owner-directed, dedicated repo): dev releases
  publish with the GitHub prerelease flag OFF and `make_latest: true` so the
  permanent `/releases/latest` command resolves; the FAILED dev.5 is flagged
  prerelease + titled FAILED (excluded from latest). `/releases/latest`
  redirects are CDN-cached for some minutes after a new release — the
  explicitly versioned command is always current.
- Published: v1.0.0-owneraudit.1-dev.6 @ 2777f9567c41 (bootstrap digest
  a63a98db873318e29d9e149864d9856f1684d3922790ab60adf467dbb9e14c81),
  correcting the FAILED dev.5 owner-VPS installation (see
  docs/OWNER-INSTALL-DEV5-FAILED.md). Anonymous literal-command verification:
  fresh flow 51 gates + INSTALLATION COMPLETE; recovery flow
  RECOVERY COMPLETE with state preserved; both token-free.
- Private key storage: GitHub Actions secret `DREVANTA_DIST_SIGNING_KEY` on
  THIS repository (write-only secret store) — never in git history, release
  assets, review packages, logs or CI artifacts. Public key additionally
  stored as `DREVANTA_DIST_PUBLIC_KEY` and published under `keys/` at
  release time, embedded in every generated bootstrap.
- Verify a download against this fingerprint out-of-band (see
  docs/VERIFICATION.md).

## Publication checklist (owner actions, in order)

1. Store the signing private key + `DIST_TOKEN` in the private repo's
   Actions secrets (fine-grained token: Contents Read+Write on the
   distribution repo only, short expiry).
2. Authorize automation (collaborator Write) OR plan to run the publisher
   manually.
3. For the acceptance test: `gh workflow run distribute.yml -f
   tag=v1.0.0-owneraudit.1-dev.5 -f channel=dev` (or the publisher script).
4. Verify the published release assets + the short dev command.
5. Stable releases: ONLY with the explicit authorization phrase; never
   before the owner-led audit completes.

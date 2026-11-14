# Distribution Architecture and Publication Runbook

The customer-facing installation is one short command against the PUBLIC
distribution repository. All verification and installation engineering lives
in the PRIVATE development repository and ships inside the signed artifact.

## Repositories

| Repository | Role | Contents |
| --- | --- | --- |
| `amungadev-ai/Drevanta_whm_repo` (private) | Development + release engineering | Full source, CI, dev prereleases, this runbook |
| `backupamunga-ops/drevanta_server_manager` (public) | Distribution ONLY | bootstrap.sh, signed SHA256SUMS, artifacts, MANIFEST.json, customer docs, public key — no credentials, no private source, no engineering records |

**Authorization status (verified 2026-11-14):** the distribution repository
exists (owned by `backupamunga-ops`, public, created 2026-11-10) but the
current automation token (`amungadev-ai`) has pull-only access to it.
Publication therefore requires ONE of:

1. Add `amungadev-ai` as a collaborator (Write) on the distribution
   repository — then the CI workflow publishes directly; or
2. Run `scripts/publish_distribution.sh` yourself with a token that has push
   access (env `DIST_TOKEN`), after loading the signing key.

Nothing has been published; the pipeline is prepared and dry-run validated.

## Customer installation commands

| Channel | Command |
| --- | --- |
| Stable (permanent, once a stable release is approved) | `curl -fsSL https://github.com/backupamunga-ops/drevanta_server_manager/releases/latest/download/bootstrap.sh \| bash` |
| Development prerelease (explicit version, honest labeling) | `curl -fsSL https://github.com/backupamunga-ops/drevanta_server_manager/releases/download/<dev-tag>/bootstrap.sh \| bash` |
| Explicit stable version | stable command with `--version <tag>` (run the downloaded file) |
| Offline | download the release package, then `bash bootstrap.sh --artifact <tarball>` |

GitHub's `/releases/latest/download/` resolves the latest NON-prerelease
release — development prereleases never pollute the stable channel, and the
stable bootstrap prints its resolved tag; the dev bootstrap is generated per
tag with the exact commit pinned inside and prints
`DEVELOPMENT PRERELEASE … not a production release`.

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
  `89cbd742c0a61e6f5c125cab62a0cadebad35ece4f9172f9331d6cfbd655dec8`**
- Rotated 2026-11-14 before any publication (the initial key never signed a
  published artifact; no revocation required).
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

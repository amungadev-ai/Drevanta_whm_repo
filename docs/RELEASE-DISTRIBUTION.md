# Release Distribution & Artifact Provenance

## Channels

| Channel | Content | Gate |
| --- | --- | --- |
| Development prerelease (`v…-dev.N` tags) | installation artifacts for the owner-led acceptance test | CI green on the release commit; owner approval of the dev artifact |
| Stable (future) | official stable distribution channel | explicit owner approval; **no stable release exists yet and none will be cut without it** |

Branch protection, required CI checks and PR #2 draft status are untouched by
release cutting: prereleases are tagged at the working-branch commit via
`target_commitish`, never by merging protected branches.

## Cutting a development release

```bash
GH_TOKEN=<token with Contents: Read+Write> scripts/cut_dev_release.sh <tag> [title]
```

What it does: runs `scripts/build_install_package.sh` (gofmt → go vet → go
test gates), generates `MANIFEST.json` (the trusted release manifest:
product, channel, tag, commit, version, artifact name → SHA-256), creates
the prerelease at the current commit via the GitHub API, uploads the
artifact, its `.sha256` sidecar and `MANIFEST.json` as release assets. The
token is used exclusively through root-side header files; it is never echoed,
logged or embedded in URLs.

## Verification chain (what each consumer checks)

1. **Bootstrap installer (Method A)** — resolves the exact release,
   downloads `MANIFEST.json` (mandatory: a release without a readable
   manifest is refused — no fallback verification path exists), validates
   its schema (product, channel, tag, 40-hex commit, version, artifact
   identity; the pinned dev release must match its expected full commit),
   downloads the artifact, requires an **exact SHA-256 match**, validates
   archive-entry safety (`tar -tvz`: absolute paths, traversal, link
   targets escaping the extraction root, special entries), verifies the
   internal `SHA256SUMS` **with complete file coverage**, then hands over
   to the engine as a child process after unsetting the GitHub token.
   Missing, malformed, mismatched, untrusted or corrupted artifacts are
   rejected with a non-zero exit and nothing is installed.
2. **Method B/C** — `sha256sum -c <package>.tar.gz.sha256` before extraction;
   the engine's artifact-internal checks still apply on install.
3. **Release provenance** — the summary prints the installed version
   (`VERSION` inside the artifact embeds the exact commit), the release tag
   and the artifact SHA-256. `MANIFEST.json` binds the tag to the commit and
   checksums.

## Integrity and authenticity disclosures

- **Implemented:** TLS-transported retrieval, authenticated access to the
  private repository (header-file-only token handling), SHA-256 artifact
  verification against the release manifest and against the internal
  per-file checksums, exact commit provenance in `VERSION` and `MANIFEST.json`.
- **Not yet implemented (outstanding):** cryptographic artifact signing
  (Ed25519/GPG detached signatures) — requires a signing-key generation and
  custody ceremony, which is an owner-governed step; the bootstrap reports
  verification it actually performed and never claims signature verification
  that did not run.
- **Bootstrap script integrity (dev phase):** pinned release tag + TLS +
  repository access control + the fact that the artifact itself is
  manifest-verified. A full trust root for the bootstrap script itself
  arrives with the signing ceremony above.

## Version identification

- Artifact name: `drevanta-install-<version>-<commit>.tar.gz`
- `VERSION` file inside the artifact: `<version>+<full-commit>` and a build timestamp
- Engine banner: prints the artifact version; the installation report
  (`/etc/drevanta/install-report.json`) records the installed version.

# Verifying a Drevanta release (for security-conscious customers)

## Trust model

Every release in this repository carries:

- `SHA256SUMS` — digests of all release assets,
- `SHA256SUMS.sig` — a detached ECDSA P-256 signature over `SHA256SUMS`,
  made with the Drevanta distribution signing key,
- the matching public key, published in this repository under `keys/` and
  embedded in every `bootstrap.sh`.

The bootstrap verifies, **in this order and before executing anything**:

1. the signature over `SHA256SUMS` against the embedded public key,
2. the artifact and `MANIFEST.json` against the signed digests,
3. the release identity chain (tag / commit / version / artifact filename /
   digest agreement),
4. the artifact-internal checksums with complete file coverage and the
   `VERSION` record.

Any failure aborts the installation with an explicit `[FAIL]` message and
the system is not modified.

## The curl-pipe-bash limitation, stated plainly

`curl ... | bash` executes the bootstrap as it is downloaded over HTTPS.
HTTPS authenticates the transport to github.com, but the bootstrap itself is
the initial trust anchor: a first-run compromise of that single download is
outside what any downloaded script can self-verify. This is an inherent
property of every one-command installer, and we mitigate it with: release
signature verification inside the bootstrap, signed releases on a dedicated
distribution repository that contains no credentials and no private source,
and the documented alternative below.

## The download-and-verify alternative (recommended for first use)

On any machine:

```bash
RELEASE_URL=https://github.com/backupamunga-ops/drevanta_server_manager/releases/latest/download
curl -fsSLO "$RELEASE_URL/bootstrap.sh"
curl -fsSLO "$RELEASE_URL/bootstrap.sh.sha256"
curl -fsSLO "$RELEASE_URL/SHA256SUMS"
curl -fsSLO "$RELEASE_URL/SHA256SUMS.sig"
curl -fsSLO keys/drevanta-dist-public.pem 2>/dev/null || \
  curl -fsSLO https://github.com/backupamunga-ops/drevanta_server_manager/raw/main/keys/drevanta-dist-public.pem

sha256sum -c bootstrap.sh.sha256
openssl dgst -sha256 -verify drevanta-dist-public.pem -signature SHA256SUMS.sig SHA256SUMS
# Expected output: Verified OK
```

If both checks pass, run the verified file on the server:

```bash
scp bootstrap.sh root@your-server-ip:/root/
ssh root@your-server-ip 'bash /root/bootstrap.sh'
```

For maximum assurance, obtain the public key fingerprint through a second
channel (the release notes) and compare:

```bash
openssl ec -pubin -in drevanta-dist-public.pem -outform DER | sha256sum
```

Record this fingerprint after your first verified installation; it must stay
stable across releases (key rotation would be announced in the release notes
with a new signed key).

## What the bootstrap refuses

- a failed or missing signature over `SHA256SUMS`,
- a digest mismatch for the artifact or the manifest,
- any disagreement in the release identity chain,
- incomplete internal checksum coverage or a `VERSION` mismatch,
- running without root, or on an unsupported/undersized system.

Nothing is downloaded-executed beyond the verified artifact, and the
bootstrap never handles credentials: the public release flow requires no
GitHub token at all.

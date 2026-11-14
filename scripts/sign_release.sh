#!/usr/bin/env bash
# Drevanta distribution release signer.
#
# Builds SHA256SUMS over the staged release assets, signs it with the
# distribution signing key (detached signature), and verifies the round trip.
# Assets signed: whatever is in the staging dir except the signature itself
# and SHA256SUMS (checksum files like *.sha256 are kept as convenience
# sidecars but the signed SHA256SUMS is the trust anchor).
#
# Usage: DREVANTA_DIST_SIGNING_KEY=<pem> scripts/sign_release.sh <staging-dir>
set -euo pipefail
DIR="${1:?usage: sign_release.sh <staging-dir>}"
KEY="${DREVANTA_DIST_SIGNING_KEY:?DREVANTA_DIST_SIGNING_KEY env var required}"
[[ -f "$KEY" ]] || { echo "[FAIL] signing key not found: $KEY"; exit 1; }
[[ -d "$DIR" ]] || { echo "[FAIL] staging dir not found: $DIR"; exit 1; }

cd "$DIR"
# Deterministic asset list: everything except the signature and SHA256SUMS.
find . -maxdepth 1 -type f ! -name 'SHA256SUMS' ! -name 'SHA256SUMS.sig' -printf '%f\n' | LC_ALL=C sort > .assets
: > SHA256SUMS
while IFS= read -r f; do sha256sum "$f" >> SHA256SUMS; done < .assets
rm -f .assets
openssl dgst -sha256 -sign "$KEY" -out SHA256SUMS.sig SHA256SUMS
# Round-trip verification with the matching public key.
openssl ec -in "$KEY" -pubout -out .pub.pem 2>/dev/null
openssl dgst -sha256 -verify .pub.pem -signature SHA256SUMS.sig SHA256SUMS >/dev/null \
  || { echo "[FAIL] signature round-trip failed"; rm -f .pub.pem; exit 1; }
rm -f .pub.pem
chmod 644 SHA256SUMS SHA256SUMS.sig
echo "[OK] SHA256SUMS signed ($(wc -l < SHA256SUMS) assets) -> $DIR/SHA256SUMS.sig"

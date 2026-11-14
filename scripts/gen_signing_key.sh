#!/usr/bin/env bash
# Drevanta distribution signing key generator.
#
# Generates the EC P-256 keypair used to sign distribution release manifests
# (SHA256SUMS). The PRIVATE key must NEVER be committed or published: it
# lives in the owner's secure storage and in the GitHub Actions secret
# DREVANTA_DIST_SIGNING_KEY. The PUBLIC key is embedded in every generated
# distribution bootstrap and published under keys/ in the distribution repo.
#
# Usage: scripts/gen_signing_key.sh <output-dir>
set -euo pipefail
OUT="${1:?usage: gen_signing_key.sh <output-dir>}"
mkdir -p "$OUT"
[[ ! -e "$OUT/drevanta-dist-signing.pem" ]] || { echo "[FAIL] $OUT/drevanta-dist-signing.pem already exists — refusing to overwrite"; exit 1; }

umask 077
openssl ecparam -name prime256v1 -genkey -noout -out "$OUT/drevanta-dist-signing.pem"
openssl ec -in "$OUT/drevanta-dist-signing.pem" -pubout -out "$OUT/drevanta-dist-public.pem" 2>/dev/null
chmod 600 "$OUT/drevanta-dist-signing.pem"
chmod 644 "$OUT/drevanta-dist-public.pem"

# Round-trip proof: sign + verify a sample payload before declaring success.
printf 'drevanta-distribution-signing-key-proof\n' > "$OUT/.proof"
openssl dgst -sha256 -sign "$OUT/drevanta-dist-signing.pem" -out "$OUT/.proof.sig" "$OUT/.proof"
openssl dgst -sha256 -verify "$OUT/drevanta-dist-public.pem" -signature "$OUT/.proof.sig" "$OUT/.proof" >/dev/null
rm -f "$OUT/.proof" "$OUT/.proof.sig"

echo "[OK] distribution signing keypair generated in $OUT"
echo "     PRIVATE (never publish, never commit): drevanta-dist-signing.pem"
echo "     PUBLIC  (embed in bootstrap, publish under keys/): drevanta-dist-public.pem"
echo "     Store the private key as the Actions secret DREVANTA_DIST_SIGNING_KEY."

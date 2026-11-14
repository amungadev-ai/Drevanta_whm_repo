#!/usr/bin/env bash
# Cuts a DEVELOPMENT release (prerelease) of the installation artifact:
#   - builds the artifact via scripts/build_install_package.sh (all gates)
#   - generates MANIFEST.json (trusted release manifest: artifact -> sha256)
#   - creates the tag + prerelease on the pinned commit via the GitHub API
#   - uploads the artifact, its checksum and MANIFEST.json as release assets
#
# Governance: development releases only — no protected-branch merge, no
# production/stable release without explicit owner approval. The tag is
# created at the CURRENT commit of the working branch (target_commitish);
# branch protections are untouched.
#
# Credentials: GH_TOKEN env var (Contents: Read+Write). The token is used
# exclusively through root-side header files and is never echoed, logged or
# embedded in URLs.
#
# Usage: GH_TOKEN=... scripts/cut_dev_release.sh <tag> [release-title]
set -euo pipefail
cd "$(dirname "$0")/.."

TAG="${1:?usage: cut_dev_release.sh <tag> [release-title]}"
TITLE="${2:-Drevanta ${TAG} — development release (owner acceptance artifacts)}"
[[ -n "${GH_TOKEN:-}" ]] || { echo "[FAIL] GH_TOKEN env var required"; exit 2; }
git diff --quiet || { echo "[FAIL] working tree not clean — commit first"; exit 2; }

echo "== building artifact (gates included) =="
bash scripts/build_install_package.sh
COMMIT=$(git rev-parse HEAD)
# Release-identity agreement: the packaged tarball must be the one cut from
# THIS commit — never a stale artifact from an earlier commit (this exact
# defect shipped dev.3: a stale -335c67d tarball was released under a
# -dev.3/9bdce4d manifest).
SHORT=$(git rev-parse --short HEAD)
TARBALL="drevanta-install-1.0.0-owneraudit.1-${SHORT}.tar.gz"
[[ -s "$TARBALL" ]] || { echo "[FAIL] expected artifact for THIS commit missing: $TARBALL"; ls drevanta-install-*.tar.gz 2>/dev/null; exit 1; }
ls drevanta-install-*.tar.gz | grep -v "^$TARBALL$" >/dev/null && { echo "[FAIL] multiple artifacts present — clean stale artifacts first"; exit 1; }
SHA256="$(sha256sum "$TARBALL" | awk '{print $1}')"
[[ "$SHA256" == "$(awk '{print $1}' "$TARBALL.sha256")" ]] || { echo "[FAIL] sidecar digest mismatch"; exit 1; }
VERSION_LINE="$(tar -xzOf "$TARBALL" "$(basename "${TARBALL%.tar.gz}")/VERSION" | head -1)"
[[ "$VERSION_LINE" == "1.0.0-owneraudit.1+${COMMIT}" ]] || { echo "[FAIL] artifact VERSION '$VERSION_LINE' does not carry this commit ($COMMIT)"; exit 1; }
echo "[OK] release identity agreement: tag=$TAG commit=$COMMIT artifact=$(basename "$TARBALL") version=$VERSION_LINE"

echo "== generating MANIFEST.json =="
python3 - "$TAG" "$COMMIT" "$TARBALL" "$SHA256" "$VERSION_LINE" > MANIFEST.json <<'PY'
import json, sys, datetime
tag, commit, tarball, sha256, version = sys.argv[1:6]
manifest = {
    "product": "Drevanta",
    "channel": "development-prerelease",
    "tag": tag,
    "commit": commit,
    "version": version,
    "generated_at": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "artifact": tarball,
    "artifacts": {tarball: sha256},
    "verification": "bootstrap verifies the artifact against this manifest AND the artifact-internal SHA256SUMS",
}
print(json.dumps(manifest, indent=2))
PY
cat MANIFEST.json

echo "== creating prerelease $TAG at commit ${COMMIT:0:12} =="
HDR="$(mktemp)"; chmod 600 "$HDR"
printf 'Authorization: Bearer %s\nAccept: application/vnd.github+json\n' "$GH_TOKEN" > "$HDR"
REL_JSON="$(mktemp)"
HTTP_CODE=$(curl -fsS --header @"$HDR" -o "$REL_JSON" -w '%{http_code}' \
  -X POST "https://api.github.com/repos/amungadev-ai/Drevanta_whm_repo/releases" \
  -d "$(python3 -c "
import json,sys
print(json.dumps({'tag_name':sys.argv[1],'target_commitish':sys.argv[2],
 'name':sys.argv[3],'prerelease':True,'draft':False,
 'body':'Development release for the owner-led fresh-installation acceptance test. Not a production release. Bootstrap: bash install.sh (--version %s). Artifact integrity: MANIFEST.json + internal SHA256SUMS.'%sys.argv[1]}))" "$TAG" "$COMMIT" "$TITLE")") \
  || { echo "[FAIL] release creation failed"; cat "$REL_JSON"; rm -f "$HDR" "$REL_JSON"; exit 1; }
REL_ID=$(python3 -c "import json;print(json.load(open('$REL_JSON'))['id'])")
REL_URL=$(python3 -c "import json;print(json.load(open('$REL_JSON'))['html_url'])")
echo "[OK] release created: $REL_URL (id $REL_ID)"

upload() { # upload <file> <name>
  curl -fsS --header @"$HDR" -H 'Content-Type: application/octet-stream' \
    --data-binary @"$1" \
    "https://uploads.github.com/repos/amungadev-ai/Drevanta_whm_repo/releases/${REL_ID}/assets?name=$2" >/dev/null \
    && echo "[OK] asset uploaded: $2" || { echo "[FAIL] asset upload failed: $2"; rm -f "$HDR"; exit 1; }
}
upload "$TARBALL" "$TARBALL"
upload "$TARBALL.sha256" "$TARBALL.sha256"
upload "MANIFEST.json" "MANIFEST.json"
rm -f "$HDR" "$REL_JSON"

echo "========================================================"
echo "Development release ready: $TAG (commit ${COMMIT:0:12})"
echo "URL: $REL_URL"
echo "Artifact SHA-256: $SHA256"
echo "Trusted manifest: MANIFEST.json (release asset)"

#!/usr/bin/env bash
# Drevanta distribution publisher.
#
# Takes a verified release from the PRIVATE development repository and
# prepares (and — when authorized and run without --dry-run — performs) its
# publication to the PUBLIC distribution repository:
#   amungadev-ai/drevanta-distribution
#
# Publication includes:
#   - bootstrap.sh (channel-aware: dev pinned to the exact tag+commit,
#     stable resolving /releases/latest at run time) + its digest sidecar
#   - SHA256SUMS + detached signature (distribution signing key)
#   - the installation artifact + sidecar + MANIFEST.json (re-verified here
#     before anything is staged)
#   - customer docs (INSTALL-CLIENT.md, VERIFICATION.md) and the public key
#   - release notes with the channel identity stated plainly
#
# Required environment:
#   DREVANTA_DIST_SIGNING_KEY   path to the distribution signing key (PEM)
#   DIST_TOKEN                  GitHub token WITH PUSH ACCESS to the
#                               distribution repository (Contents: Read+Write)
#   GH_TOKEN                    token for the PRIVATE repo (defaults to
#                               DIST_TOKEN when authorized for both)
# Usage:
#   scripts/publish_distribution.sh --tag <private-release-tag> \
#     --channel dev|stable [--dist-repo amungadev-ai/drevanta-distribution] [--dry-run]
#
# Governance: --channel stable additionally requires
#   --stable-authorization "STABLE-RELEASE-AUTHORIZED:<tag>"
# (enforced here AND in the CI workflow). Nothing is published without it.
set -euo pipefail
cd "$(dirname "$0")/.."

TAG=""; CHANNEL=""; DRY_RUN=0; STABLE_AUTH=""
DIST_REPO="amungadev-ai/drevanta-distribution"
DEV_REPO="amungadev-ai/Drevanta_whm_repo"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --tag) TAG="$2"; shift 2 ;;
    --channel) CHANNEL="$2"; shift 2 ;;
    --dist-repo) DIST_REPO="$2"; shift 2 ;;
    --stable-authorization) STABLE_AUTH="$2"; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    *) echo "[FAIL] unknown option: $1"; exit 2 ;;
  esac
done
[[ -n "$TAG" && -n "$CHANNEL" ]] || { echo "[FAIL] --tag and --channel required"; exit 2; }
[[ "$CHANNEL" == "dev" || "$CHANNEL" == "stable" ]] || { echo "[FAIL] --channel dev|stable"; exit 2; }
KEY="${DREVANTA_DIST_SIGNING_KEY:?DREVANTA_DIST_SIGNING_KEY required}"
DIST_TOKEN="${DIST_TOKEN:?DIST_TOKEN (push access to the distribution repo) required}"
PRIV_TOKEN="${GH_TOKEN:-$DIST_TOKEN}"

if [[ "$CHANNEL" == "stable" ]]; then
  [[ "$STABLE_AUTH" == "STABLE-RELEASE-AUTHORIZED:$TAG" ]] \
    || { echo "[FAIL] stable publication requires --stable-authorization \"STABLE-RELEASE-AUTHORIZED:$TAG\" (owner governance gate)"; exit 2; }
fi

WORK="$(mktemp -d)"; chmod 700 "$WORK"
PRIV_HDR="$WORK/.priv-hdr"; DIST_HDR="$WORK/.dist-hdr"
cleanup() { if [[ "$DRY_RUN" == "1" ]]; then echo "[OK] dry-run staging preserved for review: $WORK"; else rm -rf "$WORK"; fi; }
trap cleanup EXIT
printf 'Authorization: Bearer %s\n' "$PRIV_TOKEN" > "$PRIV_HDR"   # no Accept here: asset downloads need application/octet-stream; chmod 600 "$PRIV_HDR"
printf 'Authorization: Bearer %s\n' "$DIST_TOKEN" > "$DIST_HDR"; chmod 600 "$DIST_HDR"
API="https://api.github.com"

api() { curl -fsS --header @"$1" "${@:2}"; }

echo "== 1. CI gate on the release commit =="
REL_JSON="$WORK/rel.json"
api "$PRIV_HDR" "$API/repos/$DEV_REPO/releases/tags/$TAG" -o "$REL_JSON" \
  || { echo "[FAIL] private release $TAG not found"; exit 1; }
python3 -c "import json,sys; r=json.load(open(sys.argv[1])); assert not r.get('draft'), 'release is a draft'" "$REL_JSON"
TARGET_COMMIT="$(python3 -c "import json,sys; r=json.load(open(sys.argv[1])); print(r['target_commitish'])" "$REL_JSON")"
CHECKS="$(api "$PRIV_HDR" "$API/repos/$DEV_REPO/commits/$TARGET_COMMIT/check-runs" -H "Accept: application/vnd.github+json")"
echo "$CHECKS" | python3 -c '
import json, sys
runs = json.load(sys.stdin).get("check_runs", [])
bad = [r["name"] for r in runs if r["status"] != "completed" or r.get("conclusion") != "success"]
assert runs, "no CI check runs on the release commit — refusing to distribute"
assert not bad, "CI not green on the release commit: %s" % bad
print("[OK] CI green on release commit %s" % runs[0]["head_sha"][:12])'

echo "== 2. Download + re-verify the private release assets =="
STAGE="$WORK/stage"; mkdir -p "$STAGE/docs" "$STAGE/keys"
ART_NAME="$(python3 -c "import json,sys; print([a['name'] for a in json.load(open(sys.argv[1]))['assets'] if a['name'].startswith('drevanta-install-') and a['name'].endswith('.tar.gz')][0])" "$REL_JSON")"
MAN_ID="$(python3 -c "import json,sys; print([a['id'] for a in json.load(open(sys.argv[1]))['assets'] if a['name']=='MANIFEST.json'][0])" "$REL_JSON")"
ART_ID="$(python3 -c "import json,sys; print([a['id'] for a in json.load(open(sys.argv[1]))['assets'] if a['name']=='$ART_NAME'][0])" "$REL_JSON")"
SIDE_ID="$(python3 -c "import json,sys; print([a['id'] for a in json.load(open(sys.argv[1]))['assets'] if a['name']=='$ART_NAME.sha256'][0])" "$REL_JSON")"
dl() { curl -fsSL --header @"$PRIV_HDR" -H "Accept: application/octet-stream" -o "$2" "$API/repos/$DEV_REPO/releases/assets/$1"; }
dl "$MAN_ID" "$STAGE/MANIFEST.json"; dl "$ART_ID" "$STAGE/$ART_NAME"; dl "$SIDE_ID" "$STAGE/$ART_NAME.sha256"
( cd "$STAGE" && sha256sum -c "$ART_NAME.sha256" --quiet ) || { echo "[FAIL] artifact digest sidecar mismatch"; exit 1; }
python3 - "$STAGE/MANIFEST.json" "$ART_NAME" "$CHANNEL" "$TAG" <<'PY'
import json, sys
m, art, channel, tag = json.load(open(sys.argv[1])), sys.argv[2], sys.argv[3], sys.argv[4]
assert m["product"] == "Drevanta", "product mismatch"
assert m["artifact"] == art, "artifact identity mismatch"
sha = m["artifacts"][art]
assert m["version"].endswith(m["commit"]), "version/commit mismatch"
if channel == "dev":
    assert m["tag"] == tag, "dev tag mismatch"
print("[OK] manifest re-verified: %s @ %s (%s)" % (m["tag"], m["commit"][:12], sha[:16]))
PY

echo "== 3. Generate channel bootstrap + sign everything =="
PUBKEY="${DREVANTA_DIST_PUBLIC_KEY:-$(dirname "$KEY")/$(basename "$KEY" | sed 's/signing/public/')}"
[[ -s "$PUBKEY" ]] || { echo "[FAIL] public key not found: $PUBKEY (set DREVANTA_DIST_PUBLIC_KEY)"; exit 1; }
if [[ "$CHANNEL" == "dev" ]]; then
  scripts/make_dist_bootstrap.sh --channel dev --tag "$TAG" --commit "$TARGET_COMMIT" \
    --public-key "$PUBKEY" --out "$STAGE/bootstrap.sh"
else
  scripts/make_dist_bootstrap.sh --channel stable --public-key "$PUBKEY" --out "$STAGE/bootstrap.sh"
fi
cp docs/INSTALL-CLIENT.md "$STAGE/docs/" 2>/dev/null || true
cp docs/VERIFICATION.md "$STAGE/docs/" 2>/dev/null || true
mkdir -p "$STAGE/keys" && cp "$PUBKEY" "$STAGE/keys/drevanta-dist-public.pem"
DREVANTA_DIST_SIGNING_KEY="$KEY" scripts/sign_release.sh "$STAGE"
( cd "$STAGE" && sha256sum bootstrap.sh > bootstrap.sh.sha256 )
# The bootstrap digest sidecar must reflect the SIGNED tree state (bootstrap
# is not self-referential in SHA256SUMS), so refresh it after signing:
( cd "$STAGE" && sha256sum bootstrap.sh > bootstrap.sh.sha256 )

if [[ "$DRY_RUN" == "1" ]]; then
  echo "== DRY RUN — staged distribution content (nothing published) =="
  ls -la "$STAGE" "$STAGE/docs" "$STAGE/keys"
  echo "---- SHA256SUMS ----"; cat "$STAGE/SHA256SUMS"
  echo "[OK] dry run complete — no publication performed"
  exit 0  # staging preserved at $WORK for review
fi

echo "== 4. Publish to $DIST_REPO (release $TAG, channel $CHANNEL) =="
PRERELEASE="false"  # dedicated repo: /releases/latest must resolve; dev status lives in title/body/banner
if [[ "$CHANNEL" == "dev" ]]; then
  NOTES="DEVELOPMENT PRERELEASE for the owner-led acceptance test — explicitly NOT a production release.
Commit: $TARGET_COMMIT
Install: curl -fsSL https://github.com/$DIST_REPO/releases/download/$TAG/bootstrap.sh | bash
Verification: see docs/VERIFICATION.md (signature + digest checks)."
else
  NOTES="Drevanta stable release $TAG (owner-approved).
Install: curl -fsSL https://github.com/$DIST_REPO/releases/latest/download/bootstrap.sh | bash
Verification: see docs/VERIFICATION.md (signature + digest checks)."
fi
printf '%s\n' "$NOTES" > "$WORK/notes.md"
REL_BODY="$(python3 -c "
import json,sys
print(json.dumps({'tag_name':sys.argv[1],'target_commitish':'main','name':'Drevanta %s (%s)'%(sys.argv[1],sys.argv[2]),
 'body':open(sys.argv[3]).read(),'prerelease':sys.argv[4]=='true','draft':False}))" "$TAG" "$CHANNEL" "$WORK/notes.md" "$PRERELEASE")"
DIST_REL="$WORK/dist-rel.json"
REL_RC=0
curl -fsS --header @"$DIST_HDR" -o "$DIST_REL" -X POST "$API/repos/$DIST_REPO/releases" -d "$REL_BODY" || REL_RC=1
if [[ "$REL_RC" != "0" ]]; then
  # 422 = the release/tag already exists (previous partial run) — reuse it.
  curl -fsS --header @"$DIST_HDR" -o "$DIST_REL" "$API/repos/$DIST_REPO/releases/tags/$TAG" \
    || { echo "[FAIL] dist release creation failed and no existing release found"; exit 1; }
  echo "[INFO] reusing existing distribution release for $TAG"
fi
DIST_REL_ID="$(python3 -c "import json;print(json.load(open('$DIST_REL'))['id'])")"
upload() { curl -fsS --header @"$DIST_HDR" -H 'Content-Type: application/octet-stream' --data-binary @"$1" \
  "https://uploads.github.com/repos/$DIST_REPO/releases/$DIST_REL_ID/assets?name=$2" >/dev/null && echo "[OK] published asset: $2" \
  || { echo "[FAIL] asset upload failed: $2"; exit 1; }; }
for f in bootstrap.sh bootstrap.sh.sha256 SHA256SUMS SHA256SUMS.sig MANIFEST.json "$ART_NAME" "$ART_NAME.sha256"; do
  upload "$STAGE/$f" "$f"
done
upload "$STAGE/docs/INSTALL-CLIENT.md" "INSTALL-CLIENT.md"
upload "$STAGE/docs/VERIFICATION.md" "VERIFICATION.md"

echo "== 5. Sync repository docs (README, keys) =="
CRED="$WORK/.cred"
cat > "$CRED" <<CRED
#!/usr/bin/env bash
echo "username=x-access-token"
echo "password=\$DIST_TOKEN"
CRED
chmod 700 "$CRED"; export DIST_TOKEN
DIST_CLONE="$WORK/dist"; git clone -q "https://github.com/$DIST_REPO.git" "$DIST_CLONE"
cp docs/INSTALL-CLIENT.md "$DIST_CLONE/docs/INSTALL-CLIENT.md" 2>/dev/null || { mkdir -p "$DIST_CLONE/docs"; cp docs/INSTALL-CLIENT.md "$DIST_CLONE/docs/"; }
cp docs/VERIFICATION.md "$DIST_CLONE/docs/VERIFICATION.md"
mkdir -p "$DIST_CLONE/keys" && cp "$STAGE/keys/drevanta-dist-public.pem" "$DIST_CLONE/keys/"
( cd "$DIST_CLONE"
  git add -A && git -c user.name="drevanta-dist" -c user.email="dist@drevanta.invalid" \
    commit -q -m "distribution: $CHANNEL release $TAG (commit $TARGET_COMMIT)" 2>/dev/null || true
  git -c credential.helper="$CRED" push -q origin main )
echo "[OK] distribution repository docs synced"
echo "========================================================"
echo "Published: https://github.com/$DIST_REPO/releases/tag/$TAG ($CHANNEL)"
[[ "$CHANNEL" == "dev" ]] && echo "Install command: curl -fsSL https://github.com/$DIST_REPO/releases/download/$TAG/bootstrap.sh | bash"
[[ "$CHANNEL" == "stable" ]] && echo "Install command: curl -fsSL https://github.com/$DIST_REPO/releases/latest/download/bootstrap.sh | bash"

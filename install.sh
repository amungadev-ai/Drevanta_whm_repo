#!/usr/bin/env bash
# ============================================================================
# Drevanta bootstrap installer — Method A (Direct GitHub installation).
#
#   Official repository : https://github.com/amungadev-ai/Drevanta_whm_repo
#
# What it does:
#   1. Validates OS/architecture (Debian-family, linux/amd64).
#   2. Resolves the requested Drevanta release (default: the pinned
#      development release below; stable releases after owner approval).
#   3. Retrieves the release manifest and downloads the matching artifact.
#   4. Verifies the artifact SHA-256 against the trusted release manifest
#      AND the artifact's internal SHA256SUMS. Corrupted or mismatched
#      artifacts are deleted and the install FAILS — success is never
#      reported for verification that did not execute.
#   5. Extracts into a private temporary directory and executes the shared
#      installation engine (the same engine used by every method).
#   6. Cleans up temporary files.
#
# The standard customer installation never compiles from source.
#
# Minimal prerequisites (documented, auto-installed where possible):
#   bash + coreutils (present), curl (downloader), tar, python3 (manifest
#   parsing), root privileges. The engine installs nginx/Node.js itself.
#
# Private repository authentication:
#   While the repository is private, retrieval needs a GitHub token with
#   Contents:Read. The token is NEVER embedded in URLs, command arguments,
#   shell history or logs: it is read silently when needed and passed to
#   curl via a root-only header file that is deleted immediately after use.
#   Preferred: export DREVANTA_GH_TOKEN (or GITHUB_TOKEN) beforehand.
#
# Usage:
#   bash install.sh [bootstrap options] [-- engine options...]
# Bootstrap options:
#   --version <tag>   release tag to install (default: pinned dev release)
#   --tmpdir <dir>    working directory (default: mktemp -d)
#   --keep            keep the extracted artifact for inspection
#   -h | --help       this help
# Everything after `--` is passed to the shared installation engine
# (e.g. -- --access-ip 203.0.113.7 --mgmt-port 8443 --private-mgmt).
# ============================================================================
set -uo pipefail

REPO="amungadev-ai/Drevanta_whm_repo"
API="https://api.github.com/repos/${REPO}"
PINNED_TAG="v1.0.0-owneraudit.1-dev.2"     # development release (owner-approved dev artifact)
# Provenance of the default release is established by the release tag plus
# MANIFEST.json + artifact SHA-256 verification below (no credential ever
# participates in verification).
REQ_TAG=""
TMPDIR_ARG=""
KEEP="0"
TOKEN_VAR=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) REQ_TAG="$2"; shift 2 ;;
    --tmpdir) TMPDIR_ARG="$2"; shift 2 ;;
    --keep) KEEP="1"; shift ;;
    -h|--help) sed -n '2,34p' "$0" | grep -E '^#'; exit 0 ;;
    --) shift; break ;;
    *) echo "[FAIL] unknown bootstrap option: $1 (see --help)"; exit 2 ;;
  esac
done
ENGINE_ARGS=("$@")

TAG="${REQ_TAG:-$PINNED_TAG}"
[[ $EUID -eq 0 ]] || { echo "[FAIL] run as root (sudo)"; exit 2; }
echo "========================================================"
echo "   DREVANTA — installation (bootstrap)"
echo "========================================================"

# 1. Platform validation ------------------------------------------------------
ARCH="$(uname -m)"
if [[ "$ARCH" != "x86_64" ]]; then
  echo "[FAIL] architecture $ARCH is not supported by the release artifacts (linux/amd64)"; exit 1
fi
if ! command -v apt-get >/dev/null 2>&1; then
  echo "[FAIL] apt-get not found — supported: Ubuntu 20.04+, Debian 11+ (Debian family)"; exit 1
fi
echo "[OK]   platform: $(. /etc/os-release && echo "${PRETTY_NAME:-unknown}") ($ARCH)"
for c in curl tar sha256sum python3; do
  if ! command -v "$c" >/dev/null 2>&1; then
    echo "[INFO] prerequisite missing: $c — installing via apt"
    apt-get update -qq >/dev/null 2>&1
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "$c" >/dev/null 2>&1 \
      || { echo "[FAIL] could not install prerequisite '$c' (network/repo issue) — install it manually and rerun"; exit 1; }
  fi
done
echo "[OK]   prerequisites: curl, tar, sha256sum, python3"

# 2. Temporary workspace ------------------------------------------------------
WORK="${TMPDIR_ARG:-$(mktemp -d /tmp/drevanta-install.XXXXXX)}"
mkdir -p "$WORK" && chmod 700 "$WORK"
cleanup() { [[ "$KEEP" == "1" ]] && echo "[INFO] artifact kept at: $WORK" || rm -rf "$WORK"; }
trap cleanup EXIT

# 3. Authenticated retrieval (private repository) -----------------------------
# Token handling: header file only (0600, deleted after each use) — never in
# URLs, arguments, shell history or logs.
HDR="$WORK/.auth-header"
use_token() { # use_token <token-or-empty> -> (re)write header file
  if [[ -n "$1" ]]; then umask 077; printf 'Authorization: Bearer %s\nAccept: application/vnd.github+json\n' "$1" > "$HDR"; umask 022
  else : > "$HDR"; fi
}
gh_get() { # gh_get <api-path> <output-file> -> 0 on HTTP 200
  local path="$1" out="$2" code
  code=$(curl -fsS --max-time 30 --header @"$HDR" -o "$out" -w '%{http_code}' "${API}${path}" 2>/dev/null || echo "000")
  [[ "$code" == "200" ]] || { rm -f "$out"; return 1; }
  return 0
}
TOKEN_VAR="${DREVANTA_GH_TOKEN:-${GITHUB_TOKEN:-}}"
use_token "$TOKEN_VAR"
if ! gh_get "/releases/tags/$TAG" "$WORK/release.json"; then
  echo "[INFO] release $TAG not reachable — authentication required (private repository)"
  if [[ "${DREVANTA_UNATTENDED:-0}" == "1" ]]; then
    echo "[FAIL] unattended mode: export DREVANTA_GH_TOKEN (Contents:Read, short-lived) and rerun"; exit 1
  fi
  printf 'GitHub token (Contents: Read; input hidden): '
  IFS= read -rs TOKEN_VAR; echo
  [[ -n "${TOKEN_VAR:-}" ]] || { echo "[FAIL] no token provided"; exit 1; }
  use_token "$TOKEN_VAR"
  gh_get "/releases/tags/$TAG" "$WORK/release.json" \
    || { echo "[FAIL] release $TAG still not reachable — check the token scope (Contents: Read) and the tag name"; exit 1; }
fi
echo "[OK]   release manifest retrieved: $TAG"

# 4. Manifest -> artifact selection -------------------------------------------
python3 - "$WORK/release.json" "$WORK/artifact.name" "$WORK/artifact.assetid" <<'PY'
import json, sys
rel = json.load(open(sys.argv[1]))
name = assetid = ""
for a in rel.get("assets", []):
    n = a.get("name", "")
    if n.endswith(".tar.gz") and n.startswith("drevanta-install-"):
        name, assetid = n, str(a.get("id", "")); break
open(sys.argv[2], "w").write(name)
open(sys.argv[3], "w").write(assetid)
PY
ART_NAME="$(cat "$WORK/artifact.name" 2>/dev/null || true)"
ART_ID="$(cat "$WORK/artifact.assetid" 2>/dev/null || true)"
[[ -n "$ART_NAME" && -n "$ART_ID" ]] || { echo "[FAIL] no drevanta-install-*.tar.gz asset found in release $TAG"; exit 1; }

# Trusted release manifest: MANIFEST.json ships as a release asset and carries
# the expected artifact SHA-256 (generated at release time from the built
# artifact). Fetch it and cross-check the artifact against it.
ART_SHA_EXPECTED=""
use_token "$TOKEN_VAR"
if gh_get "/releases/assets?per_page=100" "$WORK/assets.json"; then
  MANIFEST_ID="$(python3 -c "
import json,sys
for a in json.load(open(sys.argv[1])):
    if a.get('name')=='MANIFEST.json': print(a['id']); break
" "$WORK/assets.json" 2>/dev/null || true)"
  if [[ -n "${MANIFEST_ID:-}" ]]; then
    umask 077; printf 'Authorization: Bearer %s\nAccept: application/octet-stream\n' "$TOKEN_VAR" > "$HDR"; umask 022
    if curl -fsS --max-time 30 --header @"$HDR" -o "$WORK/MANIFEST.json" "${API}/releases/assets/${MANIFEST_ID}" 2>/dev/null; then
      ART_SHA_EXPECTED="$(python3 -c "
import json,sys
m=json.load(open(sys.argv[1]))
arts=m.get('artifacts',{})
print(arts.get(sys.argv[2],'') if isinstance(arts,dict) else '')" "$WORK/MANIFEST.json" "$ART_NAME" 2>/dev/null || true)"
    fi
    [[ -n "$ART_SHA_EXPECTED" ]] && echo "[OK]   trusted manifest retrieved: MANIFEST.json (release asset)" \
      || echo "[WARN] MANIFEST.json present but unreadable — verification falls back to the artifact's internal SHA256SUMS"
  else
    echo "[WARN] MANIFEST.json asset not found — verification uses the artifact's internal SHA256SUMS"
  fi
fi
rm -f "$HDR"

# 5. Download artifact (retry-safe: nothing was modified before this point) ---
echo "[INFO] downloading artifact: $ART_NAME"
umask 077; printf 'Authorization: Bearer %s\nAccept: application/octet-stream\n' "$TOKEN_VAR" > "$HDR"; umask 022
curl -fL --max-time 600 --retry 2 --header @"$HDR" -o "$WORK/$ART_NAME" "${API}/releases/assets/${ART_ID}" \
  || { echo "[FAIL] artifact download failed (safe to retry — nothing has been modified)"; rm -f "$HDR"; exit 1; }
rm -f "$HDR"; use_token ""; unset TOKEN_VAR
[[ -s "$WORK/$ART_NAME" ]] || { echo "[FAIL] downloaded artifact is empty"; exit 1; }

# 6. Integrity verification (mandatory, never skipped) ------------------------
ACTUAL_SHA="$(sha256sum "$WORK/$ART_NAME" | awk '{print $1}')"
echo "[OK]   artifact SHA-256: ${ACTUAL_SHA:0:16}…"
if [[ -n "$ART_SHA_EXPECTED" ]]; then
  [[ "$ACTUAL_SHA" == "$ART_SHA_EXPECTED" ]] || { echo "[FAIL] SHA-256 MISMATCH against trusted release manifest — artifact deleted, nothing installed"; rm -f "$WORK/$ART_NAME"; exit 1; }
  echo "[OK]   integrity verified against trusted release manifest"
fi
mkdir -p "$WORK/extracted" && tar -xzf "$WORK/$ART_NAME" -C "$WORK/extracted" \
  || { echo "[FAIL] artifact is corrupted (tar extraction failed) — deleted, nothing installed"; rm -f "$WORK/$ART_NAME"; exit 1; }
PKG_DIR="$WORK/extracted/${ART_NAME%.tar.gz}"
[[ -f "$PKG_DIR/install.sh" && -f "$PKG_DIR/VERSION" ]] || { echo "[FAIL] artifact structure invalid (engine not found)"; exit 1; }
NFILES="$(wc -l < "$PKG_DIR/SHA256SUMS" 2>/dev/null || echo 0)"
( cd "$PKG_DIR" && sha256sum -c SHA256SUMS --quiet ) \
  || { echo "[FAIL] internal SHA256SUMS verification FAILED — artifact deleted, nothing installed"; rm -rf "$WORK/extracted"; exit 1; }
echo "[OK]   all $NFILES artifact files verified (internal SHA256SUMS)"

# 7. Provenance + engine execution -------------------------------------------
ART_VERSION="$(head -1 "$PKG_DIR/VERSION")"
echo "--------------------------------------------------------"
echo "[INFO] Installed version : $ART_VERSION (development release)"
echo "[INFO] Release tag       : $TAG (pinned default)"
echo "[INFO] Artifact          : $ART_NAME"
echo "[INFO] Artifact SHA-256  : $ACTUAL_SHA"
echo "--------------------------------------------------------"
echo "[INFO] handing over to the shared installation engine…"
exec bash "$PKG_DIR/install.sh" "${ENGINE_ARGS[@]}"

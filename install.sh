#!/usr/bin/env bash
# ============================================================================
# Drevanta bootstrap installer — Method A (Direct GitHub installation).
#
#   Official repository : https://github.com/amungadev-ai/Drevanta_whm_repo
#
# What it does:
#   1. Validates OS/architecture (Debian family, linux/amd64).
#   2. Resolves the requested Drevanta release (default: the pinned
#      development release; stable releases after owner approval).
#   3. Retrieves the selected release's MANIFEST.json (MANDATORY — no
#      fallback verification path exists) and validates its schema,
#      product, channel, tag, commit, version and artifact identity.
#      The default development release must additionally match the pinned
#      full commit compiled into this bootstrap.
#   4. Downloads the artifact and requires an exact SHA-256 match against
#      the trusted manifest. Mismatched or corrupted artifacts are deleted
#      and nothing is installed.
#   5. Validates archive entry safety (no absolute paths, no path
#      traversal, no unsafe link targets or device entries), extracts with
#      --no-same-owner into a private temporary directory, and verifies
#      the artifact-internal SHA256SUMS INCLUDING complete file coverage.
#   6. Removes all credential material from the environment, then executes
#      the shared installation engine as a child process (so this
#      bootstrap's cleanup always runs) and propagates its exit code.
#
# The standard customer installation never compiles from source.
# The installation engine (and everything it starts) never inherits the
# GitHub access token.
#
# Minimal prerequisites (documented, auto-installed where possible):
#   bash + coreutils (present), curl (downloader), tar, python3 (manifest
#   parsing), root privileges. The engine installs nginx/Node.js itself.
#
# Private repository authentication:
#   While the repository is private, retrieval needs a GitHub token with
#   Contents:Read (least privilege, short-lived, repository-scoped). The
#   token is NEVER embedded in URLs, command arguments, shell history or
#   logs: it is read silently when needed and passed to curl via a 0600
#   header file inside a 0700 mktemp workspace, deleted on success,
#   failure and handled interruption. Retrieval is HTTPS-only
#   (curl --proto =https). Release-asset downloads follow GitHub's
#   documented presigned-redirect flow (api.github.com ->
#   objects.githubusercontent.com); no untrusted mirror is followed.
#   The stable distribution channel (after owner approval) will not
#   require customers to hold GitHub accounts or personal access tokens.
#
# Usage:
#   bash install.sh [bootstrap options] [-- engine options...]
# Bootstrap options:
#   --version <tag>      release tag to install (default: pinned dev release)
#   --require-commit <sha>  additionally require manifest.commit to equal <sha>
#   --tmpdir <dir>       working directory (default: mktemp -d, 0700)
#   --keep               keep the extracted artifact for inspection
#                        (credentials are never preserved)
#   -h | --help          this help
# Everything after `--` is passed to the shared installation engine
# (e.g. -- --access-ip 203.0.113.7 --mgmt-port 8443 --private-mgmt).
# ============================================================================
set -uo pipefail

REPO="amungadev-ai/Drevanta_whm_repo"
# API base override: verified self-hosted mirrors (offline/air-gapped Method C
# scenarios) and verification harnesses. Default is GitHub. HTTPS-only.
API="${DREVANTA_API_BASE:-https://api.github.com/repos/${REPO}}"
PINNED_TAG="v1.0.0-owneraudit.1-dev.2"        # development release (owner-approved dev artifact)
PINNED_COMMIT="6c6b92ee17f68a85446c1ac9639d3c1980f09e61"     # full commit the pinned dev release must be cut from
REQ_TAG=""
REQ_COMMIT=""
EXPECT_SHA=""
TMPDIR_ARG=""
KEEP="0"
TOKEN_VAR=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) REQ_TAG="$2"; shift 2 ;;
    --require-commit) REQ_COMMIT="$2"; shift 2 ;;
    --expect-artifact-sha) EXPECT_SHA="$2"; shift 2 ;;
    --tmpdir) TMPDIR_ARG="$2"; shift 2 ;;
    --keep) KEEP="1"; shift ;;
    -h|--help) sed -n '2,45p' "$0" | grep -E '^#'; exit 0 ;;
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

# 1. Platform validation (mandatory, before any modification) -----------------
ARCH="$(uname -m)"
if [[ "$ARCH" != "x86_64" ]]; then
  echo "[FAIL] architecture $ARCH is not supported by the release artifacts (linux/amd64)"; exit 1
fi
if ! command -v apt-get >/dev/null 2>&1; then
  echo "[FAIL] apt-get not found — supported: Ubuntu 20.04+, Debian 11+ (Debian family)"; exit 1
fi
# Parse os-release in a SUBSHELL (sourcing it would clobber shell variables
# such as VERSION with the distribution's values).
OS_LINE="$(. /etc/os-release 2>/dev/null && printf '%s|%s' "${ID:-}" "${VERSION_ID:-}")" \
  || { echo "[FAIL] /etc/os-release unreadable — unsupported system"; exit 1; }
OS_ID="${OS_LINE%%|*}"; OS_VER="${OS_LINE#*|}"
case "$OS_ID" in
  ubuntu) [[ "${OS_VER%%.*}" -ge 20 ]] || { echo "[FAIL] Ubuntu ${OS_VER} < 20.04 is not supported"; exit 1; } ;;
  debian) [[ "${OS_VER%%.*}" -ge 11 ]] || { echo "[FAIL] Debian ${OS_VER} < 11 is not supported"; exit 1; } ;;
  *) echo "[FAIL] unsupported distribution '${OS_ID:-unknown}' — supported: Ubuntu 20.04+, Debian 11+"; exit 1 ;;
esac
echo "[OK]   platform: ${PRETTY_NAME:-unknown} ($ARCH)"
for c in curl tar sha256sum python3; do
  if ! command -v "$c" >/dev/null 2>&1; then
    echo "[INFO] prerequisite missing: $c — installing via apt"
    apt-get update -qq >/dev/null 2>&1
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "$c" >/dev/null 2>&1 \
      || { echo "[FAIL] could not install prerequisite '$c' (network/repo issue) — install it manually and rerun"; exit 1; }
  fi
done
echo "[OK]   prerequisites: curl, tar, sha256sum, python3"

# 2. Private temporary workspace (0700; cleaned on success/failure/interrupt) -
WORK="${TMPDIR_ARG:-$(mktemp -d /tmp/drevanta-install.XXXXXX)}"
mkdir -p "$WORK" && chmod 700 "$WORK"
HDR="$WORK/.auth-header"   # root-only header file; token never in argv/URLs/logs
ART_SAVED="0"
cleanup() {
  local rc=$?
  rm -f "$HDR" 2>/dev/null            # credentials are ALWAYS removed
  if [[ "$KEEP" == "1" && "$ART_SAVED" == "1" ]]; then
    echo "[INFO] artifact kept (credentials excluded) at: $WORK"
  else
    rm -rf "$WORK" 2>/dev/null
  fi
  exit "$rc"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# 3. Authenticated retrieval (private repository) -----------------------------
use_token() { # use_token <token-or-empty> -> (re)write header file (0600)
  if [[ -n "$1" ]]; then
    umask 077; printf 'Authorization: Bearer %s\nAccept: application/vnd.github+json\n' "$1" > "$HDR"; umask 022
  else : > "$HDR"; fi
}
gh_get() { # gh_get <api-path> <output-file> -> 0 on HTTP 200 (HTTPS-only)
  local path="$1" out="$2" code proto
  # HTTPS-only for the default GitHub endpoint. An explicit DREVANTA_API_BASE
  # override (verified self-hosted mirror) may declare its own scheme; the
  # override is the operator's trust decision and is documented.
  proto="=https"; [[ "$API" == http://* ]] && proto="+http"
  code=$(curl --proto "$proto" -fsS --max-time 30 --header @"$HDR" -o "$out" -w '%{http_code}' "${API}${path}" 2>/dev/null || echo "000")
  [[ "$code" == "200" ]] || { rm -f "$out"; return 1; }
  return 0
}
TOKEN_VAR="${DREVANTA_GH_TOKEN:-${GITHUB_TOKEN:-}}"
use_token "$TOKEN_VAR"
if ! gh_get "/releases/tags/$TAG" "$WORK/release.json"; then
  echo "[INFO] release $TAG not reachable — authentication required (private repository)"
  if [[ ! -t 0 || "${DREVANTA_UNATTENDED:-0}" == "1" ]]; then
    echo "[FAIL] no token available: export DREVANTA_GH_TOKEN (Contents:Read, short-lived) and rerun"; exit 1
  fi
  printf 'GitHub token (Contents: Read; input hidden): '
  IFS= read -rs TOKEN_VAR; echo
  [[ -n "${TOKEN_VAR:-}" ]] || { echo "[FAIL] no token provided"; exit 1; }
  use_token "$TOKEN_VAR"
  gh_get "/releases/tags/$TAG" "$WORK/release.json" \
    || { echo "[FAIL] release $TAG still not reachable — check the token scope (Contents: Read) and the tag name"; exit 1; }
fi
echo "[OK]   release manifest retrieved: $TAG (exact release endpoint, not a repository-wide search)"

# 4. Manifest (MANDATORY) + artifact selection + schema validation ------------
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

# MANIFEST.json must exist among THIS release's assets (release.json is the
# exact selected release's asset collection) and be readable.
MANIFEST_ID="$(python3 -c "
import json,sys
rel=json.load(open(sys.argv[1]))
for a in rel.get('assets', []):
    if a.get('name')=='MANIFEST.json': print(a['id']); break
" "$WORK/release.json" 2>/dev/null || true)"
[[ -n "${MANIFEST_ID:-}" ]] || { echo "[FAIL] MANIFEST.json asset missing from release $TAG — refusing to install without trusted release metadata"; exit 1; }
umask 077; printf 'Authorization: Bearer %s\nAccept: application/octet-stream\n' "$TOKEN_VAR" > "$HDR"; umask 022
PROTO="=https"; [[ "$API" == http://* ]] && PROTO="+http"
curl --proto "$PROTO" -fsSL --max-time 30 --header @"$HDR" -o "$WORK/MANIFEST.json" "${API}/releases/assets/${MANIFEST_ID}" \
  || { echo "[FAIL] MANIFEST.json could not be retrieved — refusing to install"; exit 1; }

# Schema validation: product, channel, tag, commit, version, artifact identity.
python3 - "$WORK/MANIFEST.json" "$ART_NAME" "$TAG" <<'PY' || { echo "[FAIL] manifest validation failed — refusing to install"; exit 1; }
import json, re, sys
m = json.load(open(sys.argv[1]))
art, tag = sys.argv[2], sys.argv[3]
errs = []
if m.get("product") != "Drevanta": errs.append("product")
if m.get("channel") not in ("development-prerelease", "stable"): errs.append("channel")
if m.get("tag") != tag: errs.append("tag")
commit = m.get("commit", "")
if not re.fullmatch(r"[0-9a-f]{40}", commit): errs.append("commit")
if not m.get("version"): errs.append("version")
arts = m.get("artifacts")
if not isinstance(arts, dict) or arts.get(art) is None: errs.append("artifact entry")
else:
    if not re.fullmatch(r"[0-9a-f]{64}", str(arts[art]).lower()): errs.append("artifact sha256 format")
if errs:
    print("manifest schema errors: " + ", ".join(errs)); sys.exit(1)
PY
MANIFEST_SHA="$(python3 -c "
import json,sys
m=json.load(open(sys.argv[1]))
print(str(m['artifacts'][sys.argv[2]]).lower())" "$WORK/MANIFEST.json" "$ART_NAME")"
MANIFEST_COMMIT="$(python3 -c "
import json,sys
print(json.load(open(sys.argv[1]))['commit'])" "$WORK/MANIFEST.json")"
MANIFEST_ARTIFACT="$(python3 -c "
import json,sys
print(json.load(open(sys.argv[1]))['artifact'])" "$WORK/MANIFEST.json" 2>/dev/null || true)"
MANIFEST_VERSION="$(python3 -c "
import json,sys
print(json.load(open(sys.argv[1]))['version'])" "$WORK/MANIFEST.json")"
# Bind the pinned development release (and any explicit --require-commit) to
# the exact expected full commit.
if [[ "$TAG" == "$PINNED_TAG" && "$MANIFEST_COMMIT" != "$PINNED_COMMIT" ]]; then
  echo "[FAIL] pinned release commit mismatch: manifest $MANIFEST_COMMIT != expected $PINNED_COMMIT — refusing to install"; exit 1
fi
if [[ -n "$REQ_COMMIT" && "$MANIFEST_COMMIT" != "$REQ_COMMIT" ]]; then
  echo "[FAIL] requested commit mismatch: manifest $MANIFEST_COMMIT != expected $REQ_COMMIT — refusing to install"; exit 1
fi
if [[ -n "$EXPECT_SHA" && "${EXPECT_SHA,,}" != "$MANIFEST_SHA" ]]; then
  echo "[FAIL] artifact digest mismatch: manifest $MANIFEST_SHA != independently communicated $EXPECT_SHA — refusing to install"; exit 1
fi
if [[ -z "$MANIFEST_ARTIFACT" || "$MANIFEST_ARTIFACT" != "$ART_NAME" ]]; then
  echo "[FAIL] manifest artifact identity mismatch: manifest declares '${MANIFEST_ARTIFACT:-none}' but the release asset is '$ART_NAME' — refusing to install"; exit 1
fi
if [[ "$MANIFEST_VERSION" != *"+$MANIFEST_COMMIT" ]]; then
  echo "[FAIL] manifest version '$MANIFEST_VERSION' does not carry the release commit $MANIFEST_COMMIT — refusing to install"; exit 1
fi
echo "[OK]   trusted manifest validated (product, channel, tag, commit, version, artifact identity)"
[[ "$TAG" == "$PINNED_TAG" ]] && echo "[OK]   pinned release bound to commit $PINNED_COMMIT"
rm -f "$HDR"

# 5. Download artifact (retry-safe: nothing has been modified) ----------------
echo "[INFO] downloading artifact: $ART_NAME"
umask 077; printf 'Authorization: Bearer %s\nAccept: application/octet-stream\n' "$TOKEN_VAR" > "$HDR"; umask 022
curl --proto "$PROTO" -fL --max-redirs 10 --max-time 900 --retry 2 --header @"$HDR" -o "$WORK/$ART_NAME" "${API}/releases/assets/${ART_ID}" \
  || { echo "[FAIL] artifact download failed (safe to retry — nothing has been modified)"; exit 1; }
rm -f "$HDR"; use_token ""; unset TOKEN_VAR
[[ -s "$WORK/$ART_NAME" ]] || { echo "[FAIL] downloaded artifact is empty"; exit 1; }

# 6. Integrity verification (mandatory; exact match against trusted metadata) -
ACTUAL_SHA="$(sha256sum "$WORK/$ART_NAME" | awk '{print $1}')"
echo "[OK]   artifact SHA-256: ${ACTUAL_SHA:0:16}…"
[[ "$ACTUAL_SHA" == "$MANIFEST_SHA" ]] \
  || { echo "[FAIL] SHA-256 MISMATCH against trusted release manifest — artifact deleted, nothing installed"; rm -f "$WORK/$ART_NAME"; exit 1; }
echo "[OK]   exact SHA-256 match against trusted release manifest"

# 7. Archive safety validation, then extraction -------------------------------
PKG_BASE="${ART_NAME%.tar.gz}"
tar -tvzf "$WORK/$ART_NAME" > "$WORK/entries.txt" 2>/dev/null \
  || { echo "[FAIL] artifact is corrupted (tar listing failed) — deleted, nothing installed"; rm -f "$WORK/$ART_NAME"; exit 1; }
python3 - "$WORK/entries.txt" "$PKG_BASE" <<'PY' || { echo "[FAIL] unsafe archive entries detected — artifact deleted, nothing installed"; rm -f "$WORK/$ART_NAME"; exit 1; }
import os, sys
pkg = sys.argv[2]
ROOT = "/drevanta-extraction-root"   # synthetic mount point for containment math
def escapes(base_rel, target):
    # Resolve the target relative to the entry's directory inside the
    # extraction root; unsafe only if it escapes that root.
    resolved = os.path.normpath(os.path.join(ROOT, os.path.dirname(base_rel), target))
    return not (resolved == ROOT or resolved.startswith(ROOT + "/"))
bad = []
# tar -tvz lines: <mode> <owner/group> <size> <date> <time> <name> [-> <target>]
for line in open(sys.argv[1]):
    line = line.rstrip("\n")
    if not line: continue
    parts = line.split()
    if len(parts) < 6: continue
    mode = parts[0]
    rest = line.split(None, 5)
    name = rest[5]
    target = ""
    if " -> " in name:
        name, target = name.split(" -> ", 1)
    if name.startswith("/") or ".." in name.split("/") or not name.startswith(pkg + "/"):
        bad.append("unsafe path: " + name)
        continue
    if mode.startswith("l"):
        if target.startswith("/") or escapes(name, target):
            bad.append("unsafe link target (escapes extraction root): " + name + " -> " + target)
    if mode[0] in "bcpDs":
        bad.append("unsafe special entry: " + name)
if bad:
    print("\n".join(bad[:5])); sys.exit(1)
PY
mkdir -p "$WORK/extracted" && tar --no-same-owner -xzf "$WORK/$ART_NAME" -C "$WORK/extracted" \
  || { echo "[FAIL] artifact is corrupted (tar extraction failed) — deleted, nothing installed"; rm -f "$WORK/$ART_NAME"; exit 1; }
PKG_DIR="$WORK/extracted/$PKG_BASE"
[[ -f "$PKG_DIR/install.sh" && -f "$PKG_DIR/VERSION" && -f "$PKG_DIR/SHA256SUMS" ]] \
  || { echo "[FAIL] artifact structure invalid (engine/checksums not found)"; exit 1; }

# 8. Internal checksums: verify every listed file AND require coverage --------
( cd "$PKG_DIR" && sha256sum -c SHA256SUMS --quiet ) \
  || { echo "[FAIL] internal SHA256SUMS verification FAILED — artifact deleted, nothing installed"; rm -rf "$WORK/extracted"; exit 1; }
python3 - "$PKG_DIR" <<'PY' || { echo "[FAIL] internal SHA256SUMS coverage INCOMPLETE (unstated files present) — refusing to install"; exit 1; }
import os, sys
root = sys.argv[1]
listed = set()
for line in open(os.path.join(root, "SHA256SUMS")):
    p = line.split(maxsplit=1)
    if len(p) == 2: listed.add(p[1].lstrip("*").strip().lstrip("./"))
missing = []
for dirpath, dirnames, filenames in os.walk(root):
    for fn in filenames:
        full = os.path.join(dirpath, fn)
        if os.path.islink(full): continue   # symlinks: not checksummed (targets validated at extraction)
        rel = os.path.relpath(full, root)
        if rel == "SHA256SUMS": continue
        if rel not in listed: missing.append(rel)
if missing:
    print("\n".join(missing[:5])); sys.exit(1)
PY
echo "[OK]   internal SHA256SUMS verified with complete file coverage"
ART_VERSION="$(head -1 "$PKG_DIR/VERSION")"
[[ "$ART_VERSION" == "$MANIFEST_VERSION" ]] \
  || { echo "[FAIL] artifact VERSION '$ART_VERSION' disagrees with the manifest version '$MANIFEST_VERSION' — refusing to install"; exit 1; }
echo "[OK]   release identity agreement: tag=$TAG commit=$MANIFEST_COMMIT version=$MANIFEST_VERSION artifact=$ART_NAME"
ART_SAVED="1"   # --keep may now preserve the workspace (credentials already removed)

# 9. Provenance + credential scrub + engine execution (child process) ---------
echo "--------------------------------------------------------"
echo "[INFO] Installed version : $ART_VERSION"
echo "[INFO] Release tag       : $TAG (pinned default)"
echo "[INFO] Release commit    : $MANIFEST_COMMIT (manifest-bound)"
echo "[INFO] Artifact          : $ART_NAME"
echo "[INFO] Artifact SHA-256  : $ACTUAL_SHA"
echo "--------------------------------------------------------"
# The engine, the package manager, systemd services and their children must
# not inherit the GitHub access token.
unset DREVANTA_GH_TOKEN GITHUB_TOKEN GH_TOKEN 2>/dev/null || true
: > "$HDR"; rm -f "$HDR"
echo "[INFO] handing over to the shared installation engine…"
bash "$PKG_DIR/install.sh" "${ENGINE_ARGS[@]}"
rc=$?
exit "$rc"   # cleanup trap removes the workspace (or preserves it with --keep)

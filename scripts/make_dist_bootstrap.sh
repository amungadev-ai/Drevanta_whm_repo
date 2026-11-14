#!/usr/bin/env bash
# Drevanta distribution bootstrap generator.
#
# Emits a self-contained, auditable bootstrap.sh for the PUBLIC distribution
# repository. The generated script:
#   1. resolves the release base URL (stable latest / explicit version / dev tag)
#   2. downloads SHA256SUMS + its detached signature
#   3. verifies the signature against the EMBEDDED distribution public key
#      BEFORE trusting or executing anything else
#   4. downloads the artifact + MANIFEST.json, verifies them against the
#      signed SHA256SUMS, and enforces the release identity chain
#      (tag / commit / version / artifact filename / digest)
#   5. extracts, verifies the artifact-internal SHA256SUMS with file coverage
#      and the VERSION record
#   6. runs the installation engine shipped inside the verified artifact
#      (no token input required: the engine issues the claim token and prints
#      the onboarding URL at completion)
#   7. cleans up its temporary workspace
#
# Channels:
#   stable — resolves the latest NON-prerelease release at run time
#            (/releases/latest/download). No pinned commit: stable releases
#            are owner-approved.
#   dev    — pinned to an exact tag + commit (identity enforcement), labeled
#            DEVELOPMENT PRERELEASE.
#
# Usage: scripts/make_dist_bootstrap.sh --channel stable|dev \
#          --public-key <pem> [--dist-repo <owner/name>] \
#          [--tag <dev-tag>] [--commit <dev-commit>] --out <bootstrap.sh>
set -euo pipefail

CHANNEL=""; PUBKEY=""; DIST_REPO="amungadev-ai/drevanta-distribution"
DEV_TAG=""; DEV_COMMIT=""; OUT=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --channel) CHANNEL="$2"; shift 2 ;;
    --public-key) PUBKEY="$(cat "$2")"; shift 2 ;;
    --dist-repo) DIST_REPO="$2"; shift 2 ;;
    --tag) DEV_TAG="$2"; shift 2 ;;
    --commit) DEV_COMMIT="$2"; shift 2 ;;
    --out) OUT="$2"; shift 2 ;;
    *) echo "[FAIL] unknown option: $1"; exit 2 ;;
  esac
done
[[ "$CHANNEL" == "stable" || "$CHANNEL" == "dev" ]] || { echo "[FAIL] --channel stable|dev required"; exit 2; }
[[ -n "$PUBKEY" ]] || { echo "[FAIL] --public-key required"; exit 2; }
[[ -n "$OUT" ]] || { echo "[FAIL] --out required"; exit 2; }
if [[ "$CHANNEL" == "dev" ]]; then
  [[ -n "$DEV_TAG" && -n "$DEV_COMMIT" ]] || { echo "[FAIL] dev bootstrap needs --tag and --commit"; exit 2; }
fi

cat > "$OUT" <<BOOT
#!/usr/bin/env bash
# ============================================================================
#  Drevanta — one-command installation bootstrap
#  Repository: $DIST_REPO
#  Channel:    $CHANNEL
#  Deterministic build: identical inputs produce byte-identical bootstraps
#  (any reviewer can regenerate and compare against the published asset).
$( [[ "$CHANNEL" == "dev" ]] && echo "#  Pinned:     $DEV_TAG @ $DEV_COMMIT
#
#  This is a DEVELOPMENT PRERELEASE for the owner-led acceptance test —
#  explicitly NOT a production release." )
#
#  Trust model: this bootstrap embeds the Drevanta distribution public key.
#  It verifies the detached signature over SHA256SUMS BEFORE using it, then
#  verifies the artifact and MANIFEST.json against the signed checksums, and
#  enforces the release identity chain before executing anything. Only then
#  does it run the installation engine shipped inside the verified artifact.
#
#  curl-pipe-bash honesty: the bootstrap itself is fetched over HTTPS and is
#  the initial trust anchor (there is no way to verify the very first file
#  without a pre-trusted copy). Security-conscious installation: download
#  bootstrap.sh + bootstrap.sh.sha256 + bootstrap.sh.sig, verify the digest
#  and signature with the published public key, then execute the verified
#  file — see docs/VERIFICATION.md in the distribution repository.
#
#  Usage:
#    curl -fsSL <release-url>/bootstrap.sh | bash
#  Options (append after a ' -- ' when piping, or run the downloaded file):
#    --version <tag>     install an explicit release version
#    --artifact <path>   install from a local verified package — mandatory
#                        dependencies (nginx, Node.js, npm, curl, git) are
#                        still provisioned online; add nothing to stay online
#    --recover           repair an existing installation in place: installs
#                        missing dependencies, re-creates the nginx
#                        configuration, restarts the console and re-verifies
#                        every management access path. Data, TLS material,
#                        installation identity and the pending owner claim
#                        are preserved (no OS reinstall, no data loss)
#    --private-mgmt      management listener on loopback only (SSH tunnel)
#    --instance <name>   multi-instance name (default: drevanta)
#    --web-port/--api-port/--nginx-port/--mgmt-port <n>
#    --upgrade           upgrade an existing installation
#    --tmpdir <dir> --keep
# ============================================================================
set -uo pipefail

DIST_REPO="$DIST_REPO"
CHANNEL="$CHANNEL"
DEV_TAG="$DEV_TAG"
DEV_COMMIT="$DEV_COMMIT"

die() { echo "[FAIL] \$*" >&2; exit 1; }
info() { echo "[INFO] \$*"; }
ok()   { echo "[OK]   \$*"; }

for tool in curl sha256sum tar python3 openssl; do
  command -v \$tool >/dev/null 2>&1 || die "\$tool is required but not installed"
done
[[ \$EUID -eq 0 ]] || die "run as root (sudo bash / curl ... | sudo bash -s -- <args>)"

WORK=""; KEEP=0
ENGINE_ARGS=()
EXPLICIT_VERSION=""; ARTIFACT_LOCAL=""
while [[ \$# -gt 0 ]]; do
  case "\$1" in
    --version) EXPLICIT_VERSION="\$2"; shift 2 ;;
    --artifact) ARTIFACT_LOCAL="\$2"; shift 2 ;;
    --private-mgmt|--upgrade|--recover) ENGINE_ARGS+=("\$1"); shift ;;
    --instance|--web-port|--api-port|--nginx-port|--mgmt-port|--access-ip) ENGINE_ARGS+=("\$1" "\$2"); shift 2 ;;
    --tmpdir) WORK="\$2"; shift 2 ;;
    --keep) KEEP=1; shift ;;
    -h|--help) sed -n '3,40p' "\$0"; exit 0 ;;
    *) die "unknown option: \$1 (usage: see the header of this file)" ;;
  esac
done
ENGINE_ARGS+=()

if [[ -z "\$WORK" ]]; then WORK="\$(mktemp -d)" || die "mktemp failed"; fi
mkdir -p "\$WORK" && chmod 700 "\$WORK"
cleanup() { [[ "\$KEEP" == "1" ]] && echo "[INFO] workspace kept: \$WORK" || rm -rf "\$WORK"; }
trap cleanup EXIT
[[ -n "\${DREVANTA_DIST_DEBUG:-}" ]] && { { echo "== bootstrap env =="; echo "PATH=\$PATH"; type -a curl; } >> "\$DREVANTA_DIST_DEBUG" 2>&1; }

# Release base URL -----------------------------------------------------------
if [[ -n "\$ARTIFACT_LOCAL" ]]; then
  [[ -s "\$ARTIFACT_LOCAL" ]] || die "offline artifact not found: \$ARTIFACT_LOCAL"
  DIST_DIR="\$(dirname "\$ARTIFACT_LOCAL")"
  info "offline installation from \$ARTIFACT_LOCAL"
else
  if [[ -n "\${DREVANTA_DIST_BASE:-}" ]]; then
    # Test/evaluation override of the release base URL. Safe: every download
    # is still signature-verified against the embedded key before use.
    BASE="\$DREVANTA_DIST_BASE"
  elif [[ "\$CHANNEL" == "stable" ]]; then
    if [[ -n "\$EXPLICIT_VERSION" ]]; then
      BASE="https://github.com/\$DIST_REPO/releases/download/\$EXPLICIT_VERSION"
    else
      BASE="https://github.com/\$DIST_REPO/releases/latest/download"
    fi
  else
    BASE="https://github.com/\$DIST_REPO/releases/download/\$DEV_TAG"
  fi
  DIST_DIR="\$WORK"
  case "\$BASE" in
    https://*) CURL_PROTO="--proto =https" ;;
    *)         CURL_PROTO="--proto =http,https" ;;   # DREVANTA_DIST_BASE test override only
  esac
  info "release base: \$BASE"
fi

# Trust anchor: the embedded distribution public key --------------------------
cat > "\$WORK/drevanta-dist.pem" <<'PUBKEY'
$PUBKEY
PUBKEY
openssl ec -pubin -in "\$WORK/drevanta-dist.pem" -noout >/dev/null 2>&1 || die "embedded public key invalid"

if [[ -z "\$ARTIFACT_LOCAL" ]]; then
  # 1. Signed checksums FIRST — nothing else is trusted before this passes.
  curl \$CURL_PROTO --tlsv1.2 -fsSL -o "\$DIST_DIR/SHA256SUMS" "\$BASE/SHA256SUMS" || die "download failed: SHA256SUMS"
  curl \$CURL_PROTO --tlsv1.2 -fsSL -o "\$DIST_DIR/SHA256SUMS.sig" "\$BASE/SHA256SUMS.sig" || die "download failed: SHA256SUMS.sig"
fi
[[ -s "\$DIST_DIR/SHA256SUMS" && -s "\$DIST_DIR/SHA256SUMS.sig" ]] || die "SHA256SUMS or signature missing"
openssl dgst -sha256 -verify "\$WORK/drevanta-dist.pem" -signature "\$DIST_DIR/SHA256SUMS.sig" "\$DIST_DIR/SHA256SUMS" >/dev/null \\
  || die "SIGNATURE VERIFICATION FAILED for SHA256SUMS — refusing to continue"
ok "SHA256SUMS signature verified against the embedded distribution key"

# 2. Artifact + manifest, verified against the signed checksums --------------
ART_NAME="\$(grep -oE 'drevanta-install-[A-Za-z0-9._+-]+\\.tar\\.gz' "\$DIST_DIR/SHA256SUMS" | head -1)"
[[ -n "\$ART_NAME" ]] || die "no installation artifact listed in SHA256SUMS"
if [[ -z "\$ARTIFACT_LOCAL" ]]; then
  curl \$CURL_PROTO --tlsv1.2 -fsSL -o "\$DIST_DIR/\$ART_NAME" "\$BASE/\$ART_NAME" || die "download failed: \$ART_NAME"
  curl \$CURL_PROTO --tlsv1.2 -fsSL -o "\$DIST_DIR/MANIFEST.json" "\$BASE/MANIFEST.json" || die "download failed: MANIFEST.json"
fi
[[ -s "\$DIST_DIR/MANIFEST.json" ]] || die "MANIFEST.json missing in the release/package"
( cd "\$DIST_DIR" && grep -E "(\\\$ART_NAME|MANIFEST.json)\$" SHA256SUMS | sha256sum --quiet -c - ) \\
  || die "checksum mismatch for the artifact or manifest — refusing to continue"
ok "artifact + manifest match the signed SHA256SUMS"

# 3. Release identity chain ---------------------------------------------------
read -r M_TAG M_COMMIT M_VERSION M_ART M_SHA <<EOFIDS
\$(python3 - "\$DIST_DIR/MANIFEST.json" <<'PYID'
import json, sys
m = json.load(open(sys.argv[1]))
print(m.get("tag",""), m.get("commit",""), m.get("version",""), m.get("artifact",""), m.get("artifacts",{}).get(m.get("artifact",""),""))
PYID
)
EOFIDS
[[ "\$(python3 -c "import json,sys; print(json.load(open(sys.argv[1])).get('product',''))" "\$DIST_DIR/MANIFEST.json")" == "Drevanta" ]] || die "manifest product mismatch"
EXPECTED_TAG="\${EXPLICIT_VERSION:-\$DEV_TAG}"
if [[ -n "\$EXPECTED_TAG" ]]; then
  [[ "\$M_TAG" == "\$EXPECTED_TAG" ]] || die "release tag mismatch: manifest \$M_TAG != expected \$EXPECTED_TAG — refusing to install"
fi
[[ -n "\$M_TAG" ]] || die "manifest tag empty — refusing to install"
if [[ "\$CHANNEL" == "dev" && -n "\$DEV_COMMIT" ]]; then
  [[ "\$M_COMMIT" == "\$DEV_COMMIT" ]] || die "pinned release commit mismatch: manifest \$M_COMMIT != expected \$DEV_COMMIT — refusing to install"
fi
[[ "\$M_VERSION" == *"+\$M_COMMIT" ]] || die "manifest version does not carry the release commit"
[[ "\$M_ART" == "\$ART_NAME" ]] || die "manifest artifact identity mismatch: \$M_ART vs \$ART_NAME"
MANIFEST_SHA="\$(grep -E " \$ART_NAME\$" "\$DIST_DIR/SHA256SUMS" | awk '{print \$1}')"
[[ -n "\$MANIFEST_SHA" ]] || die "artifact line missing from the signed SHA256SUMS"
[[ "\$M_SHA" == "\$MANIFEST_SHA" ]] || die "manifest digest disagrees with the signed SHA256SUMS"
if [[ "\$CHANNEL" == "dev" ]]; then
  info "DEVELOPMENT PRERELEASE \$M_TAG (commit \${M_COMMIT:0:12}) — not a production release"
else
  info "stable release \$M_TAG (commit \${M_COMMIT:0:12})"
fi
ok "release identity chain verified"

# 4. Extract + verify the artifact's internal checksums -----------------------
PKG="\$WORK/\${ART_NAME%.tar.gz}"
mkdir -p "\$PKG"
tar --no-same-owner -xzf "\$DIST_DIR/\$ART_NAME" -C "\$WORK" || die "artifact extraction failed"
[[ -d "\$PKG" ]] || die "unexpected artifact layout"
( cd "\$PKG" && sha256sum -c SHA256SUMS --quiet >/dev/null ) || die "internal checksum verification failed"
FILES_TOTAL=\$(find "\$PKG" -type f ! -name SHA256SUMS | wc -l)
FILES_COVERED=\$(wc -l < "\$PKG/SHA256SUMS")
[[ "\$FILES_TOTAL" -le "\$FILES_COVERED" ]] || die "internal checksum coverage incomplete (\$FILES_COVERED/\$FILES_TOTAL)"
ART_VERSION="\$(head -1 "\$PKG/VERSION")"
[[ "\$ART_VERSION" == "\$M_VERSION" ]] || die "artifact VERSION disagrees with the manifest"
ok "internal SHA256SUMS verified with full coverage (\$FILES_COVERED files)"

# 5. Credentials scrub is unnecessary: this bootstrap NEVER handles
#    credentials — the public release needs no GitHub token at all.

# 6. Run the verified installation engine ------------------------------------
echo "--------------------------------------------------------"
[[ -n "\${DREVANTA_DIST_DEBUG:-}" ]] && { { echo "== pre-engine env =="; echo "PATH=\$PATH"; type -a curl; } >> "\$DREVANTA_DIST_DEBUG" 2>&1; }
bash "\$PKG/install.sh" "\${ENGINE_ARGS[@]}"   # the engine ships as install.sh inside the verified artifact
RC=\$?
echo "--------------------------------------------------------"
if [[ \$RC -eq 0 ]]; then
  ok "installation completed — open the onboarding URL above in your browser"
else
  die "installation engine exited with \$RC — nothing was silently ignored; see the [FAIL] lines above"
fi
exit \$RC
BOOT

chmod 755 "$OUT"
bash -n "$OUT" || { echo "[FAIL] generated bootstrap failed bash -n"; exit 1; }
echo "[OK] distribution bootstrap generated: $OUT (channel $CHANNEL)"

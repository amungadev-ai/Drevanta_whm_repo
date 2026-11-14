#!/usr/bin/env bash
# Anonymous release verification for the dev.6 distribution flows.
#
# Runs the LITERAL customer commands (fresh + recovery) inside an isolated
# namespace: the bootstrap and assets are downloaded from REAL GitHub with an
# anonymous curl (no token, no auth headers anywhere in the customer flow);
# system services are stubbed per the suite harness (disclosed); platform
# endpoint responses are simulated; all cryptographic verification is real.
# Usage: scripts/verify_release_anon.sh <tag> [OUTDIR]
set -u
TAG="${1:?tag required, e.g. v1.0.0-owneraudit.1-dev.6}"
OUT="${2:-/tmp/anon-release-evidence}"
rm -rf "$OUT"; mkdir -p "$OUT/logs"
BASE="https://github.com/amungadev-ai/drevanta-distribution/releases/download/$TAG"

# Suite-style stubs (mock services; the curl stub passes github.com through to
# the REAL network so the customer flow downloads genuine release assets).
STUB="$OUT/stub"; mkdir -p "$STUB"
cat > "$STUB/systemctl" <<'EOF'
#!/usr/bin/env bash
[[ "$1" == is-active ]] && { echo active; exit 0; }
last="${@: -1}"
case "$last" in
  *-helper)
    inst="${last%-helper}"; inst="${inst#drevanta}"; inst="${inst#-}"
    run="/run/drevanta"; [[ -n "$inst" ]] && run="/run/drevanta-$inst"
    mkdir -p "$run"
    python3 -c "
import socket, sys, os
p = sys.argv[1]
try: os.unlink(p)
except OSError: pass
sk = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
sk.bind(p); os.chmod(p, 0o750)
" "$run/helper.sock" 2>/dev/null || true
    ;;
esac
exit 0
EOF
printf '#!/usr/bin/env bash\nif [[ "$1" == "install" ]]; then\n  for p in "$@"; do case "$p" in\n    nginx)  printf "#!/usr/bin/env bash\\nexit 0\\n" > "%s/nginx" ;;\n    nodejs) printf "#!/usr/bin/env bash\\n[[ \\"$1\\" == \\"--version\\" ]] && echo v22.11.0\\nexit 0\\n" > "%s/node" ;;\n    npm)    printf "#!/usr/bin/env bash\\nexit 0\\n" > "%s/npm" ;;\n    git)    printf "#!/usr/bin/env bash\\nexit 0\\n" > "%s/git" ;;\n  esac; done\nfi\nexit 0\n' "$STUB" "$STUB" "$STUB" "$STUB" > "$STUB/apt-get"
printf '#!/usr/bin/env bash\nif [[ -e /tmp/drevanta-simulate-owned-ports ]]; then\n  for p in 80 8080 3100 8443; do\n    echo "LISTEN 0 128 127.0.0.1:$p 0.0.0.0:* users:((\\"node\\",pid=4242,fd=20))"\n  done\n  exit 0\nfi\nexit 1\n' > "$STUB/ss"
printf '#!/usr/bin/env bash\n[[ "$1" == status ]] && echo "Status: inactive"\nexit 0\n' > "$STUB/ufw"
cat > "$STUB/useradd" <<'EOF'
#!/usr/bin/env bash
U="${@: -1}"
echo "$U:x:0:0::/var/empty:/usr/sbin/nologin" >> /etc/passwd
echo "$U:x:0:" >> /etc/group
exit 0
EOF
cat > "$STUB/sudo" <<'EOF'
#!/usr/bin/env bash
while [[ "$1" == "-u" || "$1" == "-g" || "$1" == "-E" ]]; do
  case "$1" in -u|-g) shift 2 ;; *) shift ;; esac
done
exec "$@"
EOF
cat > "$STUB/curl" <<'EOF'
#!/usr/bin/env bash
ORIG=("$@")
GOT_O=0
out="/dev/null"; url=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o) out="$2"; GOT_O=1; shift 2 ;;
    -w) fmt="$2"; shift ;;
    --header|--resolve|--max-time|--retry|-s|-k|-f|-S|-L|-4) shift ;;
    -*) shift ;;
    *) url="$1"; shift ;;
  esac
done
case "$url" in
  *github.com*|*githubusercontent.com*|*deb.nodesource.com*|*archive.ubuntu.com*) exec /usr/bin/curl "${ORIG[@]}" ;;
  *api.ipify.org*) if [[ "$GOT_O" == "1" ]]; then printf '203.0.113.7' > "$out"; else printf '203.0.113.7'; fi ;;
  *install/status*) printf '{"installation_id":"inst_anon0000000000000000","state":"pending"}' > "$out"; echo 200 ;;
  *) printf 'ok' > "$out"; echo 200 ;;
esac
EOF
chmod +x "$STUB"/*

cat > "$OUT/session.sh" <<'SEOF'
export OUT="${T7OUT:?}"
STUB="$OUT/stub"
BASE="${T7BASE:?}"
cp -a /etc /tmp/etc.keep 2>/dev/null
mount -t tmpfs -o size=8g tmpfs /etc && mount -t tmpfs -o size=8g tmpfs /opt && mount -t tmpfs -o size=8g tmpfs /var && mount -t tmpfs tmpfs /usr/local && mount -t tmpfs tmpfs /run
cp -a /tmp/etc.keep/. /etc/ 2>/dev/null; rm -rf /tmp/etc.keep
export HOME=/root
export PATH="$STUB:$PATH"

echo "== FLOW 1: fresh install via the LITERAL customer command (anonymous) =="
curl -fsSL "$BASE/bootstrap.sh" | bash -s -- --access-ip 203.0.113.7 > "$OUT/flow1-fresh.log" 2>&1
echo "flow1 rc: $?" >> "$OUT/flow1-fresh.log"
echo "no-token check:" >> "$OUT/flow1-fresh.log"
grep -qE "clm_[0-9a-f]{62}" "$OUT/flow1-fresh.log" && echo "TOKEN LEAK" >> "$OUT/flow1-fresh.log" || echo "no token material in output" >> "$OUT/flow1-fresh.log"

echo "== FLOW 2: recovery of a partial dev.5-style installation =="
mkdir -p /opt/drevanta-p1/bin /opt/drevanta-p1/web/build /opt/drevanta-p1/migrations /var/lib/drevanta-p1 /etc/drevanta-p1/tls /var/log/drevanta-p1
printf '1.0.0-owneraudit.1-dev.5\n' > /opt/drevanta-p1/VERSION
printf 'placeholder\n' > /opt/drevanta-p1/bin/panel
printf 'console\n' > /opt/drevanta-p1/web/build/index.js
BOOT_OUT=$("$PKGDIR/bin/panel" bootstrap -db /var/lib/drevanta-p1/panel.db -migrations "$PKGDIR/migrations" 2>&1) || true
R1TOKEN=$(printf '%s' "$BOOT_OUT" | grep -A1 "DREVANTA CLAIM TOKEN" | tail -1 | tr -d '[:space:]')
umask 077; printf '%s' "${R1TOKEN:-clm_placeholder}" > /etc/drevanta-p1/claim-token; umask 022
printf '{"policy":true}\n' > /etc/drevanta-p1/helper-policy.json
openssl req -x509 -newkey rsa:2048 -keyout /etc/drevanta-p1/tls/key.pem -out /etc/drevanta-p1/tls/cert.pem -days 5 -nodes -subj "/CN=203.0.113.7" -addext "subjectAltName=IP:203.0.113.7" >/dev/null 2>&1
sha256sum /etc/drevanta-p1/claim-token /var/lib/drevanta-p1/panel.db /etc/drevanta-p1/tls/cert.pem /etc/drevanta-p1/tls/key.pem > "$OUT/flow2-before.sha"
touch /tmp/drevanta-simulate-owned-ports
curl -fsSL "$BASE/bootstrap.sh" | bash -s -- --recover > "$OUT/flow2-recovery.log" 2>&1
echo "flow2 rc: $?" >> "$OUT/flow2-recovery.log"
rm -f /tmp/drevanta-simulate-owned-ports
sha256sum -c "$OUT/flow2-before.sha" > "$OUT/flow2-preserved.txt" 2>&1; echo "preserved rc: $?" >> "$OUT/flow2-preserved.txt"
SEOF

PKG="$OUT/pkg"; mkdir -p "$PKG"
tar --no-same-owner -xzf "$(dirname "$0")"/../drevanta-install-*.tar.gz -C "$PKG" 2>/dev/null || true
PKGDIR="$(echo "$PKG"/drevanta-install-* 2>/dev/null)"
T7OUT="$OUT" T7BASE="$BASE" T7PKGDIR="$PKGDIR" unshare -rm bash "$OUT/session.sh"

echo "=== FLOW 1 (fresh, literal command, anonymous) ==="
grep -cE "\[OK\]" "$OUT/flow1-fresh.log" 2>/dev/null | sed 's/^/gates passed: /'
grep -E "INSTALLATION COMPLETE|signature verified|identity chain verified|DEVELOPMENT PRERELEASE|no token material|TOKEN LEAK|release base" "$OUT/flow1-fresh.log" | head -6
echo "=== FLOW 2 (recovery, literal command) ==="
grep -E "RECOVERY COMPLETE|recovery rc|preserved rc|pending owner claim|management HTTPS" "$OUT/flow2-recovery.log" "$OUT/flow2-preserved.txt" 2>/dev/null | head -8

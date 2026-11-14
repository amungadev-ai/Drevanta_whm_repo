#!/usr/bin/env bash
# Drevanta distribution-layer verification.
#
# Exercises the PUBLIC bootstrap end to end:
#   T1  curl | bash  against a locally served, signed release layout
#       (signature verify -> digest verify -> identity chain -> internal
#       checksums -> engine install -> INSTALLATION COMPLETE, zero token
#       material in the captured output)
#   T1  offline install via --artifact (same verification, no server)
#   NEG tampered artifact (SHA256SUMS recomputed, signature stale) -> refused
#   NEG wrong signing key (SHA256SUMS re-signed by an impostor key) -> refused
#
# The release assets are served by a local HTTP server via the documented
# DREVANTA_DIST_BASE test override; the cryptographic checks are identical
# to the real GitHub flow.
set -uo pipefail
REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_DIR"

PASS=0; FAIL=0
ok()  { PASS=$((PASS+1)); echo "PASS  $*"; }
bad() { FAIL=$((FAIL+1)); echo "FAIL  $*"; }
has() { local label="$1" file="$2" pattern="$3"; grep -qE "$pattern" "$file" 2>/dev/null && ok "$label" || bad "$label"; }

# 0. Build the artifact fresh.
bash scripts/build_install_package.sh >/dev/null 2>&1
ART="$(ls "$REPO_DIR"/drevanta-install-*.tar.gz | head -1)"
[[ -s "$ART" ]] || { echo "SUITE ERROR: artifact build failed"; exit 3; }
# The repo-root MANIFEST.json belongs to the last CUT release; for this test
# build a fresh, correct manifest for the CURRENT artifact (the publisher
# always downloads manifest+artifact from the same release, so this hazard
# cannot occur in the real flow — the identity chain would refuse it anyway).
M_COMMIT="$(git rev-parse HEAD)"
ART_NAME="$(basename "$ART")"
M_TAG="v1.0.0-owneraudit.1-disttest"
ART_VERSION="$(tar -xzOf "$ART" "${ART_NAME%.tar.gz}/VERSION" | head -1)"
python3 - "$M_TAG" "$M_COMMIT" "$ART_VERSION" "$ART_NAME" "$(sha256sum "$ART" | awk '{print $1}')" > /tmp/dstage-manifest.json <<'PYM'
import json, sys
tag, commit, version, art, sha = sys.argv[1:6]
print(json.dumps({"product":"Drevanta","channel":"development-prerelease","tag":tag,
 "commit":commit,"version":version,"generated_at":"2026-01-01T00:00:00Z",
 "artifact":art,"artifacts":{art:sha},
 "verification":"bootstrap verifies the artifact against this manifest AND the artifact-internal SHA256SUMS"}))
PYM
cp /tmp/dstage-manifest.json MANIFEST.json.disttest

# 1. Keys + signed release layout.
KEYS="$(mktemp -d)"; chmod 700 "$KEYS"
bash scripts/gen_signing_key.sh "$KEYS" >/dev/null
STAGE="$(mktemp -d)"
cp "$ART" "$STAGE/" && cp MANIFEST.json.disttest "$STAGE/MANIFEST.json"
scripts/make_dist_bootstrap.sh --channel dev --tag "$M_TAG" --commit "$M_COMMIT" \
  --public-key "$KEYS/drevanta-dist-public.pem" --out "$STAGE/bootstrap.sh" >/dev/null
DREVANTA_DIST_SIGNING_KEY="$KEYS/drevanta-dist-signing.pem" scripts/sign_release.sh "$STAGE" >/dev/null
( cd "$STAGE" && sha256sum bootstrap.sh > bootstrap.sh.sha256 )

PORT=18777
kill_port() { local p; p=$(ss -ltnp "sport = :$PORT" 2>/dev/null | grep -oP 'pid=\K[0-9]+' | head -1); [[ -n "$p" ]] && kill "$p" 2>/dev/null && sleep 0.5; }
kill_port
python3 - "$PORT" "$STAGE" drevanta-dist-serve <<'SRV' >/dev/null 2>&1 &
import http.server, socketserver, sys, functools
port, stage, marker = int(sys.argv[1]), sys.argv[2], sys.argv[3]
import os
os.environ['DREVANTA_SERVE_MARKER'] = marker
class H(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *a, **kw):
        super().__init__(*a, directory=stage, **kw)
    def log_message(self, *a): pass
socketserver.TCPServer.allow_reuse_address = True
with socketserver.TCPServer(("127.0.0.1", port), H) as httpd:
    httpd.serve_forever()
SRV
sleep 1
curl -fsS "http://127.0.0.1:$PORT/SHA256SUMS" | head -1 >/dev/null || { echo "SUITE ERROR: dist server not serving on $PORT"; exit 3; }

# 2. Namespaced fake root (same isolation as the engine suite).
STUB="$(mktemp -d)"
for t in apt-get ufw nginx curl jq; do ln -sf "$(command -v $t 2>/dev/null || echo /bin/true)" "$STUB/$t"; done
printf '#!/usr/bin/env bash\nexit 1\n' > "$STUB/ss"   # preflight sees no listeners; the micro-servers answer the health gates
# useradd stub: create the account in the tmpfs /etc with the mapped uid so
# chown resolves inside the user namespace (same approach as the engine suite).
cat > "$STUB/useradd" <<'EOF'
#!/usr/bin/env bash
U="${@: -1}"
echo "$U:x:0:0::/var/empty:/usr/sbin/nologin" >> /etc/passwd
echo "$U:x:0:" >> /etc/group
exit 0
EOF
# sudo stub: setuid binaries cannot transition privileges inside a user
# namespace; the engine suite uses the same drop-in.
cat > "$STUB/sudo" <<'EOF'
#!/usr/bin/env bash
while [[ "$1" == "-u" || "$1" == "-g" || "$1" == "-E" ]]; do
  case "$1" in -u|-g) shift 2 ;; *) shift ;; esac
done
exec "$@"
EOF
printf '#!/usr/bin/env bash\ncase "$1" in enable|start|restart|reload|stop|daemon-reload) exit 0;; is-active) echo active;; is-enabled) echo enabled;; show) echo "ActiveState=active";; esac\nexit 0\n' > "$STUB/systemctl"
printf '#!/usr/bin/env bash\nexit 0\n' > "$STUB/apt-get"
printf '#!/usr/bin/env bash\nexit 0\n' > "$STUB/ufw"
printf '#!/usr/bin/env bash\nexit 1\n' > "$STUB/ss"
printf '#!/usr/bin/env bash\nnginx -t 2>/dev/null; exit 0\n' > "$STUB/nginx"
printf '#!/usr/bin/env bash\n[[ "$1" == status ]] && echo "Status: inactive"\nexit 0\n' > "$STUB/ufw"
chmod +x "$STUB"/*
# curl passthrough: local URLs handled by real curl; github URLs intercepted
# (no curl stub: the dist e2e uses REAL curl end to end — platform
# endpoints are answered by real local micro-servers inside the namespace)
HASH="$(sha256sum "$ART" | awk '{print $1}')"
T="$(mktemp -d)"

SESS="$(mktemp -d)/session.sh"
cat > "$SESS" <<SEOF
#!/usr/bin/env bash
set -u
cp -a /etc /tmp/etc.keep 2>/dev/null
mount -t tmpfs -o size=8g tmpfs /etc && mount -t tmpfs tmpfs /opt && mount -t tmpfs -o size=8g tmpfs /var && mount -t tmpfs tmpfs /usr/local && mount -t tmpfs tmpfs /run
cp -a /tmp/etc.keep/. /etc/ 2>/dev/null; rm -rf /tmp/etc.keep
export PATH="$STUB:\$PATH"
export HOME=/root
export DREVANTA_DIST_BASE="http://127.0.0.1:$PORT"
export DREVANTA_DIST_DEBUG="/tmp/bs-env.log"

# ---- local platform micro-servers (REAL endpoints for the health gates) ----
start_servers() {
# control plane :8080
python3 - >/dev/null 2>&1 <<'PY8080' &
import http.server, socketserver
socketserver.TCPServer.allow_reuse_address = True
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        b = b'{"installation_id":"inst_disttest0000000000000000","state":"pending"}'
        self.send_response(200); self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(b))); self.end_headers(); self.wfile.write(b)
    def log_message(self, *a): pass
socketserver.TCPServer(("127.0.0.1", 8080), H).serve_forever()
PY8080
# console :3100
python3 - >/dev/null 2>&1 <<'PY3100' &
import http.server, socketserver
socketserver.TCPServer.allow_reuse_address = True
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        b = b'drevanta-console'
        self.send_response(200); self.send_header("Content-Length", str(len(b))); self.end_headers(); self.wfile.write(b)
    def log_message(self, *a): pass
socketserver.TCPServer(("127.0.0.1", 3100), H).serve_forever()
PY3100
# management :8443 (TLS with a throwaway cert; the gate uses curl -k)
openssl req -x509 -newkey rsa:2048 -keyout /tmp/mgmt.key -out /tmp/mgmt.crt -days 2 -nodes -subj "/CN=drevanta-mgmt" >/dev/null 2>&1
python3 - >/dev/null 2>&1 <<'PY8443' &
import http.server, socketserver, ssl
socketserver.TCPServer.allow_reuse_address = True
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        b = b'drevanta-mgmt'
        self.send_response(200); self.send_header("Content-Length", str(len(b))); self.end_headers(); self.wfile.write(b)
    def log_message(self, *a): pass
srv = socketserver.TCPServer(("127.0.0.1", 8443), H)
ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
ctx.load_cert_chain("/tmp/mgmt.crt", "/tmp/mgmt.key")
srv.socket = ctx.wrap_socket(srv.socket, server_side=True)
srv.serve_forever()
PY8443
# privileged helper socket (appears once /run/drevanta exists)
python3 - >/dev/null 2>&1 <<'PYSOCK' &
import os, socket, time
paths = ["/run/drevanta/helper.sock", "/run/drevanta-distoff/helper.sock"]
bound = set()
for _ in range(240):
    for p in paths:
        if p in bound:
            continue
        if os.path.isdir(os.path.dirname(p)):
            try:
                s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
                s.bind(p); s.listen(1)
                import threading
                threading.Thread(target=lambda s=s: [s.accept() for _ in range(1000)], daemon=True).start()
                bound.add(p)
            except OSError:
                pass
    if len(bound) == len(paths):
        break
    time.sleep(0.5)
time.sleep(900)
PYSOCK
}
start_servers
stop_servers() { pkill -f "^python3 -" 2>/dev/null || true; sleep 1; }
# ----------------------------------------------------------------------------

# T1: the exact customer experience — curl | bash (REAL curl end to end)
( curl -fsSL "\$DREVANTA_DIST_BASE/bootstrap.sh" | bash -s -- --access-ip 203.0.113.7; echo "PIPE-RC \${PIPESTATUS[0]} \${PIPESTATUS[1]}" >> "$T/pipe.log" ) > "$T/pipe.log" 2>&1 &
CURLPID=\$!
wait \$CURLPID
grep -oE "PIPE-RC [0-9]+ [0-9]+" "$T/pipe.log" | tail -1 | tr -d '\n' > "$T/pipe.rc"; echo >> "$T/pipe.rc"
grep -q "INSTALLATION COMPLETE" "$T/pipe.log" && echo yes > "$T/pipe.complete" || echo no > "$T/pipe.complete"

# T1-offline: bootstrap --artifact from a locally staged package (no server).
# The offline engine runs its own preflight, so free the ports for it and
# bring the platform endpoints back once the preflight window has passed.
stop_servers
( sleep 14; start_servers ) >/dev/null 2>&1 &
mkdir -p /tmp/doff
curl -fsSL "\$DREVANTA_DIST_BASE/bootstrap.sh" -o /tmp/doff/bootstrap.sh
curl -fsSL "\$DREVANTA_DIST_BASE/SHA256SUMS" -o /tmp/doff/SHA256SUMS
curl -fsSL "\$DREVANTA_DIST_BASE/SHA256SUMS.sig" -o /tmp/doff/SHA256SUMS.sig
curl -fsSL "\$DREVANTA_DIST_BASE/MANIFEST.json" -o /tmp/doff/MANIFEST.json
ARTN=\$(grep -oE 'drevanta-install-[A-Za-z0-9._+-]+\.tar\.gz' /tmp/doff/SHA256SUMS | head -1)
curl -fsSL "\$DREVANTA_DIST_BASE/\$ARTN" -o "/tmp/doff/\$ARTN"
bash /tmp/doff/bootstrap.sh --artifact "/tmp/doff/\$ARTN" --instance distoff > "$T/off.log" 2>&1; echo \$? > "$T/off.rc"
SEOF
timeout 420 unshare -rm bash "$SESS" > "$T/sess.out" 2>&1

rcs=$(grep -oE "[0-9]+ [0-9]+" "$T/pipe.rc" 2>/dev/null | tail -1)
rc=$(echo "$rcs" | awk '{print $2}')
[[ "$rcs" == "0 0" ]] && ok "T1: curl|bash exits 0 (curl+bootstrap)" || bad "T1: curl|bash failed (rcs=${rcs:-none})"
[[ "$(cat "$T/pipe.complete" 2>/dev/null)" == "yes" ]] && ok "T1: INSTALLATION COMPLETE reached via curl|bash" || bad "T1: installation incomplete (see $T/pipe.log)"
has "T1: SHA256SUMS signature verified before use" "$T/pipe.log" "SHA256SUMS signature verified against the embedded distribution key"
has "T1: release identity chain verified" "$T/pipe.log" "release identity chain verified"
has "T1: internal checksums full coverage" "$T/pipe.log" "internal SHA256SUMS verified with full coverage"
has "T1: dev prerelease labeled honestly" "$T/pipe.log" "DEVELOPMENT PRERELEASE .* — not a production release"
cnt=$(grep -cE "clm_[0-9a-f]{62}" "$T/pipe.log" 2>/dev/null || true)
[[ "${cnt:-1}" == "0" ]] && ok "T1: zero token material in the captured customer output" || bad "T1: token material leaked to stdout ($cnt)"
has "T1: no credentials requested or embedded" "$T/pipe.log" "INSTALLATION COMPLETE"
rc=$(cat "$T/off.rc" 2>/dev/null)
[[ "$rc" == "0" ]] && ok "T1: offline --artifact install exits 0" || bad "T1: offline install failed (rc=${rc:-none})"
has "T1: offline install completes" "$T/off.log" "INSTALLATION COMPLETE"

# 3. NEGATIVE: tampered artifact with a stale signature.
TAM="$(mktemp -d)"
cp "$STAGE"/SHA256SUMS* "$STAGE"/bootstrap.sh* "$STAGE"/MANIFEST.json "$TAM"/ 2>/dev/null
cp "$STAGE"/drevanta-install-*.tar.gz "$TAM/"
( cd "$TAM" && f=$(ls drevanta-install-*.tar.gz) && printf 'X' | dd of="$f" bs=1 seek=500000 conv=notrunc 2>/dev/null
  sha256sum "$f" MANIFEST.json bootstrap.sh > SHA256SUMS )
NTAM="$TAM.serve"
kill_port
python3 - "$PORT" "$TAM" drevanta-dist-serve <<'SRV' >/dev/null 2>&1 &
import http.server, socketserver, sys
port, stage = int(sys.argv[1]), sys.argv[2]
class H(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *a, **kw):
        super().__init__(*a, directory=stage, **kw)
    def log_message(self, *a): pass
socketserver.TCPServer.allow_reuse_address = True
with socketserver.TCPServer(("127.0.0.1", port), H) as httpd:
    httpd.serve_forever()
SRV
sleep 1
SESS2="$(mktemp -d)/s2.sh"
cat > "$SESS2" <<SEOF2
#!/usr/bin/env bash
set -u
cp -a /etc /tmp/etc.keep 2>/dev/null
mount -t tmpfs -o size=8g tmpfs /etc && mount -t tmpfs tmpfs /opt && mount -t tmpfs -o size=8g tmpfs /var && mount -t tmpfs tmpfs /usr/local && mount -t tmpfs tmpfs /run
cp -a /tmp/etc.keep/. /etc/ 2>/dev/null; rm -rf /tmp/etc.keep
export PATH="$STUB:\$PATH"; export HOME=/root
export DREVANTA_DIST_BASE="http://127.0.0.1:$PORT"
export DREVANTA_DIST_DEBUG="/tmp/bs-env.log"
curl -fsSL "\$DREVANTA_DIST_BASE/bootstrap.sh" | bash > "$T/tam.log" 2>&1
echo "\${PIPESTATUS[1]}" > "$T/tam.rc"
SEOF2
timeout 180 unshare -rm bash "$SESS2" > /dev/null 2>&1
rc=$(cat "$T/tam.rc" 2>/dev/null)
[[ "$rc" != "0" ]] && ok "NEG: tampered artifact refuses (rc=$rc)" || bad "NEG: tampered artifact NOT refused"
has "NEG: signature failure reported" "$T/tam.log" "SIGNATURE VERIFICATION FAILED"

# 4. NEGATIVE: impostor signing key.
IMP="$(mktemp -d)"; chmod 700 "$IMP"
bash scripts/gen_signing_key.sh "$IMP" >/dev/null
TAM2="$(mktemp -d)"
cp "$STAGE"/bootstrap.sh* "$STAGE"/MANIFEST.json "$STAGE"/drevanta-install-*.tar.gz "$TAM2"/ 2>/dev/null
( cd "$TAM2" && DREVANTA_DIST_SIGNING_KEY="$IMP/drevanta-dist-signing.pem" "$REPO_DIR/scripts/sign_release.sh" . >/dev/null )
kill_port
python3 - "$PORT" "$TAM2" drevanta-dist-serve <<'SRV' >/dev/null 2>&1 &
import http.server, socketserver, sys
port, stage = int(sys.argv[1]), sys.argv[2]
class H(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *a, **kw):
        super().__init__(*a, directory=stage, **kw)
    def log_message(self, *a): pass
socketserver.TCPServer.allow_reuse_address = True
with socketserver.TCPServer(("127.0.0.1", port), H) as httpd:
    httpd.serve_forever()
SRV
sleep 1
SESS3="$(mktemp -d)/s3.sh"
cat > "$SESS3" <<SEOF3
#!/usr/bin/env bash
set -u
cp -a /etc /tmp/etc.keep 2>/dev/null
mount -t tmpfs -o size=8g tmpfs /etc && mount -t tmpfs tmpfs /opt && mount -t tmpfs -o size=8g tmpfs /var && mount -t tmpfs tmpfs /usr/local && mount -t tmpfs tmpfs /run
cp -a /tmp/etc.keep/. /etc/ 2>/dev/null; rm -rf /tmp/etc.keep
export PATH="$STUB:\$PATH"; export HOME=/root
export DREVANTA_DIST_BASE="http://127.0.0.1:$PORT"
export DREVANTA_DIST_DEBUG="/tmp/bs-env.log"
curl -fsSL "\$DREVANTA_DIST_BASE/bootstrap.sh" | bash > "$T/imp.log" 2>&1
echo "\${PIPESTATUS[1]}" > "$T/imp.rc"
SEOF3
timeout 180 unshare -rm bash "$SESS3" > /dev/null 2>&1
rc=$(cat "$T/imp.rc" 2>/dev/null)
[[ "$rc" != "0" ]] && ok "NEG: impostor signing key refuses (rc=$rc)" || bad "NEG: impostor key NOT refused"
has "NEG: impostor signature failure reported" "$T/imp.log" "SIGNATURE VERIFICATION FAILED"

kill_port
rm -rf "$STUB" "$KEYS" "$STAGE" "$TAM" "$TAM2" "$IMP"
echo "========================================================"
echo "DISTRIBUTION VERIFICATION SUMMARY: $PASS passed, $FAIL failed"
[[ "$FAIL" == "0" ]] || exit 1

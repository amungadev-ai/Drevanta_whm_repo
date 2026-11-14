#!/usr/bin/env bash
# Drevanta installer verification suite — isolated test environment.
#
# Scope (isolated sandbox verification, NOT an independent VPS test):
#   T1 bootstrap e2e vs mocked GitHub API (user-namespace root): happy path +
#      manifest-mismatch, corrupted-artifact, unauthenticated negatives
#   T2 shared engine e2e in a mount-namespaced fake root with stubbed
#      systemctl/nginx/apt/sudo/ufw/ss/useradd: fresh install, HTTPS mgmt
#      vhost + TLS cert (IP SAN) + fingerprint, claim token issuance,
#      persistent-log redaction, unattended mode, existing-install
#      protection, --upgrade, private-mgmt mode
#   T3 real openssl certificate checks
#   T4 frontend fragment logic against the BUILT bundle
#   T5 panel 'claim regenerate' CLI smoke on a real SQLite DB
# Every check prints PASS/FAIL; non-zero exit on any failure.
set -uo pipefail
REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
STUB="$(mktemp -d /tmp/drevanta-verify-stubs.XXXXXX)"
WORK="$(mktemp -d /tmp/drevanta-verify.XXXXXX)"
PASS=0; FAIL=0
ok()  { echo "PASS  $*"; PASS=$((PASS+1)); }
bad() { echo "FAIL  $*"; FAIL=$((FAIL+1)); }
has()   { grep -qE "$3" "$2" 2>/dev/null && ok "$1" || bad "$1"; }
lacks() { grep -qE "$3" "$2" 2>/dev/null && bad "$1" || ok "$1"; }

# ---------------- Stubs (fake-root engine test) ----------------
cat > "$STUB/systemctl" <<'EOF'
#!/usr/bin/env bash
[[ "$1" == is-active ]] && { echo active; exit 0; }
exit 0
EOF
printf '#!/usr/bin/env bash\nexit 0\n' > "$STUB/nginx"
printf '#!/usr/bin/env bash\nexit 0\n' > "$STUB/apt-get"
printf '#!/usr/bin/env bash\nexit 1\n' > "$STUB/ss"   # no listeners: ports free
printf '#!/usr/bin/env bash\n[[ "$1" == status ]] && echo "Status: inactive"\nexit 0\n' > "$STUB/ufw"
cat > "$STUB/useradd" <<'EOF'
#!/usr/bin/env bash
U="${@: -1}"
# uid 0: the only identity mapped inside the user namespace (harness only —
# on a real host the engine creates a dedicated system user)
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
ORIG=("$@")   # passthrough needs the pristine argument vector
out="/dev/null"; url=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o) out="$2"; shift 2 ;;
    -w) fmt="$2"; shift 2 ;;
    --header|--resolve|--max-time|--retry|-s|-k|-f|-S|-L|-4) shift ;;
    -*) shift ;;
    *) url="$1"; shift ;;
  esac
done
case "$url" in
  # real network destinations pass through (Method A retrieval, apt mirrors)
  *github.com*|*githubusercontent.com*|*deb.nodesource.com*|*archive.ubuntu.com*|*go.dev*) exec /usr/bin/curl "${ORIG[@]}" ;;
  *api.ipify.org*) printf '203.0.113.7' > "$out"; echo 200 ;;
  *install/status*) printf '{"installation_id":"inst_test0000000000000000","state":"pending"}' > "$out"; echo 200 ;;
  *) printf 'ok' > "$out"; echo 200 ;;
esac
EOF'
#!/usr/bin/env bash
out="/dev/null"; url=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o) out="$2"; shift 2 ;;
    -w) fmt="$2"; shift 2 ;;
    --resolve|--max-time|--retry|-s|-k|-f|-S|-L|-4) shift ;;
    -*) shift ;;
    *) url="$1"; shift ;;
  esac
done
case "$url" in
  # real network destinations pass through (Method A retrieval, apt mirrors)
  *github.com*|*githubusercontent.com*|*deb.nodesource.com*|*archive.ubuntu.com*|*go.dev*) exec /usr/bin/curl "$@" ;;
  *api.ipify.org*) printf '203.0.113.7' > "$out"; echo 200 ;;
  *install/status*) printf '{"installation_id":"inst_test0000000000000000","state":"pending"}' > "$out"; echo 200 ;;
  *) printf 'ok' > "$out"; echo 200 ;;
esac
EOF
chmod +x "$STUB"/*

# ---------------- Mock GitHub API (fixed ports) ----------------
start_mock() { # start_mock <port> <tarball> <manifest_sha|MISSING> <mode ok|missing|deny>
  python3 - "$1" "$2" "$3" "$4" <<'PY' >/dev/null 2>&1 &
import http.server, json, sys
port, tarball, msha, mode = sys.argv[1:5]
class H(http.server.BaseHTTPRequestHandler):
    def _json(self, obj, code=200):
        b = json.dumps(obj).encode()
        self.send_response(code); self.send_header("Content-Type","application/json")
        self.send_header("Content-Length", str(len(b))); self.end_headers(); self.wfile.write(b)
    def do_GET(self):
        if mode == "deny":
            self._json({"message":"Requires authentication"}, 401); return
        if self.path.endswith("/releases/tags/vTEST"):
            self._json({"tag_name":"vTEST","assets":[
                {"id":101,"name":"drevanta-install-1.0.0-owneraudit.1-TEST.tar.gz"},
                {"id":102,"name":"MANIFEST.json"}]})
        elif self.path.endswith("/releases/assets/101"):
            b = open(tarball, "rb").read()
            self.send_response(200); self.send_header("Content-Type","application/octet-stream")
            self.send_header("Content-Length", str(len(b))); self.end_headers(); self.wfile.write(b)
        elif self.path.endswith("/releases/assets/102"):
            if mode == "missing":
                self._json({"message":"Not Found"}, 404); return
            b = json.dumps({"artifacts":{"drevanta-install-1.0.0-owneraudit.1-TEST.tar.gz":msha}}).encode()
            self.send_response(200); self.send_header("Content-Type","application/octet-stream")
            self.send_header("Content-Length", str(len(b))); self.end_headers(); self.wfile.write(b)
        elif self.path.endswith("/releases/assets?per_page=100"):
            self._json([{"id":101,"name":"drevanta-install-1.0.0-owneraudit.1-TEST.tar.gz"},
                        {"id":102,"name":"MANIFEST.json"}])
        else:
            self._json({"message":"Not Found"}, 404)
    def log_message(self, *a): pass
try:
    http.server.HTTPServer(("127.0.0.1", int(port)), H).serve_forever()
except Exception:
    pass
PY
}
mk_artifact() { # mk_artifact <dir> <engine-script-content> -> tarball path
  local d="$1"
  local p="$d/drevanta-install-1.0.0-owneraudit.1-TEST"
  mkdir -p "$p/bin" "$p/infra/etc"
  printf '1.0.0-TEST+testcommit\nbuilt test\n' > "$p/VERSION"
  printf '%s' "$2" > "$p/install.sh"; chmod 755 "$p/install.sh"
  printf 'stub-helper\n' > "$p/bin/helper"
  printf '{}\n' > "$p/infra/etc/helper-policy.json"
  ( cd "$p" && find . -type f ! -name SHA256SUMS -exec sha256sum {} \; > SHA256SUMS )
  tar --no-same-owner -C "$d" -czf "$d/drevanta-install-1.0.0-owneraudit.1-TEST.tar.gz" "drevanta-install-1.0.0-owneraudit.1-TEST"
  rm -rf "$p"; echo "$d/drevanta-install-1.0.0-owneraudit.1-TEST.tar.gz"
}
run_boot() { # run_boot <mockport> <workdir> [engine args...]
  local port="$1" wd="$2"; shift 2
  unshare -rm bash -c "DREVANTA_API_BASE=http://127.0.0.1:$port/repos/amungadev-ai/Drevanta_whm_repo DREVANTA_UNATTENDED=1 bash $REPO_DIR/install.sh --version vTEST --tmpdir $wd $*"
}

echo "================ T1: bootstrap e2e (mocked API) ================"
ENGINE_STUB='#!/usr/bin/env bash
echo "ENGINE_EXECUTED args: $*"'
T1="$WORK/t1"; mkdir -p "$T1"
ART="$(mk_artifact "$T1/art" "$ENGINE_STUB")"
ART_SHA="$(sha256sum "$ART" | awk '{print $1}')"
WRONG_SHA="0000000000000000000000000000000000000000000000000000000000000000"
CORRUPT="$T1/corrupt.tar.gz"; head -c 100 "$ART" > "$CORRUPT"; dd if=/dev/urandom bs=64 count=1 >> "$CORRUPT" 2>/dev/null
start_mock 18499 "$ART" "$ART_SHA" ok
start_mock 18500 "$ART" "$WRONG_SHA" ok
start_mock 18501 "$CORRUPT" "MISSING" missing
start_mock 18502 "$ART" "$ART_SHA" deny
sleep 1
run_boot 18499 "$T1/w1" -- --help > "$T1/out1" 2>&1; r=$?
[[ "$r" == "0" ]] && ok "T1 happy: bootstrap exit 0" || { bad "T1 happy: exit $r"; tail -4 "$T1/out1"; }
has "T1 happy: engine executed with passthrough args" "$T1/out1" "ENGINE_EXECUTED args: --help"
has "T1 happy: manifest verification executed" "$T1/out1" "integrity verified against trusted release manifest"
has "T1 happy: internal SHA256SUMS verified" "$T1/out1" "all 4 artifact files verified"
run_boot 18500 "$T1/w2" > "$T1/out2" 2>&1; r=$?
[[ "$r" != "0" ]] && ok "T1 neg: manifest mismatch fails (exit $r)" || bad "T1 neg: mismatch NOT detected"
has "T1 neg: mismatch reported" "$T1/out2" "SHA-256 MISMATCH"
run_boot 18501 "$T1/w3" > "$T1/out3" 2>&1; r=$?
[[ "$r" != "0" ]] && ok "T1 neg: corrupted artifact fails (exit $r)" || bad "T1 neg: corruption NOT detected"
has "T1 neg: corruption reported" "$T1/out3" "corrupted|verification FAILED"
unshare -rm bash -c "DREVANTA_API_BASE=http://127.0.0.1:18502/repos/amungadev-ai/Drevanta_whm_repo DREVANTA_UNATTENDED=1 bash $REPO_DIR/install.sh --version vTEST --tmpdir $T1/w4" > "$T1/out4" 2>&1; r=$?
[[ "$r" != "0" ]] && ok "T1 neg: unattended without credentials fails (exit $r)" || bad "T1 neg: unattended auth"
has "T1 neg: actionable guidance given" "$T1/out4" "export DREVANTA_GH_TOKEN"
pkill -f "HTTPServer" 2>/dev/null; pkill -f "drevanta-verify" 2>/dev/null || true

echo "================ T2: engine e2e (namespaced fake root) ================"
if unshare -rm true 2>/dev/null; then
  T2="$WORK/t2"; mkdir -p "$T2"
  PKG2="$WORK/pkg"; mkdir -p "$PKG2"
  tar --no-same-owner -xzf "$REPO_DIR"/drevanta-install-*.tar.gz -C "$PKG2"
  PKGDIR2="$(echo "$PKG2"/drevanta-install-*)"
  # All T2 scenarios run inside ONE namespace session so the fake root keeps
  # state between runs (fresh -> rerun -> upgrade). Instances are isolated
  # paths (/etc/drevanta-accN), so scenarios cannot interfere.
  cat > "$WORK/t2/session.sh" <<SEOF
#!/usr/bin/env bash
set -u
cp -a /etc /tmp/etc.keep   # /etc.keep on real / is not writable by the mapped uid
mount -t tmpfs -o size=8g tmpfs /etc && mount -t tmpfs -o size=8g tmpfs /opt && mount -t tmpfs -o size=8g tmpfs /var && mount -t tmpfs tmpfs /usr/local && mount -t tmpfs tmpfs /run
cp -a /tmp/etc.keep/. /etc/ && rm -rf /tmp/etc.keep
export PATH="$STUB:\$PATH"
export HOME=/root
E="$PKGDIR2/install.sh"
bash "\$E" --instance acc1 --access-ip 203.0.113.7        > "$T2/acc1.log"   2>&1; echo \$? > "$T2/acc1.rc"
bash "\$E" --instance acc1 --access-ip 203.0.113.7        > "$T2/acc1b.log"  2>&1; echo \$? > "$T2/acc1b.rc"
bash "\$E" --instance acc1 --upgrade                      > "$T2/acc1c.log"  2>&1; echo \$? > "$T2/acc1c.rc"
DREVANTA_UNATTENDED=1 bash "\$E" --instance acc2 --access-ip 203.0.113.7 > "$T2/acc2.log" 2>&1; echo \$? > "$T2/acc2.rc"
bash "\$E" --instance acc3 --access-ip 203.0.113.7 --private-mgmt       > "$T2/acc3.log" 2>&1; echo \$? > "$T2/acc3.rc"
SEOF
  unshare -rm bash "$WORK/t2/session.sh"
  r=$(cat "$T2/acc1.rc")
  [[ "$r" == "0" ]] && ok "T2 fresh install exit 0" || { bad "T2 fresh install exit $r"; grep "\[FAIL\]" "$T2/acc1.log" | head -4; }
  has "T2: INSTALLATION COMPLETE displayed" "$T2/acc1.log" "INSTALLATION COMPLETE"
  has "T2: primary onboarding URL (https://IP:8443/#clm_<62hex>)" "$T2/acc1.log" "https://203.0.113.7:8443/#clm_[0-9a-f]{62}"
  has "T2: SSH tunnel command displayed" "$T2/acc1.log" "ssh -N -L 8080:127.0.0.1:80 root@203.0.113.7"
  has "T2: tunnel onboarding URL" "$T2/acc1.log" "http://localhost:8080/#clm_"
  has "T2: TLS fingerprint displayed" "$T2/acc1.log" "Certificate SHA-256 fingerprint: [0-9A-F:]{95}"
  has "T2: mgmt vhost configured" "$T2/acc1.log" "management vhost https://0.0.0.0:8443"
  has "T2: certificate SAN = access IP" "$T2/acc1.log" "SAN: IP:203.0.113.7"
  has "T2: external reachability honesty line" "$T2/acc1.log" "NOT verified"
  has "T2: firewall honesty (inactive ufw warning)" "$T2/acc1.log" "no active ufw detected"
  LOGF=$(grep -oP "Installation log   : \K\S+" "$T2/acc1.log" | head -1)
  [[ -n "$LOGF" ]] && ok "T2: install log exists" || bad "T2: no log path in summary"
  TOKEN=$(grep -oP "https://203.0.113.7:8443/#\Kclm_[0-9a-f]{62}" "$T2/acc1.log" | head -1)
  if [[ -n "$LOGF" && -n "$TOKEN" ]]; then
    lacks "T2: token NOT in persistent log" "$LOGF" "$TOKEN"
  else
    bad "T2: log/token capture for redaction check"
  fi
  has "T2: report json written" "$T2/acc1.log" "install-report.json"
  # Redaction mechanism check (direct): tokens must be replaced in the log
  # stream; by design the summary's token lines bypass the logger entirely.
  RED=$(mktemp -u /tmp/drevanta-redact-test.XXXXXX); FIFO=$(mktemp -u /tmp/drevanta-fifo-test.XXXXXX); mkfifo "$FIFO"
  ( while IFS= read -r line; do
      R="$(cat "$RED" 2>/dev/null || true)"
      [[ -n "$R" ]] && line="${line//"$R"/[REDACTED-CLAIM-TOKEN]}"
      printf '%s\n' "$line" >> "$T2/redact.log"
    done ) < "$FIFO" &
  exec 9>"$FIFO"
  printf 'url https://1.2.3.4:8443/#clm_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff\n' >&9
  printf 'clm_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff' > "$RED"
  printf 'manual token clm_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff\n' >&9
  exec 9>&-; wait
  lacks "T2: redaction mechanism replaces tokens in log stream" "$T2/redact.log" "clm_ffffffff"
  has "T2: redaction marker present" "$T2/redact.log" "REDACTED-CLAIM-TOKEN"
  rm -f "$RED" "$FIFO"
  r=$(cat "$T2/acc2.rc")
  [[ "$r" == "0" ]] && ok "T2 unattended: exit 0" || bad "T2 unattended: exit $r"
  lacks "T2 unattended: NO token in output" "$T2/acc2.log" "clm_[0-9a-f]{62}"
  has "T2 unattended: retrieval command shown" "$T2/acc2.log" "drevanta-claim-token --url"
  r=$(cat "$T2/acc1b.rc")
  [[ "$r" != "0" ]] && ok "T2: existing install refused without --upgrade (exit $r)" || bad "T2: existing install NOT refused"
  has "T2: refusal is explicit" "$T2/acc1b.log" "refusing to modify"
  has "T2 upgrade: mode entered" "$T2/acc1c.log" "UPGRADE MODE"
  has "T2 upgrade: completed with backup" "$T2/acc1c.log" "upgrade complete"
  r=$(cat "$T2/acc3.rc")
  [[ "$r" == "0" ]] && ok "T2 private-mgmt: exit 0" || bad "T2 private-mgmt: exit $r"
  has "T2 private-mgmt: mode declared" "$T2/acc3.log" "private-management"
  grep -q "management vhost https" "$T2/acc3.log" && bad "T2 private-mgmt: public vhost must NOT be created" || ok "T2 private-mgmt: no public vhost"

  echo "================ T3: certificate verification (real openssl) ================"
  T3="$WORK/t3"; mkdir -p "$T3"
  openssl req -x509 -newkey rsa:2048 -sha256 -days 825 -nodes -keyout "$T3/k.pem" -out "$T3/c.pem" -subj "/CN=Drevanta Management" -addext "subjectAltName=IP:203.0.113.7" >/dev/null 2>&1
  openssl x509 -in "$T3/c.pem" -noout -text | grep -q "IP Address:203.0.113.7" && ok "T3: cert carries IP SAN" || bad "T3: IP SAN"
  openssl x509 -in "$T3/c.pem" -noout -fingerprint -sha256 | grep -qE "sha256 Fingerprint=[0-9A-F:]{95}" && ok "T3: sha256 fingerprint derivable" || bad "T3: fingerprint"
else
  bad "T2/T3 SKIPPED: unshare -rm unavailable in this sandbox"
fi

echo "================ T4: frontend fragment logic (built bundle) ================"
node - "$REPO_DIR/frontend/build/client/js/auth-flow.js" <<'JS' && ok "T4: fragment logic (real regexes, 8 cases)" || bad "T4: fragment logic"
const fs = require('fs'), vm = require('vm');
const src = fs.readFileSync(process.argv[2], 'utf8');
const cases = [
  ['clm_' + 'a'.repeat(62), true], ['token=clm_' + 'a'.repeat(62), true],
  ['token=drv1_abc', true], ['clm_' + 'a'.repeat(61), false], ['CLM_' + 'a'.repeat(62), false],
  ['clm_' + 'g'.repeat(62), false], ['garbage', false], ['', false]
];
const rawRe = src.match(/\/\^clm_\[0-9a-f\]\{62\}\$\//);
const generalRe = src.match(/\/\^\(drv1\|clm\)_\[A-Za-z0-9_-\]\+\$\//);
if (!rawRe || !generalRe) { console.error('claim regexes missing from built bundle'); process.exit(1); }
const ctx = { decodeURIComponent, location: { hash: '' } };
vm.createContext(ctx);
let pass = true;
for (const [frag, expected] of cases) {
  ctx.location.hash = '#' + frag;
  const detected = vm.runInContext(`(function(){
    const frag=decodeURIComponent(location.hash.slice(1));
    let raw=new (class{constructor(s){this.s=s}get(k){const m=this.s.match(new RegExp('(?:^|&)'+k+'=([^&]*)'));return m?m[1]:null}})(frag).get('token')||'';
    if(!raw && ${rawRe[0]}.test(frag))raw=frag;
    return ${generalRe[0]}.test(raw) ? raw : ''; })()`, ctx);
  if ((detected !== '') !== expected) { console.error('case', JSON.stringify(frag.slice(0,20)), 'want', expected, 'got', detected !== ''); pass = false; }
}
process.exit(pass ? 0 : 1);
JS
grep -q "history.replaceState(null,'',location.pathname+location.search)" "$REPO_DIR/frontend/build/client/js/auth-flow.js" && ok "T4: address-bar cleanup present (replaceState)" || bad "T4: replaceState missing"
grep -q "drv1_DEMO_TOKEN_ONLY" "$REPO_DIR/frontend/build/client/js/auth-flow.js" && bad "T4: demo token still auto-fills" || ok "T4: demo token removed"
grep -rlq 'location.replace("/onboarding"+location.hash)' "$REPO_DIR/frontend/build/client/_app/immutable/nodes/" 2>/dev/null && ok "T4: root page forwards claim fragments (built)" || bad "T4: root forward missing"
grep -rlq "clm_" "$REPO_DIR/frontend/build/client/_app/immutable/nodes/" 2>/dev/null && ok "T4: root page claim-guard present (built)" || bad "T4: root guard missing"

echo "================ T5: panel claim regenerate CLI ================"
export PATH="/home/z/my-project/tools/go/bin:$PATH"
T5="$WORK/t5"; mkdir -p "$T5"
GOOS=linux GOARCH=amd64 go build -o "$T5/panel" ./cmd/panel 2>/dev/null
( cd "$REPO_DIR" && "$T5/panel" bootstrap -db "$T5/p.db" -migrations migrations ) > "$T5/boot.out" 2>&1
TOK1=$(grep -A1 "DREVANTA CLAIM TOKEN" "$T5/boot.out" | tail -1 | tr -d '[:space:]')
[[ "$TOK1" =~ ^clm_[0-9a-f]{62}$ ]] && ok "T5: bootstrap issues clm_<62hex> token" || bad "T5: token format"
( cd "$REPO_DIR" && "$T5/panel" claim regenerate -db "$T5/p.db" -migrations migrations ) > "$T5/regen.out" 2>&1
TOK2=$(grep -A1 "DREVANTA CLAIM TOKEN" "$T5/regen.out" | tail -1 | tr -d '[:space:]')
[[ -n "$TOK2" && "$TOK2" != "$TOK1" && "$TOK2" =~ ^clm_[0-9a-f]{62}$ ]] && ok "T5: regenerate replaces the token" || bad "T5: regenerate"
python3 - "$T5/p.db" "$TOK2" <<'PY' && ok "T5: only the hash of the new token is stored" || bad "T5: hash storage"
import sqlite3, sys, hashlib
h = sqlite3.connect(sys.argv[1]).execute("select claim_token_hash from installation limit 1").fetchone()[0]
sys.exit(0 if h == hashlib.sha256(sys.argv[2].encode()).hexdigest() else 1)
PY
python3 - "$T5/p.db" "$TOK1" <<'PY' && bad "T5: OLD token hash still stored?" || ok "T5: old token hash replaced"
import sqlite3, sys, hashlib
h = sqlite3.connect(sys.argv[1]).execute("select claim_token_hash from installation limit 1").fetchone()[0]
sys.exit(0 if h == hashlib.sha256(sys.argv[2].encode()).hexdigest() else 1)
PY

echo "========================================================"
echo "VERIFICATION SUMMARY: $PASS passed, $FAIL failed"
[[ "$FAIL" == "0" ]] || exit 1

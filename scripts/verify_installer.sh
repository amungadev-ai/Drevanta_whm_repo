#!/usr/bin/env bash
# Drevanta installer verification suite — isolated test environment.
#
# Scope (isolated sandbox verification, NOT an independent VPS test — mocked
# systemd/nginx/apt/curl here are stubs; see the limitation register):
#   T1 bootstrap e2e vs mocked GitHub API (user-namespace root): happy path;
#      negatives: manifest missing, schema-invalid manifest, wrong pinned/
#      required commit, SHA-256 mismatch, corrupted artifact, unsafe archive
#      entries, incomplete SHA256SUMS coverage, unauthenticated unattended;
#      workspace cleanup after failure; credential scrub before engine handover
#   T2 shared engine e2e in a mount-namespaced fake root with stubbed
#      systemctl/nginx/apt/sudo/ufw/ss/useradd: fresh install, preflight
#      validation negatives, exclusive lock, HTTPS mgmt vhost + IP-SAN cert,
#      interactive-vs-noninteractive token suppression, private-mgmt URL
#      suppression, helper socket gate, isolation gates, existing-install
#      protection, --upgrade (incl. management-port change and
#      public-listener withdrawal), verified rollback + rollback-failure
#      reporting, stale claim-token cleanup
#   T3 real openssl certificate checks
#   T4 frontend fragment logic against the BUILT bundle
#   T5 panel claim regenerate CLI + concurrent claim protection
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
# Helper unit start: materialize the unix socket the real helper creates.
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
printf '#!/usr/bin/env bash\nexit 0\n' > "$STUB/nginx"
# Provisioning-aware apt-get stub: "installs" the packages the engine asks
# for by materializing their command stubs (mirrors real apt behavior: after
# apt-get install nginx, the nginx binary exists).
cat > "$STUB/apt-get" <<APEOF
#!/usr/bin/env bash
if [[ "\$1" == "install" ]]; then
  for p in "\$@"; do case "\$p" in
    nginx)  printf '#!/usr/bin/env bash\nexit 0\n' > "$STUB/nginx" ;;
    nodejs) printf '#!/usr/bin/env bash\n[[ "\$1" == "--version" ]] && echo v22.11.0\nexit 0\n' > "$STUB/node" ;;
    npm)    printf '#!/usr/bin/env bash\nexit 0\n' > "$STUB/npm" ;;
    git)    printf '#!/usr/bin/env bash\nexit 0\n' > "$STUB/git" ;;
    curl)   : ;;  # real curl stays in PATH
  esac; done
fi
exit 0
APEOF
printf '#!/usr/bin/env bash\nexit 1\n' > "$STUB/ss"   # no listeners: ports free
printf '#!/usr/bin/env bash\n[[ "$1" == status ]] && echo "Status: inactive"\nexit 0\n' > "$STUB/ufw"
cat > "$STUB/ss" <<'SSEOF'
#!/usr/bin/env bash
if [[ -e /tmp/drevanta-simulate-owned-ports ]]; then
  for p in 80 8080 3100 8443; do
    echo "LISTEN 0 128 127.0.0.1:$p 0.0.0.0:* users:((\"node\",pid=4242,fd=20))"
  done
  exit 0
fi
if [[ -e /tmp/drevanta-simulate-foreign-port ]]; then
  echo 'LISTEN 0 128 0.0.0.0:3100 0.0.0.0:* users:(("someapp",pid=99,fd=5))'
  exit 0
fi
exit 1
SSEOF
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
# Harness curl: real-network destinations pass through; platform endpoints are
# simulated. Force-failure markers let the suite exercise rollback paths.
ORIG=("$@")   # passthrough needs the pristine argument vector
GOT_O=0
out="/dev/null"; url=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o) out="$2"; GOT_O=1; shift 2 ;;
    -w) fmt="$2"; shift 2 ;;
    --header|--resolve|--max-time|--retry|-s|-k|-f|-S|-L|-4) shift ;;
    -*) shift ;;
    *) url="$1"; shift ;;
  esac
done
case "$url" in
  *github.com*|*githubusercontent.com*|*deb.nodesource.com*|*archive.ubuntu.com*|*go.dev*) exec /usr/bin/curl "${ORIG[@]}" ;;
  *api.ipify.org*)
    if [[ "$GOT_O" == "1" ]]; then printf '203.0.113.7' > "$out"; else printf '203.0.113.7'; fi ;;
  *install/status*)
    # Rollback-phase failure: only when the upgrade-failure marker is gone
    # (i.e. the upgrade phase already failed and consumed it).
    if [[ -e /tmp/drevanta-force-rollback-unhealthy && ! -e /tmp/drevanta-force-upgrade-failure ]]; then
      printf '{}' > "$out"; echo 500
    else
      printf '{"installation_id":"inst_test0000000000000000","state":"pending"}' > "$out"; echo 200
    fi ;;
  *)
    if [[ -e /tmp/drevanta-force-upgrade-failure ]]; then rm -f /tmp/drevanta-force-upgrade-failure; printf 'err' > "$out"; echo 500
    elif [[ -e /tmp/drevanta-force-rollback-unhealthy ]]; then rm -f /tmp/drevanta-force-rollback-unhealthy; printf 'err' > "$out"; echo 500
    else printf 'ok' > "$out"; echo 200; fi ;;
esac
EOF
chmod +x "$STUB"/*

# ---------------- Mock GitHub API (fixed ports) ----------------
pkill -f drevanta-mockport 2>/dev/null || true   # kill stale mocks from any earlier run
start_mock() { # start_mock <port> <tarball> <manifest-mode> <mode>
  # manifest-mode: a JSON string for MANIFEST.json content, "MISSING" (404),
  # or a literal invalid payload
  python3 - "$1" "$2" "$3" "$4" drevanta-mockport <<'PY' >/dev/null 2>&1 &
import http.server, json, sys
port, tarball, mtext, mode = sys.argv[1:5]
class H(http.server.BaseHTTPRequestHandler):
    def _send(self, body, code=200, ctype="application/json"):
        b = body if isinstance(body, bytes) else body.encode()
        self.send_response(code); self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(b))); self.end_headers(); self.wfile.write(b)
    def _json(self, obj, code=200):
        self._send(json.dumps(obj).encode(), code)
    def do_GET(self):
        if mode == "deny":
            self._json({"message":"Requires authentication"}, 401); return
        if self.path.endswith("/releases/tags/vTEST"):
            self._json({"tag_name":"vTEST","assets":[
                {"id":101,"name":"drevanta-install-1.0.0-owneraudit.1-TEST.tar.gz"},
                {"id":102,"name":"MANIFEST.json"}]})
        elif self.path.endswith("/releases/assets/101"):
            try:
                b = open(tarball, "rb").read()
            except Exception:
                b = b"corrupt"
            self._send(b, 200, "application/octet-stream")
        elif self.path.endswith("/releases/assets/102"):
            if mtext == "MISSING":
                self._json({"message":"Not Found"}, 404); return
            self._send(mtext.encode(), 200, "application/octet-stream")
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
GOOD_SHA_FIXED="1111111111111111111111111111111111111111111111111111111111111111"
mk_manifest() { # mk_manifest <sha> <commit> [product] [tag]
  python3 - "$@" <<'PY'
import json, sys
sha, commit = sys.argv[1], sys.argv[2]
m = {"product":"Drevanta","channel":"development-prerelease","tag":"vTEST",
     "commit":commit,"version":"1.0.0-TEST+" + commit,"generated_at":"2026-01-01T00:00:00Z",
     "artifact":"drevanta-install-1.0.0-owneraudit.1-TEST.tar.gz",
     "artifacts":{"drevanta-install-1.0.0-owneraudit.1-TEST.tar.gz":sha},
     "verification":"bootstrap verifies the artifact against this manifest AND the artifact-internal SHA256SUMS"}
if len(sys.argv) > 3: m["product"] = sys.argv[3]
if len(sys.argv) > 4: m["tag"] = sys.argv[4]
print(json.dumps(m))
PY
}
mk_artifact() { # mk_artifact <dir> <engine-script-content> [unsafe|uncovered]
  local d="$1"
  local p="$d/drevanta-install-1.0.0-owneraudit.1-TEST"
  local variant="${3:-ok}"
  mkdir -p "$p/bin" "$p/infra/etc"
  printf '1.0.0-TEST+1111111111111111111111111111111111111111\nbuilt test\n' > "$p/VERSION"
  printf '%s' "$2" > "$p/install.sh"; chmod 755 "$p/install.sh"
  printf 'stub-helper\n' > "$p/bin/helper"
  printf '{}\n' > "$p/infra/etc/helper-policy.json"
  ( cd "$p" && find . -type f ! -name SHA256SUMS -exec sha256sum {} \; > SHA256SUMS )
  case "$variant" in
    unsafe)
      ln -s /etc/passwd "$p/evil-link"
      ;;
    uncovered)
      printf 'unstated\n' > "$p/extra-unstated-file.txt"
      ;;
  esac
  tar --no-same-owner -C "$d" -czf "$d/drevanta-install-1.0.0-owneraudit.1-TEST.tar.gz" "drevanta-install-1.0.0-owneraudit.1-TEST"
  rm -rf "$p"; echo "$d/drevanta-install-1.0.0-owneraudit.1-TEST.tar.gz"
}
run_boot() { # run_boot <mockport> <workdir> [bootstrap args...]
  local port="$1" wd="$2"; shift 2
  unshare -rm bash -c "DREVANTA_API_BASE=http://127.0.0.1:$port/repos/amungadev-ai/Drevanta_whm_repo DREVANTA_UNATTENDED=1 bash $REPO_DIR/install.sh --version vTEST --tmpdir $wd $*"
}

echo "================ T1: bootstrap e2e (mocked API) ================"
ENGINE_STUB='#!/usr/bin/env bash
if [[ -n "${DREVANTA_GH_TOKEN:-}" || -n "${GITHUB_TOKEN:-}" ]]; then echo "TOKEN_LEAKED"; else echo "TOKEN_SCRUBBED"; fi
echo "ENGINE_EXECUTED args: $*"'
T1="$WORK/t1"; mkdir -p "$T1"
ART="$(mk_artifact "$T1/art" "$ENGINE_STUB")"
ART_SHA="$(sha256sum "$ART" | awk '{print $1}')"
GOOD_MANIFEST="$(mk_manifest "$ART_SHA" "1111111111111111111111111111111111111111")"
WRONG_SHA_MANIFEST="$(mk_manifest "0000000000000000000000000000000000000000000000000000000000000000" "1111111111111111111111111111111111111111")"
BAD_SCHEMA_MANIFEST="$(mk_manifest "$ART_SHA" "1111111111111111111111111111111111111111" "WrongProduct")"
WRONG_COMMIT_MANIFEST="$(mk_manifest "$ART_SHA" "2222222222222222222222222222222222222222")"
UNSAFE_ART="$(mk_artifact "$T1/unsafe" "$ENGINE_STUB" unsafe)"
UNSAFE_SHA="$(sha256sum "$UNSAFE_ART" | awk '{print $1}')"
UNCOVERED_ART="$(mk_artifact "$T1/uncovered" "$ENGINE_STUB" uncovered)"
UNCOVERED_SHA="$(sha256sum "$UNCOVERED_ART" | awk '{print $1}')"
CORRUPT="$T1/corrupt.tar.gz"; head -c 100 "$ART" > "$CORRUPT"; dd if=/dev/urandom bs=64 count=1 >> "$CORRUPT" 2>/dev/null
start_mock 18499 "$ART" "$GOOD_MANIFEST" ok
start_mock 18500 "$ART" "$WRONG_SHA_MANIFEST" ok
start_mock 18501 "$ART" "MISSING" missing
start_mock 18502 "$ART" "$GOOD_MANIFEST" deny
start_mock 18503 "$ART" "$BAD_SCHEMA_MANIFEST" ok
start_mock 18504 "$UNSAFE_ART" "$(mk_manifest "$UNSAFE_SHA" "1111111111111111111111111111111111111111")" ok
start_mock 18505 "$UNCOVERED_ART" "$(mk_manifest "$UNCOVERED_SHA" "1111111111111111111111111111111111111111")" ok
# 18507: manifest version does not carry the release commit (stale metadata)
mk_manifest_variant() { # mk_manifest_variant <port-file> <python-key-expr> ...
  python3 - "$ART_SHA" "1111111111111111111111111111111111111111" "$1" "$2" <<'PY'
import json, sys
sha, commit, path, variant = sys.argv[1:5]
m = {"product":"Drevanta","channel":"development-prerelease","tag":"vTEST","commit":commit,
     "version":"1.0.0-TEST+" + commit,"generated_at":"2026-01-01T00:00:00Z",
     "artifact":"drevanta-install-1.0.0-owneraudit.1-TEST.tar.gz",
     "artifacts":{"drevanta-install-1.0.0-owneraudit.1-TEST.tar.gz":sha},
     "verification":"bootstrap verifies the artifact against this manifest AND the artifact-internal SHA256SUMS"}
if variant == "stale-version":
    m["version"] = "1.0.0-owneraudit.1+335c67dc485dc04a0fd36a08d3c5e82561d98fd0"
if variant == "wrong-artifact":
    m["artifact"] = "drevanta-install-1.0.0-owneraudit.1-OTHERCOMMIT.tar.gz"
open(path, "w").write(json.dumps(m))
PY
}
mk_manifest_variant "$T1/vmm.json" stale-version
start_mock 18507 "$ART" "$(cat "$T1/vmm.json")" ok
mk_manifest_variant "$T1/aid.json" wrong-artifact
start_mock 18508 "$ART" "$(cat "$T1/aid.json")" ok
sleep 0.5
sleep 1

run_boot 18499 "$T1/w1" -- --help > "$T1/out1" 2>&1; r=$?
[[ "$r" == "0" ]] && ok "T1 happy: bootstrap exit 0" || { bad "T1 happy: exit $r"; tail -4 "$T1/out1"; }
has "T1 happy: engine executed with passthrough args" "$T1/out1" "ENGINE_EXECUTED args: --help"
has "T1 happy: manifest validated (schema)" "$T1/out1" "trusted manifest validated"
has "T1 happy: exact SHA-256 match" "$T1/out1" "exact SHA-256 match against trusted release manifest"
has "T1 happy: internal checksums + coverage" "$T1/out1" "complete file coverage"
run_boot 18499 "$T1/w1c" --require-commit 1111111111111111111111111111111111111111 > "$T1/out1c" 2>&1; r=$?
[[ "$r" == "0" ]] && ok "T1: --require-commit matching manifest passes" || bad "T1: require-commit match failed (exit $r)"
run_boot 18499 "$T1/w1d" --require-commit 2222222222222222222222222222222222222222 > "$T1/out1d" 2>&1; r=$?
[[ "$r" != "0" ]] && ok "T1: --require-commit mismatch refuses install (exit $r)" || bad "T1: require-commit mismatch NOT refused"
has "T1: commit mismatch message" "$T1/out1d" "commit mismatch"
run_boot 18499 "$T1/w1e" > "$T1/out1e" 2>&1
has "T1: credentials scrubbed before engine" "$T1/out1e" "TOKEN_SCRUBBED"
lacks "T1: token NOT leaked to engine" "$T1/out1e" "TOKEN_LEAKED"
run_boot 18500 "$T1/w2" > "$T1/out2" 2>&1; r=$?
[[ "$r" != "0" ]] && ok "T1 neg: manifest mismatch fails (exit $r)" || bad "T1 neg: mismatch NOT detected"
has "T1 neg: mismatch reported" "$T1/out2" "SHA-256 MISMATCH"
run_boot 18501 "$T1/w3" > "$T1/out3" 2>&1; r=$?
[[ "$r" != "0" ]] && ok "T1 neg: missing manifest refuses install (exit $r)" || bad "T1 neg: missing manifest NOT refused"
has "T1 neg: missing manifest reported" "$T1/out3" "MANIFEST.json asset missing|could not be retrieved"
run_boot 18502 "$T1/w4" > "$T1/out4" 2>&1; r=$?
[[ "$r" != "0" ]] && ok "T1 neg: unauthenticated fails (exit $r)" || bad "T1 neg: unauthenticated"
has "T1 neg: actionable guidance given" "$T1/out4" "export DREVANTA_GH_TOKEN"
run_boot 18503 "$T1/w5" > "$T1/out5" 2>&1; r=$?
[[ "$r" != "0" ]] && ok "T1 neg: schema-invalid manifest refuses (exit $r)" || bad "T1 neg: schema NOT enforced"
has "T1 neg: schema errors reported" "$T1/out5" "manifest validation failed|manifest schema errors"
run_boot 18504 "$T1/w6" > "$T1/out6" 2>&1; r=$?
[[ "$r" != "0" ]] && ok "T1 neg: unsafe archive entries refuse (exit $r)" || bad "T1 neg: unsafe entries NOT rejected"
has "T1 neg: unsafe entries reported" "$T1/out6" "unsafe archive entries"
run_boot 18505 "$T1/w7" > "$T1/out7" 2>&1; r=$?
[[ "$r" != "0" ]] && ok "T1 neg: incomplete SHA256SUMS coverage refuses (exit $r)" || bad "T1 neg: coverage NOT enforced"
has "T1 neg: coverage reported" "$T1/out7" "coverage INCOMPLETE"
# Corrupted artifact (truncated): use the corrupt tarball via a mock
start_mock 18506 "$CORRUPT" "$(mk_manifest "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" "1111111111111111111111111111111111111111")" ok
sleep 0.5
run_boot 18507 "$T1/w9" > "$T1/out9" 2>&1; r=$?
[[ "$r" != "0" ]] && ok "T1 neg: stale version metadata refuses (exit $r)" || bad "T1 neg: version metadata NOT enforced"
has "T1 neg: version-commit mismatch reported" "$T1/out9" "does not carry the release commit"
run_boot 18508 "$T1/w10" > "$T1/out10" 2>&1; r=$?
[[ "$r" != "0" ]] && ok "T1 neg: manifest artifact identity mismatch refuses (exit $r)" || bad "T1 neg: artifact identity NOT enforced"
has "T1 neg: artifact identity mismatch reported" "$T1/out10" "artifact identity mismatch"
run_boot 18499 "$T1/w11" --expect-artifact-sha 0000000000000000000000000000000000000000000000000000000000000000 > "$T1/out11" 2>&1; r=$?
[[ "$r" != "0" ]] && ok "T1: --expect-artifact-sha mismatch refuses (exit $r)" || bad "T1: expect-sha NOT enforced"
has "T1: independently communicated digest mismatch reported" "$T1/out11" "independently communicated"
run_boot 18499 "$T1/w12" --expect-artifact-sha "$ART_SHA" > "$T1/out12" 2>&1; r=$?
[[ "$r" == "0" ]] && ok "T1: --expect-artifact-sha match installs" || bad "T1: expect-sha match failed (exit $r)"
run_boot 18506 "$T1/w8" > "$T1/out8" 2>&1; r=$?
[[ "$r" != "0" ]] && ok "T1 neg: corrupted artifact fails (exit $r)" || bad "T1 neg: corruption NOT detected"
has "T1 neg: corruption reported" "$T1/out8" "MISMATCH|corrupted|listing failed"
# Workspace cleanup after a failed run (default mktemp workspace)
rm -rf /tmp/drevanta-install.* 2>/dev/null   # clean slate from any earlier crash
run_boot 18500 "$T1/unused" > /dev/null 2>&1
sleep 0.5
LEFT=$(ls -d /tmp/drevanta-install.* 2>/dev/null | wc -l)
[[ "$LEFT" == "0" ]] && ok "T1: workspace removed after failure (no leftovers)" || bad "T1: $LEFT leftover workspace(s) after failure"
pkill -f drevanta-mockport 2>/dev/null || true

echo "================ T2: engine e2e (namespaced fake root) ================"
if unshare -rm true 2>/dev/null; then
  T2="$WORK/t2"; mkdir -p "$T2"
  PKG2="$WORK/pkg"; mkdir -p "$PKG2"
  tar --no-same-owner -xzf "$REPO_DIR"/drevanta-install-*.tar.gz -C "$PKG2"
  set -- "$PKG2"/drevanta-install-*
  [[ $# == 1 && -d "$1" ]] || { echo "SUITE ERROR: expected exactly one extracted package, got $#"; exit 3; }
  PKGDIR2="$1"
  # All T2 scenarios run inside ONE namespace session so the fake root keeps
  # state between runs. Instances are isolated paths (/etc/drevanta-accN).
  cat > "$T2/session.sh" <<SEOF
#!/usr/bin/env bash
set -u
cp -a /etc /tmp/etc.keep   # /etc.keep on real / is not writable by the mapped uid
mount -t tmpfs -o size=8g tmpfs /etc && mount -t tmpfs -o size=8g tmpfs /opt && mount -t tmpfs -o size=8g tmpfs /var && mount -t tmpfs tmpfs /usr/local && mount -t tmpfs tmpfs /run
cp -a /tmp/etc.keep/. /etc/ && rm -rf /tmp/etc.keep
export PATH="$STUB:\$PATH"
export HOME=/root
E="$PKGDIR2/install.sh"
r() { local f="\$1"; shift; bash "\$E" "\$@" > "$T2/\$f.log" 2>&1; echo \$? > "$T2/\$f.rc"; }
ri() { local f="\$1"; shift; script -qec "bash \$E \$*" /dev/null > "$T2/\$f.log" 2>&1; echo \$? > "$T2/\$f.rc"; }
# validation negatives (no host modification expected)
r acc7 --instance acc7 --mgmt-port 99999 --access-ip 203.0.113.7
r acc8 --instance 'Bad_Name!' --access-ip 203.0.113.7
r acc9 --instance acc9 --mgmt-port 8443 --nginx-port 8443 --access-ip 203.0.113.7
[[ -d /etc/drevanta-acc7 || -d /etc/drevanta-acc8 || -d /etc/drevanta-acc9 ]] && echo MODIFIED > "$T2/mods" || echo CLEAN > "$T2/mods"
# fresh install (non-interactive capture: token material auto-suppressed)
r acc1 --instance acc1 --access-ip 203.0.113.7
# fresh install with a REAL interactive terminal (script pty): token shown
ri acc1i --instance acc4 --access-ip 203.0.113.7
# existing-install protection
r acc1b --instance acc1 --access-ip 203.0.113.7
# upgrade with Drevanta-owned listeners present (must be legitimate)
touch /tmp/drevanta-simulate-owned-ports
r acc1c --instance acc1 --upgrade --access-ip 203.0.113.7 --mgmt-port 9443
rm -f /tmp/drevanta-simulate-owned-ports
# unrelated port conflict (must be refused, nothing modified)
touch /tmp/drevanta-simulate-foreign-port
r acc10 --instance acc10 --access-ip 203.0.113.7
rm -f /tmp/drevanta-simulate-foreign-port
# claim-token CLI disclosure gates
/usr/local/bin/drevanta-claim-token > "$T2/wrap1" 2>&1; echo $? > "$T2/wrap1.rc"   # non-TTY -> refused (acc2 pending)
python3 - <<'PYEOF2'
import sqlite3
db = sqlite3.connect('/var/lib/drevanta-acc2/panel.db')
db.execute("update installation set state='claimed'")
db.commit()
PYEOF2
/usr/local/bin/drevanta-claim-token > "$T2/wrap2" 2>&1; echo $? > "$T2/wrap2.rc"   # claimed -> refused + stale file removed
# adapter-node forwarded-protocol environment present in the generated unit
grep -q "PROTOCOL_HEADER" /etc/systemd/system/drevanta-acc1-web.service && echo present > "$T2/protohdr" || echo missing > "$T2/protohdr"
# installed VERSION record must be the artifact version (os-release clobber guard)
head -1 /opt/drevanta-acc1/VERSION > "$T2/basever" 2>/dev/null || echo missing > "$T2/basever"
# Finding A regression: vhosts preserve the external host:port (http_host variable)
grep -q 'Host \$http_host' /etc/nginx/sites-available/drevanta-acc1-mgmt.conf && echo present > "$T2/hh" || echo missing > "$T2/hh"
# Finding B regression: authorized regeneration with expired / missing token
# records. A fresh install (acc11) re-bakes the retrieval wrapper for a
# PENDING instance; the script(1) tool provides the interactive terminal the disclosure
# gates require.
r acc11 --instance acc11 --access-ip 203.0.113.7
if command -v script >/dev/null 2>&1; then
  ls -la /usr/local/bin/ > "$T2/lsusr" 2>&1
  date -u -d '25 hours ago' '+%Y-%m-%dT%H:%M:%SZ' > /etc/drevanta-acc11/claim-token-created
  script -e -qec '/usr/local/bin/drevanta-claim-token' /dev/null > "$T2/wrap-exp" 2>&1; echo \$? > "$T2/wrap-exp.rc"
  script -e -qec '/usr/local/bin/drevanta-claim-token --regenerate' /dev/null > "$T2/wrap-reg" 2>&1; echo \$? > "$T2/wrap-reg.rc"
  script -e -qec '/usr/local/bin/drevanta-claim-token' /dev/null > "$T2/wrap-after" 2>&1; echo \$? > "$T2/wrap-after.rc"
  rm -f /etc/drevanta-acc11/claim-token
  script -e -qec '/usr/local/bin/drevanta-claim-token --regenerate' /dev/null > "$T2/wrap-reg2" 2>&1; echo \$? > "$T2/wrap-reg2.rc"
  echo done > "$T2/regen.done"
else
  echo skipped > "$T2/regen.done"
fi
# public listener withdrawal
r acc1d --instance acc1 --upgrade --access-ip 203.0.113.7 --private-mgmt
# lock contention
bash "\$E" --instance acc6 --access-ip 203.0.113.7 >/dev/null 2>&1 &
P1=\$!
sleep 0.4
bash "\$E" --instance acc6 --access-ip 203.0.113.7 > "$T2/lock.log" 2>&1; echo \$? > "$T2/lock.rc"
wait \$P1
# verified rollback (forced upgrade failure via harness curl marker)
touch /tmp/drevanta-force-upgrade-failure
r acc1e --instance acc1 --upgrade --access-ip 203.0.113.7
rm -f /tmp/drevanta-force-upgrade-failure
# rollback-failure reporting (restore healthy=false via second marker)
touch /tmp/drevanta-force-upgrade-failure /tmp/drevanta-force-rollback-unhealthy
r acc1f --instance acc1 --upgrade --access-ip 203.0.113.7
rm -f /tmp/drevanta-force-upgrade-failure /tmp/drevanta-force-rollback-unhealthy
# private-mgmt fresh install (no public URLs at all)
r acc3 --instance acc3 --access-ip 203.0.113.7 --private-mgmt
# stale claim-token cleanup: claim the installation out-of-band, then upgrade
r acc2 --instance acc2 --access-ip 203.0.113.7
python3 - <<'PYEOF'
import sqlite3
db = sqlite3.connect('/var/lib/drevanta-acc2/panel.db')
db.execute("update installation set state='claimed'")
db.commit()
PYEOF
r acc2b --instance acc2 --upgrade --access-ip 203.0.113.7
SEOF
  unshare -rm bash "$T2/session.sh"

  # --- validation negatives ---
  r=$(cat "$T2/acc7.rc"); [[ "$r" != "0" ]] && ok "T2: invalid mgmt port refused (exit $r)" || bad "T2: invalid port accepted"
  has "T2: invalid port message" "$T2/acc7.log" "port MGMT_PORT=99999 is invalid"
  r=$(cat "$T2/acc8.rc"); [[ "$r" != "0" ]] && ok "T2: invalid instance name refused (exit $r)" || bad "T2: invalid instance accepted"
  r=$(cat "$T2/acc9.rc"); [[ "$r" != "0" ]] && ok "T2: duplicate ports refused (exit $r)" || bad "T2: duplicate ports accepted"
  [[ "$(cat "$T2/mods")" == "CLEAN" ]] && ok "T2: failed validation modified NOTHING" || bad "T2: host modified after failed validation"

  # --- fresh install (non-interactive: token auto-suppressed) ---
  r=$(cat "$T2/acc1.rc")
  [[ "$r" == "0" ]] && ok "T2 fresh install exit 0" || { bad "T2 fresh install exit $r"; grep "\[FAIL\]" "$T2/acc1.log" | head -4; }
  has "T2: INSTALLATION COMPLETE displayed" "$T2/acc1.log" "INSTALLATION COMPLETE"
  lacks "T2 non-interactive: NO token material without --unattended" "$T2/acc1.log" "clm_[0-9a-f]{62}"
  has "T2 non-interactive: suppression declared" "$T2/acc1.log" "Token material suppressed"
  has "T2 non-interactive: retrieval pointer" "$T2/acc1.log" "drevanta-claim-token --url"
  has "T2: mgmt vhost configured" "$T2/acc1.log" "management vhost https://0.0.0.0:8443"
  has "T2: certificate SAN = access IP" "$T2/acc1.log" "SAN covers the access address \(203.0.113.7\)"
  has "T2: cert/key match gate" "$T2/acc1.log" "certificate and key match"
  has "T2: helper socket gate" "$T2/acc1.log" "privileged helper socket present"
  has "T2: isolation gate (API loopback)" "$T2/acc1.log" "control-plane API bound to 127.0.0.1:8080 only"
  has "T2: db initialized gate" "$T2/acc1.log" "database initialized; installation pending"
  has "T2: external reachability honesty line" "$T2/acc1.log" "NOT verified"
  LOGF=$(grep -oP "Installation log   : \K\S+" "$T2/acc1.log" | head -1)
  [[ -n "$LOGF" ]] && ok "T2: install log exists" || bad "T2: no log path in summary"
  if [[ -n "$LOGF" ]]; then
    lacks "T2: NO token in persistent log" "$LOGF" "clm_[0-9a-f]{62}"
    has "T2: claim expiry recorded (created+24h)" "$T2/acc1.log" "expires 20[0-9-]+ [0-9:]+ UTC"
  fi
  has "T2: report json written" "$T2/acc1.log" "install-report.json"

  # --- interactive run: token shown ONLY in the authorized terminal ---
  r=$(cat "$T2/acc1i.rc")
  [[ "$r" == "0" ]] && ok "T2 interactive fresh install exit 0" || bad "T2 interactive exit $r"
  has "T2 interactive: token-bearing primary URL present" "$T2/acc1i.log" "https://203.0.113.7:8443/#clm_[0-9a-f]{62}"
  has "T2 interactive: tunnel onboarding URL present" "$T2/acc1i.log" "http://localhost:8080/#clm_"
  has "T2 interactive: SSH tunnel command" "$T2/acc1i.log" "ssh -N -L 8080:127.0.0.1:80 root@203.0.113.7"
  has "T2 interactive: TLS fingerprint displayed" "$T2/acc1i.log" "Certificate SHA-256 fingerprint: [0-9A-F:]{95}"
  # Presentation order is defined by the summary source; the captured file's
  # line order is skewed by the asynchronous log reader (fd3 vs FIFO), so the
  # ordering gate checks the engine source.
  CSRC=$(grep -n "CERTIFICATE VERIFICATION" "$REPO_DIR/engine/install.sh" | head -1 | cut -d: -f1)
  USRC=$(grep -n "PRIMARY ACCESS — DIRECT SERVER IP" "$REPO_DIR/engine/install.sh" | head -1 | cut -d: -f1)
  [[ -n "$CSRC" && -n "$USRC" && "$CSRC" -lt "$USRC" ]] \
    && ok "T2: certificate guidance PRECEDES token-bearing URL (summary source order)" || bad "T2: certificate guidance ordering"

  # --- existing-install protection + upgrade with port change ---
  r=$(cat "$T2/acc1b.rc"); [[ "$r" != "0" ]] && ok "T2: existing install refused without --upgrade (exit $r)" || bad "T2: existing install NOT refused"
  has "T2: refusal is explicit + non-destructive" "$T2/acc1b.log" "refusing to modify"
  r=$(cat "$T2/acc1c.rc"); [[ "$r" == "0" ]] && ok "T2 upgrade (mgmt port 8443->9443) exit 0" || bad "T2 upgrade exit $r"
  grep -q "management vhost re-rendered (port 9443)" "$T2/acc1c.log" && ok "T2 upgrade: vhost re-rendered on new port" || bad "T2 upgrade: no re-render"
  # --- public listener withdrawal ---
  r=$(cat "$T2/acc1d.rc"); [[ "$r" == "0" ]] && ok "T2 upgrade -> private-mgmt exit 0" || bad "T2 private-mgmt switch exit $r"
  grep -q "public management listener" "$T2/acc1d.log" && ok "T2: public listener withdrawn" || bad "T2: listener not withdrawn"

  # --- lock contention ---
  r=$(cat "$T2/lock.rc"); [[ "$r" != "0" ]] && ok "T2: concurrent install refused by lock (exit $r)" || bad "T2: lock NOT enforced"
  has "T2: lock message" "$T2/lock.log" "already running"

  # --- verified rollback ---
  r=$(cat "$T2/acc1e.rc"); [[ "$r" != "0" ]] && ok "T2: forced upgrade failure exits non-zero (exit $r)" || bad "T2: forced failure not reported"
  has "T2: rollback verified healthy (true previous version named)" "$T2/acc1e.log" "rollback verified: previous version 1\.0\.0-owneraudit\.1\+[0-9a-f]{40} restored"
  has "T2: rollback health evidence + simulation disclosure" "$T2/acc1e.log" "local checks only: panel:200 web:200"
  # Finding A: external host:port preserved through the proxy chain
  [[ "$(grep -c 'proxy_set_header Host \\$http_host;' engine/install.sh 2>/dev/null || echo 0)" == "5" ]] \
    && ! grep -q 'proxy_set_header Host \\$host;' engine/install.sh \
    && ok "T2: all 5 proxy locations preserve external host:port (Host \$http_host)" \
    || bad "T2: Host header preservation (\$http_host)"
  [[ "$(cat "$T2/hh" 2>/dev/null)" == "present" ]] && ok "T2: generated mgmt vhost carries Host \$http_host" || bad "T2: generated vhost missing Host \$http_host"
  # Finding B: authorized regeneration with expired / missing token records
  [[ "$(cat "$T2/regen.done" 2>/dev/null)" == "done" ]] && ok "T2: regeneration regression suite executed" || bad "T2: regeneration regression did not run (script(1) unavailable)"
  if [[ "$(cat "$T2/regen.done" 2>/dev/null)" == "done" ]]; then
    [[ "$(cat "$T2/wrap-exp.rc")" == "1" ]] && ok "T2: expired token refuses retrieval (no disclosure)" || bad "T2: expired token still disclosed (rc=$(cat "$T2/wrap-exp.rc"))"
    grep -q "has EXPIRED" "$T2/wrap-exp" && ok "T2: expired-token message names the recovery path" || bad "T2: expired-token message missing"
    [[ "$(cat "$T2/wrap-reg.rc")" == "0" ]] && ok "T2: regeneration succeeds for an EXPIRED token" || bad "T2: expired-token regeneration failed (rc=$(cat "$T2/wrap-reg.rc"))"
    [[ "$(cat "$T2/wrap-after.rc")" == "0" ]] && ok "T2: regenerated token is retrievable (fresh TTL)" || bad "T2: regenerated token not retrievable"
    [[ "$(cat "$T2/wrap-reg2.rc")" == "0" ]] && ok "T2: regeneration succeeds with MISSING plaintext token file" || bad "T2: missing-file regeneration failed (rc=$(cat "$T2/wrap-reg2.rc"))"
    newtok=$(grep -oE 'clm_[0-9a-f]+' "$T2/wrap-reg" | head -1); newtok2=$(grep -oE 'clm_[0-9a-f]+' "$T2/wrap-reg2" | head -1)
    [[ -n "$newtok" && -n "$newtok2" && "$newtok" != "$newtok2" ]] && ok "T2: each regeneration issues a distinct fresh token" || bad "T2: regeneration token uniqueness"
  fi
  r=$(cat "$T2/acc1f.rc"); [[ "$r" != "0" ]] && ok "T2: rollback-failure path exits non-zero (exit $r)" || bad "T2: rollback-failure exit"
  has "T2: rollback INCOMPLETE reported honestly" "$T2/acc1f.log" "rollback INCOMPLETE"

  # --- private-management fresh install: no public URLs anywhere ---
  r=$(cat "$T2/acc3.rc"); [[ "$r" == "0" ]] && ok "T2 private-mgmt: exit 0" || bad "T2 private-mgmt: exit $r"
  grep -q "PRIMARY ACCESS" "$T2/acc3.log" && bad "T2 private-mgmt: public PRIMARY ACCESS section shown" || ok "T2 private-mgmt: no public primary-access section"
  grep -qE "https://203.0.113.7:[0-9]+/#clm_" "$T2/acc3.log" && bad "T2 private-mgmt: public onboarding URL shown" || ok "T2 private-mgmt: no public onboarding URL"
  has "T2 private-mgmt: tunnel-only section" "$T2/acc3.log" "PRIVATE MANAGEMENT"

  # --- stale claim-token cleanup ---
  r=$(cat "$T2/acc2b.rc"); [[ "$r" == "0" ]] && ok "T2: upgrade over claimed installation exit 0" || bad "T2: claimed upgrade exit $r"
  has "T2: stale plaintext token removed" "$T2/acc2b.log" "stale claim-token file removed"

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

echo "================ T5: panel claim lifecycle CLI ================"
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
( cd "$REPO_DIR" && go test ./internal/install/ -run "TestConcurrentClaimAttempts" -count=1 ) >/dev/null 2>&1 \
  && ok "T5: concurrent claim protection (4 racers, exactly 1 winner)" || bad "T5: concurrent claim"

echo "================ T6: dependency provisioning + recovery (dev.6) ================"
if unshare -rm true 2>/dev/null; then
  T6="$WORK/t6"; mkdir -p "$T6"
  PKG6="$WORK/pkg6"; mkdir -p "$PKG6"
  tar --no-same-owner -xzf "$REPO_DIR"/drevanta-install-*.tar.gz -C "$PKG6"
  PKGDIR6="$(echo "$PKG6"/drevanta-install-*)"
  # Dependency-EMPTY stub environment: nginx/node/npm/git invisible to the
  # engine; the recording apt-get materializes them on 'install'. systemd-run
  # records the isolation probe. These are PROVISIONING-path tests; real-apt
  # provisioning evidence is collected separately (T7 / namespace e2e).
  STUB6="$T6/stub6"; mkdir -p "$STUB6"
  for t in systemctl sudo ss ufw useradd curl; do cp "$STUB/$t" "$STUB6/"; done
  cat > "$STUB6/apt-get" <<APEOF
#!/usr/bin/env bash
echo "apt-get \$*" >> "$T6/apt.record"
if [[ "\$1" == "install" ]]; then
  for p in "\$@"; do case "\$p" in
    nginx)  printf '#!/usr/bin/env bash\nexit 0\n' > "$STUB6/nginx"; chmod +x "$STUB6/nginx" ;;
    nodejs) printf '#!/usr/bin/env bash\n[[ "\$1" == "--version" ]] && echo v22.11.0\nexit 0\n' > "$STUB6/node"; chmod +x "$STUB6/node" ;;
    npm)    printf '#!/usr/bin/env bash\nexit 0\n' > "$STUB6/npm"; chmod +x "$STUB6/npm" ;;
    git)    printf '#!/usr/bin/env bash\nexit 0\n' > "$STUB6/git"; chmod +x "$STUB6/git" ;;
  esac; done
fi
exit 0
APEOF
  printf '#!/usr/bin/env bash\necho "systemd-run \$*" >> "%s"\nexit 0\n' "$T6/sdrun.record" > "$STUB6/systemd-run"
  chmod +x "$STUB6"/*
  cat > "$T6/session.sh" <<SEOF
#!/usr/bin/env bash
set -u
cp -a /etc /tmp/etc.keep
mount -t tmpfs -o size=8g tmpfs /etc && mount -t tmpfs -o size=8g tmpfs /opt && mount -t tmpfs -o size=8g tmpfs /var && mount -t tmpfs tmpfs /usr/local && mount -t tmpfs tmpfs /run
cp -a /tmp/etc.keep/. /etc/ && rm -rf /tmp/etc.keep
export HOME=/root
E6="$PKGDIR6/install.sh"
r6() { local f="\$1"; shift; PATH="$STUB6:/usr/bin:/bin" bash "\$E6" "\$@" > "$T6/\$f.log" 2>&1; echo \$? > "$T6/\$f.rc"; }
rf() { local f="\$1"; local stub="\$2"; shift 2; PATH="\$stub:/usr/bin:/bin" bash "\$E6" "\$@" > "$T6/\$f.log" 2>&1; echo \$? > "$T6/\$f.rc"; }
rw() { local f="\$1"; local stub="\$2"; shift 2; PATH="\$stub" bash "\$E6" "\$@" > "$T6/\$f.log" 2>&1; echo \$? > "$T6/\$f.rc"; }

# d1: dep-empty host, ONLINE mode -> provision the missing, verify, install
r6 d1 --instance d1 --access-ip 203.0.113.7
cp /etc/systemd/system/drevanta-d1-web.service "$T6/d1-web.service" 2>/dev/null
cp /etc/systemd/system/drevanta-d1-panel.service "$T6/d1-panel.service" 2>/dev/null
{ grep -q "ConditionPathExists=/usr/bin/node" /etc/systemd/system/drevanta-d1-web.service \
    && grep -q "StartLimitBurst=5" /etc/systemd/system/drevanta-d1-web.service \
    && echo ok || echo missing; } > "$T6/m.webunit"
{ grep -q "DREVANTA_BUILD_SCRATCH=/opt/drevanta-d1/builds" /etc/systemd/system/drevanta-d1-panel.service \
    && grep -q "StartLimitBurst=5" /etc/systemd/system/drevanta-d1-panel.service \
    && echo ok || echo missing; } > "$T6/m.panelunit"
{ grep -q "StartLimitBurst=5" /etc/systemd/system/drevanta-d1-helper.service && echo ok || echo missing; } > "$T6/m.helperunit"
cp /etc/drevanta-d1/build-bindings.json "$T6/d1-bindings.json" 2>/dev/null
{ [[ -f /etc/drevanta-d1/build-bindings.json ]] \
    && [[ "\$(stat -c %a /etc/drevanta-d1/build-bindings.json)" == "600" ]] \
    && grep -q "/opt/drevanta-d1/builds" /etc/drevanta-d1/build-bindings.json \
    && echo ok || echo missing; } > "$T6/m.bindings"
[[ -s "$T6/sdrun.record" ]] && echo ok > "$T6/m.sdrun" || echo missing > "$T6/m.sdrun"
{ grep -q "builds will fail closed" /etc/systemd/system/drevanta-d1-panel.service 2>/dev/null; } || true

# d2: provisioning FAILS -> abort BEFORE any service deployment
mkdir -p "$T6/stub6fail"; for t in systemctl sudo ss ufw useradd curl systemd-run; do cp "$STUB/\$t" "$T6/stub6fail/" 2>/dev/null || true; done
printf '#!/usr/bin/env bash\nexit 1\n' > "$T6/stub6fail/apt-get"
chmod +x "$T6/stub6fail"/*
rf d2 "$T6/stub6fail" --instance d2 --access-ip 203.0.113.7
[[ -f /etc/systemd/system/drevanta-d2-web.service ]] && echo deployed > "$T6/m.d2nodesvc" || echo nodeploy > "$T6/m.d2nodesvc"
[[ -d /var/lib/drevanta-d2 && -f /var/lib/drevanta-d2/panel.db ]] && echo datadeployed > "$T6/m.d2data" || echo nodata > "$T6/m.d2data"

# d3: --offline with missing deps -> precise abort listing, nothing deployed
# Genuinely dependency-empty offline host: a WHITELIST bin dir (stubs + the
# coreutils/python/openssl the engine itself needs) — NO node/npm/git, and
# /usr/bin is NOT on PATH so the container's real runtimes cannot leak in.
mkdir -p "$T6/stub6off"
for t in systemctl sudo ss ufw useradd curl systemd-run; do cp "$STUB/\$t" "$T6/stub6off/" 2>/dev/null || true; done
for t in bash sh mkfifo install chown df free tee awk readlink realpath groups ps pgrep which command flock ip python3 python openssl sha256sum tar mktemp date id stat ls cat sed grep sleep seq head tail wc tr cut sort uniq chmod touch cp mv rm mkdir rmdir ln basename dirname env uname shred; do
  p="\$(command -v "\$t" 2>/dev/null || true)"; [[ -n "\$p" ]] && ln -sf "\$p" "$T6/stub6off/\$t"
done
chmod +x "$T6/stub6off"/* 2>/dev/null
rw d3 "$T6/stub6off" --instance d3 --access-ip 203.0.113.7 --offline
[[ -f /etc/systemd/system/drevanta-d3-web.service ]] && echo deployed > "$T6/m.d3svc" || echo nodeploy > "$T6/m.d3svc"

# d4: node too old -> unsupported-runtime abort (before deployment)
mkdir -p "$T6/stub6old"; for t in systemctl sudo ss ufw useradd curl systemd-run nginx npm git; do cp "$STUB/\$t" "$T6/stub6old/" 2>/dev/null || true; done
printf '#!/usr/bin/env bash\n[[ "\$1" == "--version" ]] && echo v18.19.0\nexit 0\n' > "$T6/stub6old/node"
chmod +x "$T6/stub6old"/*
rf d4 "$T6/stub6old" --instance d4 --access-ip 203.0.113.7

# d5: RECOVERY of a partial dev.5-era install: deps provisioned, data/claim/TLS preserved
"$T6/stub6/useradd" drevanta-d5
mkdir -p /opt/drevanta-d5/bin /opt/drevanta-d5/web/build /opt/drevanta-d5/migrations /var/lib/drevanta-d5 /etc/drevanta-d5/tls
printf '1.0.0-owneraudit.1-dev.5\n' > /opt/drevanta-d5/VERSION
printf 'placeholder\n' > /opt/drevanta-d5/bin/panel
printf 'console\n' > /opt/drevanta-d5/web/build/index.js
python3 - <<'PYD5'
import sqlite3
db = sqlite3.connect('/var/lib/drevanta-d5/panel.db')
db.execute("create table installation (installation_id text, state text)")
db.execute("insert into installation values ('inst_d5aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','pending')")
db.commit()
PYD5
printf 'clm_d5existingpendingownerclaimtokenvalue' > /etc/drevanta-d5/claim-token
printf '{"policy":true}\n' > /etc/drevanta-d5/helper-policy.json
openssl req -x509 -newkey rsa:2048 -keyout /etc/drevanta-d5/tls/key.pem -out /etc/drevanta-d5/tls/cert.pem -days 5 -nodes -subj "/CN=203.0.113.7" -addext "subjectAltName=IP:203.0.113.7" >/dev/null 2>&1
sha256sum /etc/drevanta-d5/claim-token /var/lib/drevanta-d5/panel.db /etc/drevanta-d5/tls/cert.pem /etc/drevanta-d5/tls/key.pem > "$T6/d5.before.sha"
touch /tmp/drevanta-simulate-owned-ports
r6 d5 --instance d5 --recover --access-ip 203.0.113.7
rm -f /tmp/drevanta-simulate-owned-ports
sha256sum -c "$T6/d5.before.sha" > "$T6/d5.preserved" 2>&1; echo \$? > "$T6/d5.preserved.rc"
{ grep -q "ConditionPathExists=/usr/bin/node" /etc/systemd/system/drevanta-d5-web.service && echo ok || echo missing; } > "$T6/m.d5webunit"
{ grep -q "DREVANTA_BUILD_SCRATCH=/opt/drevanta-d5/builds" /etc/systemd/system/drevanta-d5-panel.service && echo ok || echo missing; } > "$T6/m.d5panelunit"
{ [[ -f /etc/drevanta-d5/build-bindings.json ]] && echo ok || echo missing; } > "$T6/m.d5bindings"

# d6: recovery refuses when nothing exists to recover
r6 d6 --instance d6 --recover --access-ip 203.0.113.7
SEOF
  unshare -rm bash "$T6/session.sh"

  r=$(cat "$T6/d1.rc"); [[ "$r" == "0" ]] && ok "T6: dep-empty host installs after provisioning (exit $r)" || bad "T6: dep-empty install failed (exit $r; see d1.log)"
  grep -q "apt-get update" "$T6/apt.record" 2>/dev/null && ok "T6: apt-get update invoked" || bad "T6: apt-get update not invoked"
  grep -qE "apt-get install -y --no-install-recommends( nginx| nginx nodejs npm git)$" "$T6/apt.record" 2>/dev/null \
    && ok "T6: exactly the missing mandatory packages provisioned" || bad "T6: wrong provisioning list ($(tail -1 "$T6/apt.record" 2>/dev/null))"
  has   "T6: provisioning reported" "$T6/d1.log" "provisioning mandatory dependencies"
  has   "T6: deps verified after provisioning" "$T6/d1.log" "deps verified: nginx, node v"
  has   "T6: INSTALLATION COMPLETE on dep-empty host" "$T6/d1.log" "INSTALLATION COMPLETE"
  has   "T6: build boundary probe verified" "$T6/d1.log" "build boundary: verified"
  [[ "$(cat "$T6/m.webunit")" == "ok" ]] && ok "T6: web unit hardened (node condition + restart cap)" || bad "T6: web unit hardening missing"
  [[ "$(cat "$T6/m.panelunit")" == "ok" ]] && ok "T6: panel unit carries build-boundary env + restart cap" || bad "T6: panel unit boundary env missing"
  [[ "$(cat "$T6/m.helperunit")" == "ok" ]] && ok "T6: helper unit restart cap present" || bad "T6: helper unit cap missing"
  [[ "$(cat "$T6/m.bindings")" == "ok" ]] && ok "T6: build-bindings.json root-allowlisted (0600)" || bad "T6: build bindings missing/wrong"
  [[ "$(cat "$T6/m.sdrun")" == "ok" ]] && ok "T6: isolation probe executed (systemd-run)" || bad "T6: isolation probe not executed"

  r=$(cat "$T6/d2.rc"); [[ "$r" != "0" ]] && ok "T6: provisioning failure aborts install (exit $r)" || bad "T6: provisioning failure NOT fatal"
  has "T6: apt failure surfaced" "$T6/d2.log" "deps: apt-get install"
  [[ "$(cat "$T6/m.d2nodesvc")" == "nodeploy" ]] && ok "T6: NO service units deployed after provisioning failure" || bad "T6: services deployed despite failed provisioning"
  [[ "$(cat "$T6/m.d2data")" == "nodata" ]] && ok "T6: no Drevanta data deployed after provisioning failure" || bad "T6: data deployed despite failed provisioning"

  r=$(cat "$T6/d3.rc"); [[ "$r" != "0" ]] && ok "T6: --offline with missing deps refused (exit $r)" || bad "T6: offline missing deps accepted"
  has "T6: offline abort lists nginx" "$T6/d3.log" "mandatory dependency missing: nginx"
  has "T6: offline abort lists node" "$T6/d3.log" "mandatory dependency missing: node"
  [[ "$(cat "$T6/m.d3svc")" == "nodeploy" ]] && ok "T6: offline abort deployed NOTHING" || bad "T6: offline abort still deployed units"

  r=$(cat "$T6/d4.rc"); [[ "$r" != "0" ]] && ok "T6: unsupported Node.js runtime refused (exit $r)" || bad "T6: node 18 accepted"
  has "T6: node version gate message" "$T6/d4.log" "unsupported Node.js runtime"

  r=$(cat "$T6/d5.rc"); [[ "$r" == "0" ]] && ok "T6: recovery of partial install completes (exit $r)" || bad "T6: recovery failed (exit $r)"
  has "T6: RECOVERY COMPLETE banner" "$T6/d5.log" "RECOVERY COMPLETE"
  has "T6: recovery preserved pending claim" "$T6/d5.log" "pending owner claim and installation identity preserved"
  [[ "$(cat "$T6/d5.preserved.rc")" == "0" ]] && ok "T6: claim-token, panel.db, TLS cert+key byte-preserved" || bad "T6: recovery modified preserved state ($(cat "$T6/d5.preserved"))"
  [[ "$(cat "$T6/m.d5webunit")" == "ok" ]] && ok "T6: recovery re-rendered hardened web unit" || bad "T6: recovery kept old web unit"
  [[ "$(cat "$T6/m.d5panelunit")" == "ok" ]] && ok "T6: recovery re-rendered panel unit with boundary env" || bad "T6: recovery panel unit missing boundary env"
  [[ "$(cat "$T6/m.d5bindings")" == "ok" ]] && ok "T6: recovery created build bindings" || bad "T6: recovery bindings missing"
  grep -q "apt-get install" "$T6/apt.record" && ok "T6: recovery provisions missing dependencies" || bad "T6: recovery skipped provisioning"

  r=$(cat "$T6/d6.rc"); [[ "$r" != "0" ]] && ok "T6: recovery refuses when no installation exists (exit $r)" || bad "T6: empty-host recovery accepted"
  has "T6: recovery refusal message" "$T6/d6.log" "no existing installation found"
fi

echo "========================================================"
echo "VERIFICATION SUMMARY: $PASS passed, $FAIL failed"
[[ "$FAIL" == "0" ]] || exit 1

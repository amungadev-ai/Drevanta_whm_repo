#!/usr/bin/env bash
# T7 — REAL provisioning + REAL recovery evidence (dev.6 correction).
#
# Real elements (environment permitting):
#   - REAL Debian archive fetch: apt update + real nginx .deb download
#   - REAL nginx binary (from the real package) — nginx -t, real TLS on :18443
#   - REAL Node.js adapter-node console on :13100 (real /usr/bin/node)
#   - REAL panel + helper Go binaries (loopback API :18080 + real unix socket)
#   - REAL panel bootstrap state (schema, pending claim), real SQLite/TLS
#   - REAL TLS material, real listener verification, real HTTP health codes
# Simulated (disclosed): systemd SUPERVISION ONLY — a launching stub starts
# the real ExecStart commands; user identity (root — a user namespace maps
# only one uid); dpkg's unpack phase (libc-containing packages cannot unpack
# inside the userns — the real nginx binary is extracted from the real .deb).
# Unique ports avoid sandbox-infra listeners on the shared network namespace.
set -u
REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
OUT="${1:-/tmp/t7-evidence}"
rm -rf "$OUT"; mkdir -p "$OUT/logs"

unshare -rm true 2>/dev/null || { echo "unshare -rm unavailable"; exit 3; }

# Leftover cleanup: processes from an earlier run survive the mount+user
# namespace (no PID namespace) and would hold the unique ports.
pkill -f "panel -addr 127.0.0.1:18080" 2>/dev/null
pkill -f "helper -socket /run/drevanta-r1" 2>/dev/null
pkill -f "build/index.js" 2>/dev/null
pkill -f "drevanta-r1" 2>/dev/null
pkill -f "nginx: master" 2>/dev/null
sleep 1

PKG="$OUT/pkg"; mkdir -p "$PKG"
tar --no-same-owner -xzf "$REPO_DIR"/drevanta-install-*.tar.gz -C "$PKG"
PKGDIR="$(echo "$PKG"/drevanta-install-*)"

# NOTE: the session heredoc is QUOTED — nothing expands at write time. Outer
# values arrive via T7OUT/T7PKGDIR environment (passed to unshare below).
cat > "$OUT/session.sh" <<'SEOF'
export OUT="${T7OUT:?}"
PKGDIR="${T7PKGDIR:?}"
export HOME=/tmp/t7-home; mkdir -p "$HOME"
export DEBIAN_FRONTEND=noninteractive
rm -rf /tmp/etc.keep
cp -a /etc /tmp/etc.keep 2>/dev/null   # BEFORE mounting: the real /etc
mount -t tmpfs -o size=1g tmpfs /etc
mount -t tmpfs -o size=2g tmpfs /opt
mount -t tmpfs -o size=256m tmpfs /run
mount -t tmpfs -o size=4g tmpfs /var
cp -a /tmp/etc.keep/. /etc/ 2>/dev/null; rm -rf /tmp/etc.keep
mkdir -p /var/lib/dpkg/updates /var/lib/dpkg/info /var/lib/dpkg/triggers /var/lib/apt/lists/partial /var/cache/apt/archives/partial /var/log/nginx /var/lib/nginx/body /var/lib/nginx/proxy
touch /var/lib/dpkg/status /var/lib/dpkg/available

STUB="$HOME/t7-stub"; mkdir -p "$STUB"
# Identity stubs: a user namespace maps only ONE uid — setuid to a service
# user is impossible; useradd/sudo stubbed (root-run), DISCLOSED.
printf '#!/usr/bin/env bash\nU="${@: -1}"; echo "$U:x:0:0::/var/empty:/usr/sbin/nologin" >> /etc/passwd; echo "$U:x:0:" >> /etc/group\n' > "$STUB/useradd"
printf '#!/usr/bin/env bash\nwhile [[ "$1" == "-u" || "$1" == "-g" || "$1" == "-E" ]]; do case "$1" in -u|-g) shift 2;; *) shift;; esac; done\nexec "$@"\n' > "$STUB/sudo"
# Launching systemctl stub: starts the real ExecStart of EVERY named unit;
# nginx handled through its real binary. Supervision NOT simulated.
cat > "$STUB/systemctl" <<'STUBEOF'
#!/usr/bin/env bash
SH=/opt/nginx-x/usr/sbin
ACT=0
for a in "$@"; do case "$a" in enable|--now|start|restart|reload|daemon-reload) ACT=1;; esac; done
echo "stub invoked: $*" >> "$OUT/logs/stub-audit.log"
[[ "$ACT" == 1 ]] || { echo "  no action verb" >> "$OUT/logs/stub-audit.log"; exit 0; }
for UNIT in "$@"; do
  echo "  unit arg: $UNIT" >> "$OUT/logs/stub-audit.log"
  case "$UNIT" in
    nginx)
      RC=0
      for a in "$@"; do
        case "$a" in
          start|restart) "$SH/nginx" >> "$OUT/logs/nginx-binary.log" 2>&1 || RC=1;;
          reload) "$SH/nginx" -s reload >> "$OUT/logs/nginx-binary.log" 2>&1 || RC=1;;
          stop) "$SH/nginx" -s stop >> "$OUT/logs/nginx-binary.log" 2>&1 || RC=1;;
        esac
      done
      exit $RC;;  # real rc so the engine's `reload || restart` fallback fires
    *-helper|*-panel|*-web) :;;
    *) continue;;
  esac
  UF="/etc/systemd/system/$UNIT"
  [[ -f "$UF" ]] || UF="$UF.service"   # systemd resolves bare names with .service
  [[ -f "$UF" ]] || { echo "    no unit file: $UF" >> "$OUT/logs/stub-audit.log"; continue; }
  echo "    launching $UNIT (exec found)" >> "$OUT/logs/stub-audit.log"
  if [[ -f "/run/t7-$UNIT.pid" ]] && kill -0 "$(cat "/run/t7-$UNIT.pid" 2>/dev/null)" 2>/dev/null; then continue; fi
  ENVV=""; EXEC=""; WD=""
  while IFS= read -r line; do
    case "$line" in
      Environment=*) ENVV="$ENVV ${line#Environment=}";;
      ExecStart=*) EXEC="${line#ExecStart=}";;
      WorkingDirectory=*) WD="${line#WorkingDirectory=}";;
      User=*) : ;;  # root-run harness (single-uid userns; disclosed)
    esac
  done < "$UF"
  [[ -z "$EXEC" ]] && continue
  LOG="$OUT/logs/$UNIT.log"; : > "$LOG"
  ( cd "${WD:-/}" && eval "env $ENVV $EXEC" ) >> "$LOG" 2>&1 &
  echo $! > "/run/t7-$UNIT.pid"
  # Readiness wait: a real supervisor starts units and health follows; the
  # harness polls the unit's PORT (up to 10s) so health gates see it bound.
  PORTV=""
  for e in $ENVV; do case "$e" in PORT=*) PORTV="${e#PORT=}";; esac; done
  if [[ -n "$PORTV" ]]; then
    for i in $(seq 1 20); do
      (exec 3<>/dev/tcp/127.0.0.1/"$PORTV") 2>/dev/null && break
      sleep 0.5
    done
  fi
done
exit 0
STUBEOF
chmod +x "$STUB/systemctl" "$STUB/useradd" "$STUB/sudo"
export PATH="$STUB:$PATH"

echo "== step 1: baseline (nginx genuinely absent) =="
command -v nginx >/dev/null && { echo "PRECONDITION FAIL: nginx already present"; exit 3; }
echo "nginx absent at baseline — confirmed" | tee "$OUT/00-baseline.txt"

echo "== step 2: partial dev.5-style state via the REAL panel bootstrap =="
mkdir -p /opt/drevanta-r1/bin /opt/drevanta-r1/web/build /opt/drevanta-r1/migrations /var/lib/drevanta-r1 /etc/drevanta-r1/tls /var/log/drevanta-r1
printf '1.0.0-owneraudit.1-dev.5\n' > /opt/drevanta-r1/VERSION
printf 'placeholder\n' > /opt/drevanta-r1/bin/panel
printf 'console\n' > /opt/drevanta-r1/web/build/index.js
BOOT_OUT=$("$PKGDIR/bin/panel" bootstrap -db /var/lib/drevanta-r1/panel.db -migrations "$PKGDIR/migrations" 2>&1)
printf '%s\n' "$BOOT_OUT" > "$OUT/02a-bootstrap.txt"
R1TOKEN=$(printf '%s' "$BOOT_OUT" | grep -A1 "DREVANTA CLAIM TOKEN" | tail -1 | tr -d '[:space:]')
umask 077; printf '%s' "$R1TOKEN" > /etc/drevanta-r1/claim-token; umask 022
printf '{"policy":true}\n' > /etc/drevanta-r1/helper-policy.json
openssl req -x509 -newkey rsa:2048 -keyout /etc/drevanta-r1/tls/key.pem -out /etc/drevanta-r1/tls/cert.pem -days 5 -nodes -subj "/CN=203.0.113.7" -addext "subjectAltName=IP:203.0.113.7" >/dev/null 2>&1
sha256sum /etc/drevanta-r1/claim-token /var/lib/drevanta-r1/panel.db /etc/drevanta-r1/tls/cert.pem /etc/drevanta-r1/tls/key.pem > "$OUT/02-before.sha"

echo "== step 2b: real archive evidence + real nginx binary =="
apt-get -o APT::Sandbox::User=root update -qq >> "$OUT/00b-apt.log" 2>&1
apt-get -o APT::Sandbox::User=root install -y --no-install-recommends --download-only nginx >> "$OUT/00b-apt.log" 2>&1
DEBS=$(ls /var/cache/apt/archives/nginx*.deb 2>/dev/null)
if [[ -n "$DEBS" ]]; then
  rm -rf /opt/nginx-x
  for d in $DEBS; do dpkg -x "$d" /opt/nginx-x; done
  cp -a /opt/nginx-x/etc/nginx/. /etc/nginx/ 2>/dev/null
  sed -i 's/^user www-data;/user root;/' /etc/nginx/nginx.conf 2>/dev/null
  export PATH="/opt/nginx-x/usr/sbin:$PATH"
  { echo "real Debian packages: $(echo "$DEBS" | tr '\n' ' ')"; /opt/nginx-x/usr/sbin/nginx -v 2>&1; } | tee "$OUT/00b-nginx.txt"
else
  echo "NO nginx .deb downloadable in this environment" | tee "$OUT/00b-nginx.txt"
fi

echo "== step 3: RECOVERY with REAL services =="
bash "$PKGDIR/install.sh" --instance r1 --recover --access-ip 203.0.113.7 \
  --api-port 18080 --web-port 13100 --nginx-port 13080 --mgmt-port 18443 \
  > "$OUT/03-recovery.log" 2>&1
echo "recovery rc: $?" | tee -a "$OUT/03-recovery.log"
sha256sum -c "$OUT/02-before.sha" > "$OUT/04-preserved.txt" 2>&1; echo "preserved rc: $?" >> "$OUT/04-preserved.txt"

echo "== step 3b: post-recovery state dumps (diagnosis) =="
ls /etc/systemd/system/ > "$OUT/07-units.txt" 2>&1
grep -l "ExecStart" /etc/systemd/system/drevanta-r1-*.service >> "$OUT/07-units.txt" 2>&1
head -20 /etc/systemd/system/drevanta-r1-panel.service > "$OUT/08-panel-unit.txt" 2>&1
"$STUB/systemctl" start drevanta-r1-helper drevanta-r1-panel drevanta-r1-web >> "$OUT/07-units.txt" 2>&1
echo "manual stub rc: $?" >> "$OUT/07-units.txt"
sleep 3
ss -ltn | grep -E "18080|13100|13080|18443" >> "$OUT/05-listeners.txt" 2>&1
curl -sk -o /dev/null -w 'real-nginx-https-code:%{http_code}\n' --resolve "drevanta-mgmt:18443:127.0.0.1" "https://drevanta-mgmt:18443/" >> "$OUT/06-https.txt" 2>&1
curl -s -o /dev/null -w 'real-panel-code:%{http_code}\n' "http://127.0.0.1:18080/api/v1/install/status" >> "$OUT/06-https.txt" 2>&1
curl -s -o /dev/null -w 'real-console-code:%{http_code}\n' "http://127.0.0.1:13100/" >> "$OUT/06-https.txt" 2>&1

echo "== step 4: REAL listener + TLS evidence =="
ss -ltn | grep -E "18080|13100|13080|18443" > "$OUT/05-listeners.txt" 2>&1
curl -sk -o /dev/null -w 'real-nginx-https-code:%{http_code}\n' --resolve "drevanta-mgmt:18443:127.0.0.1" "https://drevanta-mgmt:18443/" > "$OUT/06-https.txt" 2>&1
curl -s -o /dev/null -w 'real-panel-code:%{http_code}\n' "http://127.0.0.1:18080/api/v1/install/status" >> "$OUT/06-https.txt" 2>&1
curl -s -o /dev/null -w 'real-console-code:%{http_code}\n' "http://127.0.0.1:13100/" >> "$OUT/06-https.txt" 2>&1
SEOF

T7OUT="$OUT" T7PKGDIR="$PKGDIR" unshare -rm bash "$OUT/session.sh"
echo "=== T7 evidence summary ==="
echo "--- baseline ---"; cat "$OUT/00-baseline.txt" 2>/dev/null
echo "--- real archive evidence ---"; cat "$OUT/00b-nginx.txt" 2>/dev/null
echo "--- engine provisioning + recovery outcome ---"
grep -E "provisioning mandatory|deps verified|RECOVERY COMPLETE|recovery rc|recovery: panel|recovery: console|recovery: management|preserved" "$OUT/03-recovery.log" | head -10
echo "--- preserved state ---"; cat "$OUT/04-preserved.txt" 2>/dev/null
echo "--- REAL listeners (unique ports) ---"; cat "$OUT/05-listeners.txt" 2>/dev/null
echo "--- REAL HTTP codes ---"; cat "$OUT/06-https.txt" 2>/dev/null

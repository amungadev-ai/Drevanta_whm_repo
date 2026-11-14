#!/usr/bin/env bash
# Drevanta manual installer — reproducible, honest, non-destructive.
# Installs an isolated Drevanta instance: privileged helper + Go control
# plane (panel) + SvelteKit console (adapter-node) + optional nginx vhost.
# Every step prints OK/FAIL; the installer NEVER reports success for a
# failed operation and exits non-zero on the first failed gate.
#
# Usage (root):
#   sudo ./install.sh                        # default instance, nginx on 127.0.0.1:80
#   sudo ./install.sh --instance test --nginx-port 8443
#   sudo ./install.sh --instance test --public-callback-port 8788
#
# Coexistence: a named --instance installs fully isolated paths, users,
# ports and systemd units, so a test installation can coexist with an
# existing one on the same host without touching it.
set -uo pipefail

INSTANCE="drevanta"
NGINX_PORT="80"
WEB_PORT="3100"
API_PORT="8080"
CALLBACK_PORT=""
NO_NGINX="0"
PKGDIR="$(cd "$(dirname "$0")" && pwd)"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --instance) INSTANCE="$2"; shift 2 ;;
    --nginx-port) NGINX_PORT="$2"; shift 2 ;;
    --web-port) WEB_PORT="$2"; shift 2 ;;
    --api-port) API_PORT="$2"; shift 2 ;;
    --public-callback-port) CALLBACK_PORT="$2"; shift 2 ;;
    --no-nginx) NO_NGINX="1"; shift ;;
    *) echo "unknown option: $1"; exit 2 ;;
  esac
done

if [[ "$INSTANCE" == "drevanta" ]]; then
  SVC="drevanta"; BASE="/opt/drevanta"; ETC="/etc/drevanta"; VAR="/var/lib/drevanta"; RUN="/run/drevanta"; LOG="/var/log/drevanta"; USR="drevanta"; GRP="drevanta"
else
  SVC="drevanta-$INSTANCE"; BASE="/opt/drevanta-$INSTANCE"; ETC="/etc/drevanta-$INSTANCE"; VAR="/var/lib/drevanta-$INSTANCE"; RUN="/run/drevanta-$INSTANCE"; LOG="/var/log/drevanta-$INSTANCE"; USR="drevanta-$INSTANCE"; GRP="$USR"
fi

FAILURES=0
step() { # step <name> <command...>
  local name="$1"; shift
  echo -n "[..] $name "
  if "$@" >/tmp/drevanta-install-last.log 2>&1; then echo "OK"; else
    echo "FAIL"; echo "     └─ $(tail -2 /tmp/drevanta-install-last.log | tr '\n' ' ')"
    FAILURES=$((FAILURES+1)); return 1
  fi
}
note() { echo "[i] $*"; }

[[ $EUID -eq 0 ]] || { echo "ERROR: run as root (sudo)"; exit 2; }
[[ -f "$PKGDIR/VERSION" ]] || { echo "ERROR: run from the extracted package (VERSION not found)"; exit 2; }
echo "== Drevanta installer — instance '$INSTANCE' — $(cat "$PKGDIR/VERSION") =="

# 1. Preflight -------------------------------------------------------------
step "preflight: systemd" bash -c 'pidof systemd >/dev/null || systemctl is-system-running >/dev/null 2>&1'
if ! id -u "$USR" >/dev/null 2>&1; then
  step "preflight: create system user $USR" useradd --system --shell /usr/sbin/nologin --home-dir "$VAR" "$USR"
fi
step "apt index refresh" bash -c 'apt-get update -qq'
for dep in curl python3 tar; do
  command -v "$dep" >/dev/null 2>&1 && { echo "[OK] dependency: $dep"; } || { step "dependency: $dep (apt)" bash -c "DEBIAN_FRONTEND=noninteractive apt-get install -y -qq $dep"; }
done
if [[ "$NO_NGINX" == "0" ]] && ! command -v nginx >/dev/null 2>&1; then
  step "dependency: nginx (apt, declared)" bash -c 'DEBIAN_FRONTEND=noninteractive apt-get install -y -qq nginx'
fi
node_major=$(node --version 2>/dev/null | sed 's/^v//' | cut -d. -f1 || echo 0)
if [[ "${node_major:-0}" -lt 20 ]]; then
  DEBIAN_FRONTEND=noninteractive apt-get install -y -qq nodejs >/dev/null 2>&1 || true
  node_major=$(node --version 2>/dev/null | sed 's/^v//' | cut -d. -f1 || echo 0)
fi
if [[ "${node_major:-0}" -lt 20 ]]; then
  step "dependency: Node.js >= 20 (NodeSource fallback)" bash -c 'curl -fsSL https://deb.nodesource.com/setup_20.x | bash - >/dev/null && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq nodejs'
fi
node_major=$(node --version 2>/dev/null | sed 's/^v//' | cut -d. -f1 || echo 0)
if [[ "${node_major:-0}" -ge 20 ]]; then
  echo "[OK] node $(node --version)"
else
  echo "[FAIL] Node.js >= 20 required, found: $(node --version 2>/dev/null || echo none)"; FAILURES=$((FAILURES+1))
fi
for p in "$WEB_PORT" "$API_PORT"; do
  if ss -ltn "sport = :$p" 2>/dev/null | grep -q LISTEN; then
    echo "[FAIL] port $p already in use — choose another with --web-port/--api-port"; FAILURES=$((FAILURES+1))
  else echo "[OK] port $p free"; fi
done

# 2. Layout ----------------------------------------------------------------
step "layout: directories" bash -c "install -d -m 0755 $BASE $ETC && install -d -m 0750 $VAR $RUN $LOG && chown $USR:$GRP $VAR $LOG"
step "layout: binaries" bash -c "install -m 0755 '$PKGDIR/bin/helper' '$PKGDIR/bin/panel' $BASE/bin/ 2>/dev/null || { mkdir -p $BASE/bin && install -m 0755 '$PKGDIR/bin/helper' '$PKGDIR/bin/panel' $BASE/bin/; }"
step "layout: web bundle" bash -c "rm -rf $BASE/web && mkdir -p $BASE/web && cp -r '$PKGDIR/web/.' $BASE/web/ && chown -R root:root $BASE/web"
step "layout: migrations" bash -c "mkdir -p $BASE && cp -r '$PKGDIR/migrations' $BASE/migrations"
step "layout: policy + bindings" bash -c "cp '$PKGDIR/infra/etc/helper-policy.json' $ETC/helper-policy.json && cp '$PKGDIR/infra/etc/helper-policy.json.bindings' $ETC/helper-policy.json.bindings 2>/dev/null; cp '$PKGDIR/infra/etc/site-bindings.json' $ETC/site-bindings.json; cp '$PKGDIR/infra/etc/database-bindings.json' $ETC/database-bindings.json; chmod 600 $ETC/*.json*"

# 3. Bootstrap — creates the installation row and issues the single-use
#    claim token (24h TTL). The token is captured into a root-only file and
#    shown to nobody else; the OWNER consumes it during onboarding.
if [[ ! -s "$ETC/claim-token" ]] && ! sudo -u "$USR" python3 -c "
import sqlite3,sys
db=sqlite3.connect('$VAR/panel.db')
try:
    r=db.execute('select installation_id from installation').fetchone()
    sys.exit(0 if r else 1)
except Exception: sys.exit(1)
"; then
  BOOT_OUT=$(sudo -u "$USR" "$BASE/bin/panel" bootstrap -db "$VAR/panel.db" -migrations "$BASE/migrations" 2>&1 || true)
  TOKEN=$(printf '%s' "$BOOT_OUT" | grep -A1 "DREVANTA CLAIM TOKEN" | tail -1 | tr -d '[:space:]')
  INSTALL_ID=$(printf '%s' "$BOOT_OUT" | grep -oE "inst_[a-f0-9]+" | head -1)
  [[ -z "$INSTALL_ID" ]] && INSTALL_ID=$(sudo -u "$USR" python3 -c "
import sqlite3
print(sqlite3.connect('$VAR/panel.db').execute('select installation_id from installation').fetchone()[0])" 2>/dev/null || true)
  if [[ -n "$TOKEN" && -n "$INSTALL_ID" ]]; then
    umask 077; printf '%s' "$TOKEN" > "$ETC/claim-token"; umask 022
    chown root:root "$ETC/claim-token"; chmod 600 "$ETC/claim-token"
    echo "[OK] bootstrap: installation $INSTALL_ID created; single-use claim token stored at $ETC/claim-token (root-only, 24h TTL)"
  else
    echo "[FAIL] bootstrap did not produce installation + claim token"; FAILURES=$((FAILURES+1))
  fi
fi

# 4. Panel (control plane) -------------------------------------------------
PANEL_UID=$(id -u "$USR")
cat > /etc/systemd/system/$SVC-panel.service <<UNIT
[Unit]
Description=Drevanta control plane ($INSTANCE)
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
User=$USR
Group=$GRP
ExecStart=$BASE/bin/panel -addr 127.0.0.1:$API_PORT -db $VAR/panel.db -migrations $BASE/migrations -helper-socket $RUN/helper.sock
Environment=DREVANTA_HOST_ID=*
Restart=on-failure
RestartSec=2
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=$VAR
ProtectKernelTunables=yes
ProtectControlGroups=yes
RestrictSUIDSGID=yes
LockPersonality=yes
[Install]
WantedBy=multi-user.target
UNIT
step "unit: $SVC-panel" bash -c 'systemctl daemon-reload && systemctl enable --now '"$SVC"'-panel'

# Wait for the API; capture installation id + claim state.
PANEL_OK=0
for i in $(seq 1 30); do
  st=$(curl -s -o /tmp/drevanta-install-status.json -w '%{http_code}' "http://127.0.0.1:$API_PORT/api/v1/install/status" || true)
  [[ "$st" == "200" ]] && { PANEL_OK=1; break; }
  sleep 1
done
if [[ "$PANEL_OK" != "1" ]]; then
  echo "[FAIL] control plane did not become healthy on 127.0.0.1:$API_PORT (journalctl -u $SVC-panel)"
  FAILURES=$((FAILURES+1))
else
  echo "[OK] control plane healthy — install status: $(cat /tmp/drevanta-install-status.json | head -c 200)"
  INSTALL_ID=$(python3 -c "import json;d=json.load(open('/tmp/drevanta-install-status.json'));print(d.get('installation_id',''))" 2>/dev/null)
fi

# 5. Helper (privileged) — binds to the installation id --------------------
if [[ -z "${INSTALL_ID:-}" ]]; then
  INSTALL_ID=$(sudo -u "$USR" python3 -c "
import sqlite3
print(sqlite3.connect('$VAR/panel.db').execute('select installation_id from installation').fetchone()[0])" 2>/dev/null || true)
fi
cat > /etc/systemd/system/$SVC-helper.service <<UNIT
[Unit]
Description=Drevanta privileged host helper ($INSTANCE)
Before=$SVC-panel.service
[Service]
Type=simple
ExecStartPre=/usr/bin/install -d -o root -g $GRP -m 0750 $RUN
ExecStart=$BASE/bin/helper -socket $RUN/helper.sock -policy $ETC/helper-policy.json -db $VAR/helper.db -migrations $BASE/migrations -installation \${DREVANTA_INSTALLATION_ID} -panel-uid $PANEL_UID -socket-group $GRP
Environment=DREVANTA_INSTALLATION_ID=${INSTALL_ID:-}
Environment=DREVANTA_BUILD_SCRATCH=$BASE/builds
Environment=DREVANTA_PANEL_UID=$PANEL_UID
Environment=DREVANTA_SOCKET_GROUP=$GRP
Restart=on-failure
RestartSec=2
[Install]
WantedBy=multi-user.target
UNIT
if [[ -n "${INSTALL_ID:-}" ]]; then
  step "unit: $SVC-helper (installation $INSTALL_ID)" bash -c 'systemctl daemon-reload && systemctl enable --now '"$SVC"'-helper'
else
  echo "[FAIL] helper not started: no installation id (panel output above)"; FAILURES=$((FAILURES+1))
fi

# 6. Console (SvelteKit adapter-node) --------------------------------------
cat > /etc/systemd/system/$SVC-web.service <<UNIT
[Unit]
Description=Drevanta console frontend ($INSTANCE)
After=$SVC-panel.service
[Service]
Type=simple
User=$USR
Group=$GRP
WorkingDirectory=$BASE/web
ExecStart=/usr/bin/node build/index.js
Environment=PORT=$WEB_PORT
Environment=HOST=127.0.0.1
Environment=GO_API_URL=http://127.0.0.1:$API_PORT
Environment=NODE_ENV=production
Restart=on-failure
RestartSec=2
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=$LOG
[Install]
WantedBy=multi-user.target
UNIT
step "unit: $SVC-web" bash -c 'systemctl daemon-reload && systemctl enable --now '"$SVC"'-web'
WEB_OK=0
for i in $(seq 1 20); do
  st=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$WEB_PORT/" || true)
  [[ "$st" =~ ^(200|302|307)$ ]] && { WEB_OK=1; break; }
  sleep 1
done
[[ "$WEB_OK" == "1" ]] && echo "[OK] console healthy on 127.0.0.1:$WEB_PORT" || { echo "[FAIL] console not healthy (journalctl -u $SVC-web)"; FAILURES=$((FAILURES+1)); }

# 7. nginx vhost (loopback-only by default) --------------------------------
if [[ "$NO_NGINX" == "0" ]]; then
  cat > /etc/nginx/sites-available/$SVC.conf <<NG
server {
    listen 127.0.0.1:$NGINX_PORT;
    server_name _;
    location / { proxy_pass http://127.0.0.1:$WEB_PORT; proxy_set_header Host \$host; proxy_set_header X-Forwarded-Proto \$scheme; }
    location /api/v1/ { proxy_pass http://127.0.0.1:$WEB_PORT; proxy_set_header Host \$host; }
}
NG
  step "nginx: vhost $SVC" bash -c "ln -sf /etc/nginx/sites-available/$SVC.conf /etc/nginx/sites-enabled/$SVC.conf && nginx -t && (systemctl reload nginx || systemctl restart nginx)"
  echo "[OK] console via nginx http://127.0.0.1:$NGINX_PORT (loopback only — use an SSH tunnel for remote access)"
fi
if [[ -n "$CALLBACK_PORT" ]]; then
  # Payment-provider callback exposure: ONLY the callback path is proxied
  # from the external interface; callbacks are treated as untrusted and are
  # never sufficient to settle an invoice (server verifies with the gateway).
  cat > /etc/nginx/sites-available/$SVC-callback.conf <<NG
server {
    listen 0.0.0.0:$CALLBACK_PORT;
    server_name _;
    location = /api/v1/payments/mpesa/callback { proxy_pass http://127.0.0.1:$WEB_PORT; proxy_set_header Host \$host; }
    location / { return 404; }
}
NG
  step "nginx: callback vhost (port $CALLBACK_PORT, callback path only)" bash -c "ln -sf /etc/nginx/sites-available/$SVC-callback.conf /etc/nginx/sites-enabled/$SVC-callback.conf && nginx -t && (systemctl reload nginx || systemctl restart nginx)"
fi

# 8. Summary ---------------------------------------------------------------
echo "==============================================================="
if [[ "$FAILURES" -gt 0 ]]; then
  echo "INSTALLATION FINISHED WITH $FAILURES FAILED STEP(S) — review the [FAIL] lines above. Do not treat this install as complete."
  exit 1
fi
echo "INSTALLATION OK (instance: $INSTANCE)"
echo "  Console (via nginx):  http://127.0.0.1:$NGINX_PORT/"
echo "  Console (direct):     http://127.0.0.1:$WEB_PORT/"
echo "  Control plane API:    127.0.0.1:$API_PORT (loopback only)"
echo "  Services:             systemctl status $SVC-helper $SVC-panel $SVC-web"
echo
echo "NEXT — first-run onboarding (performed by YOU, the owner):"
echo "  1. Open the console through an SSH tunnel from your machine:"
echo "       ssh -L $NGINX_PORT:127.0.0.1:$NGINX_PORT <user>@<this-host>"
echo "     then browse http://localhost:$NGINX_PORT/"
echo "  2. Retrieve the single-use claim token on this host (24h TTL):"
echo "       cat $ETC/claim-token && shred -u $ETC/claim-token   # shred after claim"
echo "  3. Follow the onboarding flow: claim (admin account) -> recovery"
echo "     codes -> access address -> sign-in -> dashboard."
echo "  The claim token is single-use and consumed by the claim step."

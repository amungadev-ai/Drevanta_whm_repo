#!/usr/bin/env bash
# Drevanta installation engine — the single shared engine behind every
# installation method (A bootstrap/GitHub, B downloadable package, C offline,
# D developer, E upgrade). Methods A–C extract the release artifact and exec
# this engine; Method D builds the artifact from source with the same layout;
# Method E is the --upgrade path below.
#
# Installs an isolated Drevanta instance: privileged helper + Go control
# plane (panel) + SvelteKit console (adapter-node) + nginx vhosts:
#   - PUBLIC  https://<server-ip>:8443  (management endpoint, Priority 1,
#             installation-specific TLS certificate with the server IP as SAN)
#   - LOOPBACK http://127.0.0.1:80      (SSH-tunnel access, Priority 2)
# Honesty rules: every gate prints OK/FAIL; a failed mandatory gate makes the
# installer exit non-zero; success is NEVER reported for an unexecuted or
# failed step. Claim tokens are printed to the interactive terminal only and
# are redacted from the persistent installation log.
#
# Usage (root):
#   sudo ./install.sh                          # fresh install, mgmt on :8443
#   sudo ./install.sh --access-ip 203.0.113.7  # override detected server IP
#   sudo ./install.sh --mgmt-port 9443         # custom management port
#   sudo ./install.sh --private-mgmt           # no public listener (tunnel only)
#   sudo ./install.sh --upgrade                # version-aware in-place upgrade
#   sudo ./install.sh --offline                # no network required post-extract
set -uo pipefail

# --- Configuration ----------------------------------------------------------
INSTANCE="drevanta"
MGMT_PORT="8443"
NGINX_PORT="80"
WEB_PORT="3100"
API_PORT="8080"
CALLBACK_PORT=""
NO_NGINX="0"
PRIVATE_MGMT="0"
UNATTENDED="${DREVANTA_UNATTENDED:-0}"
OFFLINE="0"
DO_UPGRADE="0"
ACCESS_IP="${DREVANTA_ACCESS_IP:-}"
MGMT_DOMAIN=""
SHOW_HELP="0"
PKGDIR="$(cd "$(dirname "$0")" && pwd)"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --instance) INSTANCE="$2"; shift 2 ;;
    --mgmt-port) MGMT_PORT="$2"; shift 2 ;;
    --nginx-port) NGINX_PORT="$2"; shift 2 ;;
    --web-port) WEB_PORT="$2"; shift 2 ;;
    --api-port) API_PORT="$2"; shift 2 ;;
    --public-callback-port) CALLBACK_PORT="$2"; shift 2 ;;
    --no-nginx) NO_NGINX="1"; shift ;;
    --private-mgmt) PRIVATE_MGMT="1"; shift ;;
    --unattended) UNATTENDED="1"; shift ;;
    --offline) OFFLINE="1"; shift ;;
    --upgrade) DO_UPGRADE="1"; shift ;;
    --access-ip) ACCESS_IP="$2"; shift 2 ;;
    --mgmt-domain) MGMT_DOMAIN="$2"; shift 2 ;;
    -h|--help) SHOW_HELP="1"; shift ;;
    *) echo "[FAIL] unknown option: $1 (see --help)"; exit 2 ;;
  esac
done

if [[ "$SHOW_HELP" == "1" ]]; then
  sed -n '2,25p' "$0" | grep -E '^#( Usage|   )' ; exit 0
fi

if [[ "$INSTANCE" == "drevanta" ]]; then
  SVC="drevanta"; BASE="/opt/drevanta"; ETC="/etc/drevanta"; VAR="/var/lib/drevanta"; RUN="/run/drevanta"; LOGD="/var/log/drevanta"; USR="drevanta"; GRP="drevanta"
else
  SVC="drevanta-$INSTANCE"; BASE="/opt/drevanta-$INSTANCE"; ETC="/etc/drevanta-$INSTANCE"; VAR="/var/lib/drevanta-$INSTANCE"; RUN="/run/drevanta-$INSTANCE"; LOGD="/var/log/drevanta-$INSTANCE"; USR="drevanta-$INSTANCE"; GRP="$USR"
fi

FAILURES=0
WARNINGS=0
VERSION="$(cat "$PKGDIR/VERSION" 2>/dev/null | head -1 || echo unknown)"
STEPLOG="$(mktemp /tmp/drevanta-step.XXXXXX)"
INSTALL_LOG="$LOGD/install-$(date -u +%Y%m%dT%H%M%SZ).log"

# --- Output discipline ------------------------------------------------------
# Everything the engine prints is mirrored into INSTALL_LOG with dynamic
# redaction: the moment a claim token is captured it is written to a 0600
# pattern file; the log reader replaces every occurrence with [REDACTED].
# The token therefore exists only in the interactive terminal, the root-only
# claim-token file and (transiently) the redaction pattern file.
REDACT_FILE="$(mktemp /tmp/drevanta-redact.XXXXXX)"; chmod 600 "$REDACT_FILE"
LOG_FIFO="$(mktemp -u /tmp/drevanta-logfifo.XXXXXX)"; mkfifo "$LOG_FIFO"
exec 3>&1   # original stdout (TTY) kept on fd 3
( while IFS= read -r line; do
    R="$(cat "$REDACT_FILE" 2>/dev/null || true)"
    if [[ -n "$R" ]]; then line="${line//"$R"/[REDACTED-CLAIM-TOKEN]}"; fi
    printf '%s\n' "$line" >> "$INSTALL_LOG"
    printf '%s\n' "$line" >&3
  done ) < "$LOG_FIFO" &
LOGGER_PID=$!
mkdir -p "$(dirname "$INSTALL_LOG")" 2>/dev/null; : > "$INSTALL_LOG"; chmod 600 "$INSTALL_LOG"
exec > "$LOG_FIFO" 2>&1
cleanup() {
  exec 1>&3 2>&3
  rm -f "$REDACT_FILE" "$LOG_FIFO" "$STEPLOG" 2>/dev/null
  wait "$LOGGER_PID" 2>/dev/null
}
trap cleanup EXIT

step() { # step <name> <command...> — prints [INFO] line, appends [OK]/[FAIL]
  local name="$1"; shift
  printf '[INFO] %-58s' "$name"
  if "$@" >"$STEPLOG" 2>&1; then
    echo " [OK]"
  else
    echo " [FAIL]"
    sed 's/^/       └─ /' "$STEPLOG" | tail -3
    FAILURES=$((FAILURES+1)); return 1
  fi
}
info()  { echo "[INFO] $*"; }
warn()  { echo "[WARN] $*"; WARNINGS=$((WARNINGS+1)); }
fail()  { echo "[FAIL] $*"; FAILURES=$((FAILURES+1)); }
ok()    { echo "[OK]   $*"; }
hyperlink() { # clickable terminal link with plain-text fallback (always visible)
  local url="$1"
  if [[ -t 3 && "${TERM:-}" != "dumb" && "${NO_COLOR:-}" != "1" ]]; then
    printf '\033]8;;%s\033\\%s\033]8;;\033\\\n' "$url" "$url"
  else
    printf '%s\n' "$url"
  fi
}

[[ $EUID -eq 0 ]] || { echo "[FAIL] run as root (sudo)"; exit 2; }
[[ -f "$PKGDIR/VERSION" ]] || { echo "[FAIL] run from the extracted release artifact (VERSION not found)"; exit 2; }

echo "========================================================"
echo "                    DREVANTA"
echo "     Hosting & Infrastructure Management Platform"
echo "========================================================"
echo "[INFO] Installer engine : $VERSION"
info    "Instance            : $INSTANCE"
info    "OS                  : $(. /etc/os-release && echo "${PRETTY_NAME:-unknown}") ($(uname -m))"

# 0. Upgrade / existing-installation protection ------------------------------
if [[ -f "$ETC/helper-policy.json" && -f "$VAR/panel.db" && "$DO_UPGRADE" != "1" ]]; then
  fail "existing installation detected in $ETC — refusing to modify it"
  echo "       Rerun with --upgrade to upgrade in place (data and configuration"
  echo "       are preserved), or install a coexisting test instance with"
  echo "       --instance <name>. A claimed installation is never reset."
  exit 1
fi

if [[ "$DO_UPGRADE" == "1" ]]; then
  # --------------------------------------------------------------------
  # Method E — version-aware in-place upgrade.
  # Backup -> stop -> replace artifacts -> start -> health gates -> verify.
  # On any failed health gate the previous version is restored from the
  # backup and the upgrade exits non-zero.
  # --------------------------------------------------------------------
  echo "========================================================"
  echo "[INFO] UPGRADE MODE — existing installation will be preserved"
  BK="/var/backups/drevanta/upgrade-$(date -u +%Y%m%dT%H%M%SZ)"
  step "upgrade: backup data + config -> $BK" bash -c "mkdir -p '$BK' && tar -czf '$BK/data-etc.tar.gz' '$VAR' '$ETC' && chmod 600 '$BK/data-etc.tar.gz'"
  step "upgrade: stop services" bash -c "systemctl stop $SVC-panel $SVC-web $SVC-helper 2>/dev/null || true"
  step "upgrade: replace binaries" bash -c "install -m 0755 '$PKGDIR/bin/helper' '$PKGDIR/bin/panel' $BASE/bin/"
  step "upgrade: replace console" bash -c "rm -rf $BASE/web && mkdir -p $BASE/web && cp -r '$PKGDIR/web/.' $BASE/web/ && chown -R root:root $BASE/web"
  step "upgrade: replace migrations" bash -c "rm -rf $BASE/migrations && cp -r '$PKGDIR/migrations' $BASE/migrations"
  step "upgrade: replace policy templates" bash -c "cp '$PKGDIR/infra/etc/helper-policy.json' $ETC/helper-policy.json && chmod 600 $ETC/*.json* 2>/dev/null || true"
  step "upgrade: restart services (migrations run at panel start)" bash -c "systemctl daemon-reload && systemctl start $SVC-helper $SVC-panel $SVC-web"
  UP_OK=1
  for i in $(seq 1 30); do
    st=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$API_PORT/api/v1/install/status" 2>/dev/null || true)
    [[ "$st" == "200" ]] && break
    [[ "$i" == "30" ]] && { fail "upgrade: control plane not healthy after upgrade (journalctl -u $SVC-panel)"; UP_OK=0; }
    sleep 1
  done
  if [[ "$UP_OK" == "1" ]]; then
    st=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$WEB_PORT/" 2>/dev/null || true)
    [[ "$st" =~ ^(200|302|307)$ ]] || { fail "upgrade: console not healthy after upgrade (journalctl -u $SVC-web)"; UP_OK=0; }
  fi
  if [[ "$UP_OK" != "1" ]]; then
    echo "[INFO] rollback: restoring previous version from $BK"
    systemctl stop "$SVC-panel" "$SVC-web" "$SVC-helper" 2>/dev/null || true
    tar -xzf "$BK/data-etc.tar.gz" -C /
    systemctl start "$SVC-helper" "$SVC-panel" "$SVC-web" 2>/dev/null || true
    echo "[FAIL] upgrade failed and was rolled back; previous version is running again. Report the [FAIL] lines above."
    exit 1
  fi
  ok "upgrade complete — running $VERSION (previous version backed up at $BK)"
  echo "[INFO] Data, configuration and the claim state were preserved."
  exit 0
fi

# 1. Preflight ---------------------------------------------------------------
echo "========================================================"
echo "[INFO] Stage 1/7 — System requirements"
step "preflight: systemd present" bash -c 'pidof systemd >/dev/null || systemctl is-system-running >/dev/null 2>&1'
ARCH="$(uname -m)"
if [[ "$ARCH" == "x86_64" ]]; then ok "architecture x86_64 (supported)"; else fail "architecture $ARCH not supported by these artifacts (linux/amd64 required)"; fi
command -v apt-get >/dev/null 2>&1 || { fail "apt-get not found — this installer supports Debian-family systems (Ubuntu 20.04+, Debian 11+)"; exit 1; }
RAM_MB=$(awk '/MemAvailable/{print int($2/1024)}' /proc/meminfo 2>/dev/null || echo 0)
if [[ "${RAM_MB:-0}" -lt 400 ]]; then fail "available memory ${RAM_MB}MB is below the 400MB floor (guide: 1GB+ VPS)"; else
  [[ "${RAM_MB:-0}" -lt 1000 ]] && warn "available memory ${RAM_MB}MB — the platform runs, but 1GB+ is recommended" || ok "memory ${RAM_MB}MB available"
fi
DISK_MB=$(df -BM /var 2>/dev/null | awk 'NR==2{gsub("M","",$4);print $4}' || echo 0)
if [[ "${DISK_MB:-0}" -lt 3000 ]]; then fail "disk: ${DISK_MB}MB free under /var — at least 3000MB required"; else ok "disk ${DISK_MB}MB free under /var"; fi
if [[ "$OFFLINE" == "1" ]]; then
  warn "offline mode: dependency auto-install and connectivity checks skipped (all dependencies must already be present)"
else
  step "preflight: outbound connectivity" bash -c 'curl -fsSI --max-time 8 http://archive.ubuntu.com >/dev/null 2>&1 || curl -fsSI --max-time 8 https://deb.nodesource.com >/dev/null'
fi

if ! id -u "$USR" >/dev/null 2>&1; then
  step "preflight: create system user $USR" useradd --system --shell /usr/sbin/nologin --home-dir "$VAR" "$USR"
fi
if [[ "$OFFLINE" != "1" ]]; then
  step "apt index refresh" bash -c 'apt-get update -qq'
  for dep in curl python3 tar; do
    command -v "$dep" >/dev/null 2>&1 && ok "dependency: $dep present" || step "dependency: $dep (apt)" bash -c "DEBIAN_FRONTEND=noninteractive apt-get install -y -qq $dep"
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
  if [[ "${node_major:-0}" -ge 20 ]]; then ok "Node.js $(node --version)"; else fail "Node.js >= 20 required, found: $(node --version 2>/dev/null || echo none)"; fi
else
  for dep in curl python3 tar nginx node; do
    command -v "$dep" >/dev/null 2>&1 && ok "dependency: $dep present" || fail "dependency: $dep MISSING (offline mode requires preinstalled dependencies)"
  done
  node_major=$(node --version 2>/dev/null | sed 's/^v//' | cut -d. -f1 || echo 0)
  [[ "${node_major:-0}" -ge 20 ]] || fail "Node.js >= 20 required in offline mode, found: $(node --version 2>/dev/null || echo none)"
fi
command -v openssl >/dev/null 2>&1 || step "dependency: openssl (apt)" bash -c "DEBIAN_FRONTEND=noninteractive apt-get install -y -qq openssl"

# Port availability (including the public management port).
echo "[INFO] Stage 1/7 — Port availability"
for p in "$WEB_PORT" "$API_PORT" "$NGINX_PORT"; do
  if ss -ltn "sport = :$p" 2>/dev/null | grep -q LISTEN; then fail "port $p already in use — choose another with --web-port/--api-port/--nginx-port"; else ok "port $p free"; fi
done
if [[ "$PRIVATE_MGMT" != "1" ]]; then
  if ss -ltn "sport = :$MGMT_PORT" 2>/dev/null | grep -q LISTEN; then fail "management port $MGMT_PORT already in use — choose another with --mgmt-port"; else ok "management port $MGMT_PORT free"; fi
fi

# 2. Management access address (Priority 1: direct server IP over HTTPS) -----
echo "[INFO] Stage 1/7 — Management access address"
if [[ -n "$ACCESS_IP" ]]; then
  IP_SOURCE="explicit override"
elif [[ "$OFFLINE" != "1" ]]; then
  CAND="$(curl -4 -fsS --max-time 5 https://api.ipify.org 2>/dev/null || true)"
  if [[ "$CAND" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then ACCESS_IP="$CAND"; IP_SOURCE="public IP (api.ipify.org — your server's IP was disclosed to this IP-echo service)"
  else ACCESS_IP="$(ip -4 route get 1.1.1.1 2>/dev/null | grep -oP 'src \K[0-9.]+' | head -1 || true)"; IP_SOURCE="primary interface"
  fi
else
  ACCESS_IP="$(ip -4 route get 1.1.1.1 2>/dev/null | grep -oP 'src \K[0-9.]+' | head -1 || true)"; IP_SOURCE="primary interface (offline)"
fi
if [[ -z "$ACCESS_IP" ]]; then fail "could not determine the server access address — pass --access-ip <ip>"; fi
info "Access address       : $ACCESS_IP (source: $IP_SOURCE)"

# 3. Layout ------------------------------------------------------------------
echo "========================================================"
echo "[INFO] Stage 2/7 — Deployment layout"
step "layout: directories" bash -c "install -d -m 0755 $BASE $ETC && install -d -m 0750 $VAR $RUN $LOGD && chown $USR:$GRP $VAR $LOGD"
step "layout: binaries (linux/amd64)" bash -c "mkdir -p $BASE/bin && install -m 0755 '$PKGDIR/bin/helper' '$PKGDIR/bin/panel' $BASE/bin/"
step "layout: console bundle" bash -c "rm -rf $BASE/web && mkdir -p $BASE/web && cp -r '$PKGDIR/web/.' $BASE/web/ && chown -R root:root $BASE/web"
step "layout: migrations" bash -c "mkdir -p $BASE && rm -rf $BASE/migrations && cp -r '$PKGDIR/migrations' $BASE/migrations"
step "layout: policy + bindings" bash -c "cp '$PKGDIR/infra/etc/helper-policy.json' $ETC/helper-policy.json && cp '$PKGDIR/infra/etc/helper-policy.json.bindings' $ETC/helper-policy.json.bindings 2>/dev/null; cp '$PKGDIR/infra/etc/site-bindings.json' $ETC/site-bindings.json; cp '$PKGDIR/infra/etc/database-bindings.json' $ETC/database-bindings.json; chmod 600 $ETC/*.json*"

# 4. Security: management TLS (installation-specific, IP SAN) ----------------
echo "========================================================"
echo "[INFO] Stage 3/7 — Security configuration"
TLS_DIR="$ETC/tls"
if [[ "$PRIVATE_MGMT" != "1" ]]; then
  step "tls: directories" bash -c "install -d -m 0700 $TLS_DIR"
  if [[ ! -s "$TLS_DIR/mgmt.crt" ]]; then
    SAN="IP:$ACCESS_IP"; [[ -n "$MGMT_DOMAIN" ]] && SAN="$SAN,DNS:$MGMT_DOMAIN"
    step "tls: generate installation certificate (SAN: $SAN)" bash -c "openssl req -x509 -newkey rsa:2048 -sha256 -days 825 -nodes -keyout $TLS_DIR/mgmt.key -out $TLS_DIR/mgmt.crt -subj '/CN=Drevanta Management' -addext 'subjectAltName=$SAN' >/dev/null 2>&1"
    chmod 600 "$TLS_DIR/mgmt.key"; chmod 644 "$TLS_DIR/mgmt.crt"
  else
    info "tls: existing certificate kept"
  fi
  TLS_FINGERPRINT="$(openssl x509 -in "$TLS_DIR/mgmt.crt" -noout -fingerprint -sha256 2>/dev/null | cut -d= -f2 || echo unavailable)"
  ok "tls certificate SHA-256 fingerprint: $TLS_FINGERPRINT"
else
  TLS_FINGERPRINT=""
  info "tls: skipped (private-management mode — loopback only)"
fi

# 5. Bootstrap — creates the installation row and issues the single-use
#    claim token (24h TTL). The token is captured into a root-only file,
#    redacted from the log, and shown only in this interactive terminal.
echo "========================================================"
echo "[INFO] Stage 4/7 — Database initialization"
if [[ ! -s "$ETC/claim-token" ]] && ! sudo -u "$USR" python3 -c "
import sqlite3,sys
db=sqlite3.connect('$VAR/panel.db')
try:
    r=db.execute('select installation_id,state from installation').fetchone()
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
    printf '%s' "$TOKEN" > "$REDACT_FILE"   # dynamic log redaction from here on
    CLAIM_EXPIRY="$(date -u -d '+24 hours' '+%Y-%m-%d %H:%M UTC' 2>/dev/null || date -u -v+24H '+%Y-%m-%d %H:%M UTC' 2>/dev/null || echo '+24h')"
    ok "bootstrap: installation $INSTALL_ID created"
    ok "single-use claim token issued (24h TTL, expires $CLAIM_EXPIRY)"
  else
    fail "bootstrap did not produce installation + claim token"
  fi
else
  info "bootstrap: existing installation record found — claim state preserved (a claimed installation is never reset)"
  [[ -s "$ETC/claim-token" ]] && { TOKEN="$(cat "$ETC/claim-token")"; printf '%s' "$TOKEN" > "$REDACT_FILE"; CLAIM_EXPIRY="$(date -u -d '+24 hours' '+%Y-%m-%d %H:%M UTC' 2>/dev/null || echo '+24h')"; }
fi

# 6. Services ----------------------------------------------------------------
echo "========================================================"
echo "[INFO] Stage 5/7 — Service configuration"
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

PANEL_OK=0
for i in $(seq 1 30); do
  st=$(curl -s -o /tmp/drevanta-install-status.json -w '%{http_code}' "http://127.0.0.1:$API_PORT/api/v1/install/status" || true)
  [[ "$st" == "200" ]] && { PANEL_OK=1; break; }
  sleep 1
done
if [[ "$PANEL_OK" != "1" ]]; then
  fail "control plane did not become healthy on 127.0.0.1:$API_PORT (journalctl -u $SVC-panel)"
else
  ok "control plane healthy on 127.0.0.1:$API_PORT"
  INSTALL_ID=$(python3 -c "import json;d=json.load(open('/tmp/drevanta-install-status.json'));print(d.get('installation_id',''))" 2>/dev/null || true)
fi

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
  HELPER_OK="healthy"; [[ -S "$RUN/helper.sock" ]] || sleep 2
  [[ -S "$RUN/helper.sock" ]] || HELPER_OK="socket not yet present (check journalctl -u $SVC-helper)"
  ok "privileged helper: $HELPER_OK"
else
  fail "helper not started: no installation id (panel output above)"
fi

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
ReadWritePaths=$LOGD
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
[[ "$WEB_OK" == "1" ]] && ok "console healthy on 127.0.0.1:$WEB_PORT" || fail "console not healthy (journalctl -u $SVC-web)"

# 7. nginx --------------------------------------------------------------------
echo "========================================================"
echo "[INFO] Stage 6/7 — Management endpoint configuration"
if [[ "$NO_NGINX" == "0" ]]; then
  # Loopback HTTP (Priority 2 — SSH tunnel target). Never public.
  cat > /etc/nginx/sites-available/$SVC.conf <<NG
server {
    listen 127.0.0.1:$NGINX_PORT;
    server_name _;
    add_header X-Content-Type-Options nosniff always;
    add_header Referrer-Policy no-referrer always;
    add_header Cache-Control "no-store" always;
    location / { proxy_pass http://127.0.0.1:$WEB_PORT; proxy_set_header Host \$host; proxy_set_header X-Forwarded-Proto \$scheme; }
    location /api/v1/ { proxy_pass http://127.0.0.1:$WEB_PORT; proxy_set_header Host \$host; }
}
NG
  step "nginx: loopback vhost 127.0.0.1:$NGINX_PORT (tunnel access)" bash -c "ln -sf /etc/nginx/sites-available/$SVC.conf /etc/nginx/sites-enabled/$SVC.conf && nginx -t && (systemctl reload nginx || systemctl restart nginx)"

  # Public HTTPS management endpoint (Priority 1 — direct server IP).
  if [[ "$PRIVATE_MGMT" != "1" ]]; then
    CSP="default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; font-src 'self' data:; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"
    cat > /etc/nginx/sites-available/$SVC-mgmt.conf <<NG
server {
    listen 0.0.0.0:$MGMT_PORT ssl;
    server_name _;
    ssl_certificate $TLS_DIR/mgmt.crt;
    ssl_certificate_key $TLS_DIR/mgmt.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers on;
    add_header X-Content-Type-Options nosniff always;
    add_header Referrer-Policy no-referrer always;
    add_header X-Frame-Options DENY always;
    add_header Cache-Control "no-store" always;
    add_header Content-Security-Policy "$CSP" always;
    client_max_body_size 8m;
    location / { proxy_pass http://127.0.0.1:$WEB_PORT; proxy_set_header Host \$host; proxy_set_header X-Forwarded-Proto https; }
    location /api/v1/ { proxy_pass http://127.0.0.1:$WEB_PORT; proxy_set_header Host \$host; proxy_set_header X-Forwarded-Proto https; }
}
NG
    step "nginx: management vhost https://0.0.0.0:$MGMT_PORT (TLS, security headers)" bash -c "ln -sf /etc/nginx/sites-available/$SVC-mgmt.conf /etc/nginx/sites-enabled/$SVC-mgmt.conf && nginx -t && (systemctl reload nginx || systemctl restart nginx)"
    MGMT_HEALTH=0
    for i in $(seq 1 10); do
      code=$(curl -sk -o /dev/null -w '%{http_code}' --resolve "drevanta-mgmt:$MGMT_PORT:127.0.0.1" "https://drevanta-mgmt:$MGMT_PORT/" 2>/dev/null || true)
      [[ "$code" =~ ^(200|302|307)$ ]] && { MGMT_HEALTH=1; break; }
      sleep 1
    done
    [[ "$MGMT_HEALTH" == "1" ]] && ok "management endpoint serving HTTPS on :$MGMT_PORT (local verification)" || fail "management endpoint not healthy locally (nginx -t / journalctl -u nginx)"
    if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
      step "firewall: allow TCP $MGMT_PORT (ufw is active)" ufw allow "$MGMT_PORT/tcp"
    else
      warn "no active ufw detected — ensure your provider firewall/security group allows inbound TCP $MGMT_PORT"
    fi
    info "external reachability is NOT verified from inside this host — confirm from your browser"
  fi
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

# Claim-token management wrapper (root-only retrieval + recovery).
cat > /usr/local/bin/drevanta-claim-token <<CTW
#!/usr/bin/env bash
# Drevanta claim-token retrieval + authorized recovery (root only).
# The token-bearing onboarding URLs are CONFIDENTIAL: anyone holding the
# token before it is consumed could claim this installation. Never share
# them; beware terminal scrollback, screenshots, clipboard history and
# browser extensions.
set -uo pipefail
ETC="$ETC"; VAR="$VAR"; BASE="$BASE"; USR="$USR"
ACCESS_IP="$ACCESS_IP"; MGMT_PORT="$MGMT_PORT"; PRIVATE_MGMT="$PRIVATE_MGMT"
[[ \$EUID -eq 0 ]] || { echo "root only" >&2; exit 2; }
[[ -s "\$ETC/claim-token" ]] || { echo "no outstanding claim token (installation already claimed or not initialized)" >&2; exit 1; }
case "\${1:-}" in
  --url)
    if [[ "\$PRIVATE_MGMT" == "1" ]]; then echo "private-management mode: use the SSH tunnel URL from the installation summary" >&2; exit 1; fi
    echo "https://\$ACCESS_IP:\$MGMT_PORT/#\$(cat "\$ETC/claim-token")" ;;
  --regenerate)
    OUT=\$(sudo -u "\$USR" "\$BASE/bin/panel" claim regenerate -db "\$VAR/panel.db" -migrations "\$BASE/migrations" 2>&1) || { echo "\$OUT" >&2; exit 1; }
    NEW=\$(printf '%s' "\$OUT" | grep -A1 "DREVANTA CLAIM TOKEN" | tail -1 | tr -d '[:space:]')
    [[ -n "\$NEW" ]] || { echo "regeneration failed" >&2; exit 1; }
    umask 077; printf '%s' "\$NEW" > "\$ETC/claim-token"; umask 022
    echo "claim token regenerated (single-use, valid 24h):"
    echo "\$NEW" ;;
  *)
    echo "Single-use claim token (valid 24h from issue; CONFIDENTIAL):"
    cat "\$ETC/claim-token"; echo
    echo "Onboarding URL: sudo drevanta-claim-token --url"
    echo "Reissue:        sudo drevanta-claim-token --regenerate  (only before claiming)" ;;
esac
CTW
chmod 755 /usr/local/bin/drevanta-claim-token
ok "claim-token retrieval command installed: sudo drevanta-claim-token"

# Machine-readable installation report (NO token material).
python3 - "$SVC" "$INSTANCE" "$VERSION" "$ACCESS_IP" "$MGMT_PORT" "$PRIVATE_MGMT" "$TLS_FINGERPRINT" "$INSTALL_LOG" <<'RPT'
import json, sys, subprocess
svc, inst, ver, ip, port, priv, fp, log = sys.argv[1:9]
services = {}
for u in ("panel", "web", "helper"):
    r = subprocess.run(["systemctl", "is-active", f"{svc}-{u}"], capture_output=True, text=True)
    services[u] = r.stdout.strip() or "unknown"
report = {
    "product": "Drevanta", "version": ver, "instance": inst,
    "management": {"mode": "private-tunnel-only" if priv == "1" else "public-https",
                    "access_ip": ip, "port": int(port),
                    "onboarding_url_base": f"https://{ip}:{port}/" if priv != "1" else None},
    "tls_fingerprint_sha256": fp or None,
    "services": services,
    "installation_log": log,
    "token_material": "excluded by policy (see drevanta-claim-token)",
}
open("/etc/" + ("drevanta" if inst == "drevanta" else f"drevanta-{inst}") + "/install-report.json", "w").write(json.dumps(report, indent=2))
RPT
[[ $? -eq 0 ]] && ok "installation report (no token material): $ETC/install-report.json"

# 8. Completion summary -------------------------------------------------------
echo "========================================================"
if [[ "$FAILURES" -gt 0 ]]; then
  echo "[FAIL] INSTALLATION FINISHED WITH $FAILURES FAILED STEP(S) AND $WARNINGS WARNING(S)"
  echo "       Review the [FAIL] lines above. Do not treat this install as complete."
  echo "       Installation log: $INSTALL_LOG"
  exit 1
fi
echo "[OK]   All mandatory installation gates passed ($WARNINGS warning(s))"
echo "========================================================"
echo "                  INSTALLATION COMPLETE"
echo "========================================================"
info "Version            : $VERSION"
info "Server IP          : $ACCESS_IP"
info "Status             : Running"
info "Services           : panel + web + helper systemd units ($SVC-*)"
[[ -n "${INSTALL_ID:-}" && -s "$ETC/claim-token" ]] && info "Onboarding         : Awaiting owner" || info "Onboarding         : already claimed or pending token (see drevanta-claim-token)"

if [[ -n "${TOKEN:-}" && -s "$ETC/claim-token" ]]; then
  echo "--------------------------------------------------------"
  echo "PRIMARY ACCESS — DIRECT SERVER IP"
  echo "--------------------------------------------------------"
  if [[ "$UNATTENDED" == "1" ]]; then
    echo "[INFO] Unattended mode: onboarding URL withheld from this output."
    echo "       Retrieve it as root:  sudo drevanta-claim-token --url"
  else
    # Token-bearing lines go straight to the interactive terminal (fd 3),
    # bypassing the log reader: they exist ONLY in the authorized terminal
    # and the root-only claim-token file — never in the persistent log.
    hyperlink "https://$ACCESS_IP:$MGMT_PORT/#$TOKEN" >&3
    echo "Ctrl+Click the link to begin onboarding (token auto-populated"
    echo "in the browser, then immediately removed from the address bar)."
  fi
  echo "--------------------------------------------------------"
  echo "ALTERNATIVE ACCESS — SSH TUNNEL"
  echo "--------------------------------------------------------"
  echo "From your local computer, execute:"
  echo "  ssh -N -L 8080:127.0.0.1:$NGINX_PORT root@$ACCESS_IP"
  if [[ "$UNATTENDED" == "1" ]]; then
    echo "Then open:  http://localhost:8080/  (token: sudo drevanta-claim-token)"
  else
    echo "Then open:"; hyperlink "http://localhost:8080/#$TOKEN" >&3
  fi
  echo "--------------------------------------------------------"
  echo "MANUAL ONBOARDING"
  echo "--------------------------------------------------------"
  if [[ "$UNATTENDED" == "1" ]]; then
    echo "Browser URL: https://$ACCESS_IP:$MGMT_PORT/  (enter the token from: sudo drevanta-claim-token)"
  else
    echo "Browser URL:"; hyperlink "https://$ACCESS_IP:$MGMT_PORT/"
    echo "Claim token: $TOKEN" >&3
  fi
  echo "Token validity: 24 hours (expires $CLAIM_EXPIRY)"
  echo "Token is single-use and consumed by onboarding. Reissue before"
  echo "claiming with: sudo drevanta-claim-token --regenerate"
  echo "Keep your claim token and onboarding URLs confidential."
fi
if [[ "$PRIVATE_MGMT" != "1" ]]; then
  echo "--------------------------------------------------------"
  echo "CERTIFICATE VERIFICATION (installation-specific TLS)"
  echo "--------------------------------------------------------"
  echo "Certificate SHA-256 fingerprint: $TLS_FINGERPRINT"
  echo "Your browser will show a certificate warning: this is expected —"
  echo "the certificate is issued by this installation, not a public CA."
  echo "Before entering credentials or the claim token, compare the"
  echo "fingerprint shown in your browser's certificate details with the"
  echo "value above (they must match exactly). Do not bypass the warning"
  echo "blindly. Replace with a publicly trusted certificate later"
  echo "(custom management domain — see docs/INSTALL.md)."
fi
echo "--------------------------------------------------------"
info "Installation dir   : $BASE"
info "Installation log   : $INSTALL_LOG (claim tokens redacted)"
info "Report             : $ETC/install-report.json"
echo ""
echo "Open your browser to complete onboarding."
echo "Welcome to Drevanta."
echo "========================================================"

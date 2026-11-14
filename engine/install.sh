#!/usr/bin/env bash
# Drevanta installation engine — the single shared engine behind every
# installation method (A bootstrap/GitHub, B downloadable package, C offline,
# D developer, E upgrade). Methods A–C extract the release artifact and run
# this engine; Method D builds the artifact from source with the same layout;
# Method E is the --upgrade path below.
#
# Installs an isolated Drevanta instance: privileged helper + Go control
# plane (panel) + SvelteKit console (adapter-node) + nginx vhosts:
#   - PUBLIC  https://<server-ip>:8443  (management endpoint, Priority 1,
#             installation-specific TLS certificate with the server IP as SAN)
#   - LOOPBACK http://127.0.0.1:80      (SSH-tunnel access, Priority 2; nginx
#             resolves the exact loopback listener in preference to any
#             wildcard :80 hosted-website server block, so hosted sites on
#             80/443 coexist without conflict)
# Honesty rules: every gate prints OK/FAIL; a failed mandatory gate makes the
# installer exit non-zero; success is NEVER reported for an unexecuted or
# failed step. INSTALLATION COMPLETE is printed only when every mandatory
# gate has passed. Claim tokens are printed to an authorized interactive
# terminal only (any non-interactive run suppresses token material
# automatically) and are redacted from the persistent installation log.
#
# Usage (root):
#   sudo ./install.sh                          # fresh install, mgmt on :8443
#   sudo ./install.sh --access-ip 203.0.113.7  # override detected server IP
#   sudo ./install.sh --mgmt-port 9443         # custom management port
#   sudo ./install.sh --private-mgmt           # no public listener (tunnel only)
#   sudo ./install.sh --upgrade                # version-aware in-place upgrade
#   sudo ./install.sh --upgrade --mgmt-port 9443      # change management port
#   sudo ./install.sh --upgrade --private-mgmt        # withdraw public listener
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
UNATTENDED_FLAG="${DREVANTA_UNATTENDED:-0}"
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
    --unattended) UNATTENDED_FLAG="1"; shift ;;
    --offline) OFFLINE="1"; shift ;;
    --upgrade) DO_UPGRADE="1"; shift ;;
    --access-ip) ACCESS_IP="$2"; shift 2 ;;
    --mgmt-domain) MGMT_DOMAIN="$2"; shift 2 ;;
    -h|--help) SHOW_HELP="1"; shift ;;
    *) echo "[FAIL] unknown option: $1 (see --help)"; exit 2 ;;
  esac
done

if [[ "$SHOW_HELP" == "1" ]]; then
  sed -n '2,30p' "$0" | grep -E '^#( Usage|   )' ; exit 0
fi

if [[ "$INSTANCE" == "drevanta" ]]; then
  SVC="drevanta"; BASE="/opt/drevanta"; ETC="/etc/drevanta"; VAR="/var/lib/drevanta"; RUN="/run/drevanta"; LOGD="/var/log/drevanta"; USR="drevanta"; GRP="drevanta"
else
  SVC="drevanta-$INSTANCE"; BASE="/opt/drevanta-$INSTANCE"; ETC="/etc/drevanta-$INSTANCE"; VAR="/var/lib/drevanta-$INSTANCE"; RUN="/run/drevanta-$INSTANCE"; LOGD="/var/log/drevanta-$INSTANCE"; USR="drevanta-$INSTANCE"; GRP="$USR"
fi

FAILURES=0
WARNINGS=0
VERSION="$(head -1 "$PKGDIR/VERSION" 2>/dev/null || echo unknown)"
STEPLOG="$(mktemp /tmp/drevanta-step.XXXXXX)"
INSTALL_LOG="$LOGD/install-$(date -u +%Y%m%dT%H%M%SZ).log"
STATUS_JSON="$(mktemp /tmp/drevanta-status.XXXXXX)"

# --- Output discipline ------------------------------------------------------
# Everything the engine prints is mirrored into INSTALL_LOG with dynamic
# redaction: the moment a claim token is captured it is written to a 0600
# pattern file; the log reader replaces every occurrence with [REDACTED].
# Token-bearing summary lines bypass the logger entirely (fd 3): they exist
# only in the authorized interactive terminal and the root-only claim-token
# file — never in the persistent log or any redirected output copy of the
# logger stream.
REDACT_FILE="$(mktemp /tmp/drevanta-redact.XXXXXX)"; chmod 600 "$REDACT_FILE"
LOG_FIFO="$(mktemp -u /tmp/drevanta-logfifo.XXXXXX)"; mkfifo "$LOG_FIFO"
exec 3>&1   # original stdout (TTY or redirected sink) kept on fd 3
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
  rm -f "$REDACT_FILE" "$LOG_FIFO" "$STEPLOG" "$STATUS_JSON" 2>/dev/null
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

# Non-interactive runs suppress token material automatically (findings #6):
# token-bearing URLs are printed only when fd 3 is an actual authorized
# interactive terminal. --unattended/DREVANTA_UNATTENDED forces it as well.
TTY_INTERACTIVE=0; [[ -t 3 ]] && TTY_INTERACTIVE=1
UNATTENDED="$UNATTENDED_FLAG"; [[ "$TTY_INTERACTIVE" != "1" ]] && UNATTENDED=1

echo "========================================================"
echo "                    DREVANTA"
echo "     Hosting & Infrastructure Management Platform"
echo "========================================================"
echo "[INFO] Installer engine : $VERSION"
info    "Instance            : $INSTANCE"
info    "OS                  : $(. /etc/os-release && echo "${PRETTY_NAME:-unknown}") ($(uname -m))"

# 0. Exclusive installation lock (findings #3) -------------------------------
mkdir -p /var/lock 2>/dev/null
LOCKFILE="/var/lock/${SVC}-install.lock"
exec 9>"$LOCKFILE"
if ! flock -n 9; then
  fail "another installation/upgrade of '$SVC' is already running (lock: $LOCKFILE)"
  exit 1
fi
chmod 600 "$LOCKFILE" 2>/dev/null
ok "installation lock acquired: $LOCKFILE"

# 1. Preflight validation — ALL mandatory gates run BEFORE any host ----------
#    modification (findings #3).
echo "========================================================"
echo "[INFO] Stage 1/7 — System requirements (validation only — no changes yet)"
ARCH="$(uname -m)"
if [[ "$ARCH" == "x86_64" ]]; then ok "architecture x86_64 (supported)"; else fail "architecture $ARCH not supported by these artifacts (linux/amd64 required)"; fi
OS_ID=""; OS_VER=""
. /etc/os-release 2>/dev/null && { OS_ID="${ID:-}"; OS_VER="${VERSION_ID:-}"; }
case "$OS_ID" in
  ubuntu) [[ "${OS_VER%%.*}" -ge 20 ]] && ok "OS: Ubuntu $OS_VER (supported)" || fail "OS: Ubuntu $OS_VER < 20.04 not supported" ;;
  debian) [[ "${OS_VER%%.*}" -ge 11 ]] && ok "OS: Debian $OS_VER (supported)" || fail "OS: Debian $OS_VER < 11 not supported" ;;
  *) fail "OS '${OS_ID:-unknown}' not supported — supported: Ubuntu 20.04+, Debian 11+ (Debian family)" ;;
esac
command -v apt-get >/dev/null 2>&1 || fail "apt-get not found — Debian-family systems only"
command -v systemctl >/dev/null 2>&1 || { pidof systemd >/dev/null 2>&1 || fail "systemd not operational on this host"; }
RAM_MB=$(awk '/MemAvailable/{print int($2/1024)}' /proc/meminfo 2>/dev/null || echo 0)
if [[ "${RAM_MB:-0}" -lt 400 ]]; then fail "available memory ${RAM_MB}MB is below the 400MB floor (guide: 1GB+ VPS)"; else
  [[ "${RAM_MB:-0}" -lt 1000 ]] && warn "available memory ${RAM_MB}MB — the platform runs, but 1GB+ is recommended" || ok "memory ${RAM_MB}MB available"
fi
DISK_MB=$(df -BM /var 2>/dev/null | awk 'NR==2{gsub("M","",$4);print $4}' || echo 0)
if [[ "${DISK_MB:-0}" -lt 3000 ]]; then fail "disk: ${DISK_MB}MB free under /var — at least 3000MB required"; else ok "disk ${DISK_MB}MB free under /var"; fi

# Input validation (ports, instance name, access IP, management domain).
port_ok() { [[ "$1" =~ ^[0-9]+$ && "$1" -ge 1 && "$1" -le 65535 ]]; }
for spec in "MGMT_PORT:$MGMT_PORT" "NGINX_PORT:$NGINX_PORT" "WEB_PORT:$WEB_PORT" "API_PORT:$API_PORT"; do
  pn="${spec%%:*}"; pv="${spec#*:}"
  port_ok "$pv" || fail "port ${pn}=${pv} is invalid (expected 1-65535)"
done
[[ "$CALLBACK_PORT" == "" ]] || port_ok "$CALLBACK_PORT" || fail "port CALLBACK_PORT=$CALLBACK_PORT is invalid"
if [[ "$MGMT_PORT" == "$NGINX_PORT" || "$MGMT_PORT" == "$WEB_PORT" || "$MGMT_PORT" == "$API_PORT" \
   || "$NGINX_PORT" == "$WEB_PORT" || "$NGINX_PORT" == "$API_PORT" || "$WEB_PORT" == "$API_PORT" ]]; then
  fail "duplicate port assignment among mgmt/nginx/web/api ports — each listener needs its own port"
fi
if [[ "$INSTANCE" != "drevanta" && ! "$INSTANCE" =~ ^[a-z0-9]([a-z0-9-]{0,18}[a-z0-9])?$ ]]; then
  fail "instance name '$INSTANCE' invalid (lowercase alphanumerics and '-', 1-20 chars)"
fi
if [[ -n "$ACCESS_IP" && ! "$ACCESS_IP" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
  fail "access IP '$ACCESS_IP' is not a valid IPv4 dotted quad"
fi
if [[ -n "$MGMT_DOMAIN" && ! "$MGMT_DOMAIN" =~ ^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}$ ]]; then
  fail "management hostname '$MGMT_DOMAIN' is not a valid DNS name"
fi

# Port availability (including the public management port).
for p in "$WEB_PORT" "$API_PORT" "$NGINX_PORT"; do
  if ss -ltn "sport = :$p" 2>/dev/null | grep -q LISTEN; then fail "port $p already in use — choose another with --web-port/--api-port/--nginx-port"; else ok "port $p free"; fi
done
if [[ "$PRIVATE_MGMT" != "1" ]]; then
  if ss -ltn "sport = :$MGMT_PORT" 2>/dev/null | grep -q LISTEN; then fail "management port $MGMT_PORT already in use — choose another with --mgmt-port"; else ok "management port $MGMT_PORT free"; fi
fi

# Management access address (detected; validation only — used later).
if [[ -n "$ACCESS_IP" ]]; then
  IP_SOURCE="explicit override"
elif [[ "$OFFLINE" != "1" ]]; then
  CAND="$(curl -4 -fsS --max-time 5 https://api.ipify.org 2>/dev/null || true)"
  if [[ "$CAND" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then ACCESS_IP="$CAND"; IP_SOURCE="public IP (api.ipify.org — your server's IP was disclosed to this IP-echo service)"
  else ACCESS_IP="$(ip -4 route get 1.1.1.1 2>/dev/null | grep -oP 'src \K[0-9.]+' | head -1 || true)"; IP_SOURCE="primary interface"
  fi
else
  ACCESS_IP="$(ip -4 route get 1.1.1.1 2>/dev/null | grep -oP 'src \K[0-9.]+' | head -1 || true)"; IP_SOURCE="primary interface (offline)"
fi
if [[ -z "$ACCESS_IP" || ! "$ACCESS_IP" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
  fail "could not determine a valid server access address — pass --access-ip <ip>"
else
  info "Access address       : $ACCESS_IP (source: $IP_SOURCE)"
fi

# Connectivity (mandatory unless --offline).
if [[ "$OFFLINE" == "1" ]]; then
  warn "offline mode: dependency auto-install and connectivity checks skipped (all dependencies must already be present)"
else
  step "preflight: outbound connectivity" bash -c 'curl -fsSI --max-time 8 http://archive.ubuntu.com >/dev/null 2>&1 || curl -fsSI --max-time 8 https://deb.nodesource.com >/dev/null'
fi

# Existing-installation state (fresh vs upgrade separation).
if [[ -f "$ETC/helper-policy.json" && -f "$VAR/panel.db" && "$DO_UPGRADE" != "1" ]]; then
  fail "existing installation detected in $ETC — refusing to modify it"
  echo "       Rerun with --upgrade to upgrade in place (data and configuration"
  echo "       are preserved), or install a coexisting test instance with"
  echo "       --instance <name>. A claimed installation is never reset."
  echo "[FAIL] mandatory preflight gate failed — the host has NOT been modified"
  exit 1
fi

# Artifact self-check: the engine verifies its own tree before installing
# (mandatory for Method B/C/D where no bootstrap performed the checks).
echo "[INFO] Stage 1/7 — Release artifact self-verification"
if [[ -f "$PKGDIR/SHA256SUMS" ]]; then
  step "self-check: internal SHA256SUMS" bash -c "cd '$PKGDIR' && sha256sum -c SHA256SUMS --quiet"
  python3 - "$PKGDIR" <<'PY' || { fail "self-check: SHA256SUMS coverage incomplete (unstated files present) — refusing to install"; echo "[FAIL] mandatory gate failed — the host has NOT been modified"; exit 1; }
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
  ok "self-check: complete file coverage confirmed"
else
  warn "self-check skipped: no SHA256SUMS beside the engine (developer tree) — bootstrap/direct-package verification must have covered this"
fi

# If any mandatory preflight gate has failed, stop before modifying anything.
if [[ "$FAILURES" -gt 0 ]]; then
  echo "[FAIL] $FAILURES mandatory preflight gate(s) failed — the host has NOT been modified. Fix the [FAIL] items above and rerun."
  exit 1
fi
ok "all preflight gates passed — proceeding with host modifications"

# 2. Upgrade path (Method E) -------------------------------------------------
render_loopback_vhost() { # writes the loopback (tunnel) server block
  cat > "/etc/nginx/sites-available/$SVC.conf" <<NG
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
}
render_mgmt_vhost() { # writes the public HTTPS management server block
  local csp="default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; font-src 'self' data:; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"
  cat > "/etc/nginx/sites-available/$SVC-mgmt.conf" <<NG
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
    add_header Content-Security-Policy "$csp" always;
    client_max_body_size 8m;
    location / { proxy_pass http://127.0.0.1:$WEB_PORT; proxy_set_header Host \$host; proxy_set_header X-Forwarded-Proto https; }
    location /api/v1/ { proxy_pass http://127.0.0.1:$WEB_PORT; proxy_set_header Host \$host; proxy_set_header X-Forwarded-Proto https; }
}
NG
}

TLS_DIR="$ETC/tls"
if [[ "$DO_UPGRADE" == "1" ]]; then
  # ------------------------------------------------------------------
  # Method E — version-aware in-place upgrade with verified rollback.
  # Order: compatibility check -> stop services -> CONSISTENT backup of
  # data, config, binaries, frontend, migrations, units, nginx state ->
  # staged replacement -> start -> health gates -> on any failure: full
  # restore + post-rollback health checks + honest reporting.
  # ------------------------------------------------------------------
  echo "========================================================"
  echo "[INFO] UPGRADE MODE — existing installation will be preserved"
  INSTALLED_VERSION="unknown"
  [[ -f "$BASE/VERSION" ]] && INSTALLED_VERSION="$(head -1 "$BASE/VERSION")"
  info "Installed version    : $INSTALLED_VERSION"
  info "Target version       : $VERSION"
  IMaj="${INSTALLED_VERSION%%.*}"; TMaj="${VERSION%%.*}"
  if [[ "$IMaj" != "$TMaj" ]]; then
    fail "major version change $IMaj -> $TMaj requires a documented migration path — refusing"
    echo "[FAIL] mandatory preflight gate failed — the host has NOT been modified"
    exit 1
  fi
  BK="/var/backups/drevanta/upgrade-$(date -u +%Y%m%dT%H%M%SZ)"
  step "upgrade: stop services (consistent backup follows)" bash -c "systemctl stop $SVC-panel $SVC-web $SVC-helper 2>/dev/null || true"
  step "upgrade: backup data + config -> $BK" bash -c "mkdir -p '$BK' && chmod 700 '$BK' && tar -czf '$BK/data-etc.tar.gz' '$VAR' '$ETC' && chmod 600 '$BK/data-etc.tar.gz'"
  step "upgrade: backup binaries" bash -c "tar -czf '$BK/bin.tar.gz' -C '$BASE' bin && chmod 600 '$BK/bin.tar.gz'"
  step "upgrade: backup frontend bundle" bash -c "tar -czf '$BK/web.tar.gz' -C '$BASE' web && chmod 600 '$BK/web.tar.gz'"
  step "upgrade: backup migrations" bash -c "tar -czf '$BK/migrations.tar.gz' -C '$BASE' migrations && chmod 600 '$BK/migrations.tar.gz'"
  step "upgrade: backup systemd + nginx state + VERSION" bash -c "tar -czf '$BK/system-nginx.tar.gz' /etc/systemd/system/$SVC-panel.service /etc/systemd/system/$SVC-web.service /etc/systemd/system/$SVC-helper.service /etc/nginx/sites-available/$SVC*.conf /etc/nginx/sites-enabled/$SVC*.conf '$BASE/VERSION' 2>/dev/null; chmod 600 '$BK/system-nginx.tar.gz'"
  step "upgrade: staged replacement — binaries" bash -c "mkdir -p '$BASE/.staging/bin' && install -m 0755 '$PKGDIR/bin/helper' '$PKGDIR/bin/panel' '$BASE/.staging/bin/' && rm -rf '$BASE/bin' && mv '$BASE/.staging/bin' '$BASE/bin' && rmdir '$BASE/.staging'"
  step "upgrade: staged replacement — console bundle" bash -c "mkdir -p '$BASE/.staging' && cp -r '$PKGDIR/web' '$BASE/.staging/web' && rm -rf '$BASE/web' && mv '$BASE/.staging/web' '$BASE/web' && chown -R root:root '$BASE/web' && rmdir '$BASE/.staging'"
  step "upgrade: staged replacement — migrations" bash -c "rm -rf '$BASE/migrations' && cp -r '$PKGDIR/migrations' '$BASE/migrations'"
  # Preserve customized policies: only install policy files that are missing.
  step "upgrade: policy preservation (existing files kept)" bash -c "for f in '$PKGDIR'/infra/etc/*.json*; do b=\$(basename \"\$f\"); [[ -f '$ETC/'\$b ]] || cp \"\$f\" '$ETC/'\$b; done; chmod 600 '$ETC'/*.json* 2>/dev/null || true"
  # Management access changes take effect on upgrade: re-render vhosts from
  # the requested ports/IP/mode; reissue the certificate when missing or
  # when its SAN no longer matches the access address.
  if [[ "$NO_NGINX" == "0" ]]; then
    step "upgrade: nginx configuration directories" bash -c "install -d -m 0755 /etc/nginx/sites-available /etc/nginx/sites-enabled"
    NEED_CERT=0
    if [[ "$PRIVATE_MGMT" != "1" ]]; then
      if [[ ! -s "$TLS_DIR/mgmt.crt" ]] || ! openssl x509 -in "$TLS_DIR/mgmt.crt" -noout -text 2>/dev/null | grep -q "IP Address:$ACCESS_IP"; then NEED_CERT=1; fi
      [[ -n "$MGMT_DOMAIN" && -s "$TLS_DIR/mgmt.crt" ]] && { openssl x509 -in "$TLS_DIR/mgmt.crt" -noout -text 2>/dev/null | grep -q "DNS:$MGMT_DOMAIN" || NEED_CERT=1; }
    fi
    if [[ "$PRIVATE_MGMT" != "1" ]]; then
      step "upgrade: tls directories" bash -c "install -d -m 0700 '$TLS_DIR'"
      if [[ "$NEED_CERT" == "1" ]]; then
        SAN="IP:$ACCESS_IP"; [[ -n "$MGMT_DOMAIN" ]] && SAN="$SAN,DNS:$MGMT_DOMAIN"
        step "upgrade: tls certificate (re)issued (SAN: $SAN)" bash -c "openssl req -x509 -newkey rsa:2048 -sha256 -days 825 -nodes -keyout '$TLS_DIR/mgmt.key' -out '$TLS_DIR/mgmt.crt' -subj '/CN=Drevanta Management' -addext 'subjectAltName=$SAN' >/dev/null 2>&1"
        chmod 600 "$TLS_DIR/mgmt.key"; chmod 644 "$TLS_DIR/mgmt.crt"
      else
        info "tls: existing certificate kept (SAN matches)"
      fi
      TLS_FINGERPRINT="$(openssl x509 -in "$TLS_DIR/mgmt.crt" -noout -fingerprint -sha256 2>/dev/null | cut -d= -f2 || echo unavailable)"
    else
      TLS_FINGERPRINT=""
      info "tls: private-management mode — public listener withdrawn"
      step "upgrade: remove public management listener" bash -c "rm -f /etc/nginx/sites-available/$SVC-mgmt.conf /etc/nginx/sites-enabled/$SVC-mgmt.conf"
    fi
    render_loopback_vhost
    step "upgrade: loopback vhost re-rendered (port $NGINX_PORT)" bash -c "ln -sf /etc/nginx/sites-available/$SVC.conf /etc/nginx/sites-enabled/$SVC.conf"
    if [[ "$PRIVATE_MGMT" != "1" ]]; then
      render_mgmt_vhost
      step "upgrade: management vhost re-rendered (port $MGMT_PORT)" bash -c "ln -sf /etc/nginx/sites-available/$SVC-mgmt.conf /etc/nginx/sites-enabled/$SVC-mgmt.conf"
    fi
    step "upgrade: nginx configuration valid + reloaded" bash -c "nginx -t && (systemctl reload nginx || systemctl restart nginx)"
    if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
      [[ "$PRIVATE_MGMT" != "1" ]] && step "upgrade: firewall allow TCP $MGMT_PORT (ufw active)" ufw allow "$MGMT_PORT/tcp"
    fi
  fi
  step "upgrade: restart services (migrations run at panel start)" bash -c "systemctl daemon-reload && systemctl start $SVC-helper $SVC-panel $SVC-web"
  UP_OK=1
  for i in $(seq 1 30); do
    st=$(curl -s -o "$STATUS_JSON" -w '%{http_code}' "http://127.0.0.1:$API_PORT/api/v1/install/status" 2>/dev/null || true)
    [[ "$st" == "200" ]] && break
    [[ "$i" == "30" ]] && { fail "upgrade: control plane not healthy after upgrade (journalctl -u $SVC-panel)"; UP_OK=0; }
    sleep 1
  done
  if [[ "$UP_OK" == "1" ]]; then
    st=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$WEB_PORT/" 2>/dev/null || true)
    [[ "$st" =~ ^(200|302|307)$ ]] || { fail "upgrade: console not healthy after upgrade (journalctl -u $SVC-web)"; UP_OK=0; }
    for u in panel web helper; do
      systemctl is-active --quiet "$SVC-$u" || { fail "upgrade: unit $SVC-$u not active after upgrade"; UP_OK=0; }
    done
    [[ "$PRIVATE_MGMT" != "1" && "$NO_NGINX" == "0" && -S "$RUN/helper.sock" ]] || { [[ "$PRIVATE_MGMT" != "1" ]] || true; }
    [[ -S "$RUN/helper.sock" ]] || { fail "upgrade: helper socket not present after upgrade"; UP_OK=0; }
  fi
  if [[ "$UP_OK" != "1" ]]; then
    echo "[INFO] rollback: restoring the previous version from $BK"
    systemctl stop "$SVC-panel" "$SVC-web" "$SVC-helper" 2>/dev/null || true
    RB_OK=1
    # Archive member layout: data-etc and system-nginx hold absolute
    # (slash-stripped) paths; bin/web/migrations hold $BASE-relative members.
    for pair in "data-etc:/" "system-nginx:/" "bin:$BASE" "web:$BASE" "migrations:$BASE"; do
      a="${pair%%:*}"; dest="${pair#*:}"
      tar -xzf "$BK/$a.tar.gz" -C "$dest" 2>/dev/null || { echo "[FAIL] rollback: archive $a.tar.gz could not be restored"; RB_OK=0; }
    done
    systemctl daemon-reload
    systemctl start "$SVC-helper" "$SVC-panel" "$SVC-web" 2>/dev/null || true
    RBH="not verified"
    if [[ "$RB_OK" == "1" ]]; then
      pst=$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "http://127.0.0.1:$API_PORT/api/v1/install/status" 2>/dev/null || true)
      wst=$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "http://127.0.0.1:$WEB_PORT/" 2>/dev/null || true)
      if [[ "$pst" == "200" && "$wst" =~ ^(200|302|307)$ && -S "$RUN/helper.sock" ]]; then
        RBH="previous version restored and healthy"
      else
        RBH="previous version restored but NOT healthy (panel:$pst web:$wst) — investigate journalctl -u $SVC-panel"
        RB_OK=0
      fi
    fi
    if [[ "$RB_OK" == "1" ]]; then
      echo "[OK]   rollback verified: $RBH (backups remain at $BK)"
    else
      echo "[FAIL] rollback INCOMPLETE: $RBH"
    fi
    echo "[FAIL] upgrade failed; see the [FAIL] lines above. Exit 1."
    exit 1
  fi
  printf '%s\nbuilt %s\n' "$VERSION" "$(date -u +%Y-%m-%dT%H:%MZ)" > "$BASE/VERSION"
  # Stale plaintext token handling applies on upgrades too: an installation
  # that is no longer pending can never consume its outstanding token.
  UP_STATE=$(sudo -u "$USR" python3 -c "
import sqlite3
try: print(sqlite3.connect('$VAR/panel.db').execute('select state from installation limit 1').fetchone()[0])
except Exception: print('unknown')" 2>/dev/null || echo unknown)
  if [[ "$UP_STATE" != "pending" && -s "$ETC/claim-token" ]]; then
    shred -u "$ETC/claim-token" 2>/dev/null || rm -f "$ETC/claim-token"
    ok "stale claim-token file removed (installation state: $UP_STATE — the outstanding token can no longer be claimed)"
  fi
  ok "upgrade complete — running $VERSION (previous version backed up at $BK)"
  echo "[INFO] Data, configuration, claim state and customized policies were preserved."
  echo "[INFO] External reachability is NOT verified from inside this host — confirm from your browser."
  exit 0
fi

# 3. Layout ------------------------------------------------------------------
echo "========================================================"
echo "[INFO] Stage 2/7 — Deployment layout"
if ! id -u "$USR" >/dev/null 2>&1; then
  step "layout: create system user $USR" useradd --system --shell /usr/sbin/nologin --home-dir "$VAR" "$USR"
fi
step "layout: directories" bash -c "install -d -m 0755 $BASE $ETC && install -d -m 0750 $VAR $RUN $LOGD && chown $USR:$GRP $VAR $LOGD"
step "layout: binaries (linux/amd64)" bash -c "mkdir -p $BASE/bin && install -m 0755 '$PKGDIR/bin/helper' '$PKGDIR/bin/panel' $BASE/bin/"
step "layout: console bundle" bash -c "rm -rf $BASE/web && mkdir -p $BASE/web && cp -r '$PKGDIR/web/.' $BASE/web/ && chown -R root:root $BASE/web"
step "layout: migrations" bash -c "mkdir -p $BASE && rm -rf $BASE/migrations && cp -r '$PKGDIR/migrations' $BASE/migrations"
step "layout: policy + bindings (fresh install)" bash -c "cp '$PKGDIR/infra/etc/helper-policy.json' $ETC/helper-policy.json && cp '$PKGDIR/infra/etc/helper-policy.json.bindings' $ETC/helper-policy.json.bindings 2>/dev/null; cp '$PKGDIR/infra/etc/site-bindings.json' $ETC/site-bindings.json; cp '$PKGDIR/infra/etc/database-bindings.json' $ETC/database-bindings.json; chmod 600 $ETC/*.json*"
step "layout: VERSION record" bash -c "printf '%s\nbuilt %s\n' '$VERSION' \"\$(date -u +%Y-%m-%dT%H:%MZ)\" > $BASE/VERSION"

# 4. Security: management TLS (installation-specific, IP SAN) ----------------
echo "========================================================"
echo "[INFO] Stage 3/7 — Security configuration"
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
  CERT_PUB="$(openssl x509 -in "$TLS_DIR/mgmt.crt" -noout -pubkey 2>/dev/null | sha256sum | awk '{print $1}')"
  KEY_PUB="$(openssl pkey -in "$TLS_DIR/mgmt.key" -pubout 2>/dev/null | sha256sum | awk '{print $1}')"
  if [[ -n "$CERT_PUB" && "$CERT_PUB" == "$KEY_PUB" ]]; then ok "tls: certificate and key match"; else fail "tls: certificate/key mismatch — regenerate before continuing"; fi
  openssl x509 -in "$TLS_DIR/mgmt.crt" -noout -text 2>/dev/null | grep -q "IP Address:$ACCESS_IP" \
    && ok "tls: SAN covers the access address ($ACCESS_IP)" \
    || { [[ -n "$MGMT_DOMAIN" ]] && openssl x509 -in "$TLS_DIR/mgmt.crt" -noout -text 2>/dev/null | grep -q "DNS:$MGMT_DOMAIN" && ok "tls: SAN covers the management domain ($MGMT_DOMAIN)" || fail "tls: SAN does not cover $ACCESS_IP — rerun with --upgrade to reissue"; }
else
  TLS_FINGERPRINT=""
  info "tls: skipped (private-management mode — loopback only)"
fi

# 5. Bootstrap — creates the installation row and issues the single-use
#    claim token (24h TTL). The token is captured into a root-only file,
#    redacted from the log, and shown only in an authorized interactive
#    terminal (non-interactive runs suppress it automatically).
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
    date -u '+%Y-%m-%dT%H:%M:%SZ' > "$ETC/claim-token-created"
    chmod 600 "$ETC/claim-token-created"
    printf '%s' "$TOKEN" > "$REDACT_FILE"   # dynamic log redaction from here on
    CLAIM_EXPIRY="$(date -u -d '+24 hours' '+%Y-%m-%d %H:%M UTC' 2>/dev/null || date -u -v+24H '+%Y-%m-%d %H:%M UTC' 2>/dev/null || echo '+24h')"
    ok "bootstrap: installation $INSTALL_ID created"
    ok "single-use claim token issued (24h TTL, expires $CLAIM_EXPIRY)"
  else
    fail "bootstrap did not produce installation + claim token"
  fi
else
  info "bootstrap: existing installation record found — claim state preserved (a claimed installation is never reset)"
  if [[ -s "$ETC/claim-token" ]]; then
    TOKEN="$(cat "$ETC/claim-token")"
    printf '%s' "$TOKEN" > "$REDACT_FILE"
    CREATED=""; [[ -f "$ETC/claim-token-created" ]] && CREATED="$(cat "$ETC/claim-token-created")"
    if [[ -n "$CREATED" ]]; then
      CLAIM_EXPIRY="$(date -u -d "$CREATED +24 hours" '+%Y-%m-%d %H:%M UTC' 2>/dev/null || echo 'created +24h')"
    else
      CLAIM_EXPIRY="unknown (record the issue time; default TTL 24h)"
    fi
    # Stale plaintext handling: if the installation is no longer pending, the
    # outstanding token cannot be claimed anymore — remove the plaintext.
    ST=$(sudo -u "$USR" python3 -c "
import sqlite3
try: print(sqlite3.connect('$VAR/panel.db').execute('select state from installation limit 1').fetchone()[0])
except Exception: print('unknown')" 2>/dev/null || echo unknown)
    if [[ "$ST" != "pending" ]]; then
      shred -u "$ETC/claim-token" 2>/dev/null || rm -f "$ETC/claim-token"
      TOKEN=""
      warn "stale claim-token file removed (installation state: $ST — the outstanding token can no longer be claimed)"
    else
      info "claim token still outstanding (state: pending; expires $CLAIM_EXPIRY)"
    fi
  fi
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
  st=$(curl -s -o "$STATUS_JSON" -w '%{http_code}' "http://127.0.0.1:$API_PORT/api/v1/install/status" || true)
  [[ "$st" == "200" ]] && { PANEL_OK=1; break; }
  sleep 1
done
if [[ "$PANEL_OK" != "1" ]]; then
  fail "control plane did not become healthy on 127.0.0.1:$API_PORT (journalctl -u $SVC-panel)"
else
  ok "control plane healthy on 127.0.0.1:$API_PORT (authorized /install/status response)"
  systemctl is-active --quiet "$SVC-panel" && ok "unit $SVC-panel active" || fail "unit $SVC-panel not active despite healthy HTTP response"
  INSTALL_ID=$(python3 -c "import json;d=json.load(open('$STATUS_JSON'));print(d.get('installation_id',''))" 2>/dev/null || true)
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
  HELPER_OK=0
  for i in $(seq 1 10); do
    [[ -S "$RUN/helper.sock" ]] && { HELPER_OK=1; break; }
    sleep 1
  done
  if [[ "$HELPER_OK" == "1" ]]; then
    ok "privileged helper socket present: $RUN/helper.sock"
    systemctl is-active --quiet "$SVC-helper" && ok "unit $SVC-helper active" || fail "unit $SVC-helper not active"
    SM=$(stat -c '%a %U %G' "$RUN/helper.sock" 2>/dev/null || echo "unavailable")
    [[ "$SM" == *"root $GRP"* ]] && ok "helper socket ownership/mode: $SM (panel-group scoped)" || warn "helper socket mode/owner: $SM (expected root:$GRP)"
  else
    fail "privileged helper socket MISSING after 10s (journalctl -u $SVC-helper) — installation is NOT complete"
  fi
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
systemctl is-active --quiet "$SVC-web" && ok "unit $SVC-web active" || fail "unit $SVC-web not active despite healthy HTTP response"

# Backend/helper isolation gate: control-plane API must bind loopback only.
if ss -ltn "sport = :$API_PORT" 2>/dev/null | grep -qE '(0\.0\.0\.0|\*):'$API_PORT; then
  fail "isolation: control-plane API is NOT loopback-only on :$API_PORT — refusing to continue"
else
  ok "isolation: control-plane API bound to 127.0.0.1:$API_PORT only"
fi

# 7. nginx --------------------------------------------------------------------
echo "========================================================"
echo "[INFO] Stage 6/7 — Management endpoint configuration"
if [[ "$NO_NGINX" == "0" ]]; then
  step "nginx: configuration directories" bash -c "install -d -m 0755 /etc/nginx/sites-available /etc/nginx/sites-enabled"
  # Loopback HTTP (Priority 2 — SSH tunnel target). Never public. nginx
  # resolves this exact-IP listener in preference to any wildcard :80
  # hosted-website server block — coexistence without conflict.
  render_loopback_vhost
  step "nginx: loopback vhost 127.0.0.1:$NGINX_PORT (tunnel access)" bash -c "ln -sf /etc/nginx/sites-available/$SVC.conf /etc/nginx/sites-enabled/$SVC.conf && nginx -t && (systemctl reload nginx || systemctl restart nginx)"

  # Public HTTPS management endpoint (Priority 1 — direct server IP).
  if [[ "$PRIVATE_MGMT" != "1" ]]; then
    render_mgmt_vhost
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
  else
    info "private-management mode: no public listener configured (SSH tunnel / VPN access only)"
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
EXP="unknown issue time"
[[ -f "\$ETC/claim-token-created" ]] && EXP="\$(date -u -d "\$(cat "\$ETC/claim-token-created") +24 hours" '+%Y-%m-%d %H:%M UTC' 2>/dev/null || echo 'issue time + 24h')"
case "\${1:-}" in
  --url)
    if [[ "\$PRIVATE_MGMT" == "1" ]]; then echo "private-management mode: no public onboarding URL exists — use the SSH tunnel URL from the installation summary" >&2; exit 1; fi
    echo "https://\$ACCESS_IP:\$MGMT_PORT/#\$(cat "\$ETC/claim-token")" ;;
  --regenerate)
    OUT=\$(sudo -u "\$USR" "\$BASE/bin/panel" claim regenerate -db "\$VAR/panel.db" -migrations "\$BASE/migrations" 2>&1) || { echo "\$OUT" >&2; exit 1; }
    NEW=\$(printf '%s' "\$OUT" | grep -A1 "DREVANTA CLAIM TOKEN" | tail -1 | tr -d '[:space:]')
    [[ -n "\$NEW" ]] || { echo "regeneration failed" >&2; exit 1; }
    umask 077; printf '%s' "\$NEW" > "\$ETC/claim-token"; umask 022
    date -u '+%Y-%m-%dT%H:%M:%SZ' > "\$ETC/claim-token-created"; chmod 600 "\$ETC/claim-token-created"
    echo "claim token regenerated (single-use, valid 24h):"
    echo "\$NEW" ;;
  *)
    echo "Single-use claim token (expires: \$EXP; CONFIDENTIAL):"
    cat "\$ETC/claim-token"; echo
    if [[ "\$PRIVATE_MGMT" != "1" ]]; then
      echo "Onboarding URL: sudo drevanta-claim-token --url"
    else
      echo "Management mode: PRIVATE — no public onboarding URL (SSH tunnel only)"
    fi
    echo "Reissue:        sudo drevanta-claim-token --regenerate  (only before claiming)" ;;
esac
CTW
chmod 755 /usr/local/bin/drevanta-claim-token
ok "claim-token retrieval command installed: sudo drevanta-claim-token"

# Machine-readable installation report (NO token material).
python3 - "$SVC" "$INSTANCE" "$VERSION" "$ACCESS_IP" "$MGMT_PORT" "$PRIVATE_MGMT" "$TLS_FINGERPRINT" "$INSTALL_LOG" "$ETC" <<'RPT'
import json, sys, subprocess
svc, inst, ver, ip, port, priv, fp, log, etc = sys.argv[1:10]
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
open(etc + "/install-report.json", "w").write(json.dumps(report, indent=2))
RPT
[[ $? -eq 0 ]] && ok "installation report (no token material): $ETC/install-report.json"

# Database + claim-state validity gates.
DB_STATE=$(sudo -u "$USR" python3 -c "
import sqlite3
try: print(sqlite3.connect('$VAR/panel.db').execute('select state from installation limit 1').fetchone()[0])
except Exception: print('missing')" 2>/dev/null || echo missing)
case "$DB_STATE" in
  pending) ok "database initialized; installation pending owner claim" ;;
  claimed|active) ok "database initialized; installation state: $DB_STATE" ;;
  *) fail "database initialization invalid (installation state: $DB_STATE)" ;;
esac

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
[[ -n "${INSTALL_ID:-}" && -s "$ETC/claim-token" ]] && info "Onboarding         : Awaiting owner" || info "Onboarding         : already claimed or no outstanding token (see drevanta-claim-token)"

# Certificate verification guidance PRECEDES any token-bearing URL.
if [[ "$PRIVATE_MGMT" != "1" ]]; then
  echo "--------------------------------------------------------"
  echo "CERTIFICATE VERIFICATION (installation-specific TLS)"
  echo "--------------------------------------------------------"
  echo "Certificate SHA-256 fingerprint: $TLS_FINGERPRINT"
  echo "Your browser will show a certificate warning: this is expected —"
  echo "the certificate is issued by this installation, not a public CA."
  echo "Before opening the onboarding link or entering ANY credential,"
  echo "compare the fingerprint in your browser's certificate details"
  echo "with the value above (they must match exactly). Do not bypass the"
  echo "warning blindly. Replace with a publicly trusted certificate later"
  echo "(custom management domain — see docs/MANAGEMENT-ACCESS-TLS.md)."
fi
if [[ -n "${TOKEN:-}" && -s "$ETC/claim-token" ]]; then
  if [[ "$PRIVATE_MGMT" == "1" ]]; then
    # Private-management mode: NO public onboarding URL exists (findings #5).
    echo "--------------------------------------------------------"
    echo "PRIVATE MANAGEMENT — SSH TUNNEL ONBOARDING"
    echo "--------------------------------------------------------"
    if [[ "$UNATTENDED" == "1" ]]; then
      echo "[INFO] Token material suppressed (non-interactive or unattended output)."
      echo "       Retrieve the token as root:  sudo drevanta-claim-token"
      echo "       Regenerate before claiming:  sudo drevanta-claim-token --regenerate"
    else
      echo "From your local computer, execute:"
      echo "  ssh -N -L 8080:127.0.0.1:$NGINX_PORT root@$ACCESS_IP"
      echo "Then open:"; hyperlink "http://localhost:8080/#$TOKEN" >&3
      echo "Claim token: $TOKEN" >&3
    fi
  else
    echo "--------------------------------------------------------"
    echo "PRIMARY ACCESS — DIRECT SERVER IP"
    echo "--------------------------------------------------------"
    if [[ "$UNATTENDED" == "1" ]]; then
      echo "[INFO] Token material suppressed (non-interactive or unattended output)."
      echo "       Retrieve the onboarding URL as root:  sudo drevanta-claim-token --url"
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
    echo "Browser URL:"; hyperlink "https://$ACCESS_IP:$MGMT_PORT/"
    if [[ "$UNATTENDED" != "1" ]]; then
      echo "Claim token: $TOKEN" >&3
    fi
  fi
  echo "Token validity: 24 hours (expires $CLAIM_EXPIRY)"
  echo "Token is single-use and consumed by onboarding. Reissue before"
  echo "claiming with: sudo drevanta-claim-token --regenerate"
  echo "Keep your claim token and onboarding URLs confidential."
fi
echo "--------------------------------------------------------"
info "Installation dir   : $BASE"
info "Installation log   : $INSTALL_LOG (claim tokens redacted)"
info "Report             : $ETC/install-report.json"
echo ""
echo "Open your browser to complete onboarding."
echo "Welcome to Drevanta."
echo "========================================================"

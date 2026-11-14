#!/usr/bin/env bash
# G40-S3 §4 regression guard: the deployed nginx configuration must route every
# browser-facing path (including /api/v1/) to the SvelteKit BFF, never straight
# to the Go control plane, and must not expose bearer tokens to the page.
# Runs ON the disposable VPS. Exit 1 on any violation.
set -u
FAIL=0

note() { printf '%s\n' "$*"; }
violation() { printf 'VIOLATION: %s\n' "$*"; FAIL=1; }

BFF_PORT="${BFF_PORT:-3100}"
GUARD_BASE="${GUARD_BASE:-https://127.0.0.1:444}"
GO_PORT="${GO_PORT:-8080}"

# 1. The loaded nginx configuration (actual running state, not the template)
nginx -T 2>/dev/null > /tmp/nginx-guard-loaded.conf || { echo "cannot read nginx -T"; exit 1; }

# 2. No server/location block may proxy browser traffic to the Go port.
#   proxy_pass targets that hit the Go port are allowed ONLY inside an
#    explicitly marked internal block (none expected) — any occurrence fails.
if grep -nE "proxy_pass[[:space:]]+https?://[^;]*:${GO_PORT}" /tmp/nginx-guard-loaded.conf | grep -v "^\s*#" ; then
  violation "nginx proxies traffic directly to the Go control plane port ${GO_PORT}"
fi

# 3. The Go control plane must not listen on a public interface.
if ss -tln | awk '{print $4}' | grep -qE "0\.0\.0\.0:${GO_PORT}|\*:${GO_PORT}|\[::\]:${GO_PORT}|\[::\]:\*"; then
  violation "Go control plane listens on a public interface"
fi

# 4. Through the public vhost, /api/v1/ must terminate at the BFF and reject
#    unauthenticated calls with a uniform 401 (BFF-enforced session gate).
CODE=$(curl -s -m 6 -o /dev/null -w '%{http_code}' "http://127.0.0.1${VHOST_PORT:+:$VHOST_PORT}/api/v1/workspaces" -H "Host: ${VHOST_HOST:-127.0.0.1}")
if [ "$CODE" != "401" ]; then
  violation "public /api/v1/workspaces returned ${CODE}, expected 401 (BFF session gate)"
fi

# 5. Login through the public vhost must not put a bearer token in the body.
BODY=$(curl -sk -m 6 -X POST "${GUARD_BASE}/api/v1/identity/login" \
  -H "Content-Type: application/json" -d '{"email":"guard@invalid.test","password":"wrong"}' 2>/dev/null || true)
if printf '%s' "$BODY" | grep -q "dvt_"; then
  violation "bearer token leaked in login response body"
fi

# 6. Session cookie flags through the vhost (only observable with a valid
#    login; when no account exists this check is skipped by the runner).
if [ -n "${GUARD_EMAIL:-}" ] && [ -n "${GUARD_PASS:-}" ]; then
  HDR=$(curl -sk -m 6 -D - -o /dev/null -X POST "${GUARD_BASE}/api/v1/identity/login" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"${GUARD_EMAIL}\",\"password\":\"${GUARD_PASS}\"}" | grep -i "^set-cookie" | head -1)
  for flag in HttpOnly Secure SameSite=Strict; do
    case "$HDR" in *"$flag"*) : ;; *) violation "session cookie missing ${flag}" ;; esac
  done
fi

rm -f /tmp/nginx-guard-loaded.conf
if [ "$FAIL" -eq 0 ]; then note "nginx-guard: PASS (no direct Go routing, no token exposure)"; fi
exit $FAIL

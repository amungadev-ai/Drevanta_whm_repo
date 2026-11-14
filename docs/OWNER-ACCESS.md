# Owner Access — Fresh-Installation Acceptance Environment

This document governs the owner-led fresh-installation acceptance test on the
authorized VPS (`95.211.164.33`, fresh Ubuntu 26.04.1 LTS). The installation,
claim, onboarding and every configuration step are **performed personally by
the owner** — no step is pre-executed, no account pre-created, no onboarding
completed on the owner's behalf, and M-Pesa production credentials remain
deactivated.

## The intended journey

```
SSH into VPS → authenticate → single installation command → automated
installation → completion summary → Ctrl+Click onboarding URL → claim token
auto-populated → manual onboarding → Drevanta dashboard → four-persona audit
```

## 1. Install (single command, Method A)

On the VPS as root, execute the pinned development installation command
delivered with this release (see the release notes / handover message for the
exact pinned URL — it contains the release tag and commit and is reproduced
verbatim there). During the private-repository phase the command prompts once
for a short-lived GitHub token with **Contents: Read**; input is hidden and
the token is used only through a root-only header file (never in URLs,
arguments, shell history or logs).

What happens automatically: platform validation → release retrieval →
SHA-256 verification (trusted manifest + internal checksums) → shared engine
→ dependencies (nginx, Node.js ≥ 20) → system user/directories → TLS
certificate for the server IP → systemd services → database initialization →
claim-token issuance → health verification → completion summary.

## 2. Management access (in priority order)

| Priority | Method | URL/command |
| --- | --- | --- |
| 1 | Direct server IP (primary) | `https://95.211.164.33:8443/` |
| 2 | SSH tunnel | `ssh -N -L 8080:127.0.0.1:80 root@95.211.164.33` then `http://localhost:8080/` |
| 3 | Custom management domain | optional, not required (see MANAGEMENT-ACCESS-TLS.md) |

**Certificate verification before first use (mandatory):** the browser will
warn that the certificate is not publicly trusted — expected. Compare the
certificate SHA-256 fingerprint in the browser's certificate details with the
`TLS fingerprint` line from the installation summary (and/or
`sudo openssl x509 -in /etc/drevanta/tls/mgmt.crt -noout -fingerprint
-sha256`). They must match exactly before any credentials or claim token are
entered. Do not bypass the warning blindly.

If the direct route is unreachable, check the provider firewall/security
group for TCP 8443 — the installer explicitly does not claim external
reachability (it can only verify local serving).

## 3. One-click onboarding (claim-token auto-population)

The completion summary prints the confidential onboarding URL:

```
https://95.211.164.33:8443/#<single-use-claim-token>
```

Ctrl+Click it (or tunnel + `http://localhost:8080/#<token>`). Behavior,
implemented and verified: the approved onboarding interface reads the token
from the URL fragment, validates the format, auto-populates the existing
claim-token field, and immediately removes the token from the address bar.
The token is never stored (no web storage), never auto-submitted, never sent
to the backend until the owner explicitly continues. Manual entry remains
available (the field accepts `clm_…` tokens or a full link).

## 4. Onboarding steps (owner-controlled)

1. Open the onboarding URL; review the auto-populated claim-token field.
2. Continue — the claim step consumes the token atomically (single-use,
   24 h TTL) and creates the initial administrator **you** choose. This
   account holds legitimate platform-wide administrative authority through
   the platform's authorization model; no bypasses exist.
3. Save the recovery codes offline (shown exactly once).
4. Access-address step (server IP preselected; custom domain optional).
5. Sign in; complete MFA/security setup.
6. Dashboard → configure initial workspaces → begin the four-persona audit
   per OWNER-ACCEPTANCE-TESTING.md.

## 5. Claim-token handling rules

- Retrieval: `sudo drevanta-claim-token` · `--url` · `--regenerate`
  (regeneration only before claiming; a claimed installation refuses).
- After a successful claim: `sudo shred -u /etc/drevanta/claim-token`.
- Token-bearing URLs are confidential — do not share them; be aware of
  terminal scrollback, screenshots, clipboard history and browser
  extensions. They are redacted from the persistent installation log and
  absent from diagnostics, reports and analytics.
- If the URL expired (24 h): regenerate, then rerun the command.

## 6. Recovery access

- Locked out of the browser route: the SSH-tunnel loopback vhost is always
  available (`ssh -N -L 8080:127.0.0.1:80 …`) and is untouched by
  public-vhost changes.
- Administrator access lost: recovery codes → password reset via the
  registered access address.
- Installation-level recovery: docs/UPGRADE-RECOVERY.md.

## Security notes

- The claim token is single-use, time-limited and consumed by the claim
  step; the DB stores only its SHA-256.
- No development authentication bypass exists in this build; the only
  bootstrap path is the normal claim flow.
- Go API and privileged helper remain loopback-only (never exposed through
  nginx); M-Pesa production credentials are not activated in this
  environment.

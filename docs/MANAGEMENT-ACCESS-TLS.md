# Management Access & TLS

Permanent management access priorities:

1. **Direct server IP over HTTPS** (default): `https://<server-ip>:8443/`
2. **SSH tunnel**: loopback HTTP through SSH
3. **Optional custom management domain**: `https://panel.example.com/`

A domain name is never required for installation, onboarding or platform
administration. Ports 80/443 stay free for hosted websites.

## Priority 1 — direct server IP (HTTPS :8443)

The engine detects the access address in this order: `--access-ip` override →
`DREVANTA_ACCESS_IP` → public IP via api.ipify.org (disclosed in the summary
when used; skip by setting an override) → primary interface IP. It then:

- issues the installation-specific TLS certificate with the server IP as
  subjectAltName (plus `--mgmt-domain` as a DNS SAN when given),
- configures the nginx management vhost on `0.0.0.0:8443` (TLSv1.2/1.3;
  security headers: `X-Content-Type-Options`, `Referrer-Policy: no-referrer`,
  `X-Frame-Options: DENY`, `Cache-Control: no-store`, self-origin CSP with
  `frame-ancestors 'none'`),
- checks the port is free before configuring it, verifies the endpoint
  locally, and — only if ufw is ALREADY active — adds the narrow
  `allow <port>/tcp` rule. It never enables a firewall or modifies SSH.
- displays the fingerprint and the correct primary browser URL in the
  completion summary.

**Certificate verification (mandatory before first use):** the certificate
is issued by this installation, not a public CA, so the browser warning is
expected. Compare the certificate SHA-256 fingerprint in the browser's
certificate details with the `TLS fingerprint` line printed by the installer
(and shown again by `openssl x509 -in /etc/drevanta/tls/mgmt.crt -noout
-fingerprint -sha256`). They must match exactly. Do not bypass the warning
blindly — this is exactly what a machine-in-the-middle would look like; the
fingerprint comparison is the defense.

**External reachability honesty:** the installer verifies the endpoint
locally only. It prints that external reachability is NOT verified from
inside the host — confirm from your own browser. Provider security groups
must allow inbound TCP 8443.

Replacing the certificate with a publicly trusted one later: replace
`/etc/drevanta/tls/mgmt.crt|mgmt.key` (or point the vhost at new paths) and
`nginx -t && systemctl reload nginx`. IP-address certificates from public
CAs are rare (short-lived, few CAs) — a DNS name + ACME is the normal path.

## Priority 2 — SSH tunnel

```bash
ssh -N -L 8080:127.0.0.1:80 root@<server-ip>
# then open http://localhost:8080/#<single-use-claim-token>   (onboarding)
# or    http://localhost:8080/                                (after claim)
```

The loopback vhost serves HTTP **only on 127.0.0.1** — it is never reachable
from a public interface, and no authentication, claim token or session
traffic is exposed over public HTTP. The claim token rides the URL fragment
(client-side only); it is stripped from the address bar immediately and
never transmitted automatically. Production cookie/origin security is not
weakened for tunnel use.

## Priority 3 — custom management domain

Status, honestly separated:

- **Implemented:** `--mgmt-domain <name>` adds the name as a DNS SAN on the
  management certificate; the onboarding "Connect my domain" flow records
  the access address on the installation (`/api/v1/install/address`).
- **Outstanding engineering (not claimed as done):** automated ACME
  issuance/renewal (HTTP-01/TLS-ALPN-01), DNS prerequisite validation, and
  automatic reverse-proxy reconfiguration for the domain. Until delivered,
  use a manually issued certificate (DNS-01 with any ACME client) and swap
  the files as above.
- **Non-blocking:** installation, onboarding and administration all work
  with the direct-IP method; a domain is optional forever.

## Private-management mode

`--private-mgmt` (or rerunning the engine with `--upgrade --private-mgmt`)
removes the public listener entirely: the management interface is reachable
only through SSH tunnels, approved private networks or VPN. The completion
summary then shows tunnel URLs only. Reversing: rerun with `--upgrade`
without the flag — the public vhost and certificate are recreated.

## Access-address transitions

Changing the access address (IP move, domain change, certificate swap) is
performed by re-running the engine with `--upgrade` plus the new
`--access-ip`/`--mgmt-domain` — data, claim state and accounts are
preserved. A failed certificate change never locks the administrator out:
the SSH-tunnel loopback vhost is untouched by public-vhost changes, and the
upgrade path backs up and can roll back (see UPGRADE-RECOVERY.md).

# M-Pesa Daraja — Sandbox Prerequisites & Test Plan

Owner directive: sandbox-first. Production credentials provided by the owner
are stored ONLY as root-only environment references on the authorized VPS
(`/etc/drevanta/env/mpesa.env`, chmod 600) and are NOT used for sandbox
testing, NOT committed to git, NOT logged, NOT included in evidence. No live
financial transactions are performed.

## What sandbox testing needs from you

Daraja sandbox uses its OWN app credentials (separate from production):

1. Sign up / sign in at the official Daraja developer portal:
   <https://developer.safaricom.co.ke> (Safaricom account required).
2. Create an **App** with the **Mpesa Sandbox** product (Lipa na M-Pesa
   Online). The portal shows the app's **Consumer Key** and **Consumer
   Secret**.
3. Note the sandbox **Test Credentials** page values: shortcode
   (commonly 174379), and the sandbox **Passkey** for Lipa na M-Pesa Online.
4. Deliver those three values by secure channel (or paste them to be stored
   as root-only env refs exactly like the production ones). Do NOT send
   production credentials for sandbox work.

## Callback reachability

Daraja must be able to REACH the callback URL — an SSH tunnel cannot work for
callbacks. The authorized development VPS has a public IP; the installer's
`--public-callback-port` option exposes ONLY the callback path
(`/api/v1/payments/mpesa/callback`) on that port via nginx. Because Drevanta
never settles from a callback alone (every claimed outcome is verified with
the STK status query), exposure risk is bounded: the endpoint treats all
payloads as untrusted and records them for audit only. Provide the callback
URL as `http://<VPS-IP>:<callback-port>/api/v1/payments/mpesa/callback` in
the gateway configuration. If Safaricom's sandbox rejects non-HTTPS callback
URLs, the fallback is a delegated public domain behind ACME (DEC-024) —
documented in docs/INTEGRATION-READINESS.md.

## Amount guidance for tests

Per your instruction, test packages/amounts stay in the 1–5 KES range
(configuration bounds `min_amount_cents=100`, `max_amount_cents=500`).

## Verification procedure (after credentials are configured)

1. `GET /api/v1/admin/payments/mpesa/health` → `reachable` (OAuth against
   sandbox).
2. Configure the gateway via
   `PUT /api/v1/admin/payments/mpesa/config` (sandbox environment, refs only).
3. Initiate a test payment on a demo invoice
   (`POST /api/v1/invoices/{id}/mpesa`) → enter the sandbox PIN on the
   prompted phone (the sandbox simulates prompts; see portal test guides).
4. Callback arrives → recorded → engine verifies via STK query → payment
   `succeeded` with receipt; duplicate callbacks are idempotent; invoice
   settlement only after verification.
5. Cancel/timeout paths: cancel the prompt (1032 → cancelled), let it time
   out (1037 → uncertain → reconciled by
   `POST /api/v1/admin/payments/mpesa/reconcile`).

## Capability boundaries (documented, not assumed)

- STK Push (Lipa na M-Pesa Online): supported in sandbox for this app type.
- Transaction Status / STK query: supported; used for verification.
- C2B Validation & Confirmation URLs: require registering URLs on the
  app/product and, in production, additional Safaricom product approval —
  treated as EXTERNAL-ACTIVATION, not implemented as assumed-available.
- Reversals/refunds via Daraja require separate product permissions —
  the engine models outcomes and keeps audit trails; refund workflows are
  gated on that activation (no simulated success).

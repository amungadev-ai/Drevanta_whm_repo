# Integration Readiness — External Providers & Services

Status vocabulary: **OPERATIONAL-VERIFIED**, **OPERATIONAL**, **PARTIAL**
(adapter/contract present, integration incomplete), **PLANNED** (designed,
not implemented), **EXTERNAL-ACTIVATION** (needs Safaricom/owner-side product
permissions). This document is NOT a claim of completed integrations. All
approved integration requirements remain in the authoritative project scope.

## Payment gateways

| Provider | Intended functionality | Architecture / adapter contract | Status | Credentials | External infra | Security | Docs | Sandbox | Activation | Verification | Remaining work |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| M-Pesa Daraja | STK push invoice payments, verification, reconciliation | `internal/domains/mpesa.go` engine + `store/mpesa_*` persistence + API endpoints (`/admin/payments/mpesa/*`, `/invoices/{id}/mpesa`, public `/payments/mpesa/callback`); verify-then-settle | **PARTIAL→sandbox-ready** (engine unit-tested vs mock Daraja; sandbox HTTP round-trip pending owner credentials) | env:/file: refs (root-only) | reachable callback URL (public IP port or ACME domain) | OAuth cache; callbacks untrusted; settle only after STK query; idempotent events; no secrets in logs/audit | developer.safaricom.co.ke | sandbox app creds | sandbox: config only; production: separate owner approval | docs/MPESA-SANDBOX.md | sandbox live round-trip; C2B register-URL flows (EXTERNAL-ACTIVATION); reversal APIs (EXTERNAL-ACTIVATION); console configuration UI |
| Stripe | cards/wallets, subscriptions | payment adapter contract (provider-scoped, DEC-026) | **PLANNED** | test keys via env refs | webhook endpoint (HTTPS) | webhook HMAC verify + replay window | stripe.com/docs | test-mode keys | DEC-025 choice | sandbox charge+refund+webhook | adapter implementation |
| PayPal | wallet checkout | adapter contract | **PLANNED** | sandbox client id/secret | webhook endpoint | webhook certificate/crc validation | developer.paypal.com | sandbox | DEC-025 | sandbox order flow | adapter implementation |
| Mollie | EU local methods | adapter contract | **PLANNED** | test API key | webhook endpoint | webhook idempotency | docs.mollie.com | test key | DEC-025 | sandbox payment | adapter implementation |
| Other regional | per-market methods | adapter contract | **PLANNED** | per-provider | per-provider | per-provider | per-provider | — | DEC-025 | per-provider | scoping |
| Manual / bank transfer | off-platform payment marking + reconciliation | admin-marked payments with audit | **PARTIAL** (commercial core supports manual states) | none | none | admin-only marking; audit trail | — | — | config | owner test | console UI |

## Non-payment integrations

| Integration | Intended functionality | Status | Credentials / infra | Notes |
| --- | --- | --- | --- | --- |
| Domain registrars | automated registration/transfer | **PLANNED** | registrar API keys | manual DNS entry supported today |
| External DNS providers | zone sync to hosted DNS | **PLANNED** | provider tokens | authoritative nsd supported (dns.apply) |
| SMTP relay | outbound mail deliverability | **PARTIAL** (self-hosted postfix verified; relay config is postfix-native) | relay creds (refs) | sandbox tests per docs/TEST-RESULTS.md |
| External identity / SSO | org SSO login | **PLANNED** | IdP metadata | INT-01 surface |
| Source repo providers | developer source connections | **PARTIAL** (git fetch over https/ssh in build engine; provider apps not integrated) | PAT/deploy keys (refs) | FP-P4-026 webhooks engine live |
| Object storage / remote backup | off-host encrypted backups | **PARTIAL** (backup engine live; independent off-host restore awaits second VPS — BLK-010/011) | storage creds (refs) | §8.1 |
| ACME certificate providers | public TLS issuance | **PARTIAL** (tls.apply; issuance behind DEC-024 public domain) | none |—|
| Monitoring services | external monitoring/uptime | **PLANNED** | provider keys | internal monitoring surfaces PARTIAL |
| Notification providers | email/SMS/push fanout | **PARTIAL** (in-app + mail path; external providers PLANNED) | provider keys | FP-P2 notify |

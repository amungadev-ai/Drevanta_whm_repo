# DEC-025 — Commercial Gateway Integration Path (formerly numbered DEC-020; DEC-020 remains the frontend framework decision) (test credentials until production authorization)

Status: DOCUMENTED · Date: 2026-11-14 · Relates to: BLK-001, BIZ-01/02/03

## Current state (unchanged by this document)

The billing module ships with the `PaymentGateway` adapter seam
(`internal/domains/commercial.go`): the platform charges through the seam; tests
exercise a fixture gateway; production credentials were never requested or
configured.

## The integration path (what happens the moment the owner provides TEST credentials)

1. **Adapter implementation** (one file, one wave): a `StripeTestGateway`
   (or provider of the owner's choice) implementing the existing single-method
   seam (`Charge(ctx, invoiceNumber, amountCents, currency) → gatewayRef`) using
   the provider's TEST-mode API keys. No schema or service changes — the seam was
   designed for exactly this.
2. **Configuration** (fail-closed): provider + keys supplied ONLY as environment
   references to the panel unit (`DREVANTA_GATEWAY=stripe-test`,
   `DREVANTA_GATEWAY_KEY_REF=file:/etc/drevanta/secrets/gateway-test.key`).
   Without configuration the billing module uses the manual/external-payment mode
   it uses today; nothing silently switches to charging real money.
3. **Test acceptance set** (run against the provider's test mode):
   successful charge, declined card, amount/currency round-trip on invoice lines,
   idempotent retry (same invoice number → same gatewayRef), audit rows written
   with the gateway reference. The existing billing tests already pin the
   platform-side behavior; the adapter adds provider-side integration tests.
4. **Production switch (explicit owner gate)**: production keys are a separate,
   owner-issued configuration change (DEC-025 sign-off), never a test→prod flag.

## What the platform needs from the owner (when ready)

| Item | Notes |
| --- | --- |
| Provider choice + TEST-mode API key | scoped to test mode; stored root-owned 0600 via the secret workflow |
| Webhook endpoint preference | if the provider's flow needs async confirmation, a public URL on the panel host (TLS decision DEC-019 applies) |
| Currency per tenant | today the ledger/invoices are single-currency per workspace (`currency` column); multi-currency is a schema decision if wanted |

## Explicitly NOT done until production authorization

- No production keys, no live charges, no real customer data with a provider.
- No provider SDK dependency added to the tree before the owner names a provider
  (keeps the adapter seam dependency-free).

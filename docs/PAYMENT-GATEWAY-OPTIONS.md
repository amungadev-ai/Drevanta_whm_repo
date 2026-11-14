# Payment Gateway Options Register (DEC-025)

Owner-facing option set produced by PresentGateways() (internal/domains/payments.go).
Sandbox-compatible engineering is complete (charges, idempotency, webhook
HMAC+replay). NO production credentials, NO live transactions.

| Gateway | Markets | Currencies | Capabilities | Sandbox | Notes |
| --- | --- | --- | --- | --- | --- |
| Stripe | global (46+ countries) | USD, EUR, GBP, CHF, AUD, CAD, +135 | cards, SEPA, webhooks, refunds, subscriptions | api.stripe.com (test-mode keys) | test keys never charge |
| Mollie | EU/EEA | EUR, USD, GBP, +10 | iDEAL, SEPA, cards, webhooks, refunds | api.mollie.com/v2 (test key) | strong EU local methods |
| PayPal REST | global | USD, EUR, GBP, +20 | wallet, cards, webhooks, refunds | api-m.sandbox.paypal.com | sandbox mirrors live flows |

Integration requirements (any gateway): webhook signing secret (HMAC),
TEST API key in /etc/drevanta env refs (never in git), public webhook URL
behind the approved TLS termination, replay window + event-id dedup already
implemented adapter-side. Production activation = DEC-025 sign-off.

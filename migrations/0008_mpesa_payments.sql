-- M-Pesa Daraja payment integration (owner directive: sandbox-first;
-- production environment requires explicit activation).
-- Payment attempts are correlated to invoices; callbacks are UNTRUSTED and
-- stored for audit — settlement happens only after gateway verification.
CREATE TABLE IF NOT EXISTS mpesa_payment (
    payment_id          TEXT PRIMARY KEY,
    invoice_id          TEXT NOT NULL,
    scope_id            TEXT NOT NULL,
    phone               TEXT NOT NULL,
    amount_cents        INTEGER NOT NULL,
    currency            TEXT NOT NULL DEFAULT 'KES',
    state               TEXT NOT NULL,           -- initiated|pending|succeeded|failed|cancelled|uncertain
    environment         TEXT NOT NULL,           -- sandbox|production
    merchant_request_id TEXT,
    checkout_request_id TEXT UNIQUE,
    receipt_number      TEXT,
    result_desc         TEXT,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_mpesa_payment_invoice ON mpesa_payment(invoice_id);
CREATE INDEX IF NOT EXISTS idx_mpesa_payment_state ON mpesa_payment(state);

CREATE TABLE IF NOT EXISTS mpesa_callback_event (
    event_id            TEXT PRIMARY KEY,
    checkout_request_id TEXT NOT NULL,
    payload_digest      TEXT NOT NULL,
    result_code         INTEGER,
    received_at         TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_mpesa_callback_checkout ON mpesa_callback_event(checkout_request_id);

CREATE TABLE IF NOT EXISTS mpesa_gateway_config (
    singleton           INTEGER PRIMARY KEY CHECK (singleton = 1),
    config_json         TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);

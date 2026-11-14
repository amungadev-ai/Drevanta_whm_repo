package store

// SQLite-backed M-Pesa gateway store (configuration references, payment
// attempts, callback events). Secret VALUES never live here — only refs.

import (
	"context"
	"encoding/json"
	"time"
)

type MpesaStore struct{ *DB }

var _ MpesaGateway = (*MpesaStore)(nil)

func (s *MpesaStore) LoadConfig(ctx context.Context) (*MpesaConfig, error) {
	var raw string
	err := s.QueryRowContext(ctx, `SELECT config_json FROM mpesa_gateway_config WHERE singleton = 1`).Scan(&raw)
	if err != nil {
		return nil, err // sql.ErrNoRows => unconfigured
	}
	var c MpesaConfig
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *MpesaStore) SaveConfig(ctx context.Context, c MpesaConfig) error {
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	_, err = s.ExecContext(ctx, `INSERT INTO mpesa_gateway_config(singleton, config_json, updated_at)
		VALUES(1, ?, ?) ON CONFLICT(singleton) DO UPDATE SET config_json = excluded.config_json, updated_at = excluded.updated_at`,
		string(raw), Now())
	return err
}

func (s *MpesaStore) CreatePayment(ctx context.Context, paymentID, invoiceID, scopeID, phone string, amountCents int64, env, state string, now time.Time) error {
	_, err := s.ExecContext(ctx, `INSERT INTO mpesa_payment
		(payment_id, invoice_id, scope_id, phone, amount_cents, currency, state, environment, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)`,
		paymentID, invoiceID, scopeID, phone, amountCents, "KES", state, env, Now(), Now())
	return err
}

func (s *MpesaStore) LinkCheckout(ctx context.Context, paymentID, merchantRequestID, checkoutRequestID string) error {
	_, err := s.ExecContext(ctx, `UPDATE mpesa_payment SET merchant_request_id = ?, checkout_request_id = ?, updated_at = ? WHERE payment_id = ?`,
		merchantRequestID, checkoutRequestID, Now(), paymentID)
	return err
}

func (s *MpesaStore) UpdatePaymentState(ctx context.Context, paymentID, state, receipt, desc string) error {
	_, err := s.ExecContext(ctx, `UPDATE mpesa_payment SET state = ?, receipt_number = ?, result_desc = ?, updated_at = ? WHERE payment_id = ?`,
		state, receipt, desc, Now(), paymentID)
	return err
}

func (s *MpesaStore) PaymentByCheckout(ctx context.Context, checkoutRequestID string) (map[string]string, error) {
	row := s.QueryRowContext(ctx, `SELECT payment_id, COALESCE(state,'') FROM mpesa_payment WHERE checkout_request_id = ?`, checkoutRequestID)
	var id, state string
	if err := row.Scan(&id, &state); err != nil {
		return nil, err
	}
	return map[string]string{"payment_id": id, "state": state}, nil
}

func (s *MpesaStore) RecordCallback(ctx context.Context, checkoutRequestID, payloadDigest string) error {
	_, err := s.ExecContext(ctx, `INSERT INTO mpesa_callback_event(event_id, checkout_request_id, payload_digest, received_at)
		VALUES(?,?,?,?)`, NewID("cbk"), checkoutRequestID, payloadDigest, Now())
	return err
}

func (s *MpesaStore) PendingPayments(ctx context.Context, olderThan time.Duration) []map[string]string {
	horizon := time.Now().UTC().Add(-olderThan).Format(time.RFC3339Nano)
	rows, err := s.QueryContext(ctx, `SELECT payment_id, COALESCE(checkout_request_id,'') FROM mpesa_payment
		WHERE state IN ('pending','uncertain') AND updated_at < ?`, horizon)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []map[string]string
	for rows.Next() {
		var id, co string
		if rows.Scan(&id, &co) == nil && co != "" {
			out = append(out, map[string]string{"payment_id": id, "checkout_request_id": co})
		}
	}
	return out
}

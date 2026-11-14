package store

// M-Pesa gateway configuration and store contract live here so both the
// domain engine (domains) and persistence (store) can share them without an
// import cycle. Secret VALUES never live here — only references.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// MpesaConfig is the provider-scoped gateway configuration. Secrets are
// REFERENCES (env:NAME | file:/abs) — raw secrets never live in config,
// API responses, logs or audit records.
type MpesaConfig struct {
	Enabled         bool   `json:"enabled"`
	Environment     string `json:"environment"` // sandbox | production
	Shortcode       string `json:"shortcode"`   // business shortcode (paybill/till)
	TransactionType string `json:"transaction_type"`
	PartyB          string `json:"party_b,omitempty"`
	ConsumerKeyRef  string `json:"consumer_key_ref"`
	ConsumerSecRef  string `json:"consumer_secret_ref"`
	PasskeyRef      string `json:"passkey_ref"`
	CallbackURL     string `json:"callback_url"`
	Currency        string `json:"currency"`
	MinAmountCents  int64  `json:"min_amount_cents"`
	MaxAmountCents  int64  `json:"max_amount_cents"`
	ProductionAck   bool   `json:"production_acknowledged"` // required true to enable production
}

// SanitizeMpesaConfig strips secret references for API exposure.
func SanitizeMpesaConfig(c MpesaConfig) MpesaConfig {
	c.ConsumerKeyRef = refShape(c.ConsumerKeyRef)
	c.ConsumerSecRef = "***"
	c.PasskeyRef = "***"
	return c
}

func refShape(r string) string {
	if strings.HasPrefix(r, "env:") || strings.HasPrefix(r, "file:") {
		return r
	}
	if r == "" {
		return ""
	}
	return "***"
}

// ResolveRef loads a secret reference (env:NAME | file:/abs).
func ResolveRef(ref string) (string, error) {
	switch {
	case strings.HasPrefix(ref, "env:"):
		v, ok := os.LookupEnv(strings.TrimPrefix(ref, "env:"))
		if !ok || v == "" {
			return "", fmt.Errorf("secret reference not set")
		}
		return v, nil
	case strings.HasPrefix(ref, "file:"):
		b, err := os.ReadFile(strings.TrimPrefix(ref, "file:"))
		if err != nil {
			return "", fmt.Errorf("secret reference unreadable")
		}
		return strings.TrimSpace(string(b)), nil
	default:
		return "", fmt.Errorf("secret references must be env:NAME or file:/abs")
	}
}

// ValidateMpesaConfig enforces configuration rules. Production requires an
// explicit acknowledgement (the activation gate).
func ValidateMpesaConfig(c MpesaConfig) error {
	if c.Environment != "sandbox" && c.Environment != "production" {
		return errors.New("environment must be sandbox or production")
	}
	if c.Environment == "production" && !c.ProductionAck {
		return errors.New("production environment requires explicit activation acknowledgement")
	}
	if c.Shortcode == "" || len(c.Shortcode) > 12 || !regexp.MustCompile(`^[0-9]+$`).MatchString(c.Shortcode) {
		return errors.New("invalid shortcode")
	}
	if c.ConsumerKeyRef == "" || c.ConsumerSecRef == "" {
		return errors.New("consumer key/secret references required")
	}
	if c.Environment == "production" && c.PasskeyRef == "" {
		return errors.New("passkey reference required for STK push")
	}
	if c.Currency != "KES" {
		return errors.New("Daraja supports KES")
	}
	if c.MinAmountCents <= 0 || c.MaxAmountCents < c.MinAmountCents {
		return errors.New("invalid amount bounds")
	}
	if _, err := ResolveRef(c.ConsumerKeyRef); err != nil {
		return fmt.Errorf("consumer key ref: %w", err)
	}
	if _, err := ResolveRef(c.ConsumerSecRef); err != nil {
		return fmt.Errorf("consumer secret ref: %w", err)
	}
	return nil
}

// MpesaGateway is the persistence contract. -------------------------------------------------------------

// MpesaGateway persists payments and callback events.
type MpesaGateway interface {
	CreatePayment(ctx context.Context, paymentID, invoiceID, scopeID, phone string, amountCents int64, env, state string, now time.Time) error
	LinkCheckout(ctx context.Context, paymentID, merchantRequestID, checkoutRequestID string) error
	UpdatePaymentState(ctx context.Context, paymentID, state, receipt, desc string) error
	PaymentByCheckout(ctx context.Context, checkoutRequestID string) (map[string]string, error)
	RecordCallback(ctx context.Context, checkoutRequestID, payloadDigest string) error
	PendingPayments(ctx context.Context, olderThan time.Duration) []map[string]string
}

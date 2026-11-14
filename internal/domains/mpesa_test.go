package domains

// M-Pesa Daraja engine tests against a mock Daraja (httptest).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// memoryMpesaStore is an in-memory MpesaStore for tests.
type memoryMpesaStore struct {
	mu        sync.Mutex
	payments  map[string]map[string]string
	checkouts map[string]string
	callbacks map[string]bool
}

func newMemStore() *memoryMpesaStore {
	return &memoryMpesaStore{payments: map[string]map[string]string{}, checkouts: map[string]string{}, callbacks: map[string]bool{}}
}

func (s *memoryMpesaStore) CreatePayment(_ context.Context, paymentID, invoiceID, scopeID, phone string, amountCents int64, env, state string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.payments[paymentID] = map[string]string{"payment_id": paymentID, "invoice_id": invoiceID, "state": state}
	return nil
}
func (s *memoryMpesaStore) LinkCheckout(_ context.Context, paymentID, merchantRequestID, checkoutRequestID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkouts[checkoutRequestID] = paymentID
	s.payments[paymentID]["checkout_request_id"] = checkoutRequestID
	return nil
}
func (s *memoryMpesaStore) UpdatePaymentState(_ context.Context, paymentID, state, receipt, desc string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p := s.payments[paymentID]; p != nil {
		p["state"], p["receipt"], p["desc"] = state, receipt, desc
	}
	return nil
}
func (s *memoryMpesaStore) PaymentByCheckout(_ context.Context, checkoutRequestID string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.checkouts[checkoutRequestID]
	if !ok {
		return nil, errorsNew("unknown checkout")
	}
	return s.payments[id], nil
}
func (s *memoryMpesaStore) RecordCallback(_ context.Context, checkoutRequestID, payloadDigest string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.callbacks[checkoutRequestID] {
		return errorsNew("duplicate")
	}
	s.callbacks[checkoutRequestID] = true
	return nil
}
func (s *memoryMpesaStore) PendingPayments(_ context.Context, olderThan time.Duration) []map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []map[string]string
	for _, p := range s.payments {
		if p["state"] == "pending" || p["state"] == "uncertain" {
			out = append(out, p)
		}
	}
	return out
}

func errorsNew(s string) error { return &simpleError{s} }

type simpleError struct{ s string }

func (e *simpleError) Error() string { return e.s }

// mockDaraja emulates the Daraja endpoints used by the engine.
type mockDaraja struct {
	mu          sync.Mutex
	oauthHits   int
	pushResult  map[string]any
	queryResult map[string]any
	pushHits    int
	queryHits   int
}

func (m *mockDaraja) Do(r *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec := httptest.NewRecorder()
	switch {
	case strings.Contains(r.URL.Path, "/oauth/v1/generate"):
		m.oauthHits++
		if r.Header.Get("Authorization") == "" {
			rec.WriteHeader(401)
		} else {
			rec.WriteHeader(200)
			rec.Write([]byte(`{"access_token":"tk_` + itoa(m.oauthHits) + `","expires_in":"3599"}`))
		}
	case strings.Contains(r.URL.Path, "/mpesa/stkpush/v1/processrequest"):
		m.pushHits++
		b, _ := json.Marshal(m.pushResult)
		rec.WriteHeader(200)
		rec.Write(b)
	case strings.Contains(r.URL.Path, "/mpesa/stkpushquery/v1/query"):
		m.queryHits++
		b, _ := json.Marshal(m.queryResult)
		rec.WriteHeader(200)
		rec.Write(b)
	default:
		rec.WriteHeader(404)
	}
	return rec.Result(), nil
}

func itoa(i int) string { return json.Number(jsonInt(i)).String() }
func jsonInt(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}

func mpesaFixture(t *testing.T) (*MpesaEngine, *memoryMpesaStore, *mockDaraja) {
	t.Helper()
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "key")
	secFile := filepath.Join(dir, "sec")
	passFile := filepath.Join(dir, "pass")
	writeFile(keyFile, "test-consumer-key")
	writeFile(secFile, "test-consumer-secret")
	writeFile(passFile, "test-passkey")
	cfg := MpesaConfig{
		Enabled: true, Environment: "sandbox", Shortcode: "174379",
		TransactionType: "CustomerPayBillOnline", PartyB: "174379",
		ConsumerKeyRef: "file:" + keyFile, ConsumerSecRef: "file:" + secFile,
		PasskeyRef: "file:" + passFile, CallbackURL: "https://cb.example/callback",
		Currency: "KES", MinAmountCents: 100, MaxAmountCents: 500,
	}
	d := &mockDaraja{pushResult: map[string]any{"MerchantRequestID": "mr-1", "CheckoutRequestID": "ws_CO_1", "ResponseCode": "0", "ResponseDescription": "Success"},
		queryResult: map[string]any{"ResultCode": "0", "ResultDesc": "The service request is processed successfully.", "MpesaReceiptNumber": "QK71ABC123"}}
	st := newMemStore()
	return NewMpesaEngine(cfg, d, st, func() time.Time { return time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC) }), st, d
}

func writeFile(p, s string) {
	if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
		panic(err)
	}
}

func TestMpesaConfigAndRefs(t *testing.T) {
	_, _, _ = mpesaFixture(t)
	cfg := MpesaConfig{Enabled: true, Environment: "production", Shortcode: "5511376",
		ConsumerKeyRef: "env:NOPE", ConsumerSecRef: "env:NOPE2", Currency: "KES",
		MinAmountCents: 100, MaxAmountCents: 500, PasskeyRef: "env:NOPE3"}
	if err := ValidateMpesaConfig(cfg); err == nil {
		t.Fatal("production without acknowledgement must reject")
	}
	cfg.ProductionAck = true
	if err := ValidateMpesaConfig(cfg); err == nil || !strings.Contains(err.Error(), "consumer key") {
		t.Fatalf("unresolvable refs must reject: %v", err)
	}
	if _, err := NormalizePhone("0712345678"); err != nil {
		t.Fatalf("07 normalization: %v", err)
	}
	if p, _ := NormalizePhone("+254712345678"); p != "254712345678" {
		t.Fatalf("normalize: %q", p)
	}
	if _, err := NormalizePhone("07123"); err == nil {
		t.Fatal("short phone must reject")
	}
}

func TestMpesaSTKPushAndVerification(t *testing.T) {
	e, st, d := mpesaFixture(t)
	ctx := context.Background()
	// Amount bounds: owner test range 1-5 KES.
	if _, err := e.InitiateSTKPush(ctx, "pay_1", "inv_1", "ws_1", "0712345678", 700); err == nil {
		t.Fatal("amount above bounds must reject")
	}
	out, err := e.InitiateSTKPush(ctx, "pay_1", "inv_1", "ws_1", "0712345678", 200)
	if err != nil {
		t.Fatalf("stk push: %v", err)
	}
	if out["state"] != "pending" || out["checkout_request_id"] != "ws_CO_1" {
		t.Fatalf("push output: %v", out)
	}
	// OAuth cached: second call must not hit oauth again.
	before := d.oauthHits
	if _, err := e.AccessToken(ctx); err != nil {
		t.Fatal(err)
	}
	if d.oauthHits != before {
		t.Fatal("oauth token not cached")
	}
	// Untrusted callback claiming success: must NOT settle alone.
	cb := map[string]any{"Body": map[string]any{"stkCallback": map[string]any{
		"MerchantRequestID": "mr-1", "CheckoutRequestID": "ws_CO_1", "ResultCode": int64(0),
		"ResultDesc": "The service request is processed successfully.",
		"CallbackMetadata": map[string]any{"Item": []any{
			map[string]any{"Name": "Amount", "Value": float64(2)},
			map[string]any{"Name": "MpesaReceiptNumber", "Value": "QK71ABC123"},
		}}}}}
	raw, _ := json.Marshal(cb)
	state, err := e.ProcessCallback(ctx, raw)
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	if state != "succeeded" {
		t.Fatalf("verified settle expected succeeded, got %s", state)
	}
	if p, _ := st.PaymentByCheckout(ctx, "ws_CO_1"); p["state"] != "succeeded" || p["receipt"] != "QK71ABC123" {
		t.Fatalf("payment state: %v", p)
	}
	// Duplicate callback: idempotent (no error, no double processing).
	if state2, err := e.ProcessCallback(ctx, raw); err != nil || state2 != "" {
		t.Fatalf("duplicate callback must be a no-op: %v %v", state2, err)
	}
	// Unknown correlation: recorded but ignored.
	raw2, _ := json.Marshal(map[string]any{"Body": map[string]any{"stkCallback": map[string]any{
		"CheckoutRequestID": "ws_UNKNOWN", "ResultCode": int64(0)}}})
	if _, err := e.ProcessCallback(ctx, raw2); err != nil {
		t.Fatalf("unknown correlation must not error: %v", err)
	}
	// Cancelled by user (ResultCode 1032 in query).
	d.queryResult = map[string]any{"ResultCode": "1032", "ResultDesc": "Request cancelled by user"}
	e2, _, d2 := mpesaFixture(t)
	d2.queryResult = map[string]any{"ResultCode": "1032", "ResultDesc": "Request cancelled by user"}
	e2.InitiateSTKPush(ctx, "pay_2", "inv_2", "ws_1", "0712345678", 200)
	cb2, _ := json.Marshal(map[string]any{"Body": map[string]any{"stkCallback": map[string]any{"CheckoutRequestID": "ws_CO_1", "ResultCode": int64(1), "ResultDesc": "cancelled"}}})
	state3, _ := e2.ProcessCallback(ctx, cb2)
	if state3 != "cancelled" {
		t.Fatalf("cancelled settle: %s", state3)
	}
	// Reconciliation settles an uncertain payment via query truth.
	d.queryResult = map[string]any{"ResultCode": "0", "ResultDesc": "processed", "MpesaReceiptNumber": "QK71XYZ789"}
	e3, st3, _ := mpesaFixture(t)
	e3.InitiateSTKPush(ctx, "pay_3", "inv_3", "ws_1", "0712345678", 200)
	st3.payments["pay_3"]["state"] = "uncertain"
	res := e3.ReconcilePending(ctx, time.Minute)
	if len(res) != 1 || res[0]["settled_state"] != "succeeded" {
		t.Fatalf("reconciliation: %v", res)
	}
}

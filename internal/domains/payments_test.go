package domains

// Sandbox payment adapter tests: charge via an in-process fixture server,
// webhook HMAC verification, replay rejection, declined-card handling,
// fail-closed configuration.

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSandboxGatewayChargeIdempotent(t *testing.T) {
	charges := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		charges++
		var in map[string]any
		json.NewDecoder(r.Body).Decode(&in)
		if in["idempotency_key"] != "INV-2026-001" {
			t.Errorf("idempotency key missing: %v", in)
		}
		json.NewEncoder(w).Encode(map[string]string{"reference": "ch_test_123"})
	}))
	defer srv.Close()
	g := &SandboxGateway{BaseURL: srv.URL, SecretKey: "sk_test_x"}
	ref, err := g.Charge("INV-2026-001", 9900, "EUR")
	if err != nil || ref != "ch_test_123" {
		t.Fatalf("charge: %v %s", err, ref)
	}
	ref2, _ := g.Charge("INV-2026-001", 9900, "EUR")
	if ref2 != "ch_test_123" {
		t.Fatal("same invoice must reuse the idempotency key path")
	}
	if charges != 2 {
		t.Fatalf("fixture calls: %d", charges) // idempotency enforced provider-side
	}
}

func TestSandboxGatewayDeclined(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		json.NewEncoder(w).Encode(map[string]string{"error": "card_declined"})
	}))
	defer srv.Close()
	g := &SandboxGateway{BaseURL: srv.URL, SecretKey: "sk_test_x"}
	if _, err := g.Charge("INV-1", 100, "EUR"); err == nil || !strings.Contains(err.Error(), "card_declined") {
		t.Fatalf("declined: %v", err)
	}
}

func TestSandboxGatewayFailClosed(t *testing.T) {
	g := &SandboxGateway{}
	if _, err := g.Charge("INV-1", 100, "EUR"); err == nil {
		t.Fatal("unconfigured gateway must fail closed")
	}
}

func TestWebhookSignatureAndReplay(t *testing.T) {
	base := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	g := &SandboxGateway{SecretKey: "whsec_test", NowFn: func() time.Time { return base }}
	body := []byte(`{"type":"charge.succeeded","id":"evt_1"}`)
	mac := hmac.New(sha256.New, []byte("whsec_test"))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if err := g.VerifyWebhook(body, sig, "evt_1", base); err != nil {
		t.Fatalf("valid webhook rejected: %v", err)
	}
	// Replay: same event id rejects.
	if err := g.VerifyWebhook(body, sig, "evt_1", base.Add(60*time.Second)); err == nil {
		t.Fatal("replayed webhook must reject")
	}
	// Tampered body rejects.
	if err := g.VerifyWebhook([]byte(`{"type":"charge.succeeded","id":"evt_2"}`), sig, "evt_2", base); err == nil {
		t.Fatal("tampered payload must reject")
	}
	// Missing event id rejects.
	if err := g.VerifyWebhook(body, sig, "", base); err == nil {
		t.Fatal("missing event id must reject")
	}
	// Next day: same id is allowed again (window semantics).
	if err := g.VerifyWebhook(body, sig, "evt_1", base.Add(25*time.Hour)); err != nil {
		t.Fatalf("post-window event must process: %v", err)
	}
}

func TestPresentGatewaysOwnerOptions(t *testing.T) {
	opts := PresentGateways()
	if len(opts) < 3 {
		t.Fatalf("gateway options: %d", len(opts))
	}
	for _, o := range opts {
		if o.SandboxURL == "" || len(o.Currencies) == 0 {
			t.Fatalf("option %s missing sandbox/currencies", o.ID)
		}
	}
}

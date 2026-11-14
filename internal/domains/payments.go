package domains

// Sandbox payment gateway adapter (BIZ-03 / DEC-020): a provider-independent
// HTTP client + webhook verifier for sandbox integration. NO production
// credentials, NO real charges — the adapter talks to whatever base URL the
// configuration supplies (provider sandbox or an in-process fixture in
// tests), fails closed on signature/replay/idempotency violations.

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SandboxGateway implements PaymentGateway against a sandbox base URL.
type SandboxGateway struct {
	BaseURL    string           // e.g. https://sandbox.provider.test/v1
	SecretKey  string           // sandbox secret (never logged)
	HTTPClient *http.Client     // injectable; fixture server in tests
	NowFn      func() time.Time // replay-window clock (injectable)
	// WebhookReplayGuard: seen event IDs with their first-seen timestamps
	seenEvents map[string]time.Time
}

// Charge creates a sandbox payment intent and returns the provider reference.
func (g *SandboxGateway) Charge(invoiceNumber string, amountCents int64, currency string) (string, error) {
	if g.BaseURL == "" || g.SecretKey == "" {
		return "", fmt.Errorf("gateway not configured (fail closed; production credentials are a separate owner authorization)")
	}
	if g.HTTPClient == nil {
		g.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	body, _ := json.Marshal(map[string]any{
		"invoice": invoiceNumber, "amount": amountCents, "currency": currency,
		"idempotency_key": invoiceNumber, // same invoice → same intent
	})
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(g.BaseURL, "/")+"/charges", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+g.SecretKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := g.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gateway unreachable")
	}
	defer res.Body.Close()
	var out struct {
		Reference string `json:"reference"`
		Error     string `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("gateway response unparsable")
	}
	if res.StatusCode != http.StatusOK {
		if out.Error != "" {
			return "", fmt.Errorf("gateway declined: %s", out.Error)
		}
		return "", fmt.Errorf("gateway error status %d", res.StatusCode)
	}
	if out.Reference == "" {
		return "", fmt.Errorf("gateway returned no reference")
	}
	return out.Reference, nil
}

// VerifyWebhook authenticates a webhook payload: HMAC-SHA256 over the raw
// body with the sandbox secret, constant-time compare, and replay protection
// (event IDs are remembered; a repeated ID inside the window rejects).
func (g *SandboxGateway) VerifyWebhook(rawBody []byte, signature string, eventID string, at time.Time) error {
	if g.SecretKey == "" {
		return fmt.Errorf("gateway secret not configured")
	}
	mac := hmac.New(sha256.New, []byte(g.SecretKey))
	mac.Write(rawBody)
	want := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(strings.TrimPrefix(signature, "sha256="))) {
		return fmt.Errorf("webhook signature mismatch")
	}
	if eventID == "" {
		return fmt.Errorf("webhook event id required (replay protection)")
	}
	if g.seenEvents == nil {
		g.seenEvents = map[string]time.Time{}
	}
	now := at
	if now.IsZero() && g.NowFn != nil {
		now = g.NowFn()
	}
	if now.IsZero() {
		now = time.Now()
	}
	if first, ok := g.seenEvents[eventID]; ok && now.Sub(first) < 24*time.Hour {
		return fmt.Errorf("webhook replay rejected (event already processed)")
	}
	g.seenEvents[eventID] = now
	return nil
}

// PresentGateways is the owner-facing option set (DEC-020): supported
// markets, currencies, capabilities and sandbox availability.
type GatewayOption struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Markets      []string `json:"markets"`
	Currencies   []string `json:"currencies"`
	Capabilities []string `json:"capabilities"`
	SandboxURL   string   `json:"sandbox_url"`
	Notes        string   `json:"notes"`
}

func PresentGateways() []GatewayOption {
	return []GatewayOption{
		{ID: "stripe", Name: "Stripe", Markets: []string{"global (46+ countries)"},
			Currencies:   []string{"USD", "EUR", "GBP", "CHF", "AUD", "CAD", "+135 more"},
			Capabilities: []string{"cards", "SEPA", "webhooks", "refunds", "subscriptions"},
			SandboxURL:   "https://api.stripe.com (test mode keys)",
			Notes:        "test-mode keys only; production switch is an explicit owner authorization"},
		{ID: "mollie", Name: "Mollie", Markets: []string{"EU/EEA"},
			Currencies:   []string{"EUR", "USD", "GBP", "+10"},
			Capabilities: []string{"iDEAL", "SEPA", "cards", "webhooks", "refunds"},
			SandboxURL:   "https://api.mollie.com/v2 (test API key)",
			Notes:        "strong EU local methods; test key never charges"},
		{ID: "paypal", Name: "PayPal REST", Markets: []string{"global"},
			Currencies:   []string{"USD", "EUR", "GBP", "+20"},
			Capabilities: []string{"paypal wallet", "cards", "webhooks", "refunds"},
			SandboxURL:   "https://api-m.sandbox.paypal.com",
			Notes:        "sandbox accounts mirror live flows"},
	}
}

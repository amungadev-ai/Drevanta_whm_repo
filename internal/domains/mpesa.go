package domains

// M-Pesa Daraja payment gateway (owner directive §7-9): sandbox-first.
// Production environment is CONFIGURABLE but requires explicit activation;
// no live financial transactions are performed by this integration unless
// the production environment is enabled by an authorized configuration
// change. Callbacks from Daraja are treated as UNTRUSTED: an invoice is
// settled only after gateway-side verification (STK push status query),
// never on the strength of an unauthenticated callback alone. Daraja
// callbacks carry no HMAC signature (unlike other providers) — that is
// precisely why verification uses the Transaction Status / STK query API.

import (
	"context"
	"crypto/sha256"
	"drevanta/internal/store"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	DarajaSandboxBase = "https://sandbox.safaricom.co.ke"
	DarajaProdBase    = "https://api.safaricom.co.ke"
)

// DarajaClient performs the HTTP calls (interface for tests).
type DarajaClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func mpesaBaseURL(c MpesaConfig) string {
	if c.Environment == "production" {
		return DarajaProdBase
	}
	return DarajaSandboxBase
}

// MpesaEngine implements OAuth caching, STK push, callback verification and
// reconciliation over a payment store.
type MpesaEngine struct {
	Config MpesaConfig
	HTTP   DarajaClient
	Store  MpesaStore
	Now    func() time.Time

	token    string
	tokenExp time.Time
}

func NewMpesaEngine(c MpesaConfig, h DarajaClient, s MpesaStore, now func() time.Time) *MpesaEngine {
	if now == nil {
		now = time.Now
	}
	if h == nil {
		h = &http.Client{Timeout: 30 * time.Second}
	}
	return &MpesaEngine{Config: c, HTTP: h, Store: s, Now: now}
}

// ---- OAuth -----------------------------------------------------------------

// AccessToken fetches (and caches until ~60s before expiry) the OAuth token.
func (e *MpesaEngine) AccessToken(ctx context.Context) (string, error) {
	if e.token != "" && e.Now().Before(e.tokenExp.Add(-60*time.Second)) {
		return e.token, nil
	}
	key, err := ResolveRef(e.Config.ConsumerKeyRef)
	if err != nil {
		return "", err
	}
	sec, err := ResolveRef(e.Config.ConsumerSecRef)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		mpesaBaseURL(e.Config)+"/oauth/v1/generate?grant_type=client_credentials", nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(key, sec)
	res, err := e.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("oauth unreachable")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oauth rejected (%d)", res.StatusCode)
	}
	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   string `json:"expires_in"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil || body.AccessToken == "" {
		return "", fmt.Errorf("oauth response invalid")
	}
	secs, _ := strconv.Atoi(body.ExpiresIn)
	if secs <= 0 {
		secs = 3599
	}
	e.token = body.AccessToken
	e.tokenExp = e.Now().Add(time.Duration(secs) * time.Second)
	return e.token, nil
}

// HealthCheck verifies connectivity+credentials (OAuth ping).
func (e *MpesaEngine) HealthCheck(ctx context.Context) (string, error) {
	if _, err := e.AccessToken(ctx); err != nil {
		return "unreachable", err
	}
	return "reachable", nil
}

// ---- STK push ----------------------------------------------------------------

var phoneRe = regexp.MustCompile(`^2547\d{8}$|^2541\d{8}$`)

// NormalizePhone converts 07XX/01XX/+2547XX forms to 2547XXXXXXXX.
func NormalizePhone(in string) (string, error) {
	p := strings.TrimSpace(in)
	p = strings.TrimPrefix(p, "+")
	if strings.HasPrefix(p, "0") && len(p) == 10 {
		p = "254" + p[1:]
	}
	if !phoneRe.MatchString(p) {
		return "", errors.New("invalid Safaricom phone number")
	}
	return p, nil
}

func stkTimestamp(t time.Time) string { return t.Format("20060102150405") }

// InitiateSTKPush starts a payment attempt (persisted as pending) and
// returns the correlation identifiers for the customer UI.
func (e *MpesaEngine) InitiateSTKPush(ctx context.Context, paymentID, invoiceID, scopeID, rawPhone string, amountCents int64) (map[string]any, error) {
	if !e.Config.Enabled {
		return nil, errors.New("gateway disabled")
	}
	if amountCents < e.Config.MinAmountCents || amountCents > e.Config.MaxAmountCents {
		return nil, fmt.Errorf("amount outside configured bounds")
	}
	phone, err := NormalizePhone(rawPhone)
	if err != nil {
		return nil, err
	}
	tok, err := e.AccessToken(ctx)
	if err != nil {
		return nil, err
	}
	passkey, err := ResolveRef(e.Config.PasskeyRef)
	if err != nil {
		return nil, fmt.Errorf("passkey ref: %w", err)
	}
	now := e.Now()
	ts := stkTimestamp(now)
	password := base64.StdEncoding.EncodeToString([]byte(e.Config.Shortcode + passkey + ts))
	partyB := e.Config.Shortcode
	if e.Config.PartyB != "" {
		partyB = e.Config.PartyB
	}
	ttype := e.Config.TransactionType
	if ttype == "" {
		ttype = "CustomerPayBillOnline"
	}
	payload := map[string]any{
		"BusinessShortCode": e.Config.Shortcode,
		"Password":          password,
		"Timestamp":         ts,
		"TransactionType":   ttype,
		"Amount":            amountCents / 100, // Daraja takes whole KES
		"PartyA":            phone,
		"PartyB":            partyB,
		"PhoneNumber":       phone,
		"CallBackURL":       e.Config.CallbackURL,
		"AccountReference":  invoiceID,
		"TransactionDesc":   "Drevanta invoice " + invoiceID,
	}
	if err := e.Store.CreatePayment(ctx, paymentID, invoiceID, scopeID, phone, amountCents,
		e.Config.Environment, "initiated", now); err != nil {
		return nil, err
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		mpesaBaseURL(e.Config)+"/mpesa/stkpush/v1/processrequest", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	res, err := e.HTTP.Do(req)
	if err != nil {
		e.Store.UpdatePaymentState(ctx, paymentID, "failed", "", "gateway unreachable")
		return nil, fmt.Errorf("stk push unreachable")
	}
	defer res.Body.Close()
	var out struct {
		MerchantRequestID   string `json:"MerchantRequestID"`
		CheckoutRequestID   string `json:"CheckoutRequestID"`
		ResponseCode        string `json:"ResponseCode"`
		ResponseDescription string `json:"ResponseDescription"`
		ErrorMessage        string `json:"errorMessage"`
	}
	_ = json.NewDecoder(res.Body).Decode(&out)
	if res.StatusCode != http.StatusOK || out.ResponseCode != "0" || out.CheckoutRequestID == "" {
		desc := out.ResponseDescription
		if desc == "" {
			desc = out.ErrorMessage
		}
		e.Store.UpdatePaymentState(ctx, paymentID, "failed", "", desc)
		return nil, fmt.Errorf("stk push rejected: %s", desc)
	}
	_ = e.Store.LinkCheckout(ctx, paymentID, out.MerchantRequestID, out.CheckoutRequestID)
	e.Store.UpdatePaymentState(ctx, paymentID, "pending", "", "awaiting customer action")
	return map[string]any{
		"payment_id": paymentID, "state": "pending",
		"checkout_request_id": out.CheckoutRequestID,
		"message":             "Enter your M-Pesa PIN on the prompt",
	}, nil
}

// ---- Callbacks (untrusted) -----------------------------------------------------

// MpesaCallback is the STK push callback shape.
type MpesaCallback struct {
	Body struct {
		StkCallback struct {
			MerchantRequestID string `json:"MerchantRequestID"`
			CheckoutRequestID string `json:"CheckoutRequestID"`
			ResultCode        int64  `json:"ResultCode"`
			ResultDesc        string `json:"ResultDesc"`
			CallbackMetadata  struct {
				Item []struct {
					Name  string `json:"Name"`
					Value any    `json:"Value"`
				} `json:"Item"`
			} `json:"CallbackMetadata"`
		} `json:"stkCallback"`
	} `json:"Body"`
}

// Receipt extracts MpesaReceiptNumber from callback metadata.
func (c *MpesaCallback) Receipt() string {
	for _, it := range c.Body.StkCallback.CallbackMetadata.Item {
		if it.Name == "MpesaReceiptNumber" {
			s, _ := it.Value.(string)
			return s
		}
	}
	return ""
}

// ProcessCallback records the untrusted callback and verifies the claimed
// outcome with the gateway before moving the payment out of pending.
// Duplicate deliveries are idempotent. Returns the (possibly verified)
// payment state.
func (e *MpesaEngine) ProcessCallback(ctx context.Context, raw []byte) (string, error) {
	var cb MpesaCallback
	if err := json.Unmarshal(raw, &cb); err != nil {
		return "", errors.New("malformed callback")
	}
	co := cb.Body.StkCallback.CheckoutRequestID
	if co == "" {
		return "", errors.New("callback missing correlation")
	}
	sum := sha256.Sum256(raw)
	if err := e.Store.RecordCallback(ctx, co, hex.EncodeToString(sum[:])); err != nil {
		// Duplicate delivery: idempotent no-op.
		return "", nil
	}
	p, err := e.Store.PaymentByCheckout(ctx, co)
	if err != nil {
		// Unknown correlation: store the event, do nothing else.
		return "", nil
	}
	// Verification step: trust the gateway's QUERY result, not the callback.
	return e.VerifyAndSettle(ctx, p["payment_id"], co)
}

// VerifyAndSettle queries the gateway for the transaction outcome and
// transitions the payment accordingly. This is the ONLY path that settles.
func (e *MpesaEngine) VerifyAndSettle(ctx context.Context, paymentID, checkoutRequestID string) (string, error) {
	res, desc, receipt, err := e.stkQuery(ctx, checkoutRequestID)
	if err != nil {
		// Uncertain: keep pending for reconciliation retry.
		e.Store.UpdatePaymentState(ctx, paymentID, "uncertain", receipt, "verification unavailable: "+err.Error())
		return "uncertain", err
	}
	switch {
	case res == 0:
		e.Store.UpdatePaymentState(ctx, paymentID, "succeeded", receipt, desc)
		return "succeeded", nil
	case res == 1032: // cancelled by user
		e.Store.UpdatePaymentState(ctx, paymentID, "cancelled", receipt, desc)
		return "cancelled", nil
	case res == 1037: // timeout, DS timeout user unreachable — recoverable
		e.Store.UpdatePaymentState(ctx, paymentID, "uncertain", receipt, desc)
		return "uncertain", nil
	default:
		e.Store.UpdatePaymentState(ctx, paymentID, "failed", receipt, desc)
		return "failed", nil
	}
}

// stkQuery performs the STK push status query.
func (e *MpesaEngine) stkQuery(ctx context.Context, checkoutRequestID string) (int64, string, string, error) {
	tok, err := e.AccessToken(ctx)
	if err != nil {
		return 0, "", "", err
	}
	passkey, err := ResolveRef(e.Config.PasskeyRef)
	if err != nil {
		return 0, "", "", err
	}
	now := e.Now()
	ts := stkTimestamp(now)
	password := base64.StdEncoding.EncodeToString([]byte(e.Config.Shortcode + passkey + ts))
	body, _ := json.Marshal(map[string]string{
		"BusinessShortCode": e.Config.Shortcode,
		"Password":          password,
		"Timestamp":         ts,
		"CheckoutRequestID": checkoutRequestID,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		mpesaBaseURL(e.Config)+"/mpesa/stkpushquery/v1/query", strings.NewReader(string(body)))
	if err != nil {
		return 0, "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	res, err := e.HTTP.Do(req)
	if err != nil {
		return 0, "", "", fmt.Errorf("query unreachable")
	}
	defer res.Body.Close()
	var out struct {
		ResultCode         string `json:"ResultCode"`
		ResultDesc         string `json:"ResultDesc"`
		MpesaReceiptNumber string `json:"MpesaReceiptNumber"`
		ErrorMessage       string `json:"errorMessage"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return 0, "", "", fmt.Errorf("query response invalid")
	}
	if out.ResultCode == "" && out.ErrorMessage != "" {
		return 0, "", "", fmt.Errorf("query rejected: %s", out.ErrorMessage)
	}
	code, _ := strconv.ParseInt(out.ResultCode, 10, 64)
	return code, out.ResultDesc, out.MpesaReceiptNumber, nil
}

// ReconcilePending re-verifies pending/uncertain payments older than the
// grace window (timeout & uncertain-payment recovery) and settles them by
// gateway truth. Returns the transitions for audit.
func (e *MpesaEngine) ReconcilePending(ctx context.Context, olderThan time.Duration) []map[string]string {
	var out []map[string]string
	for _, p := range e.Store.PendingPayments(ctx, olderThan) {
		state, _ := e.VerifyAndSettle(ctx, p["payment_id"], p["checkout_request_id"])
		out = append(out, map[string]string{
			"payment_id": p["payment_id"], "settled_state": state,
		})
	}
	return out
}

// ---- Shared types (defined in store to avoid an import cycle) ---------------

type (
	MpesaConfig = store.MpesaConfig
	MpesaStore  = store.MpesaGateway
)

var (
	ResolveRef          = store.ResolveRef
	ValidateMpesaConfig = store.ValidateMpesaConfig
	SanitizeMpesaConfig = store.SanitizeMpesaConfig
)

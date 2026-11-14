package domains

// Inbound source-control webhooks (FP-P4-026 / INT-01, P4 §26): HMAC
// signature verification, replay protection, timestamp window, event
// deduplication, rate limiting, payload size limits and stable event IDs.
// A webhook may enqueue a build but NEVER bypasses branch/environment
// approval policies. Logs carry metadata only — never tokens or payloads.

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	MaxWebhookPayloadBytes = 1 << 20 // 1 MiB hard cap
	WebhookTimestampWindow = 10 * time.Minute
)

// WebhookDecision is the outcome of a verified, accepted webhook.
type WebhookDecision struct {
	EventID      string // stable: delivery header, else content hash
	Event        string // e.g. "push"
	Branch       string // refs/heads/<branch>
	EnqueueBuild bool
	Metadata     map[string]string // loggable metadata only
}

// WebhookRateLimiter is a fixed-window limiter per source (per secret ID).
type WebhookRateLimiter struct {
	windowStart time.Time
	windowCount int
	limit       int
}

func NewWebhookRateLimiter(limit int) *WebhookRateLimiter {
	return &WebhookRateLimiter{limit: limit}
}

func (r *WebhookRateLimiter) Allow(now time.Time) bool {
	if now.Sub(r.windowStart) >= time.Minute {
		r.windowStart, r.windowCount = now, 0
	}
	if r.windowCount >= r.limit {
		return false
	}
	r.windowCount++
	return true
}

// SourceWebhookVerifier holds per-source replay/dedup state.
type SourceWebhookVerifier struct {
	Secret string
	NowFn  func() time.Time
	rate   *WebhookRateLimiter
	seen   map[string]time.Time // event dedup
}

func NewSourceWebhookVerifier(secret string, rateLimit int, now func() time.Time) *SourceWebhookVerifier {
	return &SourceWebhookVerifier{Secret: secret, NowFn: now, rate: NewWebhookRateLimiter(rateLimit), seen: map[string]time.Time{}}
}

// ReceiveSourceWebhook validates and interprets one delivery. timestamp is
// the delivery timestamp asserted by the sender (0 = use NowFn). approval
// states whether the target branch/environment is owner-approved — a
// webhook NEVER bypasses it.
func (v *SourceWebhookVerifier) ReceiveSourceWebhook(headers map[string]string, body []byte,
	timestamp time.Time, branchApproved bool) (*WebhookDecision, error) {
	now := v.NowFn()
	if len(body) > MaxWebhookPayloadBytes {
		return nil, fmt.Errorf("payload exceeds size limit")
	}
	// 1) HMAC signature over the raw body (constant-time).
	sig := strings.TrimPrefix(headers["x-hub-signature-256"], "sha256=")
	mac := hmac.New(sha256.New, []byte(v.Secret))
	mac.Write(body)
	if !hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(sig)) {
		return nil, fmt.Errorf("webhook signature mismatch")
	}
	// 2) Timestamp window (replay protection at the transport layer).
	if timestamp.IsZero() {
		timestamp = now
	}
	if d := now.Sub(timestamp); d > WebhookTimestampWindow || d < -WebhookTimestampWindow {
		return nil, fmt.Errorf("delivery outside timestamp window")
	}
	// 3) Rate limit per source.
	if !v.rate.Allow(now) {
		return nil, fmt.Errorf("webhook rate limit exceeded")
	}
	// 4) Stable event ID: delivery header, else body content hash.
	eventID := headers["x-github-delivery"]
	if eventID == "" {
		sum := sha256.Sum256(body)
		eventID = hex.EncodeToString(sum[:8])
	}
	// 5) Event deduplication.
	if _, ok := v.seen[eventID]; ok {
		return nil, fmt.Errorf("duplicate event rejected")
	}
	v.seen[eventID] = now
	// 6) Interpret a minimal, safe event subset (metadata only).
	event := headers["x-github-event"]
	dec := &WebhookDecision{EventID: eventID, Event: event, Metadata: map[string]string{
		"event": event, "delivery": eventID, "bytes": fmt.Sprint(len(body)),
	}}
	if event == "push" {
		// Extract ONLY the branch ref (never raw payload contents in logs).
		var payload struct {
			Ref string `json:"ref"`
		}
		if err := jsonUnmarshalLimited(body, &payload); err == nil && strings.HasPrefix(payload.Ref, "refs/heads/") {
			dec.Branch = strings.TrimPrefix(payload.Ref, "refs/heads/")
			dec.Metadata["branch"] = dec.Branch
			// Build enqueue NEVER bypasses approval policies.
			if branchApproved {
				dec.EnqueueBuild = true
			} else {
				dec.Metadata["build"] = "not enqueued (branch/environment not approved)"
			}
		}
	}
	return dec, nil
}

// jsonUnmarshalLimited decodes a small prefix to avoid full payload parsing.
func jsonUnmarshalLimited(body []byte, v any) error {
	return json.Unmarshal(body, v)
}

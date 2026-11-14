package domains

// Tests for FP-P4-026 (source webhooks) and FP-P3-052 (site analytics).

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"
)

func signBody(t *testing.T, secret string, body []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestSourceWebhookAcceptsAndEnqueues(t *testing.T) {
	base := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	v := NewSourceWebhookVerifier("whsec_src", 60, func() time.Time { return base })
	body := []byte(`{"ref":"refs/heads/main"}`)
	hdrs := map[string]string{
		"x-hub-signature-256": signBody(t, "whsec_src", body),
		"x-github-event":      "push",
		"x-github-delivery":   "d-1",
	}
	dec, err := v.ReceiveSourceWebhook(hdrs, body, base.Add(-30*time.Second), true)
	if err != nil {
		t.Fatalf("accepted webhook failed: %v", err)
	}
	if !dec.EnqueueBuild || dec.Branch != "main" || dec.EventID != "d-1" {
		t.Fatalf("decision: %+v", dec)
	}
	// Metadata must not carry the payload body.
	for _, mv := range dec.Metadata {
		if strings.Contains(mv, "refs/heads") {
			t.Fatalf("raw ref leaked into metadata: %q", mv)
		}
	}
}

func TestSourceWebhookRejections(t *testing.T) {
	base := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	v := NewSourceWebhookVerifier("whsec_src", 60, func() time.Time { return base })
	body := []byte(`{"ref":"refs/heads/main"}`)
	good := signBody(t, "whsec_src", body)

	// Signature mismatch (tampered body).
	if _, err := v.ReceiveSourceWebhook(map[string]string{"x-hub-signature-256": good,
		"x-github-event": "push", "x-github-delivery": "d-a"}, []byte(`{"ref":"refs/heads/evil"}`), base, true); err == nil {
		t.Fatal("tampered body must reject")
	}
	// Replay: duplicate delivery id.
	h := map[string]string{"x-hub-signature-256": good, "x-github-event": "push", "x-github-delivery": "d-b"}
	if _, err := v.ReceiveSourceWebhook(h, body, base, true); err != nil {
		t.Fatalf("first delivery: %v", err)
	}
	if _, err := v.ReceiveSourceWebhook(h, body, base, true); err == nil {
		t.Fatal("duplicate delivery must reject")
	}
	// Stale timestamp window.
	v2 := NewSourceWebhookVerifier("whsec_src", 60, func() time.Time { return base })
	if _, err := v2.ReceiveSourceWebhook(map[string]string{"x-hub-signature-256": good,
		"x-github-event": "push", "x-github-delivery": "d-c"}, body, base.Add(-30*time.Minute), true); err == nil {
		t.Fatal("stale delivery must reject")
	}
	// Payload size cap.
	v3 := NewSourceWebhookVerifier("whsec_src", 60, func() time.Time { return base })
	big := make([]byte, MaxWebhookPayloadBytes+1)
	if _, err := v3.ReceiveSourceWebhook(map[string]string{"x-hub-signature-256": signBody(t, "whsec_src", big)}, big, base, true); err == nil {
		t.Fatal("oversize payload must reject")
	}
	// Rate limit (limit=2).
	v4 := NewSourceWebhookVerifier("whsec_src", 2, func() time.Time { return base })
	for i := 0; i < 2; i++ {
		b := []byte(fmt.Sprintf(`{"n":%d}`, i))
		if _, err := v4.ReceiveSourceWebhook(map[string]string{"x-hub-signature-256": signBody(t, "whsec_src", b),
			"x-github-delivery": fmt.Sprintf("d-r%d", i)}, b, base, true); err != nil {
			t.Fatalf("in-window delivery %d: %v", i, err)
		}
	}
	if _, err := v4.ReceiveSourceWebhook(map[string]string{"x-hub-signature-256": signBody(t, "whsec_src", []byte(`{"n":9}`)),
		"x-github-delivery": "d-r9"}, []byte(`{"n":9}`), base, true); err == nil {
		t.Fatal("rate limit must reject")
	}
	// Unapproved branch: webhook accepted but build NOT enqueued (never bypasses approval).
	v5 := NewSourceWebhookVerifier("whsec_src", 60, func() time.Time { return base })
	dec, err := v5.ReceiveSourceWebhook(map[string]string{"x-hub-signature-256": good,
		"x-github-event": "push", "x-github-delivery": "d-e"}, body, base, false)
	if err != nil {
		t.Fatalf("unapproved push should still verify: %v", err)
	}
	if dec.EnqueueBuild {
		t.Fatal("webhook bypassed branch approval")
	}
	if dec.Metadata["build"] == "" {
		t.Fatal("metadata must record why no build was enqueued")
	}
}

func TestAnalyticsParseAggregatePrivacy(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 30, 0, 0, time.UTC)
	lines := []string{
		fmt.Sprintf("%d \"GET / HTTP/1.1\" 200 512 \"https://www.example.com/landing\"", now.Unix()),
		fmt.Sprintf("%d \"GET /api/users HTTP/1.1\" 500 128 \"-\"", now.Unix()),
		fmt.Sprintf("%d \"GET /api/users HTTP/1.1\" 500 128 \"-\"", now.Unix()),
		fmt.Sprintf("%d \"GET /old-page HTTP/1.1\" 404 0 \"\"", now.Add(-200*24*time.Hour).Unix()),
	}
	parsed := make([]AccessLogLine, 0, len(lines))
	for _, l := range lines {
		pl, err := ParseAccessLogLine(l)
		if err != nil {
			t.Fatalf("parse %q: %v", l, err)
		}
		parsed = append(parsed, pl)
	}
	privacy := DefaultSitePrivacy()
	kept := RetainHits(parsed, privacy, now)
	if len(kept) != 3 {
		t.Fatalf("retention horizon kept %d (want 3)", len(kept))
	}
	view := AggregateAnalytics(kept, privacy)
	if view.TotalRequests != 3 {
		t.Fatalf("total: %d", view.TotalRequests)
	}
	if view.StatusCounts["5xx"] != 2 || view.StatusCounts["2xx"] != 1 {
		t.Fatalf("status counts: %v", view.StatusCounts)
	}
	if len(view.TopErrorRoutes) != 1 || view.TopErrorRoutes[0].Name != "/api/users" || view.TopErrorRoutes[0].Count != 2 {
		t.Fatalf("error routes: %+v", view.TopErrorRoutes)
	}
	if len(view.TopReferrers) != 1 || view.TopReferrers[0].Name != "www.example.com" {
		t.Fatalf("referrers: %+v", view.TopReferrers)
	}
	// Privacy off: nothing retained, nothing aggregated.
	privacy.Enabled = false
	if got := RetainHits(parsed, privacy, now); got != nil {
		t.Fatal("opt-out must disable retention")
	}
	if got := AggregateAnalytics(parsed, privacy); got.TotalRequests != 4 {
		t.Fatalf("aggregate ignores opt-out: %d", got.TotalRequests)
	}
	// Referrer privacy: host-only recording.
	if _, err := ParseAccessLogLine(fmt.Sprintf("%d \"GET / HTTP/1.1\" 200 1 \"https://user@mail.example.net/x?y=1\"", now.Unix())); err != nil {
		t.Fatal(err)
	}
	pl, _ := ParseAccessLogLine(fmt.Sprintf("%d \"GET / HTTP/1.1\" 200 1 \"https://user@mail.example.net/x?y=1\"", now.Unix()))
	if pl.ReferrerHost != "mail.example.net" || strings.Contains(pl.ReferrerHost, "user@") {
		t.Fatalf("referrer must reduce to host only: %q", pl.ReferrerHost)
	}
	// Retention bounds.
	if err := ValidateSitePrivacy(SitePrivacySettings{RetentionDays: 2}); err == nil {
		t.Fatal("retention below 7 days must reject")
	}
	// Trend buckets sorted.
	if len(view.Trend) != 1 || view.Trend[0].Count != 3 {
		t.Fatalf("trend: %+v", view.Trend)
	}
}

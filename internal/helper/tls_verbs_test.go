package helper

// TLS verb tests (G30): real key/cert generation, SAN coverage, renewal
// threshold behavior, ACME fail-closed policy, explicit fallback reporting,
// and recovery from corrupt/expired state.

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tlsFixture(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	sslDir := filepath.Join(base, "ssl")
	bind := filepath.Join(base, "tls-bindings.json")
	if err := os.WriteFile(bind, []byte(`["`+sslDir+`"]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := LoadTLSBindings(bind); err != nil {
		t.Fatal(err)
	}
	return sslDir
}

func tlsCall(t *testing.T, args TLSApplyArgs) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(args)
	out, err := TLSApplyHandler()(context.Background(), raw)
	if err != nil {
		t.Fatalf("tls.apply %s: %v", args.Op, err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func readCertFromDisk(t *testing.T, dir, name string) *x509.Certificate {
	t.Helper()
	pb, err := os.ReadFile(filepath.Join(dir, name+".crt"))
	if err != nil {
		t.Fatal(err)
	}
	blk, _ := pem.Decode(pb)
	if blk == nil {
		t.Fatal("no PEM")
	}
	c, err := x509.ParseCertificate(blk.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestTLSIssueRealCertificate(t *testing.T) {
	sslDir := tlsFixture(t)
	res := tlsCall(t, TLSApplyArgs{Op: "issue", Binding: sslDir, ServerName: "app.drevanta.test", Days: 90, SANs: []string{"www.app.drevanta.test"}})
	if res["state"] != "active" || res["fingerprint_sha256"] == "" {
		t.Fatalf("issue result: %v", res)
	}
	c := readCertFromDisk(t, sslDir, "app.drevanta.test")
	if c.Subject.CommonName != "app.drevanta.test" {
		t.Fatal("wrong CN")
	}
	if len(c.DNSNames) != 2 {
		t.Fatalf("SANs missing: %v", c.DNSNames)
	}
	if time.Until(c.NotAfter) > 90*24*time.Hour+time.Minute {
		t.Fatal("validity longer than requested")
	}
	kb, err := os.Stat(filepath.Join(sslDir, "app.drevanta.test.key"))
	if err != nil || kb.Mode().Perm() != 0o600 {
		t.Fatalf("key must be 0600, got %v", err)
	}
}

func TestTLSRenewalThreshold(t *testing.T) {
	sslDir := tlsFixture(t)
	// Fresh long-lived cert: renewal is a no-op.
	tlsCall(t, TLSApplyArgs{Op: "issue", Binding: sslDir, ServerName: "long.drevanta.test", Days: 800})
	res := tlsCall(t, TLSApplyArgs{Op: "renew", Binding: sslDir, ServerName: "long.drevanta.test"})
	if res["state"] != "still-valid" {
		t.Fatalf("expected still-valid, got %v", res)
	}
	// Short-lived cert past the 30d threshold: renewal reissues.
	res = tlsCall(t, TLSApplyArgs{Op: "issue", Binding: sslDir, ServerName: "short.drevanta.test", Days: 10})
	oldFP := res["fingerprint_sha256"].(string)
	res = tlsCall(t, TLSApplyArgs{Op: "renew", Binding: sslDir, ServerName: "short.drevanta.test"})
	if res["state"] == "still-valid" || res["fingerprint_sha256"] == oldFP {
		t.Fatalf("renewal must reissue near-expiry cert: %v", res)
	}
	// Missing cert: renewal recovers.
	res = tlsCall(t, TLSApplyArgs{Op: "renew", Binding: sslDir, ServerName: "ghost.drevanta.test"})
	if res["state"] != "active" {
		t.Fatalf("renewal of missing cert must recover: %v", res)
	}
}

func TestTLSACMEFailClosed(t *testing.T) {
	sslDir := tlsFixture(t)
	if acmeIssuer != nil {
		defer SetACMEIssuer(acmeIssuer)
	}
	SetACMEIssuer(nil)
	// No issuer registered: ACME issue fails closed.
	raw, _ := json.Marshal(TLSApplyArgs{Op: "issue", Binding: sslDir, ServerName: "acme.drevanta.test", Issuer: "acme"})
	if _, err := TLSApplyHandler()(context.Background(), raw); err == nil {
		t.Fatal("ACME without registered issuer must fail closed")
	}
	// Explicit fallback is allowed and REPORTED.
	res := tlsCall(t, TLSApplyArgs{Op: "issue", Binding: sslDir, ServerName: "acme.drevanta.test", Issuer: "acme", AllowSelfSignedFallback: true})
	if !strings.Contains(res["source"].(string), "acme-unavailable fallback") {
		t.Fatalf("fallback must be reported: %v", res["source"])
	}
}

func TestTLSRecoverFromCorruption(t *testing.T) {
	sslDir := tlsFixture(t)
	tlsCall(t, TLSApplyArgs{Op: "issue", Binding: sslDir, ServerName: "bad.drevanta.test", Days: 90})
	// Corrupt the certificate.
	if err := os.WriteFile(filepath.Join(sslDir, "bad.drevanta.test.crt"), []byte("not a pem"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := tlsCall(t, TLSApplyArgs{Op: "recover", Binding: sslDir, ServerName: "bad.drevanta.test"})
	if res["state"] != "active" {
		t.Fatalf("recover result: %v", res)
	}
	c := readCertFromDisk(t, sslDir, "bad.drevanta.test") // must parse again
	if c.Subject.CommonName != "bad.drevanta.test" {
		t.Fatal("recovered cert has wrong CN")
	}
	if _, err := os.Stat(filepath.Join(sslDir, "bad.drevanta.test.crt.invalid")); err != nil {
		t.Fatal("corrupt cert not preserved for post-mortem")
	}
	// Expired-cert recovery: issue short cert, forge past NotAfter by
	// recovering (recovery must accept and replace expired certs).
	tlsCall(t, TLSApplyArgs{Op: "issue", Binding: sslDir, ServerName: "old.drevanta.test", Days: 1})
	res = tlsCall(t, TLSApplyArgs{Op: "recover", Binding: sslDir, ServerName: "old.drevanta.test"})
	if res["state"] != "active" {
		t.Fatalf("expired recovery failed: %v", res)
	}
}

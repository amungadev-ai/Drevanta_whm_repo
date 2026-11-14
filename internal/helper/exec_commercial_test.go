package helper

// Tests: scoped terminal sessions (WEB-07) + commercial configuration (DEC-022).

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func execFixture(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "site")
	os.MkdirAll(root, 0o750)
	os.WriteFile(filepath.Join(base, "site-bindings.json"), []byte(`{"site":"`+root+`"}`), 0o600)
	LoadFileBindings(filepath.Join(base, "site-bindings.json"))
	return root
}

func TestExecSessionAuditedSandboxed(t *testing.T) {
	execFixture(t) // registers the site binding
	const siteBinding = "site"
	h := ExecSessionHandler()
	// Unknown interpreter rejects.
	raw, _ := json.Marshal(ExecSessionArgs{Binding: siteBinding, Command: "rm"})
	if _, err := h(context.Background(), raw); err == nil {
		t.Fatal("rm must not be an allowlisted interpreter")
	}
	// Shell metacharacter arguments reject.
	raw, _ = json.Marshal(ExecSessionArgs{Binding: siteBinding, Command: "sh", Args: []string{"-c", "evil\nnext"}})
	if _, err := h(context.Background(), raw); err == nil {
		t.Fatal("newline in args must reject")
	}
	// Without isolation (CI) the session refuses — same fail-closed contract
	// as builds. The real sandbox path is verified in the VPS drill.
	raw, _ = json.Marshal(ExecSessionArgs{Binding: siteBinding, Command: "sh", Args: []string{"-c", "echo hi"}})
	if _, err := h(context.Background(), raw); err == nil {
		t.Log("isolation available in this environment (unexpected in CI)")
	} else if !strings.Contains(err.Error(), "isolation probe failed") {
		t.Fatalf("expected probe refusal, got: %v", err)
	}
}

func TestCommercialConfigValidation(t *testing.T) {
	ok := CommercialConfig{Currency: "EUR", TaxMode: "exclusive", TaxRatePercent: 21,
		TaxLabel: "VAT", SLAUptimePercent: 99.9, BackupRetentionDays: 30,
		GracePeriodDays: 7, InvoicePrefix: "INV", LegalFooter: "Seller GmbH, DE", IsProduction: true}
	if err := ValidateCommercialConfig(ok); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	bad := []CommercialConfig{
		{Currency: "eur", TaxMode: "exclusive", BackupRetentionDays: 30, GracePeriodDays: 1, InvoicePrefix: "INV"},
		{Currency: "EUR", TaxMode: "other", BackupRetentionDays: 30, GracePeriodDays: 1, InvoicePrefix: "INV"},
		{Currency: "EUR", TaxMode: "exclusive", TaxRatePercent: 21, BackupRetentionDays: 30, GracePeriodDays: 1, InvoicePrefix: "INV"}, // no label
		{Currency: "EUR", TaxMode: "exclusive", BackupRetentionDays: 0, GracePeriodDays: 1, InvoicePrefix: "INV"},
		{Currency: "EUR", TaxMode: "exclusive", BackupRetentionDays: 30, GracePeriodDays: 1, InvoicePrefix: "", IsProduction: false},
		{Currency: "EUR", TaxMode: "exclusive", BackupRetentionDays: 30, GracePeriodDays: 1, InvoicePrefix: "INV", IsProduction: true}, // no footer
	}
	for i, b := range bad {
		if err := ValidateCommercialConfig(b); err == nil {
			t.Fatalf("invalid config %d accepted", i)
		}
	}
}

func TestComputeInvoiceTotalTaxModes(t *testing.T) {
	exclusive := CommercialConfig{TaxMode: "exclusive", TaxRatePercent: 21}
	total, tax := ComputeInvoiceTotal(10000, exclusive)
	if total != 12100 || tax != 2100 {
		t.Fatalf("exclusive: %d %d", total, tax)
	}
	inclusive := CommercialConfig{TaxMode: "inclusive", TaxRatePercent: 21}
	total, tax = ComputeInvoiceTotal(12100, inclusive)
	if total != 12100 {
		t.Fatalf("inclusive total must stay: %d", total)
	}
	if tax < 2099 || tax > 2101 { // rounding tolerance on extraction
		t.Fatalf("inclusive tax extraction: %d", tax)
	}
	zero := CommercialConfig{TaxMode: "exclusive", TaxRatePercent: 0}
	total, tax = ComputeInvoiceTotal(10000, zero)
	if total != 10000 || tax != 0 {
		t.Fatalf("zero-rated: %d %d", total, tax)
	}
}

func TestExecSessionOpenWriteCloseLifecycle(t *testing.T) {
	execFixture(t)
	const siteBinding = "site"
	// Interpreter allowlist applies to interactive shells too.
	raw, _ := json.Marshal(ExecOpenArgs{Binding: siteBinding, Shell: "zsh"})
	if _, err := ExecOpenHandler()(context.Background(), raw); err == nil {
		t.Fatal("zsh must not be allowlisted")
	}
	// Without isolation (CI): fail closed with the probe refusal.
	raw, _ = json.Marshal(ExecOpenArgs{Binding: siteBinding, Shell: "sh"})
	if _, err := ExecOpenHandler()(context.Background(), raw); err == nil {
		t.Log("isolation available (unexpected in CI)")
	}
	// Write/close on unknown sessions reject.
	raw, _ = json.Marshal(ExecWriteArgs{Session: "term_missing", Input: "ls\n"})
	if _, err := ExecWriteHandler()(context.Background(), raw); err == nil {
		t.Fatal("unknown session write must reject")
	}
	raw, _ = json.Marshal(ExecCloseArgs{Session: "term_missing"})
	if _, err := ExecCloseHandler()(context.Background(), raw); err == nil {
		t.Fatal("unknown session close must reject")
	}
}

func TestCacheApplyGeneratedAuth(t *testing.T) {
	root := infraFixture(t, LoadCacheBindings)
	SetInfraRunners(func(_ context.Context, packages string) error { return nil },
		func(_ context.Context, action, unit string) error { return nil })
	defer SetInfraRunners(nil, nil)
	res := cacheCall(t, CacheApplyArgs{Op: "provision", Binding: root, Engine: "redis",
		Instance: "auth-cache", MaxMemMB: 64, GeneratePW: true})
	if res["auth_required"] != true {
		t.Fatalf("auth_required: %v", res)
	}
	gen := res["generated_password"].(string)
	cfg, _ := os.ReadFile(filepath.Join(root, "auth-cache", "redis.conf"))
	if !strings.Contains(string(cfg), "requirepass "+gen) {
		t.Fatal("requirepass not written with generated credential")
	}
}

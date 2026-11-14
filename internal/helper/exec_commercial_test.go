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

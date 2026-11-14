package helper

// Scoped terminal sessions (WEB-07) — DEC-016-RULING. A terminal session is a
// SCOPE-PINNED, audited, resource-limited command execution inside a site
// root: never an interactive root shell. Sessions run under the sandbox
// (same boundary as builds: no-network option, non-root, limits, timeout),
// the full command line is journaled, and only allowlisted interpreters run.

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ExecSessionArgs is the typed argument for exec.session.
type ExecSessionArgs struct {
	Binding     string   `json:"binding"`                 // site root binding
	Command     string   `json:"command"`                 // single command word (allowlisted interpreter)
	Args        []string `json:"args,omitempty"`          // typed arguments (no shell)
	TimeoutS    int      `json:"timeout_s,omitempty"`     // default 30, max 300
	AllowNet    bool     `json:"allow_net,omitempty"`     // default: NO network
	MaxOutputKB int      `json:"max_output_kb,omitempty"` // output cap (default 64)
}

var execInterpreters = map[string]bool{
	"php": true, "python3": true, "node": true, "sh": true,
}

// ExecSessionHandler runs one audited, sandboxed command in a site root.
// Interactive PTY streaming is a follow-on wave; this is the audited
// execution core with the exact trust boundaries the UI terminal uses.
func ExecSessionHandler() VerbHandler {
	return func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		dec := json.NewDecoder(strings.NewReader(string(args)))
		dec.DisallowUnknownFields()
		var a ExecSessionArgs
		if err := dec.Decode(&a); err != nil {
			return nil, fmt.Errorf("invalid exec.session arguments")
		}
		root, ok := fileBindingRoots[a.Binding]
		if !ok {
			return nil, fmt.Errorf("binding not allowlisted")
		}
		if !execInterpreters[a.Command] {
			return nil, fmt.Errorf("interpreter not allowlisted")
		}
		if len(a.Args) > 32 {
			return nil, fmt.Errorf("too many arguments")
		}
		for _, arg := range a.Args {
			if strings.ContainsAny(arg, "\n\x00") {
				return nil, fmt.Errorf("invalid argument")
			}
		}
		timeout := a.TimeoutS
		if timeout <= 0 {
			timeout = 30
		}
		if timeout > 300 {
			timeout = 300
		}
		cctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
		// Isolation: identical contract to builds — probe first, fail closed.
		if err := isolationProbeFn(); err != nil {
			return nil, err
		}
		argv := []string{a.Command}
		argv = append(argv, a.Args...)
		runArgs := []string{"--quiet", "--wait",
			"-p", "CPUQuota=50%", "-p", "MemoryMax=512M", "-p", "TasksMax=64",
			"--uid=nobody", "-p", "WorkingDirectory=" + root}
		if !a.AllowNet {
			runArgs = append(runArgs, "-p", "PrivateNetwork=yes")
		}
		for _, e := range buildEnv(root) {
			runArgs = append(runArgs, "-E", e)
		}
		runArgs = append(runArgs, "--")
		runArgs = append(runArgs, argv...)
		cmd := exec.CommandContext(cctx, "systemd-run", runArgs...)
		var out strings.Builder
		cmd.Stdout = &out
		cmd.Stderr = &out
		runErr := cmd.Run()
		exit := 0
		if runErr != nil {
			if cmd.ProcessState != nil {
				exit = cmd.ProcessState.ExitCode()
			} else {
				exit = -1
			}
		}
		cap := a.MaxOutputKB
		if cap <= 0 {
			cap = 64
		}
		output := out.String()
		if len(output) > cap*1024 {
			output = output[:cap*1024]
		}
		// Full command line + exit code journaled by the registry; output is
		// returned but size-capped.
		return json.Marshal(map[string]any{
			"binding": a.Binding, "command": strings.Join(argv, " "),
			"exit_code": exit, "output": output,
			"isolated": true, "network": a.AllowNet,
		})
	}
}

// ---- Commercial configuration (DEC-022): platform/organization/plan-scoped
// configuration with validation and change audit — service side (real DB). ----

// CommercialConfig is a validated commercial policy bundle. Values live in the
// platform/organization/plan scopes; NOTHING silently falls back to fixtures.
type CommercialConfig struct {
	Currency            string  `json:"currency"`            // ISO-4217
	TaxMode             string  `json:"tax_mode"`            // exclusive | inclusive
	TaxRatePercent      float64 `json:"tax_rate_percent"`    // 0 = zero-rated (explicit)
	TaxLabel            string  `json:"tax_label,omitempty"` // e.g. "VAT"
	SLAUptimePercent    float64 `json:"sla_uptime_percent"`  // 0 = no SLA stated
	BackupRetentionDays int     `json:"backup_retention_days"`
	GracePeriodDays     int     `json:"nonpayment_grace_days"`
	InvoicePrefix       string  `json:"invoice_prefix"`
	LegalFooter         string  `json:"legal_footer,omitempty"`
	IsProduction        bool    `json:"is_production"` // false = development defaults in effect
}

// ValidateCommercialConfig enforces DEC-022 constraints. A production
// configuration must not carry non-production defaults.
func ValidateCommercialConfig(c CommercialConfig) error {
	if len(c.Currency) != 3 || strings.ToUpper(c.Currency) != c.Currency {
		return fmt.Errorf("currency must be ISO-4217 alpha-3 uppercase")
	}
	if c.TaxMode != "exclusive" && c.TaxMode != "inclusive" {
		return fmt.Errorf("tax_mode must be exclusive or inclusive")
	}
	if c.TaxRatePercent < 0 || c.TaxRatePercent > 100 {
		return fmt.Errorf("tax rate out of range")
	}
	if c.TaxRatePercent > 0 && c.TaxLabel == "" {
		return fmt.Errorf("non-zero tax requires a label (e.g. VAT)")
	}
	if c.SLAUptimePercent < 0 || c.SLAUptimePercent > 100 {
		return fmt.Errorf("SLA uptime out of range")
	}
	if c.BackupRetentionDays < 1 || c.BackupRetentionDays > 3650 {
		return fmt.Errorf("backup retention out of range")
	}
	if c.GracePeriodDays < 0 || c.GracePeriodDays > 90 {
		return fmt.Errorf("grace period out of range")
	}
	if c.InvoicePrefix == "" || len(c.InvoicePrefix) > 16 || strings.ContainsAny(c.InvoicePrefix, " \t\n/") {
		return fmt.Errorf("invalid invoice prefix")
	}
	if c.IsProduction {
		// DEC-022: production must be explicit about legal text.
		if c.LegalFooter == "" {
			return fmt.Errorf("production config requires legal footer text")
		}
	}
	return nil
}

// ComputeInvoiceTotal applies the configured tax rules to a subtotal
// (amounts in minor units). Tax-exclusive adds tax; inclusive extracts it.
func ComputeInvoiceTotal(subtotalCents int64, c CommercialConfig) (totalCents, taxCents int64) {
	if c.TaxMode == "inclusive" {
		// tax is inside the subtotal: extract it
		if c.TaxRatePercent == 0 {
			return subtotalCents, 0
		}
		tax := subtotalCents - int64(float64(subtotalCents)/(1+c.TaxRatePercent/100))
		return subtotalCents, tax
	}
	tax := int64(float64(subtotalCents) * c.TaxRatePercent / 100)
	return subtotalCents + tax, tax
}

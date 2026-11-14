package jobs

// TLS outcome observation (G30): successful tls.issue/tls.renew operations
// update real tls_certificate records from the helper's typed result.
// Observer errors never fail the operation pipeline.

import (
	"context"
	"encoding/json"
	"time"

	"drevanta/internal/domains"
	"drevanta/internal/operations"
)

// TlsObserver records TLS outcomes into the tls_certificate store.
type TlsObserver struct {
	Service *domains.TlsService
}

// Observe implements the Worker.Observer seam.
func (t *TlsObserver) Observe(ctx context.Context, op *operations.Operation, outcome, resultJSON string) error {
	if outcome != "succeeded" {
		if (op.ActionID == "tls.issue" || op.ActionID == "tls.renew") && resultJSON != "" {
			// Failed issuance keeps a failed record only if the scope is
			// derivable from the operation.
			_ = t.Service.MarkFailed(ctx, op.ScopeID, tlsServerName(op))
			return nil
		}
		return nil
	}
	switch op.ActionID {
	case "tls.issue", "tls.renew":
	default:
		return nil
	}
	if resultJSON == "" {
		return nil
	}
	var res struct {
		ServerName        string `json:"server_name"`
		FingerprintSHA256 string `json:"fingerprint_sha256"`
		NotAfter          string `json:"not_after"`
		Source            string `json:"source"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &res); err != nil {
		return nil // unparsable: record nothing, invent nothing
	}
	if res.ServerName == "" {
		res.ServerName = tlsServerName(op)
	}
	if res.ServerName == "" {
		return nil
	}
	source := res.Source
	if source == "" {
		source = "selfsigned"
	}
	notAfter := res.NotAfter
	if notAfter == "" {
		notAfter = time.Now().UTC().AddDate(0, 0, 825).Format(time.RFC3339)
	}
	return t.Service.RecordIssued(ctx, op.ScopeID, res.ServerName, res.FingerprintSHA256, notAfter, source)
}

// tlsServerName extracts the requested server name from the intent payload.
func tlsServerName(op *operations.Operation) string {
	if op.Payload == "" {
		return ""
	}
	var p struct {
		Arguments struct {
			ServerName string `json:"server_name"`
		} `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(op.Payload), &p); err != nil {
		return ""
	}
	return p.Arguments.ServerName
}

// ObserverList fans a terminal outcome out to several observers; individual
// observer errors are swallowed (pipeline availability first), matching the
// Worker.Observer contract.
type ObserverList []func(ctx context.Context, op *operations.Operation, outcome, resultJSON string) error

// Observe implements the Worker.Observer seam.
func (l ObserverList) Observe(ctx context.Context, op *operations.Operation, outcome, resultJSON string) error {
	for _, o := range l {
		_ = o(ctx, op, outcome, resultJSON)
	}
	return nil
}

package jobs

// TLS outcome observer tests (G30): real tls.apply results upsert certificate
// records; failures mark failed; the composite keeps pipeline semantics.

import (
	"context"
	"testing"

	"drevanta/internal/domains"
	"drevanta/internal/operations"
)

func TestTlsObserverRecordsRealOutcomes(t *testing.T) {
	f := newJobsFixture(t)
	tls := domains.NewTlsService(f.db)
	o := &TlsObserver{Service: tls}
	ctx := context.Background()
	op := &operations.Operation{OperationID: "op_t1", ScopeID: "ws_x", ActionID: "tls.issue",
		Payload: `{"arguments":{"server_name":"renew.drevanta.test"}}`}
	result := `{"server_name":"renew.drevanta.test","fingerprint_sha256":"abc123","not_after":"2027-01-01T00:00:00Z","source":"selfsigned"}`
	if err := o.Observe(ctx, op, "succeeded", result); err != nil {
		t.Fatal(err)
	}
	certs, err := tls.ListCertificates(ctx, "ws_x")
	if err != nil || len(certs) != 1 {
		t.Fatalf("list: %v %d", err, len(certs))
	}
	if certs[0]["server_name"] != "renew.drevanta.test" || certs[0]["state"] != "active" {
		t.Fatalf("cert row: %v", certs[0])
	}
	// Renewal with a new fingerprint updates in place.
	if err := o.Observe(ctx, &operations.Operation{OperationID: "op_t2", ScopeID: "ws_x", ActionID: "tls.renew"},
		"succeeded", `{"server_name":"renew.drevanta.test","fingerprint_sha256":"def456","not_after":"2027-06-01T00:00:00Z","source":"selfsigned (renewal)"}`); err != nil {
		t.Fatal(err)
	}
	certs, _ = tls.ListCertificates(ctx, "ws_x")
	if len(certs) != 1 || certs[0]["fingerprint"] != "def456" {
		t.Fatalf("renewal did not update in place: %v", certs)
	}
	// Failure marks failed.
	if err := o.Observe(ctx, op, "failed", `{}`); err != nil {
		t.Fatal(err)
	}
	certs, _ = tls.ListCertificates(ctx, "ws_x")
	if certs[0]["state"] != "failed" {
		t.Fatalf("failure not recorded: %v", certs[0])
	}
}

func TestObserverListSwallowsIndividualErrors(t *testing.T) {
	f := newJobsFixture(t)
	l := ObserverList{
		func(ctx context.Context, op *operations.Operation, outcome, resultJSON string) error {
			return context.DeadlineExceeded // must not propagate
		},
		(&TlsObserver{Service: domains.NewTlsService(f.db)}).Observe,
	}
	if err := l.Observe(context.Background(),
		&operations.Operation{OperationID: "op_t3", ScopeID: "ws_x", ActionID: "tls.issue"},
		"succeeded", `{"server_name":"x.drevanta.test","fingerprint_sha256":"f","not_after":"2027-01-01T00:00:00Z","source":"selfsigned"}`); err != nil {
		t.Fatalf("composite observer must not propagate: %v", err)
	}
}

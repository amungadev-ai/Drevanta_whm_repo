package jobs

// G22 verification suite: durable jobs, leases, fences, crash
// reconciliation, outbox events, ambiguity handling (phase 22, DEC-009).

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"drevanta/internal/audit"
	"drevanta/internal/authn"
	"drevanta/internal/authz"
	"drevanta/internal/operations"
	"drevanta/internal/store"
)

type jobsFixture struct {
	t     *testing.T
	db    *store.DB
	ops   *operations.Engine
	authz *authz.Checker
	auth  *authn.Service
}

func newJobsFixture(t *testing.T) *jobsFixture {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(context.Background(), "../../migrations"); err != nil {
		t.Fatal(err)
	}
	return &jobsFixture{t: t, db: db, ops: operations.NewEngine(db), authz: authz.NewChecker(db),
		auth: authn.NewService(db, time.Hour, 30*time.Minute)}
}

// recordingExecutor simulates domain effects with scripted outcomes per call.
type recordingExecutor struct {
	calls []recCall
	// behavior: on each Execute, pop next outcome (default succeeded).
	script []execOutcome
}

type recCall struct {
	OpID   string
	Fence  int64
	Action string
}

type execOutcome struct {
	outcome string
	result  string
	gen     *int64
	err     error
}

func (e *recordingExecutor) Execute(_ context.Context, op *operations.Operation, fence int64) (string, string, *int64, error) {
	e.calls = append(e.calls, recCall{OpID: op.OperationID, Fence: fence, Action: op.ActionID})
	if len(e.script) == 0 {
		gen := op.ExpectedGeneration + 1
		return "succeeded", `{"ok":true}`, &gen, nil
	}
	o := e.script[0]
	e.script = e.script[1:]
	return o.outcome, o.result, o.gen, o.err
}

func (f *jobsFixture) seedResource() (*store.Resource, *operations.Accepted) {
	f.t.Helper()
	ctx := context.Background()
	email := "owner-" + store.NewID("e")[2:10] + "@example.test"
	p, err := f.db.CreatePrincipal(ctx, email, "Owner")
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.auth.SetPrimaryPassword(ctx, p.PrincipalID, "Whatever-Long-Password-1!"); err != nil {
		f.t.Fatal(err)
	}
	ws, err := f.db.CreateWorkspace(ctx, "website", p.PrincipalID, "ws")
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.db.AddMembership(ctx, store.Membership{
		PrincipalID: p.PrincipalID, WorkspaceID: ws.WorkspaceID,
		RoleID: "website.owner", Revision: 1, State: "active",
	}); err != nil {
		f.t.Fatal(err)
	}
	res, err := f.db.CreateResource(ctx, ws.WorkspaceID, "", "filesystem entry", "site")
	if err != nil {
		f.t.Fatal(err)
	}
	intent := operations.Intent{
		ActionID: "file.write", ResourceType: "filesystem entry",
		ExpectedOwnerRevision: 1, ExpectedGeneration: 0,
	}
	acc, err := f.ops.Accept(ctx, p.PrincipalID, res, intent, "seed-key")
	if err != nil {
		f.t.Fatal(err)
	}
	return res, acc
}

// full run authorization: allow everything (subject already checked at accept).
func allowAll(ctx context.Context, op *operations.Operation) error { return nil }

func TestWorkerSucceedsWithObservation(t *testing.T) {
	f := newJobsFixture(t)
	res, acc := f.seedResource()
	ex := &recordingExecutor{}
	w := NewWorker(f.db, f.ops, ex, allowAll, audit.NewWriter(f.db), time.Minute)

	n, err := w.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("processed %d", n)
	}
	op, _ := f.ops.Get(context.Background(), acc.OperationID)
	if op.State != "succeeded" {
		t.Fatalf("state: %q", op.State)
	}
	after, _ := f.db.ResourceByID(context.Background(), res.ResourceID)
	if after.ObservedGeneration != 1 {
		t.Fatalf("observed generation: %d", after.ObservedGeneration)
	}
	// Terminal outbox event recorded exactly once.
	var outbox int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE operation_id=? AND type='operation.succeeded'`,
		acc.OperationID).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if outbox != 1 {
		t.Fatalf("outbox events: %d", outbox)
	}
	// A second run processes nothing (no due work).
	n, err = w.RunOnce(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("second run: n=%d err=%v", n, err)
	}
}

func TestWorkerCrashReconciliationReleasesLease(t *testing.T) {
	f := newJobsFixture(t)
	_, acc := f.seedResource()
	ex := &recordingExecutor{}
	worker := NewWorker(f.db, f.ops, ex, allowAll, audit.NewWriter(f.db), 1*time.Second)

	// First dispatch: executor "crashes" (never returns terminal evidence —
	// we simulate by not running the worker's terminal transition). Achieve
	// this by running the guard + executor manually and leaving 'dispatched'.
	ctx := context.Background()
	now := time.Now().UTC()
	fence, err := f.ops.DispatchGuard(ctx, acc.OperationID, now.Add(-time.Hour), allowAll) // lease already in the past
	if err != nil {
		t.Fatal(err)
	}
	if fence != 1 {
		t.Fatalf("first fence: %d", fence)
	}
	_, _, _, _ = ex.Execute(ctx, mustGet(t, f, acc.OperationID), fence)
	// Worker crashed before terminal transition → state remains dispatched.

	expired, err := worker.ReconcileExpired(ctx)
	if err != nil || expired != 1 {
		t.Fatalf("reconcile: %d %v", expired, err)
	}

	// Next RunOnce re-dispatches with a higher fence; the executor observes
	// prior completion and reports the journaled outcome (idempotent).
	n, err := worker.RunOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("processed %d", n)
	}
	op, _ := f.ops.Get(ctx, acc.OperationID)
	if op.State != "succeeded" {
		t.Fatalf("state after reconciliation: %q", op.State)
	}
	if len(ex.calls) != 2 {
		t.Fatalf("executor calls: %d", len(ex.calls))
	}
	if ex.calls[1].Fence != 2 {
		t.Fatalf("second fence: %d", ex.calls[1].Fence)
	}
	// Exactly one accepted + one terminal outbox event each phase.
	var events int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE operation_id=?`, acc.OperationID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 2 { // accepted + succeeded
		t.Fatalf("outbox events: %d", events)
	}
}

func TestWorkerFailedAndUncertainOutcomes(t *testing.T) {
	f := newJobsFixture(t)
	gen := int64(0)
	ex := &recordingExecutor{script: []execOutcome{
		{outcome: "failed", result: `{"safe_reason":"disk quota exceeded"}`},
		{outcome: "", result: "", gen: &gen, err: context.DeadlineExceeded},
	}}
	w := NewWorker(f.db, f.ops, ex, allowAll, audit.NewWriter(f.db), time.Minute)
	ctx := context.Background()

	_, acc1 := f.seedResource()
	_, acc2 := f.seedResource()
	if _, err := w.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	op1, _ := f.ops.Get(ctx, acc1.OperationID)
	if op1.State != "failed" {
		t.Fatalf("op1: %q", op1.State)
	}
	op2, _ := f.ops.Get(ctx, acc2.OperationID)
	if op2.State != "uncertain" {
		t.Fatalf("op2: %q", op2.State)
	}
	// Uncertain blocks unsafe automated retry: next run processes nothing.
	n, err := w.RunOnce(ctx)
	if err != nil || n != 0 {
		t.Fatalf("retry after uncertain: n=%d err=%v", n, err)
	}
}

// Revoked actor's queued operation cancels at dispatch (control-plane part of
// TC-P18-01, now exercised through the worker).
func TestWorkerRevokedActorCancelled(t *testing.T) {
	f := newJobsFixture(t)
	ctx := context.Background()
	p, err := f.db.CreatePrincipal(ctx, "dev@example.test", "Dev")
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.auth.SetPrimaryPassword(ctx, p.PrincipalID, "Whatever-Long-Password-1!"); err != nil {
		f.t.Fatal(err)
	}
	ownerWS, _ := f.db.CreateWorkspace(ctx, "website", "no-such-owner", "ws")
	_ = ownerWS
	ws, err := f.db.CreateWorkspace(ctx, "website", p.PrincipalID, "ws")
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.db.AddMembership(ctx, store.Membership{
		PrincipalID: p.PrincipalID, WorkspaceID: ws.WorkspaceID,
		RoleID: "website.developer", Revision: 1, State: "active",
	}); err != nil {
		f.t.Fatal(err)
	}
	res, err := f.db.CreateResource(ctx, ws.WorkspaceID, "", "filesystem entry", "site")
	if err != nil {
		f.t.Fatal(err)
	}
	intent := operations.Intent{ActionID: "file.write", ResourceType: "filesystem entry",
		ExpectedOwnerRevision: 1, ExpectedGeneration: 0}
	acc, err := f.ops.Accept(ctx, p.PrincipalID, res, intent, "")
	if err != nil {
		f.t.Fatal(err)
	}

	// Revoke, then run worker with a real reauthorizer.
	if err := f.db.RevokeMembership(ctx, p.PrincipalID, ws.WorkspaceID, "website.developer"); err != nil {
		f.t.Fatal(err)
	}
	ex := &recordingExecutor{}
	w := NewWorker(f.db, f.ops, ex, func(ctx context.Context, op *operations.Operation) error {
		_, err := f.authz.Authorize(ctx, op.ActorID, op.ActionID, "filesystem entry", op.ScopeID, 1)
		return err
	}, audit.NewWriter(f.db), time.Minute)
	if _, err := w.RunOnce(ctx); err != nil && err != ErrDispatchDenied {
		t.Fatal(err)
	}
	op, _ := f.ops.Get(ctx, acc.OperationID)
	if op.State != "cancelled" {
		t.Fatalf("state: %q", op.State)
	}
	if len(ex.calls) != 0 {
		t.Fatalf("executor invoked for cancelled op: %d", len(ex.calls))
	}
}

func mustGet(t *testing.T, f *jobsFixture, opID string) *operations.Operation {
	t.Helper()
	op, err := f.ops.Get(context.Background(), opID)
	if err != nil {
		t.Fatal(err)
	}
	return op
}

// seedPrincipalWorkspace creates an owner with a website workspace.
func (f *jobsFixture) seedPrincipalWorkspace() (struct {
	PrincipalID string
	WorkspaceID string
}, error) {
	var out struct {
		PrincipalID string
		WorkspaceID string
	}
	ctx := context.Background()
	email := "owner-" + store.NewID("e")[2:10] + "@example.test"
	p, err := f.db.CreatePrincipal(ctx, email, "Owner")
	if err != nil {
		return out, err
	}
	if err := f.auth.SetPrimaryPassword(ctx, p.PrincipalID, "Whatever-Long-Password-1!"); err != nil {
		return out, err
	}
	ws, err := f.db.CreateWorkspace(ctx, "website", p.PrincipalID, "ws")
	if err != nil {
		return out, err
	}
	if err := f.db.AddMembership(ctx, store.Membership{
		PrincipalID: p.PrincipalID, WorkspaceID: ws.WorkspaceID,
		RoleID: "website.owner", Revision: 1, State: "active",
	}); err != nil {
		return out, err
	}
	out.PrincipalID = p.PrincipalID
	out.WorkspaceID = ws.WorkspaceID
	return out, nil
}

// seedResourceIn creates a resource inside the given workspace.
func (f *jobsFixture) seedResourceIn(workspaceID, typ, name string) *store.Resource {
	res, err := f.db.CreateResource(context.Background(), workspaceID, "", typ, name)
	if err != nil {
		f.t.Fatal(err)
	}
	return res
}

// operationsIntentFor builds a canonical intent for the resource.
func operationsIntentFor(res *store.Resource, action string, args map[string]any) operations.Intent {
	return operations.Intent{
		ActionID:              action,
		ResourceType:          res.Type,
		ExpectedOwnerRevision: res.OwnerRevision,
		ExpectedGeneration:    res.DesiredGeneration,
		Arguments:             args,
	}
}

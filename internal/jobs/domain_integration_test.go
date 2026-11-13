package jobs

// Domain wave integration: intent → worker → helper verb → observed effect.
// Uses an in-process helper registry over a real Unix socket (the same path
// verified on the VPS drill).

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"drevanta/internal/audit"
	"drevanta/internal/helper"
	"drevanta/internal/hostagent"
	"drevanta/internal/store"
)

func TestFileWriteIntentToHelperEffect(t *testing.T) {
	f := newJobsFixture(t)
	ctx := context.Background()

	// Site root binding for the helper.
	dir := t.TempDir()
	siteRoot := filepath.Join(dir, "site")
	if err := os.MkdirAll(siteRoot, 0o750); err != nil {
		t.Fatal(err)
	}
	helper.LoadFileBindingsForTest(map[string]string{"site-alpha": siteRoot})

	// Helper registry on a real socket.
	socket := filepath.Join(dir, "h.sock")
	hdb, err := store.Open(filepath.Join(dir, "helper.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer hdb.Close()
	if err := hdb.Migrate(ctx, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	const instID = "inst_domtest00000000000000000000000"
	policy := &helper.Policy{Verbs: map[string][]string{
		"file.apply": {"*"},
	}}
	reg := helper.NewRegistry(policy, "", &helper.SQLiteJournal{DB: hdb}, instID, map[int]bool{0: true})
	reg.Register("file.apply", helper.FileApplyHandler())
	sctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { _ = reg.Serve(sctx, socket, "") }()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := net.Dial("unix", socket); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Seed owner + website workspace + filesystem resource.
	p, err := f.seedPrincipalWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	res := f.seedResourceIn(p.WorkspaceID, "filesystem entry", "public_html")
	intent := operationsIntentFor(res, "file.write", map[string]any{
		"binding": "site-alpha", "rel_path": "index.html",
		"op": "write", "content": "<h1>Drevanta</h1>", "mode": "0644",
	})
	acc, err := f.ops.Accept(ctx, p.PrincipalID, res, intent, "file-key-1")
	if err != nil {
		t.Fatal(err)
	}

	// Worker with helper-backed executor.
	client := &hostagent.Client{SocketPath: socket, Timeout: 5 * time.Second}
	exec := &hostagent.VerbExecutor{Client: client, HostID: "*", InstallID: instID}
	w := NewWorker(f.db, f.ops, exec, allowAll, audit.NewWriter(f.db), time.Minute)
	if _, err := w.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}

	op, _ := f.ops.Get(ctx, acc.OperationID)
	if op.State != "succeeded" {
		t.Fatalf("state: %q result=%s", op.State, op.ResultJSON)
	}
	data, err := os.ReadFile(filepath.Join(siteRoot, "index.html"))
	if err != nil || string(data) != "<h1>Drevanta</h1>" {
		t.Fatalf("effect: %q err=%v result=%s", data, err, op.ResultJSON)
	}

	// The same intent replayed (same key) returns the original operation and
	// does NOT double-write (idempotency at the acceptance boundary).
	acc2, err := f.ops.Accept(ctx, p.PrincipalID, res, intent, "file-key-1")
	if err != nil {
		t.Fatal(err)
	}
	if acc2.OperationID != acc.OperationID || !acc2.Replayed {
		t.Fatalf("replay: %+v", acc2)
	}

	// Helper journal has exactly one effect record for the operation.
	var hjdb = hdb
	var n int
	if err := hjdb.QueryRow(`SELECT COUNT(*) FROM host_journal WHERE operation_id=?`, acc.OperationID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("helper journal rows: %d", n)
	}
}

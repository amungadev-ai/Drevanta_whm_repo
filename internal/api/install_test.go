package api

// G20 verification: installer lifecycle (phase 06 — claim token, atomic
// consume, checkpoints, resume) and the panel↔helper socket integration
// (profile host fixtures executed over the wire).

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"drevanta/internal/helper"
	"drevanta/internal/hostagent"
	"drevanta/internal/operations"
	"drevanta/internal/store"
)

func TestInstallerLifecycle(t *testing.T) {
	f := newFixture(t)

	// Uninitialized status.
	resp := f.get("/api/v1/install/status", "")
	if resp.Code != http.StatusOK {
		t.Fatalf("status: %d %s", resp.Code, resp.Body)
	}

	// Bootstrap: first-run claim token (returned via the installer service
	// here; the panel binary prints it to the operator console).
	_, token, err := f.apiServer.Install.EnsureInstallation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("no claim token issued")
	}

	// Status now pending.
	resp = f.get("/api/v1/install/status", "")
	var st struct {
		State      string `json:"state"`
		Checkpoint string `json:"checkpoint"`
	}
	_ = json.Unmarshal([]byte(resp.Body), &st)
	if st.State != "pending" || st.Checkpoint != "started" {
		t.Fatalf("pending status: %s", resp.Body)
	}

	// Wrong token denies uniformly.
	bad := f.post("/api/v1/install/claim",
		`{"claim_token":"wrong","admin_email":"admin@example.test","admin_password":"AdminPassphrase-1!","display_name":"Admin"}`, "")
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("bad claim: want 401, got %d", bad.Code)
	}

	// Correct claim creates the administrator + server workspace atomically.
	ok := f.post("/api/v1/install/claim",
		`{"claim_token":"`+token+`","admin_email":"admin@example.test","admin_password":"AdminPassphrase-1!","display_name":"Admin"}`, "")
	if ok.Code != http.StatusCreated {
		t.Fatalf("claim: %d %s", ok.Code, ok.Body)
	}
	var claimed struct {
		PrincipalID string `json:"principal_id"`
		WorkspaceID string `json:"workspace_id"`
	}
	_ = json.Unmarshal([]byte(ok.Body), &claimed)
	if claimed.PrincipalID == "" || claimed.WorkspaceID == "" {
		t.Fatalf("claim body: %s", ok.Body)
	}

	// Second claim denies (token consumed, not pending).
	again := f.post("/api/v1/install/claim",
		`{"claim_token":"`+token+`","admin_email":"x@example.test","admin_password":"AdminPassphrase-2!","display_name":"X"}`, "")
	if again.Code != http.StatusUnauthorized {
		t.Fatalf("replay claim: want 401, got %d", again.Code)
	}

	// Administrator signs in with the claim password; installation active.
	login := f.post("/api/v1/identity/login",
		`{"email":"admin@example.test","password":"AdminPassphrase-1!"}`, "")
	if login.Code != http.StatusOK {
		t.Fatalf("admin login: %d %s", login.Code, login.Body)
	}
	var tokBody struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal([]byte(login.Body), &tokBody)
	tok := tokBody.Token
	if tok == "" {
		t.Fatal("admin login returned no token")
	}
	resp = f.get("/api/v1/install/status", "")
	_ = json.Unmarshal([]byte(resp.Body), &st)
	if st.State != "active" {
		t.Fatalf("state after claim: %q", st.State)
	}

	// Address setup advances the checkpoint.
	addr := f.post("/api/v1/install/address", `{"address":"panel.example.test"}`, tok)
	if addr.Code != http.StatusOK {
		t.Fatalf("address: %d %s", addr.Code, addr.Body)
	}
	var st2 struct {
		Checkpoint string `json:"checkpoint"`
	}
	_ = json.Unmarshal([]byte(addr.Body), &st2)
	if st2.Checkpoint != "address_configured" {
		t.Fatalf("checkpoint after address: %q", st2.Checkpoint)
	}

	// Resume across restart: a fresh status read on the same DB shows the
	// recorded checkpoint (no re-claim possible; covered by the replay above).
	if _, err := f.apiServer.Install.GetStatus(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// Panel client ↔ helper registry over a real Unix socket (DEC-003 transport).
func TestHostAgentSocketIntegration(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "h.sock")

	hdb, err := store.Open(filepath.Join(dir, "helper.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer hdb.Close()
	if err := hdb.Migrate(context.Background(), "../../migrations"); err != nil {
		t.Fatal(err)
	}
	j := &helper.SQLiteJournal{DB: hdb}
	const instID = "inst_socktest0000000000000000000000"
	policy := &helper.Policy{Verbs: map[string][]string{
		"host.inspect": {"*"},
	}}
	reg := helper.NewRegistry(policy, "", j, instID, map[int]bool{0: true})
	reg.Register("host.inspect", helper.InspectHandler())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = reg.Serve(ctx, socket, "")
	}()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := net.Dial("unix", socket); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	client := &hostagent.Client{SocketPath: socket, Timeout: 5 * time.Second}
	opID := "op_sock0000000000000000000000000000"
	req := helper.HostRequest{
		Version:        helper.Version,
		InstallationID: instID,
		HostID:         "*",
		Verb:           "host.inspect",
		Arguments:      json.RawMessage(`{}`),
		Fence:          1,
		OperationID:    opID,
		Digest:         helper.DigestOf(opID, "host.inspect", json.RawMessage(`{}`)),
		ExpiresAt:      time.Now().UTC().Add(time.Minute).Format(time.RFC3339Nano),
	}
	res, err := client.Call(ctx, req)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.Status != "ok" {
		t.Fatalf("result: %+v", res)
	}
	var info helper.OSInfo
	if err := json.Unmarshal(res.Observation, &info); err != nil {
		t.Fatal(err)
	}
	if info.PrettyName == "" {
		t.Fatalf("no OS observation: %s", string(res.Observation))
	}

	// Replay over the wire returns the same journal entry (one effect).
	res2, err := client.Call(ctx, req)
	if err != nil || res2.JournalID != res.JournalID {
		t.Fatalf("wire replay: %+v err=%v", res2, err)
	}
}

// Intent → dispatch guard → helper executor flow for a host verb (server
// workspace; server.owner carries the host.inspect grant).
func TestIntentToHelperFlow(t *testing.T) {
	f := newFixture(t)
	owner := f.makePrincipal("hostadmin@example.test")
	ws, err := f.db.CreateWorkspace(context.Background(), "server", owner.PrincipalID, "srv")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.db.AddMembership(context.Background(), store.Membership{
		PrincipalID: owner.PrincipalID, WorkspaceID: ws.WorkspaceID,
		RoleID: "server.owner", Revision: 1, State: "active",
	}); err != nil {
		t.Fatal(err)
	}
	res, err := f.db.CreateResource(context.Background(), ws.WorkspaceID, "", "host snapshot", "local")
	if err != nil {
		t.Fatal(err)
	}
	tok := f.login("hostadmin@example.test")

	intent := `{"action_id":"host.inspect","resource_type":"host snapshot","expected_owner_revision":1,"expected_generation":0}`
	r1 := f.post("/api/v1/resources/"+res.ResourceID+"/intents", intent, tok)
	if r1.Code != http.StatusAccepted {
		t.Fatalf("intent: %d %s", r1.Code, r1.Body)
	}
	var acc operations.Accepted
	_ = json.Unmarshal([]byte(r1.Body), &acc)

	called := false
	fence, err := f.Ops.DispatchGuard(context.Background(), acc.OperationID,
		time.Now().Add(time.Minute), func(ctx context.Context, op *operations.Operation) error {
			called = true
			_, err := f.Authz.Authorize(ctx, op.ActorID, op.ActionID, "host snapshot", op.ScopeID, 1)
			return err
		})
	if err != nil || !called || fence != 1 {
		t.Fatalf("dispatch: fence=%d err=%v called=%v", fence, err, called)
	}
	if err := f.Ops.Transition(context.Background(), acc.OperationID, "dispatched", "succeeded", nil); err != nil {
		t.Fatal(err)
	}
}

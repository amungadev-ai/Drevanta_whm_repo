package api

// G22 API surface tests: scope-pinned API keys (phase 23 INT-01) and
// interval schedules (WEB-08 core).

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"drevanta/internal/jobs"
	"drevanta/internal/store"
)

func TestAPIKeyLifecycleScopePinning(t *testing.T) {
	f := newFixture(t)
	owner := f.makePrincipal("keyowner@example.test")
	wsA, resA := f.makeWebsiteWorkspace(owner, "ws-key-a")
	// Second workspace of the same owner (key pinned to A must not see B).
	wsB, _ := f.makeWebsiteWorkspace(owner, "ws-key-b")
	_ = wsB
	ownerTok := f.login("keyowner@example.test")

	// Issue key scoped to workspace A.
	issue := f.post("/api/v1/scopes/"+wsA.WorkspaceID+"/api-keys", `{"name":"ci-deploy"}`, ownerTok)
	if issue.Code != http.StatusCreated {
		t.Fatalf("issue: %d %s", issue.Code, issue.Body)
	}
	var issued struct {
		APIKey string `json:"api_key"`
	}
	_ = json.Unmarshal([]byte(issue.Body), &issued)
	if issued.APIKey == "" {
		t.Fatal("no raw key returned")
	}

	// Key authenticates and is pinned to scope A: resources list of A works,
	// B conceals.
	hdr := map[string]string{"X-API-Key": issued.APIKey}
	ok := f.do("GET", "/api/v1/resources?scope_id="+wsA.WorkspaceID, "", "", hdr)
	if ok.Code != http.StatusOK {
		t.Fatalf("key on own scope: %d %s", ok.Code, ok.Body)
	}
	concealed := f.do("GET", "/api/v1/resources?scope_id="+wsB.WorkspaceID, "", "", hdr)
	if concealed.Code != http.StatusNotFound {
		t.Fatalf("key on other scope: want 404, got %d", concealed.Code)
	}
	// Key can act on a resource within its scope.
	acc := f.do("POST", "/api/v1/resources/"+resA.ResourceID+"/intents", validIntent(1, 0), "", hdr)
	if acc.Code != http.StatusAccepted {
		t.Fatalf("key intent: %d %s", acc.Code, acc.Body)
	}

	// List shows metadata (never the raw key).
	list := f.get("/api/v1/scopes/"+wsA.WorkspaceID+"/api-keys", ownerTok)
	if list.Code != http.StatusOK || containsStr(list.Body, issued.APIKey) {
		t.Fatalf("key list leaks raw key: %d %s", list.Code, list.Body)
	}

	// Revocation denies at next use.
	var keyList struct {
		Items []struct {
			KeyID string `json:"key_id"`
		} `json:"items"`
	}
	_ = json.Unmarshal([]byte(list.Body), &keyList)
	if len(keyList.Items) != 1 {
		t.Fatalf("key list items: %d", len(keyList.Items))
	}
	revoke := f.do("DELETE", "/api/v1/scopes/"+wsA.WorkspaceID+"/api-keys/"+keyList.Items[0].KeyID, "", ownerTok, nil)
	if revoke.Code != http.StatusOK {
		t.Fatalf("revoke: %d %s", revoke.Code, revoke.Body)
	}
	denied := f.do("GET", "/api/v1/resources?scope_id="+wsA.WorkspaceID, "", "", hdr)
	if denied.Code != http.StatusUnauthorized {
		t.Fatalf("revoked key: want 401, got %d", denied.Code)
	}
}

func TestScheduleArmsFiresAndCoalesces(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	owner := f.makePrincipal("sched@example.test")
	ws, res := f.makeWebsiteWorkspace(owner, "ws-sched")
	ownerTok := f.login("sched@example.test")

	// Arm a 1-second schedule for a granted action.
	arm := f.post("/api/v1/scopes/"+ws.WorkspaceID+"/schedules",
		`{"resource_id":"`+res.ResourceID+`","action_id":"file.write","every_seconds":1,"arguments":{"path":"tick.html"}}`, ownerTok)
	if arm.Code != http.StatusCreated {
		t.Fatalf("arm: %d %s", arm.Code, arm.Body)
	}
	var armed struct {
		ScheduleID string `json:"schedule_id"`
	}
	_ = json.Unmarshal([]byte(arm.Body), &armed)

	// No due tick yet (first run due after the interval).
	ops1, err := jobs.TickDueSchedules(context.Background(), f.db, f.Ops)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops1) != 0 {
		t.Fatalf("fired before due: %d", len(ops1))
	}
	// Advance past the interval by arming state directly (time-shift).
	future := store.Now()
	_, _ = f.db.ExecContext(ctx, `UPDATE schedule SET next_run_at=? WHERE schedule_id=?`,
		"2000-01-01T00:00:00Z", armed.ScheduleID)
	ops2, err := jobs.TickDueSchedules(ctx, f.db, f.Ops)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops2) != 1 {
		t.Fatalf("due tick created %d operations", len(ops2))
	}
	// Immediate second tick does not double-fire (row advanced atomically).
	ops3, err := jobs.TickDueSchedules(ctx, f.db, f.Ops)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops3) != 0 {
		t.Fatalf("double fire: %d", len(ops3))
	}
	// The created operation is owned by the schedule principal.
	op, err := f.Ops.Get(ctx, ops2[0])
	if err != nil {
		t.Fatal(err)
	}
	if op.ActorID != owner.PrincipalID || op.ActionID != "file.write" {
		t.Fatalf("scheduled op owner/action: %s/%s", op.ActorID, op.ActionID)
	}
	_ = future
}

func containsStr(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) &&
		func() bool {
			for i := 0; i+len(needle) <= len(haystack); i++ {
				if haystack[i:i+len(needle)] == needle {
					return true
				}
			}
			return false
		}()
}

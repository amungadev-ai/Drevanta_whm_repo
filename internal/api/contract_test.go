package api

// Contract tests implementing the "Reference fixtures for contract reviewers"
// scenarios from spec/specification/contracts/profile.md. These bind the API
// envelope semantics (DEC-004/009/012) as executable acceptance evidence for
// wave G10. Fixture-specified stimuli and oracles are followed exactly;
// fixture values are test data only (DEC-018).

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"drevanta/internal/audit"
	"drevanta/internal/authn"
	"drevanta/internal/authz"
	"drevanta/internal/operations"
	"drevanta/internal/store"
)

// fixtureEnv is one isolated server instance per test.
type fixtureEnv struct {
	t         *testing.T
	db        *store.DB
	srv       *httptest.Server
	auth      *authn.Service
	Authz     *authz.Checker
	Ops       *operations.Engine
	apiServer *Server
}

func newFixture(t *testing.T) *fixtureEnv {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, "../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	authSvc := authn.NewService(db, time.Hour, 30*time.Minute)
	checker := authz.NewChecker(db)
	eng := operations.NewEngine(db)
	aw := audit.NewWriter(db)
	apiSrv := NewServer(db, authSvc, checker, eng, aw)
	srv := httptest.NewServer(apiSrv.Handler())
	t.Cleanup(srv.Close)
	return &fixtureEnv{t: t, db: db, srv: srv, auth: authSvc, Authz: checker, Ops: eng, apiServer: apiSrv}
}

func (f *fixtureEnv) makePrincipal(email string) *store.Principal {
	f.t.Helper()
	p, err := f.db.CreatePrincipal(context.Background(), email, "Test "+email)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.auth.SetPrimaryPassword(context.Background(), p.PrincipalID, "CorrectHorse-42!"); err != nil {
		f.t.Fatal(err)
	}
	return p
}

func (f *fixtureEnv) login(email string) string {
	f.t.Helper()
	body := `{"email":"` + email + `","password":"CorrectHorse-42!"}`
	resp := f.post("/api/v1/identity/login", body, "")
	if resp.Code != http.StatusOK {
		f.t.Fatalf("login %s: %d %s", email, resp.Code, resp.Body)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &out); err != nil {
		f.t.Fatal(err)
	}
	return out.Token
}

// makeWebsiteWorkspace creates a website workspace owned by ownerScope with
// principal as active "owner" member and one filesystem resource.
func (f *fixtureEnv) makeWebsiteWorkspace(owner *store.Principal, name string) (*store.Workspace, *store.Resource) {
	f.t.Helper()
	ctx := context.Background()
	ws, err := f.db.CreateWorkspace(ctx, "website", owner.PrincipalID, name)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.db.AddMembership(ctx, store.Membership{
		PrincipalID: owner.PrincipalID, WorkspaceID: ws.WorkspaceID,
		RoleID: "website.owner", Revision: 1, State: "active",
	}); err != nil {
		f.t.Fatal(err)
	}
	res, err := f.db.CreateResource(ctx, ws.WorkspaceID, "", "filesystem entry", "public_html")
	if err != nil {
		f.t.Fatal(err)
	}
	return ws, res
}

type httpResult struct {
	Code   int
	Body   string
	Header http.Header
}

func (f *fixtureEnv) do(method, path, body, token string, headers map[string]string) httpResult {
	f.t.Helper()
	var rd *strings.Reader
	if body == "" {
		rd = strings.NewReader("")
	} else {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, f.srv.URL+path, rd)
	if err != nil {
		f.t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := f.srv.Client().Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		f.t.Fatal(err)
	}
	return httpResult{Code: resp.StatusCode, Body: string(bodyBytes), Header: resp.Header}
}

func (f *fixtureEnv) post(path, body, token string) httpResult {
	return f.do("POST", path, body, token, nil)
}

func (f *fixtureEnv) get(path, token string) httpResult {
	return f.do("GET", path, "", token, nil)
}

func validIntent(rev, gen int64) string {
	return fmt.Sprintf(`{"action_id":"file.write","resource_type":"filesystem entry","expected_owner_revision":%d,"expected_generation":%d,"arguments":{"path":"index.html"}}`, rev, gen)
}

// countRows is a test-side state probe.
func (f *fixtureEnv) countRows(table string) int {
	f.t.Helper()
	var n int
	if err := f.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

// ---- Scenario: unknown mutation field → 422 invalid_input, no state change ----

func TestContractUnknownField422(t *testing.T) {
	f := newFixture(t)
	owner := f.makePrincipal("owner@example.test")
	_, res := f.makeWebsiteWorkspace(owner, "ws-a")
	token := f.login("owner@example.test")

	beforeOps := f.countRows("operation")
	beforeOutbox := f.countRows("outbox")

	intent := strings.TrimSuffix(validIntent(1, 0), "}") + `,"run_shell":"rm -rf /"}`
	resp := f.post("/api/v1/resources/"+res.ResourceID+"/intents", intent, token)

	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d: %s", resp.Code, resp.Body)
	}
	var eb errorBody
	if err := json.Unmarshal([]byte(resp.Body), &eb); err != nil {
		t.Fatal(err)
	}
	if eb.Code != CodeInvalidInput {
		t.Fatalf("want code invalid_input, got %q", eb.Code)
	}
	if after := f.countRows("operation"); after != beforeOps {
		t.Fatalf("operation rows changed: %d → %d", beforeOps, after)
	}
	if after := f.countRows("outbox"); after != beforeOutbox {
		t.Fatalf("outbox rows changed: %d → %d", beforeOutbox, after)
	}
}

// ---- Scenario: stale desired generation → 412, generation unchanged ----

func TestContractStaleGeneration412(t *testing.T) {
	f := newFixture(t)
	owner := f.makePrincipal("owner@example.test")
	_, res := f.makeWebsiteWorkspace(owner, "ws-b")
	token := f.login("owner@example.test")

	// Resource is at desired generation 0 (fresh). Intent claims generation 7.
	intent := fmt.Sprintf(`{"action_id":"file.write","resource_type":"filesystem entry","expected_owner_revision":%d,"expected_generation":7,"arguments":{"path":"x.html"}}`, res.OwnerRevision)
	resp := f.post("/api/v1/resources/"+res.ResourceID+"/intents", intent, token)

	if resp.Code != http.StatusPreconditionFailed {
		t.Fatalf("want 412, got %d: %s", resp.Code, resp.Body)
	}
	var eb errorBody
	_ = json.Unmarshal([]byte(resp.Body), &eb)
	if eb.Code != CodeStalePrecondition {
		t.Fatalf("want stale_precondition, got %q", eb.Code)
	}
	after, err := f.db.ResourceByID(context.Background(), res.ResourceID)
	if err != nil {
		t.Fatal(err)
	}
	if after.DesiredGeneration != 0 {
		t.Fatalf("generation moved: %d", after.DesiredGeneration)
	}
}

// ---- Scenario: hidden resource → 404, no B identifier in body ----

func TestContractHiddenResource404(t *testing.T) {
	f := newFixture(t)
	ownerB := f.makePrincipal("owner-b@example.test")
	_, resB := f.makeWebsiteWorkspace(ownerB, "ws-b-secret")

	actorA := f.makePrincipal("actor-a@example.test")
	_ = actorA
	f.makeWebsiteWorkspace(f.makePrincipal("owner-a@example.test"), "ws-a")
	tokenA := f.login("owner-a@example.test")

	// Actor A uses valid B resource ID in path.
	resp := f.post("/api/v1/resources/"+resB.ResourceID+"/intents", validIntent(1, 0), tokenA)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d: %s", resp.Code, resp.Body)
	}
	if strings.Contains(resp.Body, resB.ResourceID) {
		t.Fatalf("body leaks concealed resource identifier: %s", resp.Body)
	}
	var eb errorBody
	_ = json.Unmarshal([]byte(resp.Body), &eb)
	if eb.Code != CodeNotFound {
		t.Fatalf("want not_found code, got %q", eb.Code)
	}
}

// ---- Scenario: stable replay → same operation; one intent + one outbox event ----

func TestContractStableReplay(t *testing.T) {
	f := newFixture(t)
	owner := f.makePrincipal("owner@example.test")
	_, res := f.makeWebsiteWorkspace(owner, "ws-c")
	token := f.login("owner@example.test")

	key := "replay-key-1"
	headers := map[string]string{"Idempotency-Key": key}
	body := validIntent(1, 0)

	r1 := f.do("POST", "/api/v1/resources/"+res.ResourceID+"/intents", body, token, headers)
	if r1.Code != http.StatusAccepted {
		t.Fatalf("first accept: %d %s", r1.Code, r1.Body)
	}
	var a1 operations.Accepted
	if err := json.Unmarshal([]byte(r1.Body), &a1); err != nil {
		t.Fatal(err)
	}

	// Replay with the same key and same canonical intent (response "lost").
	r2 := f.do("POST", "/api/v1/resources/"+res.ResourceID+"/intents", body, token, headers)
	if r2.Code != http.StatusAccepted {
		t.Fatalf("replay: %d %s", r2.Code, r2.Body)
	}
	var a2 operations.Accepted
	if err := json.Unmarshal([]byte(r2.Body), &a2); err != nil {
		t.Fatal(err)
	}

	if a1.OperationID != a2.OperationID {
		t.Fatalf("replay returned different operation: %s vs %s", a1.OperationID, a2.OperationID)
	}
	if a2.CorrelationID != a1.CorrelationID {
		t.Fatalf("correlation mismatch")
	}
	if f.countRows("idempotency") != 1 {
		t.Fatalf("want exactly 1 idempotency row, got %d", f.countRows("idempotency"))
	}
	if f.countRows("outbox") != 1 {
		t.Fatalf("want exactly 1 outbox event, got %d", f.countRows("outbox"))
	}
	if !a2.Replayed {
		t.Fatalf("replay flag missing")
	}
}

// ---- Scenario: altered replay → 409; existing operation unchanged ----

func TestContractAlteredReplay409(t *testing.T) {
	f := newFixture(t)
	owner := f.makePrincipal("owner@example.test")
	_, res := f.makeWebsiteWorkspace(owner, "ws-d")
	token := f.login("owner@example.test")

	key := "altered-key-1"
	hdr := map[string]string{"Idempotency-Key": key}
	r1 := f.do("POST", "/api/v1/resources/"+res.ResourceID+"/intents", validIntent(1, 0), token, hdr)
	if r1.Code != http.StatusAccepted {
		t.Fatalf("first: %d %s", r1.Code, r1.Body)
	}
	var a1 operations.Accepted
	_ = json.Unmarshal([]byte(r1.Body), &a1)

	// Same actor, scope, action and key; changed arguments.
	altered := strings.Replace(validIntent(1, 0), `"path":"index.html"`, `"path":"other.html"`, 1)
	r2 := f.do("POST", "/api/v1/resources/"+res.ResourceID+"/intents", altered, token, hdr)
	if r2.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d %s", r2.Code, r2.Body)
	}
	var eb errorBody
	_ = json.Unmarshal([]byte(r2.Body), &eb)
	if eb.Code != CodeConflict {
		t.Fatalf("want conflict code, got %q", eb.Code)
	}
	// Existing operation remains unchanged (still accepted).
	var state string
	if err := f.db.QueryRow("SELECT state FROM operation WHERE operation_id=?", a1.OperationID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "accepted" {
		t.Fatalf("operation state changed to %q", state)
	}
	if f.countRows("idempotency") != 1 {
		t.Fatalf("idempotency rows: %d", f.countRows("idempotency"))
	}
}

// ---- Scenario: revoked replay → denial under current visibility ----

func TestContractRevokedReplay(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	owner := f.makePrincipal("owner@example.test")
	ws, res := f.makeWebsiteWorkspace(owner, "ws-e")

	// Operator principal with its own operator membership (action grant).
	op := f.makePrincipal("operator@example.test")
	if err := f.db.AddMembership(ctx, store.Membership{
		PrincipalID: op.PrincipalID, WorkspaceID: ws.WorkspaceID,
		RoleID: "website.operator", Revision: 1, State: "active",
	}); err != nil {
		f.t.Fatal(err)
	}
	// Operator role currently lacks file.write on website kind (see seed
	// migration), so grant it by using the manager role for the same principal
	// and revoking after acceptance.
	if err := f.db.AddMembership(ctx, store.Membership{
		PrincipalID: op.PrincipalID, WorkspaceID: ws.WorkspaceID,
		RoleID: "website.manager", Revision: 1, State: "active",
	}); err != nil {
		f.t.Fatal(err)
	}
	token := f.login("operator@example.test")

	key := "revoked-key-1"
	hdr := map[string]string{"Idempotency-Key": key}
	r1 := f.do("POST", "/api/v1/resources/"+res.ResourceID+"/intents", validIntent(1, 0), token, hdr)
	if r1.Code != http.StatusAccepted {
		t.Fatalf("first: %d %s", r1.Code, r1.Body)
	}

	// Revoke actor's manager membership before replay of the previous key.
	if err := f.db.RevokeMembership(ctx, op.PrincipalID, ws.WorkspaceID, "website.manager"); err != nil {
		f.t.Fatal(err)
	}
	r2 := f.do("POST", "/api/v1/resources/"+res.ResourceID+"/intents", validIntent(1, 0), token, hdr)

	// Denial under current visibility: operator membership remains, so the
	// resource is visible (403), and no operation ID is returned.
	if r2.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d %s", r2.Code, r2.Body)
	}
	if strings.Contains(r2.Body, "operation_id") {
		t.Fatalf("denial leaked operation reference: %s", r2.Body)
	}
}

// ---- Additional envelope checks ----

func TestUnauthorizedRequests401(t *testing.T) {
	f := newFixture(t)
	for _, path := range []string{"/api/v1/me", "/api/v1/workspaces"} {
		resp := f.get(path, "")
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("%s: want 401, got %d", path, resp.Code)
		}
	}
	// Invalid token denies uniformly.
	resp := f.get("/api/v1/me", "dvt_deadbeef")
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token: want 401, got %d", resp.Code)
	}
}

func TestMeAndWorkspaces(t *testing.T) {
	f := newFixture(t)
	owner := f.makePrincipal("owner@example.test")
	ws, _ := f.makeWebsiteWorkspace(owner, "my-site")
	token := f.login("owner@example.test")

	resp := f.get("/api/v1/me", token)
	if resp.Code != http.StatusOK {
		t.Fatalf("me: %d %s", resp.Code, resp.Body)
	}
	var me meResponse
	if err := json.Unmarshal([]byte(resp.Body), &me); err != nil {
		t.Fatal(err)
	}
	if len(me.Memberships) != 1 || me.Memberships[0] != ws.WorkspaceID {
		t.Fatalf("memberships: %v", me.Memberships)
	}

	resp = f.get("/api/v1/workspaces", token)
	var list workspaceList
	if err := json.Unmarshal([]byte(resp.Body), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].WorkspaceID != ws.WorkspaceID {
		t.Fatalf("workspaces: %+v", list.Items)
	}
	if list.Items[0].Role != "website.owner" {
		t.Fatalf("role: %q", list.Items[0].Role)
	}
}

func TestOperationVisibilityAnd404(t *testing.T) {
	f := newFixture(t)
	owner := f.makePrincipal("owner@example.test")
	_, res := f.makeWebsiteWorkspace(owner, "ws-f")
	token := f.login("owner@example.test")

	r1 := f.post("/api/v1/resources/"+res.ResourceID+"/intents", validIntent(1, 0), token)
	var a1 operations.Accepted
	_ = json.Unmarshal([]byte(r1.Body), &a1)

	resp := f.get("/api/v1/operations/"+a1.OperationID, token)
	if resp.Code != http.StatusOK {
		t.Fatalf("operation read: %d %s", resp.Code, resp.Body)
	}
	var st operationStatusBody
	if err := json.Unmarshal([]byte(resp.Body), &st); err != nil {
		t.Fatal(err)
	}
	if st.State != "accepted" {
		t.Fatalf("state: %q", st.State)
	}

	// Foreign principal must see 404, body must not confirm existence.
	stranger := f.makePrincipal("stranger@example.test")
	_ = stranger
	f.makeWebsiteWorkspace(f.makePrincipal("other@example.test"), "ws-other")
	stoken := f.login("stranger@example.test")
	resp2 := f.get("/api/v1/operations/"+a1.OperationID, stoken)
	if resp2.Code != http.StatusNotFound {
		t.Fatalf("foreign op read: want 404, got %d", resp2.Code)
	}
	if strings.Contains(resp2.Body, a1.OperationID) {
		t.Fatalf("404 leaks operation id")
	}
}

func TestAuditTrailRecordsDenyAndAllow(t *testing.T) {
	f := newFixture(t)
	owner := f.makePrincipal("owner@example.test")
	ws, res := f.makeWebsiteWorkspace(owner, "ws-g")
	token := f.login("owner@example.test")

	_ = f.post("/api/v1/resources/"+res.ResourceID+"/intents", validIntent(1, 0), token)
	// Deny: unknown action.
	resp := f.post("/api/v1/resources/"+res.ResourceID+"/intents",
		`{"action_id":"terminal.open","resource_type":"filesystem entry","expected_owner_revision":1,"expected_generation":0}`, token)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("unknown action: want 403, got %d %s", resp.Code, resp.Body)
	}
	events, err := audit.NewWriter(f.db).Query(context.Background(), ws.WorkspaceID, 100)
	if err != nil {
		t.Fatal(err)
	}
	var allow, deny bool
	for _, e := range events {
		if e["outcome"] == "allow" {
			allow = true
		}
		if e["outcome"] == "deny" {
			deny = true
		}
	}
	if !allow || !deny {
		t.Fatalf("audit missing outcomes: allow=%v deny=%v events=%d", allow, deny, len(events))
	}
}

func TestLoginMFALifecycle(t *testing.T) {
	f := newFixture(t)
	p := f.makePrincipal("mfa@example.test")
	if _, err := f.auth.EnrollTOTP(context.Background(), p.PrincipalID); err != nil {
		t.Fatal(err)
	}
	secret, err := f.auth.MFASecret(context.Background(), p.PrincipalID)
	if err != nil {
		t.Fatal(err)
	}

	// Password proof alone returns mfa_required, no token.
	resp := f.post("/api/v1/identity/login", `{"email":"mfa@example.test","password":"CorrectHorse-42!"}`, "")
	var lr loginResponse
	if err := json.Unmarshal([]byte(resp.Body), &lr); err != nil {
		t.Fatal(err)
	}
	if resp.Code != http.StatusOK || !lr.MFARequired || lr.Token != "" {
		t.Fatalf("login: %d %s", resp.Code, resp.Body)
	}

	code := totpNow(t, secret)
	resp2 := f.post("/api/v1/identity/mfa", fmt.Sprintf(`{"principal_id":%q,"code":%q}`, lr.PrincipalID, code), "")
	if resp2.Code != http.StatusOK {
		t.Fatalf("mfa: %d %s", resp2.Code, resp2.Body)
	}
	var mr mfaResponse
	_ = json.Unmarshal([]byte(resp2.Body), &mr)
	if mr.Token == "" {
		t.Fatalf("no token after mfa")
	}

	// Epoch/revocation: invalid code denies.
	resp3 := f.post("/api/v1/identity/mfa", fmt.Sprintf(`{"principal_id":%q,"code":"000000"}`, lr.PrincipalID), "")
	if resp3.Code != http.StatusUnauthorized {
		t.Fatalf("bad code: want 401, got %d", resp3.Code)
	}
}

// totpNow computes the current TOTP code for a base32 secret (test helper
// sharing the RFC 6238 implementation under test).
func totpNow(t *testing.T, secretB32 string) string {
	t.Helper()
	code, err := authn.TOTPCodeAt(secretB32, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return code
}

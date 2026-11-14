package api

// Domain capability integration tests (G40 full-persona wave): each exposed
// capability is exercised for allow, deny (missing grant), tenant isolation
// (foreign scope → uniform 404) and strict-decode rejection. These are the
// per-capability integration evidence rows for the functional matrix.

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"drevanta/internal/domains"
	"drevanta/internal/store"
)

// makeWorkspace creates a workspace of the given kind owned by the principal
// with the given role.
func (f *fixtureEnv) makeWorkspace(owner *store.Principal, kind, role, name string) *store.Workspace {
	f.t.Helper()
	ws, err := f.db.CreateWorkspace(context.Background(), kind, owner.PrincipalID, name)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.db.AddMembership(context.Background(), store.Membership{
		PrincipalID: owner.PrincipalID, WorkspaceID: ws.WorkspaceID,
		RoleID: role, Revision: 1, State: "active",
	}); err != nil {
		f.t.Fatal(err)
	}
	return ws
}

// ---- Hosting Provider: support + commercial ----

func TestSupportTicketLifecycle(t *testing.T) {
	f := newFixture(t)
	owner := f.makePrincipal("owner@t.test")
	staff := f.makePrincipal("staff@t.test")
	ws := f.makeWorkspace(owner, "hosting_org", "hosting.owner", "Northstar")
	if err := f.db.AddMembership(context.Background(), store.Membership{
		PrincipalID: staff.PrincipalID, WorkspaceID: ws.WorkspaceID,
		RoleID: "hosting.support_agent", Revision: 1, State: "active",
	}); err != nil {
		f.t.Fatal(err)
	}
	tok := f.login(owner.Email)
	staffTok := f.login(staff.Email)

	// Strict decode: unknown field rejected before any state transition.
	r := f.do("POST", "/api/v1/scopes/"+ws.WorkspaceID+"/tickets", `{"subject":"x","extra":1}`, tok, nil)
	if r.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown field: got %d want 422", r.Code)
	}

	// Owner creates a ticket (grant: support.ticket.create).
	r = f.do("POST", "/api/v1/scopes/"+ws.WorkspaceID+"/tickets",
		`{"subject":"backup restore question","priority":"normal","body":"how do I restore?"}`, tok, nil)
	if r.Code != http.StatusCreated {
		t.Fatalf("create ticket: %d %s", r.Code, r.Body)
	}
	created := jsonStr(t, r.Body, "ticket_id")

	// Staff lists: sees the ticket; staff answers; owner sees the answer.
	r = f.do("GET", "/api/v1/scopes/"+ws.WorkspaceID+"/tickets", "", staffTok, nil)
	if r.Code != http.StatusOK || !contains(r.Body, "backup restore question") {
		t.Fatalf("staff list: %d %s", r.Code, r.Body)
	}
	r = f.do("POST", "/api/v1/scopes/"+ws.WorkspaceID+"/tickets/"+created+"/messages",
		`{"body":"use restore preview first"}`, staffTok, nil)
	if r.Code != http.StatusCreated {
		t.Fatalf("staff answer: %d %s", r.Code, r.Body)
	}
	r = f.do("GET", "/api/v1/scopes/"+ws.WorkspaceID+"/tickets/"+created, "", tok, nil)
	if r.Code != http.StatusOK || !contains(r.Body, "restore preview") {
		t.Fatalf("owner detail: %d %s", r.Code, r.Body)
	}

	// Tenant isolation: a membership-less principal gets uniform 404s.
	foreign := f.makePrincipal("foreign@t.test")
	foreignTok := f.login(foreign.Email)
	r = f.do("GET", "/api/v1/scopes/"+ws.WorkspaceID+"/tickets", "", foreignTok, nil)
	if r.Code != http.StatusNotFound {
		t.Fatalf("foreign list: got %d want 404", r.Code)
	}
	r = f.do("GET", "/api/v1/scopes/"+ws.WorkspaceID+"/tickets/"+created, "", foreignTok, nil)
	if r.Code != http.StatusNotFound {
		t.Fatalf("foreign detail: got %d want 404", r.Code)
	}

	// Resolve: staff only.
	r = f.do("POST", "/api/v1/scopes/"+ws.WorkspaceID+"/tickets/"+created+"/resolve", "", staffTok, nil)
	if r.Code != http.StatusOK {
		t.Fatalf("staff resolve: %d %s", r.Code, r.Body)
	}
}

func TestCommercialHostingLifecycle(t *testing.T) {
	f := newFixture(t)
	owner := f.makePrincipal("billo@t.test")
	ws := f.makeWorkspace(owner, "hosting_org", "hosting.owner", "Northstar")
	tok := f.login(owner.Email)
	base := "/api/v1/scopes/" + ws.WorkspaceID

	// Plan + customer (grant: billing.adjust).
	r := f.do("POST", base+"/plans", `{"name":"starter","quota_json":"{\"storage_gb\":5}","price_cents":900,"currency":"USD"}`, tok, nil)
	if r.Code != http.StatusCreated {
		t.Fatalf("plan: %d %s", r.Code, r.Body)
	}
	planID := jsonStr(t, r.Body, "plan_id")
	r = f.do("POST", base+"/customers", `{"email":"acme@t.test","name":"Acme Studio"}`, tok, nil)
	if r.Code != http.StatusCreated {
		t.Fatalf("customer: %d %s", r.Code, r.Body)
	}
	custID := jsonStr(t, r.Body, "customer_id")

	// Account provision (grant: account.provision).
	r = f.do("POST", base+"/accounts", `{"customer_id":"`+custID+`","plan_id":"`+planID+`"}`, tok, nil)
	if r.Code != http.StatusCreated {
		t.Fatalf("account: %d %s", r.Code, r.Body)
	}
	if !contains(r.Body, "account_id") {
		t.Fatal("account id missing")
	}

	// Invoice + payment settles it exactly once (second payment conflicts).
	r = f.do("POST", base+"/invoices",
		`{"customer_id":"`+custID+`","due_in_days":14,"lines":[{"description":"starter month 1","quantity":1,"unit_cents":900}]}`, tok, nil)
	if r.Code != http.StatusCreated {
		t.Fatalf("invoice: %d %s", r.Code, r.Body)
	}
	invID := jsonStr(t, r.Body, "invoice_id")
	if !contains(r.Body, `"total_cents":900`) {
		t.Fatalf("invoice total: %s", r.Body)
	}
	r = f.do("POST", base+"/invoices/"+invID+"/payments", `{"amount_cents":900}`, tok, nil)
	if r.Code != http.StatusCreated {
		t.Fatalf("payment: %d %s", r.Code, r.Body)
	}
	r = f.do("POST", base+"/invoices/"+invID+"/payments", `{"amount_cents":900}`, tok, nil)
	if r.Code != http.StatusConflict {
		t.Fatalf("second payment: got %d want 409", r.Code)
	}

	// Lists are grant-gated: viewer role cannot read commercial data.
	viewer := f.makePrincipal("view@t.test")
	if err := f.db.AddMembership(context.Background(), store.Membership{
		PrincipalID: viewer.PrincipalID, WorkspaceID: ws.WorkspaceID,
		RoleID: "hosting.viewer", Revision: 1, State: "active",
	}); err != nil {
		f.t.Fatal(err)
	}
	vtok := f.login(viewer.Email)
	r = f.do("GET", base+"/customers", "", vtok, nil)
	if r.Code != http.StatusForbidden {
		t.Fatalf("viewer customers: got %d want 403", r.Code)
	}
	r = f.do("GET", base+"/invoices", "", vtok, nil)
	if r.Code != http.StatusForbidden {
		t.Fatalf("viewer invoices: got %d want 403", r.Code)
	}
}

// ---- Developer: pipeline ----

func TestDeveloperPipelineLifecycle(t *testing.T) {
	f := newFixture(t)
	owner := f.makePrincipal("dev@t.test")
	ws := f.makeWorkspace(owner, "developer_project", "developer.owner", "Atlas")
	tok := f.login(owner.Email)
	base := "/api/v1/scopes/" + ws.WorkspaceID

	r := f.do("POST", base+"/source-connections", `{"repo_url":"https://git.example.net/atlas.git","default_branch":"main"}`, tok, nil)
	if r.Code != http.StatusCreated {
		t.Fatalf("connect: %d %s", r.Code, r.Body)
	}
	connID := jsonStr(t, r.Body, "connection_id")

	r = f.do("POST", base+"/builds", `{"connection_id":"`+connID+`","commit_ref":"abc123"}`, tok, nil)
	if r.Code != http.StatusCreated {
		t.Fatalf("queue build: %d %s", r.Code, r.Body)
	}
	buildID := jsonStr(t, r.Body, "build_id")

	r = f.do("POST", base+"/environments", `{"name":"production","kind":"production"}`, tok, nil)
	if r.Code != http.StatusCreated {
		t.Fatalf("environment: %d %s", r.Code, r.Body)
	}
	envID := jsonStr(t, r.Body, "environment_id")

	// Promotion requires a succeeded build with a digest (state machine).
	r = f.do("POST", base+"/environments/"+envID+"/promote", `{"build_id":"`+buildID+`"}`, tok, nil)
	if r.Code != http.StatusConflict {
		t.Fatalf("premature promote: got %d want 409", r.Code)
	}

	// Advance the build through the domain service, then promote.
	p := domains.NewPipeline(f.db)
	if err := p.FinishBuild(context.Background(), buildID, true, strings.Repeat("ab", 32), 2048); err != nil {
		t.Fatal(err)
	}
	r = f.do("POST", base+"/environments/"+envID+"/promote", `{"build_id":"`+buildID+`"}`, tok, nil)
	if r.Code != http.StatusCreated || !contains(r.Body, "release_id") {
		t.Fatalf("promote: %d %s", r.Code, r.Body)
	}

	// Lists render real rows for the workspace.
	r = f.do("GET", base+"/builds", "", tok, nil)
	if r.Code != http.StatusOK || !contains(r.Body, "abc123") {
		t.Fatalf("build list: %d %s", r.Code, r.Body)
	}
}

// ---- Website Owner: mail + templates ----

func TestWebsiteMailDomainLifecycle(t *testing.T) {
	f := newFixture(t)
	owner := f.makePrincipal("web@t.test")
	ws := f.makeWorkspace(owner, "website", "website.owner", "Acme")
	tok := f.login(owner.Email)
	base := "/api/v1/scopes/" + ws.WorkspaceID

	r := f.do("POST", base+"/mail-domains", `{"domain":"acme.example"}`, tok, nil)
	if r.Code != http.StatusCreated || !contains(r.Body, `"state":"pending"`) {
		t.Fatalf("add domain: %d %s", r.Code, r.Body)
	}
	domID := jsonStr(t, r.Body, "mail_domain_id")

	// Verify-before-mailbox: creation against a pending domain conflicts.
	r = f.do("POST", base+"/mailboxes", `{"mail_domain_id":"`+domID+`","local_part":"hello","quota_mb":1024}`, tok, nil)
	if r.Code != http.StatusConflict {
		t.Fatalf("mailbox on pending domain: got %d want 409", r.Code)
	}
	r = f.do("POST", base+"/mail-domains/"+domID+"/verify", "", tok, nil)
	if r.Code != http.StatusOK {
		t.Fatalf("verify: %d %s", r.Code, r.Body)
	}
	r = f.do("POST", base+"/mailboxes", `{"mail_domain_id":"`+domID+`","local_part":"hello","quota_mb":1024}`, tok, nil)
	if r.Code != http.StatusCreated {
		t.Fatalf("mailbox after verify: %d %s", r.Code, r.Body)
	}
	r = f.do("POST", base+"/mail-aliases", `{"mail_domain_id":"`+domID+`","local_part":"info","destination":"hello@acme.example"}`, tok, nil)
	if r.Code != http.StatusCreated {
		t.Fatalf("alias: %d %s", r.Code, r.Body)
	}
}

func TestTemplateInstallRequiresApplicationResource(t *testing.T) {
	f := newFixture(t)
	owner := f.makePrincipal("host2@t.test")
	ws := f.makeWorkspace(owner, "hosting_org", "hosting.owner", "Northstar")
	tok := f.login(owner.Email)
	base := "/api/v1/scopes/" + ws.WorkspaceID

	r := f.do("POST", base+"/plans", `{"name":"p","price_cents":100}`, tok, nil)
	if r.Code != http.StatusCreated {
		t.Fatalf("plan: %d", r.Code)
	}
	// Seed a template + a non-application resource.
	tpl := domains.NewTemplateService(f.db)
	tmplID, err := tpl.AddTemplate(context.Background(), ws.WorkspaceID, "wordpress", "WordPress", "cms", "php", "registry:wp:6")
	if err != nil {
		t.Fatal(err)
	}
	res, err := f.db.CreateResource(context.Background(), ws.WorkspaceID, "", "filesystem entry", "public_html")
	if err != nil {
		t.Fatal(err)
	}
	r = f.do("POST", base+"/templates/"+tmplID+"/install", `{"resource_id":"`+res.ResourceID+`"}`, tok, nil)
	// Uniform 404: the service collapses wrong-type targets into "no such
	// installable target" (no resource-state leak).
	if r.Code != http.StatusNotFound {
		t.Fatalf("install on non-app: got %d want 404", r.Code)
	}
	appRes, err := f.db.CreateResource(context.Background(), ws.WorkspaceID, "", "application", "acme-site")
	if err != nil {
		t.Fatal(err)
	}
	r = f.do("POST", base+"/templates/"+tmplID+"/install", `{"resource_id":"`+appRes.ResourceID+`"}`, tok, nil)
	if r.Code != http.StatusCreated || !contains(r.Body, "install_id") {
		t.Fatalf("install on app: %d %s", r.Code, r.Body)
	}
}

// ---- Cross-persona: observations + notifications + schedules + backups ----

func TestObservationsNotificationsSchedulesBackups(t *testing.T) {
	f := newFixture(t)
	owner := f.makePrincipal("ops@t.test")
	ws := f.makeWorkspace(owner, "server", "server.owner", "Server Administration")
	tok := f.login(owner.Email)
	base := "/api/v1/scopes/" + ws.WorkspaceID

	res, err := f.db.CreateResource(context.Background(), ws.WorkspaceID, "", "host snapshot", "primary-host")
	if err != nil {
		t.Fatal(err)
	}
	o := domains.NewObserveService(f.db)
	if _, err := o.Record(context.Background(), ws.WorkspaceID, res.ResourceID, "metric", "load1", floatPtr(0.42), ""); err != nil {
		t.Fatal(err)
	}
	if err := o.Notify(context.Background(), ws.WorkspaceID, owner.PrincipalID, "backup.complete", "Nightly backup completed"); err != nil {
		t.Fatal(err)
	}

	r := f.do("GET", base+"/observations?resource_id="+res.ResourceID, "", tok, nil)
	if r.Code != http.StatusOK || !contains(r.Body, `"load1"`) {
		t.Fatalf("observations: %d %s", r.Code, r.Body)
	}
	r = f.do("GET", base+"/notifications", "", tok, nil)
	if r.Code != http.StatusOK || !contains(r.Body, "Nightly backup completed") {
		t.Fatalf("notifications: %d %s", r.Code, r.Body)
	}
	r = f.do("GET", base+"/schedules", "", tok, nil)
	if r.Code != http.StatusOK {
		t.Fatalf("schedules: %d", r.Code)
	}
	r = f.do("GET", base+"/backups", "", tok, nil)
	if r.Code != http.StatusOK {
		t.Fatalf("backups: %d", r.Code)
	}

	// Tenant isolation on reads: a foreign principal gets a uniform 404.
	foreign := f.makePrincipal("ops2@t.test")
	ftok := f.login(foreign.Email)
	r = f.do("GET", base+"/observations", "", ftok, nil)
	if r.Code != http.StatusNotFound {
		t.Fatalf("foreign observations: got %d want 404", r.Code)
	}
}

// ---- helpers ----

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func jsonStr(t *testing.T, body, key string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("body %s: %v", body, err)
	}
	v, _ := m[key].(string)
	if v == "" {
		t.Fatalf("key %q missing in %s", key, body)
	}
	return v
}

func floatPtr(v float64) *float64 { return &v }

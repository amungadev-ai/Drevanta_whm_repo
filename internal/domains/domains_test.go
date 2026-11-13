package domains

// Domain service verification: commercial (BIZ-01/02/03), support
// (SUP-01/02), pipelines (APP-05/06/07), mail (MAIL-01), observability
// (OBS-01), templates (MKT-01), backups (WEB-09/DEC-010).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"drevanta/internal/store"
)

func domFixture(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(context.Background(), "../../migrations"); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedScope(t *testing.T, db *store.DB) (scopeID, principalID string) {
	t.Helper()
	ctx := context.Background()
	p, err := db.CreatePrincipal(ctx, "org-"+store.NewID("e")[2:10]+"@example.test", "Org")
	if err != nil {
		t.Fatal(err)
	}
	ws, err := db.CreateWorkspace(ctx, "hosting_org", p.PrincipalID, "Org")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AddMembership(ctx, store.Membership{
		PrincipalID: p.PrincipalID, WorkspaceID: ws.WorkspaceID,
		RoleID: "hosting.owner", Revision: 1, State: "active",
	}); err != nil {
		t.Fatal(err)
	}
	return ws.WorkspaceID, p.PrincipalID
}

func TestCommercialLifecycle(t *testing.T) {
	db := domFixture(t)
	scope, principal := seedScope(t, db)
	c := NewCommercial(db, nil)
	ctx := context.Background()

	planID, err := c.CreatePlan(ctx, scope, "starter", `{"databases":2,"storage_gb":10}`, 500, "USD")
	if err != nil {
		t.Fatal(err)
	}
	cusID, err := c.CreateCustomer(ctx, scope, "buyer@example.test", "Buyer")
	if err != nil {
		t.Fatal(err)
	}
	accID, err := c.ProvisionAccount(ctx, scope, cusID, planID)
	if err != nil {
		t.Fatal(err)
	}
	// Foreign-scope objects reject.
	otherScope, _ := seedScope(t, db)
	if _, err := c.ProvisionAccount(ctx, otherScope, cusID, planID); err == nil {
		t.Fatal("foreign scope account accepted")
	}
	// Duplicate email rejects.
	if _, err := c.CreateCustomer(ctx, scope, "buyer@example.test", "Dup"); err == nil {
		t.Fatal("duplicate customer accepted")
	}

	invID, total, err := c.CreateInvoice(ctx, scope, cusID, 0, []InvoiceLine{
		{Description: "starter monthly", Quantity: 1, UnitCents: 500, TaxCents: 50},
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 550 {
		t.Fatalf("total: %d", total)
	}
	// Underpayment rejects.
	if _, err := c.RecordPayment(ctx, scope, invID, 499); err == nil {
		t.Fatal("underpayment accepted")
	}
	payID, err := c.RecordPayment(ctx, scope, invID, 550)
	if err != nil {
		t.Fatal(err)
	}
	if payID == "" {
		t.Fatal("no payment id")
	}
	// Double payment conflicts (invoice settles once).
	if _, err := c.RecordPayment(ctx, scope, invID, 550); !errors.Is(err, ErrState) {
		t.Fatalf("double payment: %v", err)
	}
	if err := c.AdjustCredit(ctx, scope, cusID, 100, "goodwill"); err != nil {
		t.Fatal(err)
	}
	if err := c.ActivateAccount(ctx, accID, "ws_placeholder"); err != nil {
		t.Fatal(err)
	}
	_ = principal
}

func TestSupportTicketFlow(t *testing.T) {
	db := domFixture(t)
	scope, principal := seedScope(t, db)
	s := NewSupport(db)
	ctx := context.Background()

	tktID, err := s.CreateTicket(ctx, scope, principal, "Cannot reach panel", "high", "Details inside")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AnswerTicket(ctx, tktID, principal, "Staff reply", true); err != nil {
		t.Fatal(err)
	}
	if err := s.AnswerTicket(ctx, tktID, principal, "Still broken", false); err != nil {
		t.Fatal(err)
	}
	if err := s.ResolveTicket(ctx, tktID); err != nil {
		t.Fatal(err)
	}
	// Resolved is terminal.
	if err := s.AnswerTicket(ctx, tktID, principal, "after resolve", true); !errors.Is(err, ErrState) {
		t.Fatalf("post-resolve answer: %v", err)
	}
	// SLA deadlines set by priority.
	tkt2, _ := s.CreateTicket(ctx, scope, principal, "Urgent issue", "urgent", "")
	var slaDue string
	if err := db.QueryRow(`SELECT sla_due_at FROM ticket WHERE ticket_id=?`, tkt2).Scan(&slaDue); err != nil {
		t.Fatal(err)
	}
	if slaDue == "" {
		t.Fatal("no SLA deadline")
	}
	artID, err := s.PublishArticle(ctx, scope, "getting-started", "Getting started", "# Steps\n...")
	if err != nil {
		t.Fatal(err)
	}
	// Update path (same slug).
	artID2, err := s.PublishArticle(ctx, scope, "getting-started", "Getting started v2", "updated")
	if err != nil || artID2 != artID {
		t.Fatalf("article update: %v %s vs %s", err, artID2, artID)
	}
	// Invalid slug rejects.
	if _, err := s.PublishArticle(ctx, scope, "Bad Slug!", "t", "b"); err == nil {
		t.Fatal("invalid slug accepted")
	}
}

func TestPipelinePromotionCAS(t *testing.T) {
	db := domFixture(t)
	scope, principal := seedScope(t, db)
	p := NewPipeline(db)
	ctx := context.Background()

	connID, err := p.ConnectSource(ctx, scope, principal, "https://git.example.test/acme/app.git", "main")
	if err != nil {
		t.Fatal(err)
	}
	// Invalid URL rejects.
	if _, err := p.ConnectSource(ctx, scope, principal, "not a url", ""); err == nil {
		t.Fatal("invalid repo URL accepted")
	}
	b1, err := p.QueueBuild(ctx, scope, connID, "abc123")
	if err != nil {
		t.Fatal(err)
	}
	// Second queued build on the same connection conflicts (concurrency bound).
	if _, err := p.QueueBuild(ctx, scope, connID, "def456"); !errors.Is(err, ErrState) {
		t.Fatalf("parallel build: %v", err)
	}
	digest := sha256.Sum256([]byte("artifact"))
	dhex := hex.EncodeToString(digest[:])
	if err := p.FinishBuild(ctx, b1, true, dhex, 1024); err != nil {
		t.Fatal(err)
	}
	// Only succeeded builds with digests promote.
	envID, err := p.CreateEnvironment(ctx, scope, "production", "production")
	if err != nil {
		t.Fatal(err)
	}
	rel1, err := p.PromoteRelease(ctx, envID, b1)
	if err != nil {
		t.Fatal(err)
	}
	// A second build promotes over the first (CAS winner), prior release recorded.
	b2, _ := p.QueueBuild(ctx, scope, connID, "def456")
	if err := p.FinishBuild(ctx, b2, true, dhex, 1024); err != nil {
		t.Fatal(err)
	}
	if _, err := p.PromoteRelease(ctx, envID, b2); err != nil {
		t.Fatal(err)
	}
	_ = rel1
	// Failed build cannot promote.
	b3, _ := p.QueueBuild(ctx, scope, connID, "aaa000")
	_ = p.FinishBuild(ctx, b3, false, "", 0)
	if _, err := p.PromoteRelease(ctx, envID, b3); !errors.Is(err, ErrState) {
		t.Fatalf("failed build promoted: %v", err)
	}
	// Outbox carries promotion events.
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE type='release.promoted'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("promotion events: %d", n)
	}
}

func TestMailDomainMailbox(t *testing.T) {
	db := domFixture(t)
	scope, _ := seedScope(t, db)
	m := NewMailService(db)
	ctx := context.Background()

	domID, err := m.AddMailDomain(ctx, scope, "Mail.Example.test")
	if err != nil {
		t.Fatal(err)
	}
	// Mailbox on pending domain rejects (must be verified first).
	if _, err := m.CreateMailbox(ctx, scope, domID, "hello", 1024); !errors.Is(err, ErrInvalid) {
		t.Fatalf("mailbox on pending domain: %v", err)
	}
	if err := m.VerifyMailDomain(ctx, domID); err != nil {
		t.Fatal(err)
	}
	mbID, err := m.CreateMailbox(ctx, scope, domID, "hello", 1024)
	if err != nil {
		t.Fatal(err)
	}
	// Duplicate mailbox rejects (UNIQUE).
	if _, err := m.CreateMailbox(ctx, scope, domID, "hello", 512); err == nil {
		t.Fatal("duplicate mailbox accepted")
	}
	if _, err := m.CreateAlias(ctx, scope, domID, "info", "hello@mail.example.test"); err != nil {
		t.Fatal(err)
	}
	// Bad destination rejects.
	if _, err := m.CreateAlias(ctx, scope, domID, "sales", "not-an-email"); err == nil {
		t.Fatal("invalid alias destination accepted")
	}
	_ = mbID
}

func TestObservabilityAndTemplates(t *testing.T) {
	db := domFixture(t)
	scope, principal := seedScope(t, db)
	o := NewObserveService(db)
	tpl := NewTemplateService(db)
	ctx := context.Background()

	ws, err := db.CreateWorkspace(ctx, "website", principal, "site")
	if err != nil {
		t.Fatal(err)
	}
	res, err := db.CreateResource(ctx, ws.WorkspaceID, "", "application", "blog")
	if err != nil {
		t.Fatal(err)
	}
	v := 0.97
	if _, err := o.Record(ctx, scope, res.ResourceID, "health", "uptime_ratio", &v, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := o.Record(ctx, scope, res.ResourceID, "event", "deploy", nil, "release r1 active"); err != nil {
		t.Fatal(err)
	}
	// Invalid kind rejects.
	if _, err := o.Record(ctx, scope, res.ResourceID, "junk", "x", nil, ""); err == nil {
		t.Fatal("invalid observation kind accepted")
	}
	latest, err := o.LatestHealth(ctx, res.ResourceID)
	if err != nil || len(latest) != 2 {
		t.Fatalf("latest: %v err=%v", latest, err)
	}
	if err := o.Notify(ctx, scope, principal, "backup.completed", "manifest bak_x verified"); err != nil {
		t.Fatal(err)
	}

	tID, err := tpl.AddTemplate(ctx, ws.WorkspaceID, "wordpress", "WordPress", "cms", "php", "registry://wp-6.tar.zst")
	if err != nil {
		t.Fatal(err)
	}
	istID, err := tpl.QueueInstall(ctx, ws.WorkspaceID, res.ResourceID, tID)
	if err != nil {
		t.Fatal(err)
	}
	if istID == "" {
		t.Fatal("no install id")
	}
	// Installing onto a non-application resource rejects.
	res2, _ := db.CreateResource(ctx, ws.WorkspaceID, "", "filesystem entry", "x")
	if _, err := tpl.QueueInstall(ctx, ws.WorkspaceID, res2.ResourceID, tID); !errors.Is(err, ErrInvalid) {
		t.Fatalf("template onto filesystem entry: %v", err)
	}
}

func TestBackupManifestGating(t *testing.T) {
	db := domFixture(t)
	_, principal := seedScope(t, db)
	b := NewBackupService(db)
	ctx := context.Background()

	ws, _ := db.CreateWorkspace(ctx, "website", principal, "site")
	res, _ := db.CreateResource(ctx, ws.WorkspaceID, "", "filesystem entry", "site")
	digest := sha256.Sum256([]byte("backup-bytes"))
	dhex := hex.EncodeToString(digest[:])

	bakID, err := b.RecordManifest(ctx, ws.WorkspaceID, res.ResourceID, dhex, "keyv1", "schema-0006", "offhost://vault/backups/b1", 4096)
	if err != nil {
		t.Fatal(err)
	}
	// Unverified manifest cannot restore.
	if err := b.GateRestore(ctx, bakID, "keyv1"); err == nil || !strings.Contains(err.Error(), "not verified") {
		t.Fatalf("unverified restore gate: %v", err)
	}
	if err := b.VerifyBackup(ctx, bakID); err != nil {
		t.Fatal(err)
	}
	// Key reference mismatch rejects (independent key gate).
	if err := b.GateRestore(ctx, bakID, "keyv9"); err == nil {
		t.Fatal("key mismatch accepted")
	}
	if err := b.GateRestore(ctx, bakID, "keyv1"); err != nil {
		t.Fatalf("legitimate restore gate: %v", err)
	}
	// Foreign-scope resource rejects.
	otherScope, _ := seedScope(t, db)
	if _, err := b.RecordManifest(ctx, otherScope, res.ResourceID, dhex, "k", "s", "t", 1); !errors.Is(err, ErrInvalid) {
		t.Fatalf("foreign scope manifest: %v", err)
	}
}

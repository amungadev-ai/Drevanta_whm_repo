package store

// Domain read queries (G40 full-persona wave): scope-scoped lists backing the
// HTTP API for support, commercial hosting, developer pipelines, mail,
// observability, templates, backups and schedules. Ordering is newest-first
// with bounded limits; every query is scope-pinned (tenant isolation at the
// store layer).

import (
	"context"
	"database/sql"
	"errors"
)

func queryAll[T any](ctx context.Context, db *DB, scan func(func(...any) error) T, q string, args ...any) ([]T, error) {
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		out = append(out, scan(rows.Scan))
	}
	return out, rows.Err()
}

// ---- Support ----

type TicketRow struct {
	TicketID    string
	ScopeID     string
	RequesterID string
	Subject     string
	State       string
	Priority    string
	SlaDueAt    sql.NullString
	CreatedAt   string
	UpdatedAt   string
}

func (db *DB) ListTickets(ctx context.Context, scopeID string, limit int) ([]TicketRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	return queryAll(ctx, db, func(sc func(...any) error) TicketRow {
		var r TicketRow
		_ = sc(&r.TicketID, &r.ScopeID, &r.RequesterID, &r.Subject, &r.State, &r.Priority, &r.SlaDueAt, &r.CreatedAt, &r.UpdatedAt)
		return r
	}, `SELECT ticket_id, scope_id, requester_id, subject, state, priority, sla_due_at, created_at, updated_at
	    FROM ticket WHERE scope_id = ? ORDER BY updated_at DESC LIMIT ?`, scopeID, limit)
}

func (db *DB) TicketByID(ctx context.Context, scopeID, ticketID string) (TicketRow, error) {
	row := db.QueryRowContext(ctx, `SELECT ticket_id, scope_id, requester_id, subject, state, priority, sla_due_at, created_at, updated_at
	    FROM ticket WHERE scope_id = ? AND ticket_id = ?`, scopeID, ticketID)
	var r TicketRow
	err := row.Scan(&r.TicketID, &r.ScopeID, &r.RequesterID, &r.Subject, &r.State, &r.Priority, &r.SlaDueAt, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return TicketRow{}, ErrNotFound
	}
	return r, err
}

type TicketMessageRow struct {
	MessageID string
	TicketID  string
	AuthorID  string
	Body      string
	CreatedAt string
}

func (db *DB) TicketMessages(ctx context.Context, ticketID string) ([]TicketMessageRow, error) {
	return queryAll(ctx, db, func(sc func(...any) error) TicketMessageRow {
		var r TicketMessageRow
		_ = sc(&r.MessageID, &r.TicketID, &r.AuthorID, &r.Body, &r.CreatedAt)
		return r
	}, `SELECT message_id, ticket_id, author_id, body, created_at FROM ticket_message
	    WHERE ticket_id = ? ORDER BY created_at ASC`, ticketID)
}

type KBArticleRow struct {
	ArticleID string
	Slug      string
	Title     string
	Body      string
	State     string
	UpdatedAt string
}

func (db *DB) ListPublishedArticles(ctx context.Context, scopeID string) ([]KBArticleRow, error) {
	return queryAll(ctx, db, func(sc func(...any) error) KBArticleRow {
		var r KBArticleRow
		_ = sc(&r.ArticleID, &r.Slug, &r.Title, &r.Body, &r.State, &r.UpdatedAt)
		return r
	}, `SELECT article_id, slug, title, body, state, updated_at FROM kb_article
	    WHERE scope_id = ? AND state = 'published' ORDER BY updated_at DESC LIMIT 200`, scopeID)
}

// ---- Commercial hosting ----

type PlanRow struct {
	PlanID     string
	Name       string
	QuotaJSON  string
	PriceCents int64
	Currency   string
	State      string
	CreatedAt  string
}

func (db *DB) ListPlans(ctx context.Context, scopeID string) ([]PlanRow, error) {
	return queryAll(ctx, db, func(sc func(...any) error) PlanRow {
		var r PlanRow
		_ = sc(&r.PlanID, &r.Name, &r.QuotaJSON, &r.PriceCents, &r.Currency, &r.State, &r.CreatedAt)
		return r
	}, `SELECT plan_id, name, quota_json, price_cents, currency, state, created_at FROM hosting_plan
	    WHERE scope_id = ? ORDER BY created_at DESC LIMIT 200`, scopeID)
}

type CustomerRow struct {
	CustomerID string
	Email      string
	Name       string
	State      string
	CreatedAt  string
}

func (db *DB) ListCustomers(ctx context.Context, scopeID string) ([]CustomerRow, error) {
	return queryAll(ctx, db, func(sc func(...any) error) CustomerRow {
		var r CustomerRow
		_ = sc(&r.CustomerID, &r.Email, &r.Name, &r.State, &r.CreatedAt)
		return r
	}, `SELECT customer_id, email, name, state, created_at FROM customer
	    WHERE scope_id = ? ORDER BY created_at DESC LIMIT 500`, scopeID)
}

type AccountRow struct {
	AccountID  string
	CustomerID string
	PlanID     string
	WorkspaceID sql.NullString
	State      string
	CreatedAt  string
}

func (db *DB) ListAccounts(ctx context.Context, scopeID string) ([]AccountRow, error) {
	return queryAll(ctx, db, func(sc func(...any) error) AccountRow {
		var r AccountRow
		_ = sc(&r.AccountID, &r.CustomerID, &r.PlanID, &r.WorkspaceID, &r.State, &r.CreatedAt)
		return r
	}, `SELECT account_id, customer_id, plan_id, workspace_id, state, created_at FROM hosting_account
	    WHERE scope_id = ? ORDER BY created_at DESC LIMIT 500`, scopeID)
}

type InvoiceRow struct {
	InvoiceID  string
	CustomerID string
	Number     string
	TotalCents int64
	Currency   string
	State      string
	IssuedAt   sql.NullString
	DueAt      sql.NullString
	CreatedAt  string
}

func (db *DB) ListInvoices(ctx context.Context, scopeID string) ([]InvoiceRow, error) {
	return queryAll(ctx, db, func(sc func(...any) error) InvoiceRow {
		var r InvoiceRow
		_ = sc(&r.InvoiceID, &r.CustomerID, &r.Number, &r.TotalCents, &r.Currency, &r.State, &r.IssuedAt, &r.DueAt, &r.CreatedAt)
		return r
	}, `SELECT invoice_id, customer_id, number, total_cents, currency, state, issued_at, due_at, created_at FROM invoice
	    WHERE scope_id = ? ORDER BY created_at DESC LIMIT 500`, scopeID)
}

// ---- Developer pipelines ----

type SourceConnectionRow struct {
	ConnectionID  string
	RepoURL       string
	DefaultBranch string
	State         string
	CreatedAt     string
}

func (db *DB) ListSourceConnections(ctx context.Context, scopeID string) ([]SourceConnectionRow, error) {
	return queryAll(ctx, db, func(sc func(...any) error) SourceConnectionRow {
		var r SourceConnectionRow
		_ = sc(&r.ConnectionID, &r.RepoURL, &r.DefaultBranch, &r.State, &r.CreatedAt)
		return r
	}, `SELECT connection_id, repo_url, default_branch, state, created_at FROM source_connection
	    WHERE scope_id = ? ORDER BY created_at DESC LIMIT 200`, scopeID)
}

type BuildRow struct {
	BuildID        string
	ConnectionID   string
	CommitRef      string
	State          string
	ArtifactDigest string
	ArtifactSize   int64
	CreatedAt      string
	FinishedAt     sql.NullString
}

func (db *DB) ListBuilds(ctx context.Context, scopeID string, limit int) ([]BuildRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return queryAll(ctx, db, func(sc func(...any) error) BuildRow {
		var r BuildRow
		_ = sc(&r.BuildID, &r.ConnectionID, &r.CommitRef, &r.State, &r.ArtifactDigest, &r.ArtifactSize, &r.CreatedAt, &r.FinishedAt)
		return r
	}, `SELECT build_id, connection_id, commit_ref, state, artifact_digest, artifact_size, created_at, finished_at FROM build
	    WHERE scope_id = ? ORDER BY created_at DESC LIMIT ?`, scopeID, limit)
}

type EnvironmentRow struct {
	EnvironmentID  string
	Name           string
	Kind           string
	CurrentRelease sql.NullString
	CreatedAt      string
}

func (db *DB) ListEnvironments(ctx context.Context, scopeID string) ([]EnvironmentRow, error) {
	return queryAll(ctx, db, func(sc func(...any) error) EnvironmentRow {
		var r EnvironmentRow
		_ = sc(&r.EnvironmentID, &r.Name, &r.Kind, &r.CurrentRelease, &r.CreatedAt)
		return r
	}, `SELECT environment_id, name, kind, current_release, created_at FROM environment
	    WHERE scope_id = ? ORDER BY created_at DESC LIMIT 200`, scopeID)
}

// ---- Mail ----

type MailDomainRow struct {
	MailDomainID string
	Domain       string
	State        string
	CreatedAt    string
}

func (db *DB) ListMailDomains(ctx context.Context, scopeID string) ([]MailDomainRow, error) {
	return queryAll(ctx, db, func(sc func(...any) error) MailDomainRow {
		var r MailDomainRow
		_ = sc(&r.MailDomainID, &r.Domain, &r.State, &r.CreatedAt)
		return r
	}, `SELECT mail_domain_id, domain, state, created_at FROM mail_domain
	    WHERE scope_id = ? ORDER BY created_at DESC LIMIT 200`, scopeID)
}

type MailboxRow struct {
	MailboxID    string
	MailDomainID string
	LocalPart    string
	QuotaMB      int64
	State        string
	CreatedAt    string
}

func (db *DB) ListMailboxes(ctx context.Context, scopeID string) ([]MailboxRow, error) {
	return queryAll(ctx, db, func(sc func(...any) error) MailboxRow {
		var r MailboxRow
		_ = sc(&r.MailboxID, &r.MailDomainID, &r.LocalPart, &r.QuotaMB, &r.State, &r.CreatedAt)
		return r
	}, `SELECT mailbox_id, mail_domain_id, local_part, quota_mb, state, created_at FROM mailbox
	    WHERE scope_id = ? ORDER BY created_at DESC LIMIT 500`, scopeID)
}

type MailAliasRow struct {
	AliasID      string
	MailDomainID string
	LocalPart    string
	Destination  string
	CreatedAt    string
}

func (db *DB) ListMailAliases(ctx context.Context, scopeID string) ([]MailAliasRow, error) {
	return queryAll(ctx, db, func(sc func(...any) error) MailAliasRow {
		var r MailAliasRow
		_ = sc(&r.AliasID, &r.MailDomainID, &r.LocalPart, &r.Destination, &r.CreatedAt)
		return r
	}, `SELECT alias_id, mail_domain_id, local_part, destination, created_at FROM mail_alias
	    WHERE scope_id = ? ORDER BY created_at DESC LIMIT 500`, scopeID)
}

// ---- Observability ----

type ObservationRow struct {
	ObservationID string
	ResourceID    string
	Kind          string
	Name          string
	ValueNum      sql.NullFloat64
	ValueText     sql.NullString
	ObservedAt    string
}

func (db *DB) ListObservations(ctx context.Context, scopeID, resourceID string, limit int) ([]ObservationRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT observation_id, resource_id, kind, name, value_num, value_text, observed_at FROM health_observation
	      WHERE scope_id = ?`
	args := []any{scopeID}
	if resourceID != "" {
		q += ` AND resource_id = ?`
		args = append(args, resourceID)
	}
	q += ` ORDER BY observed_at DESC LIMIT ?`
	args = append(args, limit)
	return queryAll(ctx, db, func(sc func(...any) error) ObservationRow {
		var r ObservationRow
		_ = sc(&r.ObservationID, &r.ResourceID, &r.Kind, &r.Name, &r.ValueNum, &r.ValueText, &r.ObservedAt)
		return r
	}, q, args...)
}

type NotificationRow struct {
	NotificationID string
	PrincipalID    sql.NullString
	Kind           string
	Body           string
	ReadAt         sql.NullString
	CreatedAt      string
}

// ListNotifications returns the principal's notifications plus scope-wide
// system notifications (principal_id IS NULL).
func (db *DB) ListNotifications(ctx context.Context, scopeID, principalID string, limit int) ([]NotificationRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return queryAll(ctx, db, func(sc func(...any) error) NotificationRow {
		var r NotificationRow
		_ = sc(&r.NotificationID, &r.PrincipalID, &r.Kind, &r.Body, &r.ReadAt, &r.CreatedAt)
		return r
	}, `SELECT notification_id, principal_id, kind, body, read_at, created_at FROM notification
	    WHERE scope_id = ? AND (principal_id = ? OR principal_id IS NULL)
	    ORDER BY created_at DESC LIMIT ?`, scopeID, principalID, limit)
}

// ---- Templates / backups / schedules ----

type TemplateRow struct {
	TemplateID  string
	Slug        string
	Title       string
	Category    string
	Runtime     string
	ArtifactRef string
	State       string
	CreatedAt   string
}

func (db *DB) ListTemplates(ctx context.Context, scopeID string) ([]TemplateRow, error) {
	return queryAll(ctx, db, func(sc func(...any) error) TemplateRow {
		var r TemplateRow
		_ = sc(&r.TemplateID, &r.Slug, &r.Title, &r.Category, &r.Runtime, &r.ArtifactRef, &r.State, &r.CreatedAt)
		return r
	}, `SELECT template_id, slug, title, category, runtime, artifact_ref, state, created_at FROM catalog_template
	    WHERE scope_id = ? AND state = 'listed' ORDER BY created_at DESC LIMIT 200`, scopeID)
}

type SiteBackupRow struct {
	BackupID   string
	ResourceID string
	SizeBytes  int64
	State      string
	CreatedAt  string
}

func (db *DB) ListSiteBackups(ctx context.Context, scopeID string) ([]SiteBackupRow, error) {
	return queryAll(ctx, db, func(sc func(...any) error) SiteBackupRow {
		var r SiteBackupRow
		_ = sc(&r.BackupID, &r.ResourceID, &r.SizeBytes, &r.State, &r.CreatedAt)
		return r
	}, `SELECT backup_id, resource_id, size_bytes, state, created_at FROM site_backup
	    WHERE scope_id = ? ORDER BY created_at DESC LIMIT 200`, scopeID)
}

type ScheduleRow struct {
	ScheduleID   string
	ResourceID   string
	ActionID     string
	EverySeconds int64
	State        string
	NextRunAt    string
	LastRunAt    sql.NullString
	CreatedAt    string
}

func (db *DB) ListSchedules(ctx context.Context, scopeID string) ([]ScheduleRow, error) {
	return queryAll(ctx, db, func(sc func(...any) error) ScheduleRow {
		var r ScheduleRow
		_ = sc(&r.ScheduleID, &r.ResourceID, &r.ActionID, &r.EverySeconds, &r.State, &r.NextRunAt, &r.LastRunAt, &r.CreatedAt)
		return r
	}, `SELECT schedule_id, resource_id, action_id, every_seconds, state, next_run_at, last_run_at, created_at FROM schedule
	    WHERE scope_id = ? AND state != 'removed' ORDER BY created_at DESC LIMIT 200`, scopeID)
}

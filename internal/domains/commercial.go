// Package domains implements the commercial hosting domain (phases 20,
// BIZ-01/02/03) and support (SUP-01/02).
//
// Contract basis: DEC-015 development disposition — native billing core with
// a gateway adapter seam; payment provider credentials and tax rules remain
// owner inputs (BLK-001). Money amounts are integer cents; currency explicit.
package domains

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"drevanta/internal/store"
)

var ErrInvalid = errors.New("invalid_input")
var ErrState = errors.New("invalid_state")

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Commercial implements BIZ capabilities.
type Commercial struct {
	db *store.DB
	// Gateway adapter seam (BLK-001): fixture gateway in tests; real
	// providers need owner-issued credentials before release certification.
	Gateway PaymentGateway
}

// PaymentGateway is the provider seam. Charge records an intent with the
// provider and returns a provider reference; idempotency is the caller's
// payment row identity.
type PaymentGateway interface {
	Charge(ctx context.Context, invoiceNumber string, amountCents int64, currency string) (gatewayRef string, err error)
}

// ManualGateway records payments without an external provider (default).
type ManualGateway struct{}

// Charge implements PaymentGateway (no external effect).
func (ManualGateway) Charge(_ context.Context, _ string, _ int64, _ string) (string, error) {
	return "manual", nil
}

// NewCommercial builds the service.
func NewCommercial(db *store.DB, gw PaymentGateway) *Commercial {
	if gw == nil {
		gw = ManualGateway{}
	}
	return &Commercial{db: db, Gateway: gw}
}

// CreatePlan defines a commercial plan with quotas and pricing.
func (c *Commercial) CreatePlan(ctx context.Context, scopeID, name string, quotaJSON string, priceCents int64, currency string) (string, error) {
	if name == "" || priceCents < 0 || len(quotaJSON) > 4096 {
		return "", ErrInvalid
	}
	if currency == "" {
		currency = "USD"
	}
	id := store.NewID("plan")
	_, err := c.db.ExecContext(ctx,
		`INSERT INTO hosting_plan(plan_id, scope_id, name, quota_json, price_cents, currency, state, created_at)
		 VALUES(?,?,?,?,?,?,'active',?)`,
		id, scopeID, name, quotaJSON, priceCents, currency, store.Now())
	if err != nil {
		return "", err
	}
	return id, nil
}

// CreateCustomer registers a customer (unique email per org).
func (c *Commercial) CreateCustomer(ctx context.Context, scopeID, email, name string) (string, error) {
	if !emailRe.MatchString(email) || len(name) > 200 {
		return "", ErrInvalid
	}
	id := store.NewID("cus")
	_, err := c.db.ExecContext(ctx,
		`INSERT INTO customer(customer_id, scope_id, email, name, state, created_at)
		 VALUES(?,?,?,?,'active',?)`,
		id, scopeID, strings.ToLower(email), name, store.Now())
	if err != nil {
		return "", err
	}
	return id, nil
}

// ProvisionAccount creates a hosting account for a customer on a plan
// (BIZ-01: order → account). The linked website workspace is provisioned by
// the account.provision operation flow; here the record is born 'pending'.
func (c *Commercial) ProvisionAccount(ctx context.Context, scopeID, customerID, planID string) (string, error) {
	var cState, cScope, pScope string
	err := c.db.QueryRowContext(ctx,
		`SELECT state, scope_id FROM customer WHERE customer_id=?`, customerID).Scan(&cState, &cScope)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (cState != "active" || cScope != scopeID)) {
		return "", ErrInvalid
	}
	if err != nil {
		return "", err
	}
	err = c.db.QueryRowContext(ctx,
		`SELECT scope_id FROM hosting_plan WHERE plan_id=? AND state='active'`, planID).Scan(&pScope)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && pScope != scopeID) {
		return "", ErrInvalid
	}
	if err != nil {
		return "", err
	}
	id := store.NewID("acc")
	if _, err := c.db.ExecContext(ctx,
		`INSERT INTO hosting_account(account_id, scope_id, customer_id, plan_id, state, created_at)
		 VALUES(?,?,?,?, 'pending', ?)`,
		id, scopeID, customerID, planID, store.Now()); err != nil {
		return "", err
	}
	if _, err := c.db.ExecContext(ctx,
		`INSERT INTO subscription(subscription_id, scope_id, account_id, plan_id, state, started_at)
		 VALUES(?,?,?,?,'active',?)`,
		store.NewID("sub"), scopeID, id, planID, store.Now()); err != nil {
		return "", err
	}
	return id, nil
}

// ActivateAccount marks the account active once its workspace exists.
func (c *Commercial) ActivateAccount(ctx context.Context, accountID, workspaceID string) error {
	_, err := c.db.ExecContext(ctx,
		`UPDATE hosting_account SET state='active', workspace_id=? WHERE account_id=? AND state='pending'`,
		workspaceID, accountID)
	return err
}

// CreateInvoice issues an invoice with lines; totals are computed from lines
// plus explicit tax per line (tax rules remain owner inputs — BLK-001).
func (c *Commercial) CreateInvoice(ctx context.Context, scopeID, customerID string, dueIn time.Duration, lines []InvoiceLine) (string, int64, error) {
	if len(lines) == 0 {
		return "", 0, ErrInvalid
	}
	var cScope string
	err := c.db.QueryRowContext(ctx, `SELECT scope_id FROM customer WHERE customer_id=?`, customerID).Scan(&cScope)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && cScope != scopeID) {
		return "", 0, ErrInvalid
	}
	if err != nil {
		return "", 0, err
	}
	total := int64(0)
	for _, l := range lines {
		if l.Quantity < 1 || l.UnitCents < 0 || l.TaxCents < 0 {
			return "", 0, ErrInvalid
		}
		total += int64(l.Quantity)*l.UnitCents + l.TaxCents
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return "", 0, err
	}
	defer tx.Rollback()
	id := store.NewID("inv")
	number := fmt.Sprintf("INV-%s", id[len(id)-10:])
	now := store.Now()
	due := time.Now().UTC().Add(dueIn).Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO invoice(invoice_id, scope_id, customer_id, number, total_cents, currency, state, issued_at, due_at, created_at)
		 VALUES(?,?,?,?,?,?,'issued',?,?,?)`,
		id, scopeID, customerID, number, total, "USD", now, due, now); err != nil {
		return "", 0, err
	}
	for _, l := range lines {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO invoice_line(line_id, invoice_id, description, quantity, unit_cents, tax_cents)
			 VALUES(?,?,?,?,?,?)`,
			store.NewID("lin"), id, l.Description, l.Quantity, l.UnitCents, l.TaxCents); err != nil {
			return "", 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return "", 0, err
	}
	return id, total, nil
}

// InvoiceLine is one invoice row.
type InvoiceLine struct {
	Description string
	Quantity    int64
	UnitCents   int64
	TaxCents    int64
}

// RecordPayment records a payment against an invoice through the gateway
// adapter; success marks the invoice paid (idempotent per invoice: a second
// payment on a paid invoice conflicts).
func (c *Commercial) RecordPayment(ctx context.Context, scopeID, invoiceID string, amountCents int64) (string, error) {
	var state, number string
	var total int64
	err := c.db.QueryRowContext(ctx,
		`SELECT state, number, total_cents FROM invoice WHERE invoice_id=? AND scope_id=?`,
		invoiceID, scopeID).Scan(&state, &number, &total)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrInvalid
	}
	if err != nil {
		return "", err
	}
	if state == "paid" {
		return "", ErrState // idempotency: invoice settles once
	}
	if state != "issued" && state != "draft" {
		return "", ErrState
	}
	if amountCents < total {
		return "", ErrInvalid // underpayment rejects; overpayment handling is policy (owner input)
	}
	ref, err := c.Gateway.Charge(ctx, number, amountCents, "USD")
	if err != nil {
		return "", err
	}
	id := store.NewID("pay")
	if _, err := c.db.ExecContext(ctx,
		`INSERT INTO payment(payment_id, scope_id, customer_id, invoice_id, gateway, gateway_ref, amount_cents, currency, state, created_at)
		 SELECT ?, scope_id, customer_id, ?, ?, ?, ?, 'USD', 'succeeded', ? FROM invoice WHERE invoice_id=?`,
		id, invoiceID, "manual", ref, amountCents, store.Now(), invoiceID); err != nil {
		return "", err
	}
	if _, err := c.db.ExecContext(ctx,
		`UPDATE invoice SET state='paid' WHERE invoice_id=? AND state IN ('issued','draft')`,
		invoiceID); err != nil {
		return "", err
	}
	return id, nil
}

// AdjustCredit applies a credit/debit entry (billing.adjust).
func (c *Commercial) AdjustCredit(ctx context.Context, scopeID, customerID string, deltaCents int64, reason string) error {
	if deltaCents == 0 || len(reason) > 500 {
		return ErrInvalid
	}
	_, err := c.db.ExecContext(ctx,
		`INSERT INTO credit_ledger(entry_id, scope_id, customer_id, delta_cents, reason, created_at)
		 VALUES(?,?,?,?,?,?)`,
		store.NewID("cre"), scopeID, customerID, deltaCents, reason, store.Now())
	return err
}

// ---- Support (SUP-01/02) ----

// Support implements tickets and the knowledge base.
type Support struct {
	db *store.DB
	// SLADue computes the SLA deadline for a priority (policy seam; DEC-018
	// fixture defaults pending owner ratification).
	SLADue func(priority string) time.Duration
}

// NewSupport builds the service with fixture-default SLA windows.
func NewSupport(db *store.DB) *Support {
	return &Support{db: db, SLADue: func(priority string) time.Duration {
		switch priority {
		case "urgent":
			return 1 * time.Hour
		case "high":
			return 4 * time.Hour
		case "low":
			return 72 * time.Hour
		default:
			return 24 * time.Hour
		}
	}}
}

// CreateTicket opens a ticket with an SLA deadline.
func (s *Support) CreateTicket(ctx context.Context, scopeID, requesterID, subject, priority, body string) (string, error) {
	if strings.TrimSpace(subject) == "" || len(subject) > 300 || len(body) > 20000 {
		return "", ErrInvalid
	}
	switch priority {
	case "low", "normal", "high", "urgent":
	default:
		return "", ErrInvalid
	}
	id := store.NewID("tkt")
	now := store.Now()
	sla := time.Now().UTC().Add(s.SLADue(priority)).Format(time.RFC3339Nano)
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO ticket(ticket_id, scope_id, requester_id, subject, state, priority, sla_due_at, created_at, updated_at)
		 VALUES(?,?,?,?,'open',?,?,?,?)`,
		id, scopeID, requesterID, subject, priority, sla, now, now); err != nil {
		return "", err
	}
	if body != "" {
		if _, err := s.db.ExecContext(ctx,
			`INSERT INTO ticket_message(message_id, ticket_id, author_id, body, created_at)
			 VALUES(?,?,?,?,?)`,
			store.NewID("msg"), id, requesterID, body, now); err != nil {
			return "", err
		}
	}
	return id, nil
}

// AnswerTicket appends a message and moves the state forward (open→answered
// by staff; answered→pending by requester — state machine enforced).
func (s *Support) AnswerTicket(ctx context.Context, ticketID, authorID, body string, byStaff bool) error {
	if strings.TrimSpace(body) == "" || len(body) > 20000 {
		return ErrInvalid
	}
	var state string
	err := s.db.QueryRowContext(ctx, `SELECT state FROM ticket WHERE ticket_id=?`, ticketID).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) || err != nil {
		return ErrInvalid
	}
	next := ""
	switch {
	case byStaff && (state == "open" || state == "pending"):
		next = "answered"
	case !byStaff && state == "answered":
		next = "pending"
	default:
		return ErrState
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO ticket_message(message_id, ticket_id, author_id, body, created_at) VALUES(?,?,?,?,?)`,
		store.NewID("msg"), ticketID, authorID, body, store.Now()); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE ticket SET state=?, updated_at=? WHERE ticket_id=?`, next, store.Now(), ticketID)
	return err
}

// ResolveTicket closes the ticket (terminal: resolved).
func (s *Support) ResolveTicket(ctx context.Context, ticketID string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE ticket SET state='resolved', updated_at=? WHERE ticket_id=? AND state IN ('open','pending','answered')`,
		store.Now(), ticketID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrState
	}
	return nil
}

// PublishArticle creates or updates a knowledge-base article (SUP-02).
func (s *Support) PublishArticle(ctx context.Context, scopeID, slug, title, body string) (string, error) {
	if slug == "" || len(slug) > 100 || title == "" || len(body) > 100000 {
		return "", ErrInvalid
	}
	if !regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(slug) {
		return "", ErrInvalid
	}
	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT article_id FROM kb_article WHERE scope_id=? AND slug=?`, scopeID, slug).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		id = store.NewID("kb")
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO kb_article(article_id, scope_id, slug, title, body, state, updated_at)
			 VALUES(?,?,?,?,?,'published',?)`,
			id, scopeID, slug, title, body, store.Now())
		return id, err
	}
	if err != nil {
		return "", err
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE kb_article SET title=?, body=?, updated_at=? WHERE article_id=?`,
		title, body, store.Now(), id)
	return id, err
}

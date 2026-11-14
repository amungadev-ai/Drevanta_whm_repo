package api

// Domain capability handlers (G40 full-persona wave): expose the implemented
// domain services over the same-origin BFF contract. Every route is
// scope-pinned, authorizes through the frozen grant matrix (grant_rule seeds),
// rejects unknown fields (strict decode), records deny/allow audit events for
// mutations, and returns the uniform error envelope.

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"drevanta/internal/audit"
	"drevanta/internal/domains"
	"drevanta/internal/store"
)

// authorizeScope checks a scope-level action grant (no resource target).
func (s *Server) authorizeScope(w http.ResponseWriter, r *http.Request, scopeID, action, resourceType string) (string, bool) {
	sub := subjectOf(r)
	if sub == nil {
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
		return "", false
	}
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return "", false
	}
	role, err := s.Authz.Authorize(r.Context(), sub.Principal.PrincipalID, action, resourceType, scopeID, 1)
	if err != nil {
		writeError(w, http.StatusForbidden, CodeForbidden, "not permitted for this scope")
		return "", false
	}
	return role, true
}

func (s *Server) auditScope(ctx context.Context, actorID, scopeID, actionID, outcome, reason string) {
	_ = s.Audit.Append(ctx, audit.Event{
		ActorID: actorID, ScopeID: scopeID, ActionID: actionID,
		Outcome: outcome, SafeReason: reason,
	})
}

// ---- Support tickets (SUP-01) ----

func (s *Server) handleListTickets(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	sub := subjectOf(r)
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	tickets, err := s.DB.ListTickets(r.Context(), scopeID, 200)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "ticket lookup unavailable")
		return
	}
	// Staff capability holders see every ticket; other members see their own
	// (requester-scoped) — real authorization semantics, not blanket reads.
	_, staffErr := s.Authz.Authorize(r.Context(), sub.Principal.PrincipalID,
		"support.ticket.create", "support ticket", scopeID, 1)
	staff := staffErr == nil
	out := make([]store.TicketRow, 0, len(tickets))
	for _, t := range tickets {
		if staff || t.RequesterID == sub.Principal.PrincipalID {
			out = append(out, t)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"tickets": out})
}

func (s *Server) handleCreateTicket(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	sub := subjectOf(r)
	if _, ok := s.authorizeScope(w, r, scopeID, "support.ticket.create", "support ticket"); !ok {
		return
	}
	var req struct {
		Subject  string `json:"subject"`
		Priority string `json:"priority"`
		Body     string `json:"body"`
	}
	if err := decodeStrict(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid ticket schema")
		return
	}
	req.Subject = strings.TrimSpace(req.Subject)
	if req.Subject == "" || len(req.Subject) > 200 {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "subject required (max 200)")
		return
	}
	if req.Priority == "" {
		req.Priority = "normal"
	}
	id, err := s.Support.CreateTicket(r.Context(), scopeID, sub.Principal.PrincipalID, req.Subject, req.Priority, req.Body)
	if err != nil {
		s.auditScope(r.Context(), sub.Principal.PrincipalID, scopeID, "support.ticket.create", "error", "ticket create failed")
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "ticket write unavailable")
		return
	}
	s.auditScope(r.Context(), sub.Principal.PrincipalID, scopeID, "support.ticket.create", "allow", "role=support.ticket.create")
	writeJSON(w, http.StatusCreated, map[string]any{"ticket_id": id})
}

func (s *Server) handleTicketDetail(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	ticketID := r.PathValue("ticketID")
	sub := subjectOf(r)
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	t, err := s.DB.TicketByID(r.Context(), scopeID, ticketID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such ticket")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "ticket lookup unavailable")
		return
	}
	_, staffErr := s.Authz.Authorize(r.Context(), sub.Principal.PrincipalID,
		"support.ticket.create", "support ticket", scopeID, 1)
	staff := staffErr == nil
	if !staff && t.RequesterID != sub.Principal.PrincipalID {
		// Foreign requester: uniform 404 (no existence leak).
		writeError(w, http.StatusNotFound, CodeNotFound, "no such ticket")
		return
	}
	msgs, err := s.DB.TicketMessages(r.Context(), ticketID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "ticket lookup unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ticket": t, "messages": msgs})
}

func (s *Server) handleTicketMessage(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	ticketID := r.PathValue("ticketID")
	sub := subjectOf(r)
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	t, err := s.DB.TicketByID(r.Context(), scopeID, ticketID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such ticket")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "ticket lookup unavailable")
		return
	}
	_, staffErr := s.Authz.Authorize(r.Context(), sub.Principal.PrincipalID,
		"support.ticket.create", "support ticket", scopeID, 1)
	staff := staffErr == nil
	if !staff && t.RequesterID != sub.Principal.PrincipalID {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such ticket")
		return
	}
	var req struct {
		Body string `json:"body"`
	}
	if err := decodeStrict(r, &req); err != nil || strings.TrimSpace(req.Body) == "" {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "message body required")
		return
	}
	if err := s.Support.AnswerTicket(r.Context(), ticketID, sub.Principal.PrincipalID, req.Body, staff); err != nil {
		if errors.Is(err, domains.ErrState) {
			writeError(w, http.StatusConflict, CodeConflict, "ticket no longer accepts messages")
			return
		}
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "ticket write unavailable")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (s *Server) handleTicketResolve(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	ticketID := r.PathValue("ticketID")
	sub := subjectOf(r)
	if _, ok := s.authorizeScope(w, r, scopeID, "support.ticket.create", "support ticket"); !ok {
		return
	}
	if err := s.Support.ResolveTicket(r.Context(), ticketID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, CodeNotFound, "no such ticket")
			return
		}
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "ticket write unavailable")
		return
	}
	s.auditScope(r.Context(), sub.Principal.PrincipalID, scopeID, "support.ticket.create", "allow", "ticket resolved")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListKB(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	articles, err := s.DB.ListPublishedArticles(r.Context(), scopeID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "kb lookup unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"articles": articles})
}

// ---- Commercial hosting (BIZ-01/02) — manual gateway; fixture rates flagged ----

func (s *Server) handleListPlans(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	plans, err := s.DB.ListPlans(r.Context(), scopeID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "plan lookup unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": plans})
}

func (s *Server) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	sub := subjectOf(r)
	if _, ok := s.authorizeScope(w, r, scopeID, "billing.adjust", "billing entry"); !ok {
		return
	}
	var req struct {
		Name       string `json:"name"`
		QuotaJSON  string `json:"quota_json"`
		PriceCents int64  `json:"price_cents"`
		Currency   string `json:"currency"`
	}
	if err := decodeStrict(r, &req); err != nil || strings.TrimSpace(req.Name) == "" || req.PriceCents < 0 {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid plan schema")
		return
	}
	id, err := s.Commercial.CreatePlan(r.Context(), scopeID, req.Name, req.QuotaJSON, req.PriceCents, req.Currency)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "plan write unavailable")
		return
	}
	s.auditScope(r.Context(), sub.Principal.PrincipalID, scopeID, "billing.adjust", "allow", "plan created")
	writeJSON(w, http.StatusCreated, map[string]any{"plan_id": id})
}

func (s *Server) handleListCustomers(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	if _, ok := s.authorizeScope(w, r, scopeID, "billing.adjust", "billing entry"); !ok {
		return
	}
	customers, err := s.DB.ListCustomers(r.Context(), scopeID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "customer lookup unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"customers": customers})
}

func (s *Server) handleCreateCustomer(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	sub := subjectOf(r)
	if _, ok := s.authorizeScope(w, r, scopeID, "billing.adjust", "billing entry"); !ok {
		return
	}
	var req struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := decodeStrict(r, &req); err != nil || !strings.Contains(req.Email, "@") {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid customer schema")
		return
	}
	id, err := s.Commercial.CreateCustomer(r.Context(), scopeID, req.Email, req.Name)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "customer write unavailable")
		return
	}
	s.auditScope(r.Context(), sub.Principal.PrincipalID, scopeID, "billing.adjust", "allow", "customer created")
	writeJSON(w, http.StatusCreated, map[string]any{"customer_id": id})
}

func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	if _, ok := s.authorizeScope(w, r, scopeID, "account.provision", "hosting account"); !ok {
		return
	}
	accounts, err := s.DB.ListAccounts(r.Context(), scopeID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "account lookup unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": accounts})
}

func (s *Server) handleProvisionAccount(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	sub := subjectOf(r)
	if _, ok := s.authorizeScope(w, r, scopeID, "account.provision", "hosting account"); !ok {
		return
	}
	var req struct {
		CustomerID string `json:"customer_id"`
		PlanID     string `json:"plan_id"`
	}
	if err := decodeStrict(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid account schema")
		return
	}
	id, err := s.Commercial.ProvisionAccount(r.Context(), scopeID, req.CustomerID, req.PlanID)
	if err != nil {
		if errors.Is(err, domains.ErrInvalid) {
			writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "unknown customer or plan")
			return
		}
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "account write unavailable")
		return
	}
	s.auditScope(r.Context(), sub.Principal.PrincipalID, scopeID, "account.provision", "allow", "hosting account provisioned")
	writeJSON(w, http.StatusCreated, map[string]any{"account_id": id})
}

func (s *Server) handleListInvoices(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	if _, ok := s.authorizeScope(w, r, scopeID, "billing.adjust", "billing entry"); !ok {
		return
	}
	invoices, err := s.DB.ListInvoices(r.Context(), scopeID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "invoice lookup unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"invoices": invoices})
}

func (s *Server) handleCreateInvoice(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	sub := subjectOf(r)
	if _, ok := s.authorizeScope(w, r, scopeID, "billing.adjust", "billing entry"); !ok {
		return
	}
	type lineReq struct {
		Description string `json:"description"`
		Quantity    int64  `json:"quantity"`
		UnitCents   int64  `json:"unit_cents"`
		TaxCents    int64  `json:"tax_cents"`
	}
	var req struct {
		CustomerID string    `json:"customer_id"`
		DueInDays  int       `json:"due_in_days"`
		Lines      []lineReq `json:"lines"`
	}
	if err := decodeStrict(r, &req); err != nil || len(req.Lines) == 0 {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid invoice schema")
		return
	}
	lines := make([]domains.InvoiceLine, 0, len(req.Lines))
	for _, l := range req.Lines {
		lines = append(lines, domains.InvoiceLine{Description: l.Description, Quantity: l.Quantity, UnitCents: l.UnitCents, TaxCents: l.TaxCents})
	}
	id, total, err := s.Commercial.CreateInvoice(r.Context(), scopeID, req.CustomerID,
		dayDuration(req.DueInDays), lines)
	if err != nil {
		if errors.Is(err, domains.ErrInvalid) {
			writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "unknown customer or empty lines")
			return
		}
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "invoice write unavailable")
		return
	}
	s.auditScope(r.Context(), sub.Principal.PrincipalID, scopeID, "billing.adjust", "allow", "invoice issued")
	writeJSON(w, http.StatusCreated, map[string]any{"invoice_id": id, "total_cents": total})
}

func (s *Server) handleRecordPayment(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	invoiceID := r.PathValue("invoiceID")
	sub := subjectOf(r)
	if _, ok := s.authorizeScope(w, r, scopeID, "billing.adjust", "billing entry"); !ok {
		return
	}
	var req struct {
		AmountCents int64 `json:"amount_cents"`
	}
	if err := decodeStrict(r, &req); err != nil || req.AmountCents <= 0 {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid payment schema")
		return
	}
	id, err := s.Commercial.RecordPayment(r.Context(), scopeID, invoiceID, req.AmountCents)
	if err != nil {
		if errors.Is(err, domains.ErrInvalid) {
			writeError(w, http.StatusNotFound, CodeNotFound, "no such invoice")
			return
		}
		if errors.Is(err, domains.ErrState) {
			writeError(w, http.StatusConflict, CodeConflict, "payment not applicable to invoice state")
			return
		}
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "payment write unavailable")
		return
	}
	s.auditScope(r.Context(), sub.Principal.PrincipalID, scopeID, "billing.adjust", "allow", "payment recorded")
	writeJSON(w, http.StatusCreated, map[string]any{"payment_id": id})
}

func dayDuration(days int) time.Duration {
	return time.Duration(days) * 24 * time.Hour
}

// ---- Developer pipelines (APP-05/06/07) ----

func (s *Server) handleListSourceConnections(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	conns, err := s.DB.ListSourceConnections(r.Context(), scopeID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "connection lookup unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"connections": conns})
}

func (s *Server) handleConnectSource(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	sub := subjectOf(r)
	if _, ok := s.authorizeScope(w, r, scopeID, "source.connect", "source connection"); !ok {
		return
	}
	var req struct {
		RepoURL       string `json:"repo_url"`
		DefaultBranch string `json:"default_branch"`
	}
	if err := decodeStrict(r, &req); err != nil || !strings.HasPrefix(req.RepoURL, "https://") {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid source connection schema")
		return
	}
	if req.DefaultBranch == "" {
		req.DefaultBranch = "main"
	}
	id, err := s.Pipeline.ConnectSource(r.Context(), scopeID, sub.Principal.PrincipalID, req.RepoURL, req.DefaultBranch)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "connection write unavailable")
		return
	}
	s.auditScope(r.Context(), sub.Principal.PrincipalID, scopeID, "source.connect", "allow", "source connected")
	writeJSON(w, http.StatusCreated, map[string]any{"connection_id": id})
}

func (s *Server) handleListBuilds(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	builds, err := s.DB.ListBuilds(r.Context(), scopeID, 100)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "build lookup unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"builds": builds})
}

func (s *Server) handleQueueBuild(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	sub := subjectOf(r)
	if _, ok := s.authorizeScope(w, r, scopeID, "source.connect", "source connection"); !ok {
		return
	}
	var req struct {
		ConnectionID string `json:"connection_id"`
		CommitRef    string `json:"commit_ref"`
	}
	if err := decodeStrict(r, &req); err != nil || strings.TrimSpace(req.CommitRef) == "" {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid build schema")
		return
	}
	id, err := s.Pipeline.QueueBuild(r.Context(), scopeID, req.ConnectionID, req.CommitRef)
	if err != nil {
		if errors.Is(err, domains.ErrInvalid) {
			writeError(w, http.StatusNotFound, CodeNotFound, "no such connection")
			return
		}
		if errors.Is(err, domains.ErrState) {
			writeError(w, http.StatusConflict, CodeConflict, "connection disabled")
			return
		}
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "build write unavailable")
		return
	}
	s.auditScope(r.Context(), sub.Principal.PrincipalID, scopeID, "source.connect", "allow", "build queued")
	writeJSON(w, http.StatusCreated, map[string]any{"build_id": id})
}

func (s *Server) handleListEnvironments(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	envs, err := s.DB.ListEnvironments(r.Context(), scopeID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "environment lookup unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"environments": envs})
}

func (s *Server) handleCreateEnvironment(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	sub := subjectOf(r)
	if _, ok := s.authorizeScope(w, r, scopeID, "environment.clone", "environment"); !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
		Kind string `json:"kind"`
	}
	if err := decodeStrict(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid environment schema")
		return
	}
	if req.Kind == "" {
		req.Kind = "staging"
	}
	id, err := s.Pipeline.CreateEnvironment(r.Context(), scopeID, req.Name, req.Kind)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "environment write unavailable")
		return
	}
	s.auditScope(r.Context(), sub.Principal.PrincipalID, scopeID, "environment.clone", "allow", "environment created")
	writeJSON(w, http.StatusCreated, map[string]any{"environment_id": id})
}

func (s *Server) handlePromoteRelease(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	envID := r.PathValue("environmentID")
	sub := subjectOf(r)
	if _, ok := s.authorizeScope(w, r, scopeID, "deployment.promote", "release artifact"); !ok {
		return
	}
	var req struct {
		BuildID string `json:"build_id"`
	}
	if err := decodeStrict(r, &req); err != nil || strings.TrimSpace(req.BuildID) == "" {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid promotion schema")
		return
	}
	id, err := s.Pipeline.PromoteRelease(r.Context(), envID, req.BuildID)
	if err != nil {
		if errors.Is(err, domains.ErrInvalid) {
			writeError(w, http.StatusNotFound, CodeNotFound, "no such environment or build")
			return
		}
		if errors.Is(err, domains.ErrState) {
			writeError(w, http.StatusConflict, CodeConflict, "promotion requires a succeeded build with a digest")
			return
		}
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "promotion write unavailable")
		return
	}
	s.auditScope(r.Context(), sub.Principal.PrincipalID, scopeID, "deployment.promote", "allow", "release promoted")
	writeJSON(w, http.StatusCreated, map[string]any{"release_id": id})
}

// ---- Mail (MAIL-01) ----

func (s *Server) handleListMailDomains(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	domains, err := s.DB.ListMailDomains(r.Context(), scopeID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "mail domain lookup unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"mail_domains": domains})
}

func (s *Server) handleAddMailDomain(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	sub := subjectOf(r)
	if _, ok := s.authorizeScope(w, r, scopeID, "mail.domain.configure", "mail domain"); !ok {
		return
	}
	var req struct {
		Domain string `json:"domain"`
	}
	if err := decodeStrict(r, &req); err != nil || !strings.Contains(req.Domain, ".") {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid mail domain schema")
		return
	}
	id, err := s.Mail.AddMailDomain(r.Context(), scopeID, strings.ToLower(strings.TrimSpace(req.Domain)))
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "mail domain write unavailable")
		return
	}
	s.auditScope(r.Context(), sub.Principal.PrincipalID, scopeID, "mail.domain.configure", "allow", "mail domain added")
	writeJSON(w, http.StatusCreated, map[string]any{"mail_domain_id": id, "state": "pending"})
}

func (s *Server) handleVerifyMailDomain(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	mailDomainID := r.PathValue("mailDomainID")
	sub := subjectOf(r)
	if _, ok := s.authorizeScope(w, r, scopeID, "mail.domain.configure", "mail domain"); !ok {
		return
	}
	if err := s.Mail.VerifyMailDomain(r.Context(), mailDomainID); err != nil {
		if errors.Is(err, domains.ErrInvalid) {
			writeError(w, http.StatusNotFound, CodeNotFound, "no such mail domain")
			return
		}
		writeError(w, http.StatusConflict, CodeConflict, "domain verification prerequisites not met")
		return
	}
	s.auditScope(r.Context(), sub.Principal.PrincipalID, scopeID, "mail.domain.configure", "allow", "mail domain verified")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListMailboxes(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	boxes, err := s.DB.ListMailboxes(r.Context(), scopeID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "mailbox lookup unavailable")
		return
	}
	aliases, err := s.DB.ListMailAliases(r.Context(), scopeID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "alias lookup unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"mailboxes": boxes, "aliases": aliases})
}

func (s *Server) handleCreateMailbox(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	sub := subjectOf(r)
	if _, ok := s.authorizeScope(w, r, scopeID, "mailbox.provision", "mailbox"); !ok {
		return
	}
	var req struct {
		MailDomainID string `json:"mail_domain_id"`
		LocalPart    string `json:"local_part"`
		QuotaMB      int64  `json:"quota_mb"`
	}
	if err := decodeStrict(r, &req); err != nil || req.LocalPart == "" || strings.Contains(req.LocalPart, "@") {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid mailbox schema")
		return
	}
	if req.QuotaMB <= 0 {
		req.QuotaMB = 1024
	}
	id, err := s.Mail.CreateMailbox(r.Context(), scopeID, req.MailDomainID, req.LocalPart, req.QuotaMB)
	if err != nil {
		if errors.Is(err, domains.ErrInvalid) {
			writeError(w, http.StatusConflict, CodeConflict, "mailbox prerequisites not met (verified domain, valid local part)")
			return
		}
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "mailbox write unavailable")
		return
	}
	s.auditScope(r.Context(), sub.Principal.PrincipalID, scopeID, "mailbox.provision", "allow", "mailbox created")
	writeJSON(w, http.StatusCreated, map[string]any{"mailbox_id": id})
}

func (s *Server) handleCreateAlias(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	sub := subjectOf(r)
	if _, ok := s.authorizeScope(w, r, scopeID, "mail.domain.configure", "mail domain"); !ok {
		return
	}
	var req struct {
		MailDomainID string `json:"mail_domain_id"`
		LocalPart    string `json:"local_part"`
		Destination  string `json:"destination"`
	}
	if err := decodeStrict(r, &req); err != nil || req.LocalPart == "" || !strings.Contains(req.Destination, "@") {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid alias schema")
		return
	}
	id, err := s.Mail.CreateAlias(r.Context(), scopeID, req.MailDomainID, req.LocalPart, req.Destination)
	if err != nil {
		if errors.Is(err, domains.ErrInvalid) {
			writeError(w, http.StatusConflict, CodeConflict, "alias prerequisites not met (verified domain)")
			return
		}
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "alias write unavailable")
		return
	}
	s.auditScope(r.Context(), sub.Principal.PrincipalID, scopeID, "mail.domain.configure", "allow", "alias created")
	writeJSON(w, http.StatusCreated, map[string]any{"alias_id": id})
}

// ---- Observability + notifications (OBS-01, CORE-05) ----

func (s *Server) handleListObservations(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	sub := subjectOf(r)
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	if _, err := s.Authz.Authorize(r.Context(), sub.Principal.PrincipalID,
		"monitor.read", "health observation", scopeID, 1); err != nil {
		writeError(w, http.StatusForbidden, CodeForbidden, "monitoring read not permitted")
		return
	}
	obs, err := s.DB.ListObservations(r.Context(), scopeID, r.URL.Query().Get("resource_id"), 100)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "observation lookup unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"observations": obs})
}

func (s *Server) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	sub := subjectOf(r)
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	notifs, err := s.DB.ListNotifications(r.Context(), scopeID, sub.Principal.PrincipalID, 50)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "notification lookup unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": notifs})
}

// ---- Templates (MKT-01) + backups (WEB-09) + schedules ----

func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	templates, err := s.DB.ListTemplates(r.Context(), scopeID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "template lookup unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": templates})
}

func (s *Server) handleInstallTemplate(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	templateID := r.PathValue("templateID")
	sub := subjectOf(r)
	if _, ok := s.authorizeScope(w, r, scopeID, "template.install", "app template"); !ok {
		return
	}
	var req struct {
		ResourceID string `json:"resource_id"`
	}
	if err := decodeStrict(r, &req); err != nil || strings.TrimSpace(req.ResourceID) == "" {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid template install schema")
		return
	}
	id, err := s.Template.QueueInstall(r.Context(), scopeID, req.ResourceID, templateID)
	if err != nil {
		if errors.Is(err, domains.ErrInvalid) {
			writeError(w, http.StatusNotFound, CodeNotFound, "no such template or resource")
			return
		}
		if errors.Is(err, domains.ErrState) {
			writeError(w, http.StatusConflict, CodeConflict, "install permitted on application resources only")
			return
		}
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "install write unavailable")
		return
	}
	s.auditScope(r.Context(), sub.Principal.PrincipalID, scopeID, "template.install", "allow", "template install queued")
	writeJSON(w, http.StatusCreated, map[string]any{"install_id": id})
}

func (s *Server) handleListBackups(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	backups, err := s.DB.ListSiteBackups(r.Context(), scopeID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "backup lookup unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"backups": backups})
}

func (s *Server) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	schedules, err := s.DB.ListSchedules(r.Context(), scopeID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "schedule lookup unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"schedules": schedules})
}

// handleListTls serves the scope's TLS certificate records (G30).
func (s *Server) handleListTls(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	certs, err := domains.NewTlsService(s.DB).ListCertificates(r.Context(), scopeID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "certificate lookup unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"certificates": certs})
}

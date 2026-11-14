package api

import (
	"net/http"
	"time"

	"drevanta/internal/audit"
	"drevanta/internal/authn"
	"drevanta/internal/authz"
	"drevanta/internal/domains"
	"drevanta/internal/install"
	"drevanta/internal/operations"
	"drevanta/internal/store"
)

// Server aggregates the control-plane dependencies.
type Server struct {
	DB      *store.DB
	Authn   *authn.Service
	Authz   *authz.Checker
	Ops     *operations.Engine
	Audit   *audit.Writer
	Install *install.Installer
	Mailer  Mailer           // recovery delivery seam (tests may capture)
	Clock   func() time.Time // injectable for tests
	MaxBody int64

	// Domain capability services (G40 full-persona wave).
	Support    *domains.Support
	Commercial *domains.Commercial
	Pipeline   *domains.Pipeline
	Mail       *domains.MailService
	Observe    *domains.ObserveService
	Template   *domains.TemplateService
	Backup     *domains.BackupService
}

// NewServer wires a Server with defaults.
func NewServer(db *store.DB, authnSvc *authn.Service, checker *authz.Checker, eng *operations.Engine, aw *audit.Writer) *Server {
	return &Server{DB: db, Authn: authnSvc, Authz: checker, Ops: eng, Audit: aw,
		Install: install.NewInstaller(db, authnSvc),
		Mailer:  LogMailer{}, Clock: time.Now, MaxBody: 1 << 20,
		Support:    domains.NewSupport(db),
		Commercial: domains.NewCommercial(db, domains.ManualGateway{}),
		Pipeline:   domains.NewPipeline(db),
		Mail:       domains.NewMailService(db),
		Observe:    domains.NewObserveService(db),
		Template:   domains.NewTemplateService(db),
		Backup:     domains.NewBackupService(db),
	}
}

// Handler builds the full route tree.
func (s *Server) Handler() http.Handler {
	protected := http.NewServeMux()
	protected.HandleFunc("GET /api/v1/me", s.handleMe)
	protected.HandleFunc("GET /api/v1/workspaces", s.handleWorkspaces)
	protected.HandleFunc("GET /api/v1/resources", s.handleResources)
	protected.HandleFunc("POST /api/v1/resources/{resourceID}/intents", s.handleIntents)
	protected.HandleFunc("GET /api/v1/operations/{operationID}", s.handleOperation)
	protected.HandleFunc("GET /api/v1/scopes/{scopeID}/audit", s.handleScopeAudit)
	// Domain capabilities (G40 full-persona wave)
	protected.HandleFunc("GET /api/v1/scopes/{scopeID}/tickets", s.handleListTickets)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/tickets", s.handleCreateTicket)
	protected.HandleFunc("GET /api/v1/scopes/{scopeID}/tickets/{ticketID}", s.handleTicketDetail)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/tickets/{ticketID}/messages", s.handleTicketMessage)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/tickets/{ticketID}/resolve", s.handleTicketResolve)
	protected.HandleFunc("GET /api/v1/scopes/{scopeID}/tls", s.handleListTls)
	protected.HandleFunc("GET /api/v1/scopes/{scopeID}/kb", s.handleListKB)
	protected.HandleFunc("GET /api/v1/scopes/{scopeID}/plans", s.handleListPlans)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/plans", s.handleCreatePlan)
	protected.HandleFunc("GET /api/v1/scopes/{scopeID}/customers", s.handleListCustomers)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/customers", s.handleCreateCustomer)
	protected.HandleFunc("GET /api/v1/scopes/{scopeID}/accounts", s.handleListAccounts)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/accounts", s.handleProvisionAccount)
	protected.HandleFunc("GET /api/v1/scopes/{scopeID}/invoices", s.handleListInvoices)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/invoices", s.handleCreateInvoice)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/invoices/{invoiceID}/payments", s.handleRecordPayment)
	protected.HandleFunc("GET /api/v1/scopes/{scopeID}/source-connections", s.handleListSourceConnections)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/source-connections", s.handleConnectSource)
	protected.HandleFunc("GET /api/v1/scopes/{scopeID}/builds", s.handleListBuilds)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/builds", s.handleQueueBuild)
	protected.HandleFunc("GET /api/v1/scopes/{scopeID}/environments", s.handleListEnvironments)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/environments", s.handleCreateEnvironment)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/environments/{environmentID}/promote", s.handlePromoteRelease)
	protected.HandleFunc("GET /api/v1/scopes/{scopeID}/mail-domains", s.handleListMailDomains)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/mail-domains", s.handleAddMailDomain)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/mail-domains/{mailDomainID}/verify", s.handleVerifyMailDomain)
	protected.HandleFunc("GET /api/v1/scopes/{scopeID}/mailboxes", s.handleListMailboxes)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/mailboxes", s.handleCreateMailbox)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/mail-aliases", s.handleCreateAlias)
	protected.HandleFunc("GET /api/v1/scopes/{scopeID}/observations", s.handleListObservations)
	protected.HandleFunc("GET /api/v1/scopes/{scopeID}/notifications", s.handleListNotifications)
	protected.HandleFunc("GET /api/v1/scopes/{scopeID}/templates", s.handleListTemplates)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/templates/{templateID}/install", s.handleInstallTemplate)
	protected.HandleFunc("GET /api/v1/scopes/{scopeID}/backups", s.handleListBackups)
	protected.HandleFunc("GET /api/v1/scopes/{scopeID}/schedules", s.handleListSchedules)

	protected.HandleFunc("POST /api/v1/identity/logout", s.handleLogout)
	protected.HandleFunc("POST /api/v1/identity/recovery-codes", s.handleRecoveryCodes)
	protected.HandleFunc("GET /api/v1/identity/mfa/status", s.handleMFAStatus)
	protected.HandleFunc("POST /api/v1/identity/mfa/enroll", s.handleMFAEnroll)
	protected.HandleFunc("GET /api/v1/workspaces/{workspaceID}/memberships", s.handleListMemberships)
	protected.HandleFunc("POST /api/v1/workspaces/{workspaceID}/memberships", s.handleGrantMembership)
	protected.HandleFunc("DELETE /api/v1/workspaces/{workspaceID}/memberships/{principalID}/{roleID}", s.handleRevokeMembership)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/api-keys", s.handleIssueAPIKey)
	protected.HandleFunc("GET /api/v1/scopes/{scopeID}/api-keys", s.handleListAPIKeys)
	protected.HandleFunc("DELETE /api/v1/scopes/{scopeID}/api-keys/{keyID}", s.handleRevokeAPIKey)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/schedules", s.handleArmSchedule)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/resources", s.handleCreateResource)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/schedules/{scheduleID}/pause", s.handlePauseSchedule)
	protected.HandleFunc("POST /api/v1/install/address", s.handleInstallAddress)

	public := http.NewServeMux()
	public.HandleFunc("POST /api/v1/identity/login", s.handleLogin)
	public.HandleFunc("POST /api/v1/identity/mfa", s.handleMFAVerify)
	public.HandleFunc("POST /api/v1/identity/recover-request", s.handleRecoverRequest)
	public.HandleFunc("POST /api/v1/identity/recover-confirm", s.handleRecoverConfirm)
	public.HandleFunc("GET /api/v1/install/status", s.handleInstallStatus)
	public.HandleFunc("POST /api/v1/install/claim", s.handleInstallClaim)

	// Unknown /api routes fail closed with the taxonomy.
	catchAll := http.NewServeMux()
	catchAll.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such route")
	})

	root := http.NewServeMux()
	// Exact identity paths that require authentication win over the public
	// identity prefix (Go mux precedence: most specific pattern first).
	root.Handle("/api/v1/identity/logout", s.bodyLimit(s.authenticate(protected)))
	root.Handle("/api/v1/identity/recovery-codes", s.bodyLimit(s.authenticate(protected)))
	root.Handle("/api/v1/identity/mfa/status", s.bodyLimit(s.authenticate(protected)))
	root.Handle("/api/v1/identity/mfa/enroll", s.bodyLimit(s.authenticate(protected)))
	root.Handle("/api/v1/identity/", s.bodyLimit(public))
	// Installer routes are public while pending (claim token guards effect).
	root.Handle("/api/v1/install/status", s.bodyLimit(public))
	root.Handle("/api/v1/install/claim", s.bodyLimit(public))
	root.Handle("/api/v1/", s.bodyLimit(s.authenticate(protected)))
	root.Handle("/api/", catchAll)
	root.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Static UI is served by the panel binary at / (wired in a later wave;
		// the API contract forbids UI payloads leaking into /api error paths).
		writeError(w, http.StatusNotFound, CodeNotFound, "no such route")
	})
	return root
}

// bodyLimit caps request body size (DoS bound; profile: bounded messages).
func (s *Server) bodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, s.MaxBody)
		}
		next.ServeHTTP(w, r)
	})
}

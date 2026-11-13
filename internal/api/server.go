package api

import (
	"net/http"
	"time"

	"drevanta/internal/audit"
	"drevanta/internal/authn"
	"drevanta/internal/authz"
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
	Mailer  Mailer           // recovery delivery seam (tests may capture)
	Clock   func() time.Time // injectable for tests
	MaxBody int64
}

// NewServer wires a Server with defaults.
func NewServer(db *store.DB, authnSvc *authn.Service, checker *authz.Checker, eng *operations.Engine, aw *audit.Writer) *Server {
	return &Server{DB: db, Authn: authnSvc, Authz: checker, Ops: eng, Audit: aw,
		Mailer: LogMailer{}, Clock: time.Now, MaxBody: 1 << 20}
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
	protected.HandleFunc("POST /api/v1/identity/logout", s.handleLogout)
	protected.HandleFunc("POST /api/v1/identity/recovery-codes", s.handleRecoveryCodes)
	protected.HandleFunc("GET /api/v1/workspaces/{workspaceID}/memberships", s.handleListMemberships)
	protected.HandleFunc("POST /api/v1/workspaces/{workspaceID}/memberships", s.handleGrantMembership)
	protected.HandleFunc("DELETE /api/v1/workspaces/{workspaceID}/memberships/{principalID}/{roleID}", s.handleRevokeMembership)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/api-keys", s.handleIssueAPIKey)
	protected.HandleFunc("GET /api/v1/scopes/{scopeID}/api-keys", s.handleListAPIKeys)
	protected.HandleFunc("DELETE /api/v1/scopes/{scopeID}/api-keys/{keyID}", s.handleRevokeAPIKey)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/schedules", s.handleArmSchedule)
	protected.HandleFunc("POST /api/v1/scopes/{scopeID}/schedules/{scheduleID}/pause", s.handlePauseSchedule)

	public := http.NewServeMux()
	public.HandleFunc("POST /api/v1/identity/login", s.handleLogin)
	public.HandleFunc("POST /api/v1/identity/mfa", s.handleMFAVerify)
	public.HandleFunc("POST /api/v1/identity/recover-request", s.handleRecoverRequest)
	public.HandleFunc("POST /api/v1/identity/recover-confirm", s.handleRecoverConfirm)

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
	root.Handle("/api/v1/identity/", s.bodyLimit(public))
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

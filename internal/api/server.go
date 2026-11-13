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
	Clock   func() time.Time // injectable for tests
	MaxBody int64
}

// NewServer wires a Server with defaults.
func NewServer(db *store.DB, authnSvc *authn.Service, checker *authz.Checker, eng *operations.Engine, aw *audit.Writer) *Server {
	return &Server{DB: db, Authn: authnSvc, Authz: checker, Ops: eng, Audit: aw, Clock: time.Now, MaxBody: 1 << 20}
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

	public := http.NewServeMux()
	public.HandleFunc("POST /api/v1/identity/login", s.handleLogin)
	public.HandleFunc("POST /api/v1/identity/mfa", s.handleMFAVerify)

	// Unknown /api routes fail closed with the taxonomy.
	catchAll := http.NewServeMux()
	catchAll.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such route")
	})

	root := http.NewServeMux()
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

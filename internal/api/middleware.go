package api

import (
	"context"
	"net/http"
	"strings"

	"drevanta/internal/authn"
	"drevanta/internal/jobs"
	"drevanta/internal/store"
)

// ctxKey is the request-context key type.
type ctxKey int

const ctxSubject ctxKey = 1

// Subject carries the authenticated principal for handlers.
type Subject struct {
	Principal *store.Principal
	Session   *authn.SessionInfo
	// APIKeyScope is set when authenticated via API key: requests are
	// restricted to exactly this scope (phase 23 INT-01).
	APIKeyScope string
}

// authenticate enforces 401 for absent/invalid credentials. Bearer sessions
// and API keys (X-API-Key) are both accepted; API keys carry a scope bound.
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if key := r.Header.Get("X-API-Key"); key != "" {
			principalID, scopeID, err := jobs.ResolveAPIKey(r.Context(), s.DB, key)
			if err != nil {
				writeError(w, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
				return
			}
			p, err := s.DB.PrincipalByID(r.Context(), principalID)
			if err != nil || p.Status != "active" {
				writeError(w, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxSubject,
				&Subject{Principal: p, APIKeyScope: scopeID})))
			return
		}
		bearer := bearerToken(r)
		if bearer == "" {
			writeError(w, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
			return
		}
		p, si, err := s.Authn.Authenticate(r.Context(), bearer)
		if err != nil {
			writeError(w, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxSubject, &Subject{Principal: p, Session: si})))
	})
}

// bearerToken extracts the Authorization: Bearer value.
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// subjectOf retrieves the authenticated subject from the request context.
func subjectOf(r *http.Request) *Subject {
	if v, ok := r.Context().Value(ctxSubject).(*Subject); ok {
		return v
	}
	return nil
}

// requireAuthz performs the authorization predicate for a target resource and
// action, mapping denials:
//   - concealed/foreign or nonexistent target → 404 (no identifiers)
//   - visible (any active relationship with owning scope) but denied → 403
//
// It returns the carrying role on allow.
func (s *Server) requireAuthz(w http.ResponseWriter, r *http.Request, res *store.Resource, action string) (string, bool) {
	sub := subjectOf(r)
	if sub == nil {
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
		return "", false
	}
	if !s.visibility(r, res.ScopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound,
			"no such resource") // safe: carries no resource identifier
		return "", false
	}
	role, err := s.Authz.Authorize(r.Context(), sub.Principal.PrincipalID, action, res.Type, res.ScopeID, res.OwnerRevision)
	if err != nil {
		writeError(w, http.StatusForbidden, CodeForbidden,
			"action not permitted on this resource")
		return "", false
	}
	return role, true
}

// visibility reports whether the request's subject has any relationship with
// a scope: active membership, any support lease, or (API-key subjects) the
// key's bound scope. Foreign scopes conceal.
func (s *Server) visibility(r *http.Request, scopeID string) bool {
	sub := subjectOf(r)
	if sub == nil {
		return false
	}
	// API-key subjects are pinned to exactly their key's scope (phase 23).
	if sub.APIKeyScope != "" {
		return sub.APIKeyScope == scopeID
	}
	if ms, err := s.DB.ActiveMemberships(r.Context(), sub.Principal.PrincipalID); err == nil {
		for _, m := range ms {
			if m.WorkspaceID == scopeID {
				return true
			}
		}
	}
	if ok, err := s.Authz.LeaseVisible(r.Context(), sub.Principal.PrincipalID, scopeID); err == nil && ok {
		return true
	}
	return false
}

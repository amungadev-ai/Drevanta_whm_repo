package api

import (
	"context"
	"net/http"
	"strings"

	"drevanta/internal/authn"
	"drevanta/internal/store"
)

// ctxKey is the request-context key type.
type ctxKey int

const ctxSubject ctxKey = 1

// Subject carries the authenticated principal for handlers.
type Subject struct {
	Principal *store.Principal
	Session   *authn.SessionInfo
}

// authenticate enforces 401 for absent/invalid credentials.
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	if !s.visibilityFor(sub.Principal.PrincipalID, res.ScopeID) {
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

// visibilityFor reports whether the principal has any active relationship
// (membership or support lease) with a scope. Foreign scopes conceal.
func (s *Server) visibilityFor(principalID, scopeID string) bool {
	if ms, err := s.DB.ActiveMemberships(context.Background(), principalID); err == nil {
		for _, m := range ms {
			if m.WorkspaceID == scopeID {
				return true
			}
		}
	}
	if ok, err := s.Authz.LeaseVisible(context.Background(), principalID, scopeID); err == nil && ok {
		return true
	}
	return false
}

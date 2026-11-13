// Package api implements the versioned JSON API at /api/v1.
//
// Contract basis: profile.md §API envelope (DEC-004/009/012). Status taxonomy:
// 401 absent/invalid auth, 403 denied action on a visible resource, 404
// concealed/nonexistent (no resource identifiers), 409 conflicting intent/state,
// 412 stale precondition, 422 invalid payload (unknown fields rejected),
// 503 dependency unavailable before acceptance, 202 async acceptance.
// No credentials or protected identifiers in URLs, errors, logs or evidence.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"drevanta/internal/operations"
	"drevanta/internal/store"
)

// Error codes (stable, machine-readable; never embed credentials or IDs of
// concealed objects).
const (
	CodeUnauthorized          = "unauthorized"
	CodeForbidden             = "forbidden"
	CodeNotFound              = "not_found"
	CodeConflict              = "conflict"
	CodeStalePrecondition     = "stale_precondition"
	CodeInvalidInput          = "invalid_input"
	CodeDependencyFailure     = "dependency_unavailable"
	CodeCapabilityUnavailable = "capability_unavailable"
)

// errorBody is the uniform error envelope.
type errorBody struct {
	Error         string `json:"error"`
	Code          string `json:"code"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

// writeError renders the taxonomy. safeDetail must be a caller-approved safe
// string (profile: "body carries correlation only and approved safe detail").
func writeError(w http.ResponseWriter, status int, code, safeDetail string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	b := errorBody{Error: safeDetail, Code: code}
	_ = json.NewEncoder(w).Encode(b)
}

// writeJSON writes a 2xx JSON body.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// decodeStrict decodes a JSON body rejecting unknown fields and enforcing
// exact schema (profile: "Reject unknown request properties").
func decodeStrict(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	// Reject trailing content.
	if dec.More() {
		return errors.New("unexpected trailing JSON content")
	}
	return nil
}

// mapOperationsError converts engine errors into taxonomy responses and
// reports whether an audit "deny/error" event should be recorded.
func mapOperationsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, operations.ErrStale):
		writeError(w, http.StatusPreconditionFailed, CodeStalePrecondition,
			"precondition failed: current generation or owner revision does not match intent")
	case errors.Is(err, operations.ErrConflict):
		writeError(w, http.StatusConflict, CodeConflict,
			"conflicting intent: idempotency key already used with different canonical intent")
	case errors.Is(err, operations.ErrInvalidIntent):
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput,
			"invalid intent: action or resource type mismatch")
	case errors.Is(err, operations.ErrNotFound):
		writeError(w, http.StatusNotFound, CodeNotFound, "operation not found")
	default:
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure,
			"dependency unavailable; nothing accepted")
	}
}

// errStoreNotFound is a convenience alias.
func errStoreNotFound(err error) bool { return errors.Is(err, store.ErrNotFound) }

// methodNotAllowed writes 405 with Allow header.
func methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	w.Header().Set("Allow", fmt.Sprintf("%s", joinComma(allowed)))
	writeError(w, http.StatusMethodNotAllowed, CodeInvalidInput, "method not allowed")
}

func joinComma(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}

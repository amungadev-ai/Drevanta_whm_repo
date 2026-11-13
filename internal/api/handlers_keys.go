package api

// API key and schedule management handlers (phase 23 INT-01, WEB-08 core).
// API keys are scope-bound (requests restrict to exactly the key's scope);
// raw keys are returned once; revocation denies at next use.

import (
	"net/http"
	"time"

	"drevanta/internal/audit"
	"drevanta/internal/jobs"
)

// ---- POST /api/v1/scopes/{scopeID}/api-keys ----

type apiKeyRequest struct {
	Name      string  `json:"name"`
	ExpiresAt *string `json:"expires_at"` // optional RFC3339
}

func (s *Server) handleIssueAPIKey(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	sub := subjectOf(r)
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	if _, err := s.Authz.Authorize(r.Context(), sub.Principal.PrincipalID,
		"integration.configure", "integration grant", scopeID, 1); err != nil {
		writeError(w, http.StatusForbidden, CodeForbidden, "integration administration not permitted")
		return
	}
	var req apiKeyRequest
	if err := decodeStrict(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid api key schema")
		return
	}
	var exp *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339Nano, *req.ExpiresAt)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid expiry timestamp")
			return
		}
		exp = &t
	}
	raw, err := jobs.IssueAPIKey(r.Context(), s.DB, sub.Principal.PrincipalID, scopeID, req.Name, exp)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "api key issuance unavailable")
		return
	}
	_ = s.Audit.Append(r.Context(), audit.Event{
		ActorID: sub.Principal.PrincipalID, ScopeID: scopeID,
		ActionID: "integration.configure", Outcome: "allow",
		SafeReason: "api key issued",
	})
	// Raw key returned exactly once; never logged or audited.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, map[string]string{"api_key": raw})
}

// ---- GET /api/v1/scopes/{scopeID}/api-keys ----

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	sub := subjectOf(r)
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	if _, err := s.Authz.Authorize(r.Context(), sub.Principal.PrincipalID,
		"integration.configure", "integration grant", scopeID, 1); err != nil {
		writeError(w, http.StatusForbidden, CodeForbidden, "integration administration not permitted")
		return
	}
	rows, err := s.DB.QueryContext(r.Context(),
		`SELECT api_key_id, name, created_at, expires_at, revoked_at, last_used_at FROM api_key
		 WHERE principal_id=? AND scope_id=? ORDER BY created_at DESC`,
		sub.Principal.PrincipalID, scopeID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "api key lookup unavailable")
		return
	}
	defer rows.Close()
	type keyItem struct {
		KeyID      string  `json:"key_id"`
		Name       string  `json:"name"`
		CreatedAt  string  `json:"created_at"`
		ExpiresAt  *string `json:"expires_at"`
		RevokedAt  *string `json:"revoked_at"`
		LastUsedAt *string `json:"last_used_at"`
	}
	items := []keyItem{}
	for rows.Next() {
		var it keyItem
		if err := rows.Scan(&it.KeyID, &it.Name, &it.CreatedAt, &it.ExpiresAt, &it.RevokedAt, &it.LastUsedAt); err != nil {
			writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "api key lookup unavailable")
			return
		}
		items = append(items, it)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ---- DELETE /api/v1/scopes/{scopeID}/api-keys/{keyID} ----

func (s *Server) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	keyID := r.PathValue("keyID")
	sub := subjectOf(r)
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	if _, err := s.Authz.Authorize(r.Context(), sub.Principal.PrincipalID,
		"integration.configure", "integration grant", scopeID, 1); err != nil {
		writeError(w, http.StatusForbidden, CodeForbidden, "integration administration not permitted")
		return
	}
	if err := jobs.RevokeAPIKey(r.Context(), s.DB, sub.Principal.PrincipalID, keyID); err != nil {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such key")
		return
	}
	_ = s.Audit.Append(r.Context(), audit.Event{
		ActorID: sub.Principal.PrincipalID, ScopeID: scopeID,
		ActionID: "integration.configure", Outcome: "allow",
		SafeReason: "api key revoked",
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// ---- POST /api/v1/scopes/{scopeID}/schedules ----

type scheduleRequest struct {
	ResourceID   string         `json:"resource_id"`
	ActionID     string         `json:"action_id"`
	EverySeconds int            `json:"every_seconds"`
	Arguments    map[string]any `json:"arguments,omitempty"`
}

func (s *Server) handleArmSchedule(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	sub := subjectOf(r)
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	var req scheduleRequest
	if err := decodeStrict(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid schedule schema")
		return
	}
	res, err := s.DB.ResourceByID(r.Context(), req.ResourceID)
	if err != nil {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such resource")
		return
	}
	if res.ScopeID != scopeID {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such resource")
		return
	}
	if _, err := s.Authz.Authorize(r.Context(), sub.Principal.PrincipalID,
		"schedule.configure", "schedule", scopeID, res.OwnerRevision); err != nil {
		writeError(w, http.StatusForbidden, CodeForbidden, "schedule administration not permitted")
		return
	}
	// Action must be granted to the scheduler principal on this resource type.
	if _, err := s.Authz.Authorize(r.Context(), sub.Principal.PrincipalID,
		req.ActionID, res.Type, scopeID, res.OwnerRevision); err != nil {
		writeError(w, http.StatusForbidden, CodeForbidden, "scheduled action not permitted")
		return
	}
	id, err := jobs.ArmSchedule(r.Context(), s.DB, scopeID, sub.Principal.PrincipalID,
		req.ResourceID, req.ActionID, req.EverySeconds, req.Arguments)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid schedule parameters")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"schedule_id": id, "state": "armed"})
}

// ---- POST /api/v1/scopes/{scopeID}/schedules/{scheduleID}/pause ----

func (s *Server) handlePauseSchedule(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	scheduleID := r.PathValue("scheduleID")
	sub := subjectOf(r)
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	if _, err := s.Authz.Authorize(r.Context(), sub.Principal.PrincipalID,
		"schedule.configure", "schedule", scopeID, 1); err != nil {
		writeError(w, http.StatusForbidden, CodeForbidden, "schedule administration not permitted")
		return
	}
	if err := jobs.PauseSchedule(r.Context(), s.DB, scheduleID); err != nil {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such schedule")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "paused"})
}

// scheduleScopeGuard narrows API-key subjects to their key scope: a key
// subject may only act inside APIKeyScope.
func (s *Server) scheduleScopeGuard(r *http.Request, scopeID string) bool {
	sub := subjectOf(r)
	if sub == nil {
		return false
	}
	if sub.APIKeyScope != "" && sub.APIKeyScope != scopeID {
		return false
	}
	return true
}

// ---- POST /api/v1/scopes/{scopeID}/resources (CORE-03 registration) ----

type createResourceRequest struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	HostID string `json:"host_id,omitempty"`
}

// knownResourceTypes is the governed vocabulary subset enabled so far
// (extended per domain wave).
var knownResourceTypes = map[string]bool{
	"host snapshot": true, "filesystem entry": true, "SQL database": true,
	"backup artifact": true, "hostname binding": true, "vhost": true,
	"application": true, "release artifact": true, "source connection": true,
	"environment": true, "hosting account": true, "support ticket": true,
	"schedule": true, "health observation": true, "volume": true,
}

func (s *Server) handleCreateResource(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	sub := subjectOf(r)
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	var req createResourceRequest
	if err := decodeStrict(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid resource schema")
		return
	}
	if !knownResourceTypes[req.Type] {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "unknown resource type")
		return
	}
	// Explicit action grant for linking resources of this type.
	if _, err := s.Authz.Authorize(r.Context(), sub.Principal.PrincipalID,
		"resource.link", req.Type, scopeID, 1); err != nil {
		writeError(w, http.StatusForbidden, CodeForbidden, "resource registration not permitted")
		return
	}
	res, err := s.DB.CreateResource(r.Context(), scopeID, req.HostID, req.Type, req.Name)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "resource write unavailable")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"resource_id":         res.ResourceID,
		"type":                res.Type,
		"name":                res.Name,
		"desired_generation":  res.DesiredGeneration,
		"observed_generation": res.ObservedGeneration,
		"owner_revision":      res.OwnerRevision,
	})
}

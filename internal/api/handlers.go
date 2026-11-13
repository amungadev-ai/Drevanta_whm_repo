package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"drevanta/internal/audit"
	"drevanta/internal/authn"
	"drevanta/internal/operations"
	"drevanta/internal/store"
)

// ---- GET /api/v1/me ----

type meResponse struct {
	PrincipalID string   `json:"principal_id"`
	Email       string   `json:"email"`
	DisplayName string   `json:"display_name"`
	Memberships []string `json:"memberships"` // workspace IDs the subject can read (self)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	sub := subjectOf(r)
	ms, err := s.DB.ActiveMemberships(r.Context(), sub.Principal.PrincipalID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "membership lookup unavailable")
		return
	}
	resp := meResponse{
		PrincipalID: sub.Principal.PrincipalID,
		Email:       sub.Principal.Email,
		DisplayName: sub.Principal.DisplayName,
		Memberships: []string{},
	}
	for _, m := range ms {
		resp.Memberships = append(resp.Memberships, m.WorkspaceID)
	}
	writeJSON(w, http.StatusOK, resp)
}

// ---- GET /api/v1/workspaces ----

type workspaceItem struct {
	WorkspaceID string `json:"workspace_id"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Role        string `json:"role"` // subject's active role in this workspace
}

type workspaceList struct {
	Items      []workspaceItem `json:"items"`
	NextCursor *string         `json:"next_cursor"`
}

func (s *Server) handleWorkspaces(w http.ResponseWriter, r *http.Request) {
	sub := subjectOf(r)
	ms, err := s.DB.ActiveMemberships(r.Context(), sub.Principal.PrincipalID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "membership lookup unavailable")
		return
	}
	// Active scoped membership required for each item (profile route table).
	seen := map[string]bool{}
	out := workspaceList{Items: []workspaceItem{}}
	for _, m := range ms {
		if seen[m.WorkspaceID] {
			continue
		}
		seen[m.WorkspaceID] = true
		ws, err := s.DB.WorkspaceByID(r.Context(), m.WorkspaceID)
		if err != nil {
			continue
		}
		out.Items = append(out.Items, workspaceItem{
			WorkspaceID: ws.WorkspaceID, Kind: ws.Kind, Name: ws.Name, Role: m.RoleID,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// ---- GET /api/v1/resources ----

func (s *Server) handleResources(w http.ResponseWriter, r *http.Request) {
	scopeID := r.URL.Query().Get("scope_id")
	if scopeID == "" {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "scope_id query parameter required")
		return
	}
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	typ := r.URL.Query().Get("type")
	res, err := s.DB.ResourcesInScope(r.Context(), scopeID, typ)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "resource lookup unavailable")
		return
	}
	items := []map[string]any{}
	for _, rr := range res {
		items = append(items, map[string]any{
			"resource_id":         rr.ResourceID,
			"type":                rr.Type,
			"name":                rr.Name,
			"desired_generation":  rr.DesiredGeneration,
			"observed_generation": rr.ObservedGeneration,
			"owner_revision":      rr.OwnerRevision,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ---- POST /api/v1/resources/{resourceID}/intents ----

type operationAcceptedBody struct {
	OperationID        string `json:"operation_id"`
	CorrelationID      string `json:"correlation_id"`
	State              string `json:"state"`
	OperationStatusURL string `json:"operation_status_url"`
	Replayed           bool   `json:"replayed"`
}

func (s *Server) handleIntents(w http.ResponseWriter, r *http.Request) {
	resourceID := r.PathValue("resourceID")
	res, err := s.DB.ResourceByID(r.Context(), resourceID)
	if err != nil {
		// Nonexistent or foreign: uniform 404, no identifiers.
		writeError(w, http.StatusNotFound, CodeNotFound, "no such resource")
		return
	}
	sub := subjectOf(r)

	// The intent body is decoded only after the target loads; unknown fields
	// are rejected (422) before any state transition.
	var intent operations.Intent
	if err := decodeStrict(r, &intent); err != nil {
		s.Audit.Append(r.Context(), audit.Event{
			ActorID: sub.Principal.PrincipalID, ScopeID: res.ScopeID,
			ActionID: intent.ActionID, ResourceID: res.ResourceID,
			Outcome: "deny", SafeReason: "invalid intent schema",
		})
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput,
			"invalid intent: schema validation failed")
		return
	}

	role, allowed := s.requireAuthz(w, r, res, intent.ActionID)
	if !allowed {
		if role == "" {
			// Response already written (404 or 403). Record a deny event with
			// the action that was attempted; scope known for visible targets.
			_ = s.Audit.Append(r.Context(), audit.Event{
				ActorID: sub.Principal.PrincipalID, ScopeID: res.ScopeID,
				ActionID: intent.ActionID, ResourceID: res.ResourceID,
				Outcome: "deny", SafeReason: "authorization denied",
			})
		}
		return
	}

	key := r.Header.Get("Idempotency-Key")
	acc, err := s.Ops.Accept(r.Context(), sub.Principal.PrincipalID, res, intent, key)
	if err != nil {
		// Audit error events never carry protected detail.
		_ = s.Audit.Append(r.Context(), audit.Event{
			ActorID: sub.Principal.PrincipalID, ScopeID: res.ScopeID,
			ActionID: intent.ActionID, ResourceID: res.ResourceID,
			Outcome: "error", SafeReason: "intent acceptance failed",
		})
		mapOperationsError(w, err)
		return
	}
	outcome := "allow"
	if acc.Replayed {
		outcome = "replay"
	}
	_ = s.Audit.Append(r.Context(), audit.Event{
		ActorID: sub.Principal.PrincipalID, ScopeID: res.ScopeID,
		ActionID: intent.ActionID, ResourceID: res.ResourceID,
		CorrelationID: acc.CorrelationID,
		Outcome:       outcome,
		SafeReason:    "role=" + role,
	})
	writeJSON(w, http.StatusAccepted, operationAcceptedBody{
		OperationID:        acc.OperationID,
		CorrelationID:      acc.CorrelationID,
		State:              acc.State,
		OperationStatusURL: acc.OperationStatusURL,
		Replayed:           acc.Replayed,
	})
}

// ---- GET /api/v1/operations/{operationID} ----

type operationStatusBody struct {
	OperationID   string          `json:"operation_id"`
	State         string          `json:"state"`
	ActionID      string          `json:"action_id"`
	ScopeID       string          `json:"scope_id"`
	CorrelationID string          `json:"correlation_id"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
	Result        json.RawMessage `json:"result,omitempty"`
}

func (s *Server) handleOperation(w http.ResponseWriter, r *http.Request) {
	opID := r.PathValue("operationID")
	op, err := s.Ops.Get(r.Context(), opID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, CodeNotFound, "no such operation")
			return
		}
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "operation lookup unavailable")
		return
	}
	sub := subjectOf(r)
	// Owner scope plus operation.read (profile route table). Pending the
	// permission-vocabulary freeze, operation.read is enforced as: the actor
	// who created the operation, or any active member of its scope.
	isOwner := op.ActorID == sub.Principal.PrincipalID
	isMember := false
	if !isOwner {
		ms, err := s.DB.ActiveMemberships(r.Context(), sub.Principal.PrincipalID)
		if err == nil {
			for _, m := range ms {
				if m.WorkspaceID == op.ScopeID {
					isMember = true
					break
				}
			}
		}
	}
	if !isOwner && !isMember {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such operation")
		return
	}
	body := operationStatusBody{
		OperationID:   op.OperationID,
		State:         op.State,
		ActionID:      op.ActionID,
		ScopeID:       op.ScopeID,
		CorrelationID: op.CorrelationID,
		CreatedAt:     op.CreatedAt,
		UpdatedAt:     op.UpdatedAt,
	}
	if isOwner && op.Payload != "" {
		// Payload visible to owner only; members see status.
		body.Result = json.RawMessage(op.Payload)
	}
	writeJSON(w, http.StatusOK, body)
}

// ---- GET /api/v1/scopes/{scopeID}/audit ----

func (s *Server) handleScopeAudit(w http.ResponseWriter, r *http.Request) {
	scopeID := r.PathValue("scopeID")
	sub := subjectOf(r)
	if !s.visibility(r, scopeID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such scope")
		return
	}
	if _, err := s.Authz.Authorize(r.Context(), sub.Principal.PrincipalID, "audit.read", "audit event", scopeID, 1); err != nil {
		writeError(w, http.StatusForbidden, CodeForbidden, "audit read not permitted")
		return
	}
	events, err := s.Audit.Query(r.Context(), scopeID, 200)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "audit lookup unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

// ---- POST /api/v1/identity/login ----

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	PrincipalID string `json:"principal_id"`
	Token       string `json:"token,omitempty"`
	MFARequired bool   `json:"mfa_required"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeStrict(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid login schema")
		return
	}
	sess, err := s.Authn.Login(r.Context(), req.Email, req.Password)
	var mfa *authn.MFARequired
	switch {
	case errors.As(err, &mfa):
		writeJSON(w, http.StatusOK, loginResponse{PrincipalID: mfa.PrincipalID, MFARequired: true})
	case errors.Is(err, authn.ErrInvalidCredential), errors.Is(err, authn.ErrPrincipalDisabled):
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "invalid credentials")
	case err != nil:
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "authentication unavailable")
	default:
		writeJSON(w, http.StatusOK, loginResponse{PrincipalID: sess.PrincipalID, Token: sess.Token})
	}
}

// ---- POST /api/v1/identity/mfa ----

type mfaRequest struct {
	PrincipalID string `json:"principal_id"`
	Code        string `json:"code"`
}

type mfaResponse struct {
	Token       string `json:"token"`
	PrincipalID string `json:"principal_id"`
}

func (s *Server) handleMFAVerify(w http.ResponseWriter, r *http.Request) {
	var req mfaRequest
	if err := decodeStrict(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid mfa schema")
		return
	}
	sess, err := s.Authn.VerifyMFA(r.Context(), req.PrincipalID, req.Code)
	switch {
	case errors.Is(err, authn.ErrInvalidMFA), errors.Is(err, authn.ErrInvalidCredential):
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "invalid mfa code")
	case errors.Is(err, store.ErrNotFound), errors.Is(err, authn.ErrPrincipalDisabled):
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "invalid mfa code")
	case err != nil:
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "authentication unavailable")
	default:
		writeJSON(w, http.StatusOK, mfaResponse{Token: sess.Token, PrincipalID: sess.PrincipalID})
	}
}

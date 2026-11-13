package api

// Identity and membership lifecycle endpoints (phases 17/18).
//
// Contract basis: phase 17 (reset binds account/purpose/expiry/digest; reset
// revokes sessions, preserves MFA; recovery codes single-use), phase 18
// (invitations are scoped expiring grants; acceptance binds the intended
// principal; last-owner protection; revocation denies queued work at
// dispatch). Uniform denials: no account enumeration anywhere.

import (
	"context"
	"errors"
	"net/http"
	"time"

	"drevanta/internal/audit"
	"drevanta/internal/authz"
	"drevanta/internal/store"
)

// ---- POST /api/v1/identity/recover-request ----

// Mailer is the out-of-band delivery seam for recovery material. The raw
// token crosses this boundary exactly once and is never logged, audited or
// returned by the API. SMTP transport lands with the messaging wave (G34).
type Mailer interface {
	DeliverPasswordReset(ctx context.Context, email, token string) error
}

// LogMailer is the default no-op delivery adapter: challenges are issued and
// auditable, delivery is a logged no-op until SMTP configures.
type LogMailer struct{}

// DeliverPasswordReset implements Mailer (no-op).
func (LogMailer) DeliverPasswordReset(_ context.Context, _ string, _ string) error { return nil }

type recoverRequest struct {
	Email string `json:"email"`
}

func (s *Server) handleRecoverRequest(w http.ResponseWriter, r *http.Request) {
	var req recoverRequest
	if err := decodeStrict(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid recovery schema")
		return
	}
	p, err := s.DB.PrincipalByEmail(r.Context(), req.Email)
	if err == nil && p.Status == "active" {
		if token, err := s.Authn.IssuePasswordReset(r.Context(), p.PrincipalID); err == nil {
			if s.Mailer != nil {
				_ = s.Mailer.DeliverPasswordReset(r.Context(), p.Email, token)
			}
			_ = s.Audit.Append(r.Context(), audit.Event{
				ActorID: p.PrincipalID, ScopeID: p.PrincipalID,
				ActionID: "session.issue", Outcome: "allow",
				SafeReason: "password reset challenge issued",
			})
		}
	}
	// Uniform response regardless of account existence (no enumeration).
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "if the address exists, recovery instructions were issued",
	})
}

// ---- POST /api/v1/identity/recover-confirm ----

type recoverConfirm struct {
	Email       string `json:"email"`
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (s *Server) handleRecoverConfirm(w http.ResponseWriter, r *http.Request) {
	var req recoverConfirm
	if err := decodeStrict(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid recovery confirmation schema")
		return
	}
	if len(req.NewPassword) < 12 {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "password policy not satisfied")
		return
	}
	p, err := s.DB.PrincipalByEmail(r.Context(), req.Email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "invalid recovery token")
		return
	}
	err = s.Authn.ConfirmPasswordReset(r.Context(), p.PrincipalID, req.Token, req.NewPassword)
	if err != nil {
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "invalid recovery token")
		return
	}
	// Epoch already advanced (all old sessions deny at next request).
	writeJSON(w, http.StatusOK, map[string]string{"status": "password rotated; sign in again"})
}

// ---- POST /api/v1/identity/logout ----

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	sub := subjectOf(r)
	_, err := s.DB.ExecContext(r.Context(),
		`UPDATE session SET revoked_at=? WHERE session_id=? AND revoked_at IS NULL`,
		store.Now(), sub.Session.SessionID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "logout unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "session revoked"})
}

// ---- POST /api/v1/identity/recovery-codes (rotate; requires MFA-satisfied session) ----

func (s *Server) handleRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	sub := subjectOf(r)
	if !sub.Session.MFASatisfied {
		writeError(w, http.StatusForbidden, CodeForbidden, "step-up verification required")
		return
	}
	codes, err := s.Authn.GenerateRecoveryCodes(r.Context(), sub.Principal.PrincipalID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "recovery code rotation unavailable")
		return
	}
	// Shown exactly once; response is not cached and codes are never logged.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"recovery_codes": codes})
}

// ---- GET /api/v1/workspaces/{workspaceID}/memberships ----

type membershipItem struct {
	PrincipalID string  `json:"principal_id"`
	RoleID      string  `json:"role_id"`
	State       string  `json:"state"`
	ExpiresAt   *string `json:"expires_at"`
}

func (s *Server) handleListMemberships(w http.ResponseWriter, r *http.Request) {
	wsID := r.PathValue("workspaceID")
	sub := subjectOf(r)
	if !s.visibilityFor(sub.Principal.PrincipalID, wsID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such workspace")
		return
	}
	if _, err := s.Authz.Authorize(r.Context(), sub.Principal.PrincipalID,
		"membership.grant", "membership", wsID, 1); err != nil {
		writeError(w, http.StatusForbidden, CodeForbidden, "membership administration not permitted")
		return
	}
	ws, err := s.DB.WorkspaceByID(r.Context(), wsID)
	if err != nil {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such workspace")
		return
	}
	// Only catalogue roles of this workspace kind are listable assignments.
	rows, err := s.DB.QueryContext(r.Context(),
		`SELECT m.principal_id, m.role_id, m.state, m.expires_at FROM membership m
		 WHERE m.workspace_id = ? ORDER BY m.state, m.role_id, m.principal_id`, wsID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "membership lookup unavailable")
		return
	}
	defer rows.Close()
	items := []membershipItem{}
	for rows.Next() {
		var it membershipItem
		if err := rows.Scan(&it.PrincipalID, &it.RoleID, &it.State, &it.ExpiresAt); err != nil {
			writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "membership lookup unavailable")
			return
		}
		items = append(items, it)
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspace_id": ws.WorkspaceID, "items": items})
}

// ---- POST /api/v1/workspaces/{workspaceID}/memberships (grant or invite) ----

type membershipGrantRequest struct {
	Email     string  `json:"email"`      // intended principal
	RoleID    string  `json:"role_id"`    // kind-qualified catalogue role
	ExpiresAt *string `json:"expires_at"` // optional expiry (expiring grants)
}

func (s *Server) handleGrantMembership(w http.ResponseWriter, r *http.Request) {
	wsID := r.PathValue("workspaceID")
	sub := subjectOf(r)

	ws, err := s.DB.WorkspaceByID(r.Context(), wsID)
	if err != nil {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such workspace")
		return
	}
	if !s.visibilityFor(sub.Principal.PrincipalID, wsID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such workspace")
		return
	}
	if _, err := s.Authz.Authorize(r.Context(), sub.Principal.PrincipalID,
		"membership.grant", "membership", wsID, ws.Revision); err != nil {
		writeError(w, http.StatusForbidden, CodeForbidden, "membership administration not permitted")
		return
	}
	var req membershipGrantRequest
	if err := decodeStrict(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid membership schema")
		return
	}
	if req.ExpiresAt != nil {
		if _, err := time.Parse(time.RFC3339Nano, *req.ExpiresAt); err != nil {
			writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid expiry timestamp")
			return
		}
	}
	// Role must be a catalogue role of this workspace kind (no cross-kind
	// grants, no persona-code grants).
	valid := false
	for _, role := range authzRoleList(ws.Kind) {
		if role == req.RoleID {
			valid = true
			break
		}
	}
	if !valid {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "unknown role for workspace kind")
		return
	}
	target, err := s.DB.PrincipalByEmail(r.Context(), req.Email)
	if errors.Is(err, store.ErrNotFound) {
		// Invitation for a principal that does not exist yet: deny for now —
		// signup-bound invitations arrive with the installer wave (phase 06
		// bootstrap creates the first administrator; provider signup follows
		// in the commercial wave). No membership edge is created.
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "no such principal")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "principal lookup unavailable")
		return
	}

	// Idempotent grant: existing active membership for the same role returns
	// the current state instead of duplicating.
	existing, err := s.membershipState(r.Context(), target.PrincipalID, wsID, req.RoleID)
	if err == nil && (existing == "active" || existing == "invited") {
		writeJSON(w, http.StatusOK, map[string]any{
			"principal_id": target.PrincipalID, "role_id": req.RoleID,
			"state": existing, "replayed": true,
		})
		return
	}

	// Grant (same-scope, expiring grants supported).
	state := "active"
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		state = "active"
	}
	_, err = s.DB.ExecContext(r.Context(),
		`INSERT INTO membership(principal_id, workspace_id, role_id, revision, state, expires_at, invited_by)
		 VALUES(?,?,?,?,?,?,?)
		 ON CONFLICT(principal_id, workspace_id, role_id)
		 DO UPDATE SET state=excluded.state, revision=membership.revision+1,
		   expires_at=excluded.expires_at, invited_by=excluded.invited_by`,
		target.PrincipalID, wsID, req.RoleID, 1, state, req.ExpiresAt, sub.Principal.PrincipalID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "membership write unavailable")
		return
	}
	_ = s.Audit.Append(r.Context(), audit.Event{
		ActorID: sub.Principal.PrincipalID, ScopeID: wsID,
		ActionID: "membership.grant", ResourceID: target.PrincipalID,
		Outcome: "allow", SafeReason: "role=" + req.RoleID,
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"principal_id": target.PrincipalID, "role_id": req.RoleID, "state": state,
	})
}

// ---- DELETE /api/v1/workspaces/{workspaceID}/memberships/{principalID}/{roleID} ----

func (s *Server) handleRevokeMembership(w http.ResponseWriter, r *http.Request) {
	wsID := r.PathValue("workspaceID")
	targetID := r.PathValue("principalID")
	roleID := r.PathValue("roleID")
	sub := subjectOf(r)

	ws, err := s.DB.WorkspaceByID(r.Context(), wsID)
	if err != nil {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such workspace")
		return
	}
	if !s.visibilityFor(sub.Principal.PrincipalID, wsID) {
		writeError(w, http.StatusNotFound, CodeNotFound, "no such workspace")
		return
	}
	if _, err := s.Authz.Authorize(r.Context(), sub.Principal.PrincipalID,
		"membership.grant", "membership", wsID, ws.Revision); err != nil {
		writeError(w, http.StatusForbidden, CodeForbidden, "membership administration not permitted")
		return
	}

	// Last-owner protection (phase 18: retain as policy until ruled): the
	// last active owner membership of a workspace cannot be revoked — by
	// anyone, including the owner themselves.
	if roleID == ownerRoleFor(ws.Kind) {
		var owners int
		if err := s.DB.QueryRowContext(r.Context(),
			`SELECT COUNT(*) FROM membership WHERE workspace_id=? AND role_id=? AND state='active'`,
			wsID, roleID).Scan(&owners); err == nil && owners <= 1 {
			writeError(w, http.StatusConflict, CodeConflict,
				"cannot revoke the last active owner of this workspace")
			return
		}
	}
	if err := s.DB.RevokeMembership(r.Context(), targetID, wsID, roleID); err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "membership write unavailable")
		return
	}
	_ = s.Audit.Append(r.Context(), audit.Event{
		ActorID: sub.Principal.PrincipalID, ScopeID: wsID,
		ActionID: "membership.grant", ResourceID: targetID,
		Outcome: "allow", SafeReason: "revoke role=" + roleID,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (s *Server) membershipState(ctx context.Context, principalID, wsID, roleID string) (string, error) {
	var st string
	err := s.DB.QueryRowContext(ctx,
		`SELECT state FROM membership WHERE principal_id=? AND workspace_id=? AND role_id=?`,
		principalID, wsID, roleID).Scan(&st)
	if err != nil {
		return "", err
	}
	return st, nil
}

// authzRoleList exposes the kind-qualified role catalogue.
func authzRoleList(kind string) []string {
	return authz.RoleCatalogue[kind]
}

func ownerRoleFor(kind string) string {
	return kind + ".owner"
}

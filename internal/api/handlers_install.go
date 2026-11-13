package api

// Installation bootstrap endpoints (phase 06). Public while the installation
// is pending (the operator holds the claim token printed at first start);
// after claim, only authenticated administrators continue setup.

import (
	"errors"
	"net/http"

	"drevanta/internal/install"
)

func (s *Server) handleInstallStatus(w http.ResponseWriter, r *http.Request) {
	st, err := s.Install.GetStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "installation status unavailable")
		return
	}
	writeJSON(w, http.StatusOK, st)
}

type claimRequest struct {
	ClaimToken    string `json:"claim_token"`
	AdminEmail    string `json:"admin_email"`
	AdminPassword string `json:"admin_password"`
	DisplayName   string `json:"display_name"`
}

func (s *Server) handleInstallClaim(w http.ResponseWriter, r *http.Request) {
	var req claimRequest
	if err := decodeStrict(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid claim schema")
		return
	}
	if req.AdminEmail == "" || len(req.AdminPassword) < 12 {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "administrator credentials rejected by policy")
		return
	}
	res, err := s.Install.Claim(r.Context(), req.ClaimToken, req.AdminEmail, req.AdminPassword, req.DisplayName)
	switch {
	case errors.Is(err, install.ErrNotPending), errors.Is(err, install.ErrBadClaimToken):
		// Uniform denial: no hint which one failed.
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "claim rejected")
		return
	case err != nil:
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "claim unavailable")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"installation_id": res.InstallationID,
		"principal_id":    res.PrincipalID,
		"workspace_id":    res.WorkspaceID,
		"checkpoint":      "administrator_enrolled",
	})
}

type addressRequest struct {
	Address string `json:"address"`
}

func (s *Server) handleInstallAddress(w http.ResponseWriter, r *http.Request) {
	sub := subjectOf(r)
	// Only the installation owner continues setup: require an active server
	// workspace relationship (visibility check) — setup UI calls this with
	// the claimant's session.
	vis := false
	if ms, err := s.DB.ActiveMemberships(r.Context(), sub.Principal.PrincipalID); err == nil {
		for range ms {
			vis = true
			break
		}
	}
	if !vis {
		writeError(w, http.StatusForbidden, CodeForbidden, "setup not permitted")
		return
	}
	var req addressRequest
	if err := decodeStrict(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid address schema")
		return
	}
	if req.Address == "" {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "address required")
		return
	}
	if err := s.Install.SetAddress(r.Context(), req.Address); err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "address write unavailable")
		return
	}
	st, _ := s.Install.GetStatus(r.Context())
	writeJSON(w, http.StatusOK, st)
}

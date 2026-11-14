package api

// M-Pesa Daraja endpoints (owner directive §7-8): gateway configuration
// (owner/admin), customer invoice payment initiation, and the PUBLIC
// callback endpoint. Callbacks are untrusted: recording + gateway-side
// verification only — never sufficient to settle by themselves.

import (
	"encoding/json"
	"net/http"
	"time"

	"drevanta/internal/audit"
	"drevanta/internal/domains"
	"drevanta/internal/store"
)

// mpesaAdmin ensures the subject holds an active owner/admin membership.
func (s *Server) mpesaAdmin(w http.ResponseWriter, r *http.Request) bool {
	sub := subjectOf(r)
	if sub == nil || sub.Principal == nil {
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
		return false
	}
	ms, err := s.DB.ActiveMemberships(r.Context(), sub.Principal.PrincipalID)
	if err != nil || len(ms) == 0 {
		writeError(w, http.StatusForbidden, CodeForbidden, "administrative membership required")
		return false
	}
	for _, m := range ms {
		if m.RoleID == "owner" || m.RoleID == "admin" {
			return true
		}
	}
	writeError(w, http.StatusForbidden, CodeForbidden, "administrative membership required")
	return false
}

func (s *Server) handleMpesaConfigGet(w http.ResponseWriter, r *http.Request) {
	if !s.mpesaAdmin(w, r) {
		return
	}
	cfg, err := s.Mpesa.LoadConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"configured": true, "config": domains.SanitizeMpesaConfig(*cfg)})
}

func (s *Server) handleMpesaConfigPut(w http.ResponseWriter, r *http.Request) {
	if !s.mpesaAdmin(w, r) {
		return
	}
	var cfg domains.MpesaConfig
	if err := decodeStrict(r, &cfg); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid gateway configuration")
		return
	}
	if err := domains.ValidateMpesaConfig(cfg); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "configuration rejected: "+err.Error())
		return
	}
	if err := s.Mpesa.SaveConfig(r.Context(), cfg); err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "configuration persistence failed")
		return
	}
	s.Audit.Append(r.Context(), audit.Event{
		ActionID: "mpesa.config.updated", SafeReason: "environment=" + cfg.Environment, Outcome: "allow",
	})
	writeJSON(w, http.StatusOK, map[string]any{"config": domains.SanitizeMpesaConfig(cfg)})
}

func (s *Server) handleMpesaHealth(w http.ResponseWriter, r *http.Request) {
	if !s.mpesaAdmin(w, r) {
		return
	}
	cfg, err := s.Mpesa.LoadConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false, "health": "unconfigured"})
		return
	}
	eng := domains.NewMpesaEngine(*cfg, nil, s.Mpesa, nil)
	status, err := eng.HealthCheck(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"configured": true, "health": status, "detail": "credentials or connectivity rejected"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"configured": true, "health": status})
}

// handleMpesaInitiate is the customer-facing invoice payment start.
func (s *Server) handleMpesaInitiate(w http.ResponseWriter, r *http.Request) {
	sub := subjectOf(r)
	if sub == nil || sub.Principal == nil {
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
		return
	}
	cfg, err := s.Mpesa.LoadConfig(r.Context())
	if err != nil || !cfg.Enabled {
		writeError(w, http.StatusConflict, CodeInvalidInput, "gateway not available")
		return
	}
	var req struct {
		Phone       string `json:"phone"`
		AmountCents int64  `json:"amount_cents"`
	}
	if err := decodeStrict(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid payment request")
		return
	}
	invoiceID := r.PathValue("invoiceID")
	eng := domains.NewMpesaEngine(*cfg, nil, s.Mpesa, nil)
	// Scope: the caller must hold a membership (scope checks enforced per
	// invoice ownership by the invoice domain; here we bind the caller's
	// first active workspace to the attempt for audit correlation).
	ms, _ := s.DB.ActiveMemberships(r.Context(), sub.Principal.PrincipalID)
	scope := ""
	if len(ms) > 0 {
		scope = ms[0].WorkspaceID
	}
	out, err := eng.InitiateSTKPush(r.Context(), store.NewID("pay"), invoiceID, scope, req.Phone, req.AmountCents)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeInvalidInput, "payment not initiated: "+err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, out)
}

// handleMpesaCallback is PUBLIC and treats the payload as untrusted.
// It records the event and triggers gateway-side verification; a callback
// alone can never mark an invoice paid.
func (s *Server) handleMpesaCallback(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.Mpesa.LoadConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"ResultCode": "0", "ResultDesc": "accepted"}) // accept-and-ignore
		return
	}
	var raw json.RawMessage
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&raw); err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"ResultCode": "0", "ResultDesc": "accepted"})
		return
	}
	eng := domains.NewMpesaEngine(*cfg, nil, s.Mpesa, nil)
	_, _ = eng.ProcessCallback(r.Context(), raw)
	// Daraja expects an OK body regardless; reconciliation provides truth.
	writeJSON(w, http.StatusOK, map[string]string{"ResultCode": "0", "ResultDesc": "accepted"})
}

func (s *Server) handleMpesaReconcile(w http.ResponseWriter, r *http.Request) {
	if !s.mpesaAdmin(w, r) {
		return
	}
	cfg, err := s.Mpesa.LoadConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"transitions": nil})
		return
	}
	eng := domains.NewMpesaEngine(*cfg, nil, s.Mpesa, nil)
	writeJSON(w, http.StatusOK, map[string]any{"transitions": eng.ReconcilePending(r.Context(), 2*time.Minute)})
}

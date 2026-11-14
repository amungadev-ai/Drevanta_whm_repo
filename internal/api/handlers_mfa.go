package api

import (
	"net/http"
	"net/url"
)

// ---- GET /api/v1/identity/mfa/status ----
// Reports whether the subject has an active TOTP credential. The secret
// itself is never returned here (server-side verification only).

type mfaStatusBody struct {
	Enrolled bool `json:"enrolled"`
}

func (s *Server) handleMFAStatus(w http.ResponseWriter, r *http.Request) {
	sub := subjectOf(r)
	enrolled, err := s.Authn.HasActiveMFA(r.Context(), sub.Principal.PrincipalID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "mfa status unavailable")
		return
	}
	writeJSON(w, http.StatusOK, mfaStatusBody{Enrolled: enrolled})
}

// ---- POST /api/v1/identity/mfa/enroll ----
// Activates a TOTP credential for the authenticated subject and returns the
// enrollment secret plus the otpauth:// URI for authenticator provisioning.
// The secret crosses this boundary exactly once, is never logged or audited,
// and requires Cache-Control: no-store.
//
// Completing enrollment satisfies step-up for the enrolling session (the
// subject just proved possession by enrolling), which allows the approved
// onboarding order: administrator -> MFA -> recovery codes -> address.

type mfaEnrollBody struct {
	Secret     string `json:"secret"`
	OtpauthURI string `json:"otpauth_uri"`
}

func (s *Server) handleMFAEnroll(w http.ResponseWriter, r *http.Request) {
	sub := subjectOf(r)
	enrolled, err := s.Authn.HasActiveMFA(r.Context(), sub.Principal.PrincipalID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "mfa status unavailable")
		return
	}
	if enrolled {
		writeError(w, http.StatusConflict, CodeInvalidInput, "mfa already enrolled; rotate recovery codes instead")
		return
	}
	secret, err := s.Authn.EnrollTOTP(r.Context(), sub.Principal.PrincipalID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "mfa enrollment unavailable")
		return
	}
	// Step-up satisfied by enrollment (documented design decision).
	if _, err := s.DB.ExecContext(r.Context(),
		`UPDATE session SET mfa_satisfied=1 WHERE session_id=?`, sub.Session.SessionID); err != nil {
		writeError(w, http.StatusServiceUnavailable, CodeDependencyFailure, "mfa enrollment unavailable")
		return
	}
	uri := "otpauth://totp/" + url.PathEscape("Drevanta:"+sub.Principal.Email) +
		"?secret=" + secret + "&issuer=Drevanta&algorithm=SHA1&digits=6&period=30"
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, mfaEnrollBody{Secret: secret, OtpauthURI: uri})
}

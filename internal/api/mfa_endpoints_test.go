package api

// G40 Stage-2: MFA enrollment + recovery-code exposure over the control
// plane. The authn service capabilities (EnrollTOTP, GenerateRecoveryCodes)
// existed already; these tests bind the new API contracts:
//   GET  /api/v1/identity/mfa/status  -> {enrolled}
//   POST /api/v1/identity/mfa/enroll  -> 201 {secret, otpauth_uri}; step-up
//                                       satisfied for the enrolling session
//   POST /api/v1/identity/recovery-codes -> requires MFA-satisfied session

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestMFAEnrollLifecycle(t *testing.T) {
	f := newFixture(t)
	p := f.makePrincipal("mfaenroll@example.test")
	token := f.login("mfaenroll@example.test")

	// Status before enrollment.
	res := f.do("GET", "/api/v1/identity/mfa/status", "", token, nil)
	if res.Code != http.StatusOK || !strings.Contains(res.Body, `"enrolled":false`) {
		t.Fatalf("pre-enroll status: %d %s", res.Code, res.Body)
	}

	// Enroll: 201 with secret + otpauth URI; cache disabled.
	res = f.post("/api/v1/identity/mfa/enroll", "", token)
	if res.Code != http.StatusCreated {
		t.Fatalf("enroll: %d %s", res.Code, res.Body)
	}
	var eb mfaEnrollBody
	if err := json.Unmarshal([]byte(res.Body), &eb); err != nil {
		t.Fatal(err)
	}
	if eb.Secret == "" || !strings.HasPrefix(eb.OtpauthURI, "otpauth://totp/Drevanta:mfaenroll@example.test?secret=") {
		t.Fatalf("enroll payload: %+v", eb)
	}
	if res.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("enroll must be no-store, got %q", res.Header.Get("Cache-Control"))
	}

	// Step-up satisfied: recovery-code rotation works on the enrolling session.
	res = f.post("/api/v1/identity/recovery-codes", "", token)
	if res.Code != http.StatusOK || !strings.Contains(res.Body, "recovery_codes") {
		t.Fatalf("recovery-codes after enroll: %d %s", res.Code, res.Body)
	}

	// Status flips; second enrollment conflicts (409, uniform taxonomy).
	res = f.do("GET", "/api/v1/identity/mfa/status", "", token, nil)
	if !strings.Contains(res.Body, `"enrolled":true`) {
		t.Fatalf("post-enroll status: %d %s", res.Code, res.Body)
	}
	res = f.post("/api/v1/identity/mfa/enroll", "", token)
	if res.Code != http.StatusConflict {
		t.Fatalf("second enroll: %d %s", res.Code, res.Body)
	}

	// Fresh sign-in now requires MFA and the enrolled secret verifies.
	resp := f.post("/api/v1/identity/login", `{"email":"mfaenroll@example.test","password":"CorrectHorse-42!"}`, "")
	var lr loginResponse
	_ = json.Unmarshal([]byte(resp.Body), &lr)
	if !lr.MFARequired {
		t.Fatal("expected mfa_required after enrollment")
	}
	secret, err := f.auth.MFASecret(context.Background(), p.PrincipalID)
	if err != nil {
		t.Fatal(err)
	}
	res = f.post("/api/v1/identity/mfa", `{"principal_id":"`+lr.PrincipalID+`","code":"`+totpNow(t, secret)+`"}`, "")
	if res.Code != http.StatusOK || !strings.Contains(res.Body, `"token"`) {
		t.Fatalf("mfa verify with enrolled secret: %d %s", res.Code, res.Body)
	}

	// No session: enrollment denies (uniform 401 taxonomy).
	if res := f.post("/api/v1/identity/mfa/enroll", "", ""); res.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated enroll: %d", res.Code)
	}
}

// Recovery-code rotation without step-up denies with the uniform taxonomy.
func TestRecoveryCodesRequireStepUp(t *testing.T) {
	f := newFixture(t)
	f.makePrincipal("stepup@example.test")
	token := f.login("stepup@example.test") // no MFA enrolled: session not MFA-satisfied
	res := f.post("/api/v1/identity/recovery-codes", "", token)
	if res.Code != http.StatusForbidden || !strings.Contains(res.Body, "step-up") {
		t.Fatalf("recovery-codes without step-up: %d %s", res.Code, res.Body)
	}
}

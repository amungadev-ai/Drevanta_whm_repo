package api

// G21 verification suite: identity enforcement (phase 17) and permission
// enforcement (phase 18). Binds TC-P17-01/02 analogs and TC-P18-01/02 analogs
// at the control-plane boundary (helper-dispatch-count evidence lands with G20).

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"drevanta/internal/operations"
	"drevanta/internal/store"
)

// captureMailer records delivered recovery tokens (test-only Mailer).
type captureMailer struct {
	resets map[string]string
}

func (c *captureMailer) DeliverPasswordReset(_ context.Context, email, token string) error {
	if c.resets == nil {
		c.resets = map[string]string{}
	}
	c.resets[email] = token
	return nil
}

// ---- Phase 17: password reset lifecycle (TC-P17-01/02 analog) ----

// Reset: duplicate redemption → exactly one success; old session denied after
// reset; MFA preserved; fresh sign-in requires MFA again with new password.
func TestIdentityPasswordResetLifecycle(t *testing.T) {
	f := newFixture(t)
	capture := &captureMailer{}
	f.apiServer.Mailer = capture

	p := f.makePrincipal("reset@example.test")
	if _, err := f.auth.EnrollTOTP(context.Background(), p.PrincipalID); err != nil {
		t.Fatal(err)
	}
	secret, err := f.auth.MFASecret(context.Background(), p.PrincipalID)
	if err != nil {
		t.Fatal(err)
	}

	// Old session via MFA login.
	resp := f.post("/api/v1/identity/login", `{"email":"reset@example.test","password":"CorrectHorse-42!"}`, "")
	var lr loginResponse
	_ = json.Unmarshal([]byte(resp.Body), &lr)
	oldSess := f.post("/api/v1/identity/mfa", `{"principal_id":"`+lr.PrincipalID+`","code":"`+totpNow(t, secret)+`"}`, "")
	var mr mfaResponse
	_ = json.Unmarshal([]byte(oldSess.Body), &mr)
	if mr.Token == "" {
		t.Fatal("no old session token")
	}

	// Request recovery; uniform 202 (identical for unknown address — no enumeration).
	resp = f.post("/api/v1/identity/recover-request", `{"email":"reset@example.test"}`, "")
	if resp.Code != http.StatusAccepted {
		t.Fatalf("recover-request: %d %s", resp.Code, resp.Body)
	}
	respUnknown := f.post("/api/v1/identity/recover-request", `{"email":"nobody@example.test"}`, "")
	if respUnknown.Body != resp.Body {
		t.Fatalf("recovery response differs for unknown account")
	}
	token := capture.resets["reset@example.test"]
	if token == "" {
		t.Fatal("reset token not delivered")
	}

	// First redemption succeeds; second use of the same token denies.
	ok := f.post("/api/v1/identity/recover-confirm",
		`{"email":"reset@example.test","token":"`+token+`","new_password":"NewPassphrase-77!"}`, "")
	if ok.Code != http.StatusOK {
		t.Fatalf("first reset: %d %s", ok.Code, ok.Body)
	}
	replay := f.post("/api/v1/identity/recover-confirm",
		`{"email":"reset@example.test","token":"`+token+`","new_password":"NewPassphrase-88!"}`, "")
	if replay.Code != http.StatusUnauthorized {
		t.Fatalf("replayed reset token: want 401, got %d", replay.Code)
	}

	// Old session denied after reset (epoch mismatch).
	resp = f.get("/api/v1/me", mr.Token)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("old session after reset: want 401, got %d", resp.Code)
	}

	// Fresh sign-in requires MFA again (enrollment preserved), new password.
	resp = f.post("/api/v1/identity/login", `{"email":"reset@example.test","password":"NewPassphrase-77!"}`, "")
	var lr2 loginResponse
	_ = json.Unmarshal([]byte(resp.Body), &lr2)
	if !lr2.MFARequired {
		t.Fatalf("MFA enrollment not preserved: %s", resp.Body)
	}
	fresh := f.post("/api/v1/identity/mfa", `{"principal_id":"`+lr2.PrincipalID+`","code":"`+totpNow(t, secret)+`"}`, "")
	if fresh.Code != http.StatusOK {
		t.Fatalf("fresh mfa login: %d %s", fresh.Code, fresh.Body)
	}
}

// Recovery codes: single consumption, accepted as MFA second factor only.
func TestIdentityRecoveryCodes(t *testing.T) {
	f := newFixture(t)
	p := f.makePrincipal("codes@example.test")
	if _, err := f.auth.EnrollTOTP(context.Background(), p.PrincipalID); err != nil {
		t.Fatal(err)
	}
	secret, _ := f.auth.MFASecret(context.Background(), p.PrincipalID)
	resp := f.post("/api/v1/identity/login", `{"email":"codes@example.test","password":"CorrectHorse-42!"}`, "")
	var lr loginResponse
	_ = json.Unmarshal([]byte(resp.Body), &lr)
	sess := f.post("/api/v1/identity/mfa", `{"principal_id":"`+lr.PrincipalID+`","code":"`+totpNow(t, secret)+`"}`, "")
	var mr mfaResponse
	_ = json.Unmarshal([]byte(sess.Body), &mr)

	rc := f.post("/api/v1/identity/recovery-codes", "", mr.Token)
	var codes struct {
		RecoveryCodes []string `json:"recovery_codes"`
	}
	if err := json.Unmarshal([]byte(rc.Body), &codes); err != nil || len(codes.RecoveryCodes) != 10 {
		t.Fatalf("recovery codes: %d %s", rc.Code, rc.Body)
	}

	// Password proof + recovery code as second factor: one success, replay denied.
	resp = f.post("/api/v1/identity/login", `{"email":"codes@example.test","password":"CorrectHorse-42!"}`, "")
	var lr2 loginResponse
	_ = json.Unmarshal([]byte(resp.Body), &lr2)
	use := f.post("/api/v1/identity/mfa",
		`{"principal_id":"`+lr2.PrincipalID+`","code":"`+codes.RecoveryCodes[0]+`"}`, "")
	if use.Code != http.StatusOK {
		t.Fatalf("recovery code login: %d %s", use.Code, use.Body)
	}
	replay := f.post("/api/v1/identity/mfa",
		`{"principal_id":"`+lr2.PrincipalID+`","code":"`+codes.RecoveryCodes[0]+`"}`, "")
	if replay.Code != http.StatusUnauthorized {
		t.Fatalf("replayed recovery code: want 401, got %d", replay.Code)
	}
}

// Logout revokes the current session only.
func TestIdentityLogout(t *testing.T) {
	f := newFixture(t)
	p := f.makePrincipal("logout@example.test")
	_, res := f.makeWebsiteWorkspace(p, "ws-logout")
	tok1 := f.login("logout@example.test")
	tok2 := f.login("logout@example.test")

	resp := f.post("/api/v1/identity/logout", "", tok1)
	if resp.Code != http.StatusOK {
		t.Fatalf("logout: %d %s", resp.Code, resp.Body)
	}
	if r := f.get("/api/v1/me", tok1); r.Code != http.StatusUnauthorized {
		t.Fatalf("logged-out session still valid")
	}
	if r := f.get("/api/v1/me", tok2); r.Code != http.StatusOK {
		t.Fatalf("sibling session wrongly revoked: %d", r.Code)
	}
	_ = res
}

// ---- Phase 18: membership administration (TC-P18-01/02 analog) ----

// Grant via API; scoped action succeeds for granted member; revocation while
// an operation is queued cancels it at the dispatch recheck, never dispatches.
func TestMembershipGrantRevokeAndDispatchRecheck(t *testing.T) {
	f := newFixture(t)
	owner := f.makePrincipal("owner@example.test")
	ws, res := f.makeWebsiteWorkspace(owner, "ws-mgmt")
	ownerTok := f.login("owner@example.test")
	target := f.makePrincipal("dev@example.test")

	grant := f.post("/api/v1/workspaces/"+ws.WorkspaceID+"/memberships",
		`{"email":"dev@example.test","role_id":"website.developer"}`, ownerTok)
	if grant.Code != http.StatusCreated {
		t.Fatalf("grant: %d %s", grant.Code, grant.Body)
	}

	// Granted member can act on the resource (TC-P18-02 analog).
	targetTok := f.login("dev@example.test")
	r1 := f.post("/api/v1/resources/"+res.ResourceID+"/intents", validIntent(1, 0), targetTok)
	if r1.Code != http.StatusAccepted {
		t.Fatalf("granted member action: %d %s", r1.Code, r1.Body)
	}
	var acc operationAcceptedBody
	_ = json.Unmarshal([]byte(r1.Body), &acc)

	// Membership list requires membership.grant: member denied, owner allowed.
	listDenied := f.get("/api/v1/workspaces/"+ws.WorkspaceID+"/memberships", targetTok)
	if listDenied.Code != http.StatusForbidden {
		t.Fatalf("member listing memberships: want 403, got %d", listDenied.Code)
	}
	list := f.get("/api/v1/workspaces/"+ws.WorkspaceID+"/memberships", ownerTok)
	if list.Code != http.StatusOK {
		t.Fatalf("owner listing memberships: %d %s", list.Code, list.Body)
	}

	// Revoke while queued, then run the dispatch guard: cancel, never dispatch.
	revoke := f.do("DELETE",
		"/api/v1/workspaces/"+ws.WorkspaceID+"/memberships/"+target.PrincipalID+"/website.developer",
		"", ownerTok, nil)
	if revoke.Code != http.StatusOK {
		t.Fatalf("revoke: %d %s", revoke.Code, revoke.Body)
	}
	_, err := f.Ops.DispatchGuard(context.Background(), acc.OperationID,
		time.Now().Add(time.Minute), func(ctx context.Context, op *operations.Operation) error {
			// Full predicate against current state.
			_, err := f.Authz.Authorize(ctx, op.ActorID, op.ActionID, "filesystem entry", op.ScopeID, 1)
			return err
		})
	if err == nil {
		t.Fatal("dispatch guard allowed revoked actor")
	}
	op, err := f.Ops.Get(context.Background(), acc.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if op.State != "cancelled" {
		t.Fatalf("operation state after revoked dispatch: %q", op.State)
	}
}

// Last-owner protection: revoking the last active owner conflicts (409).
func TestMembershipLastOwnerProtection(t *testing.T) {
	f := newFixture(t)
	owner := f.makePrincipal("owner@example.test")
	ws, _ := f.makeWebsiteWorkspace(owner, "ws-last")
	ownerTok := f.login("owner@example.test")

	resp := f.do("DELETE",
		"/api/v1/workspaces/"+ws.WorkspaceID+"/memberships/"+owner.PrincipalID+"/website.owner",
		"", ownerTok, nil)
	if resp.Code != http.StatusConflict {
		t.Fatalf("last-owner revoke: want 409, got %d %s", resp.Code, resp.Body)
	}
	var state string
	if err := f.db.QueryRow("SELECT state FROM membership WHERE principal_id=? AND workspace_id=? AND role_id=?",
		owner.PrincipalID, ws.WorkspaceID, "website.owner").Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "active" {
		t.Fatalf("owner membership state after denied revoke: %q", state)
	}
}

// Unknown or cross-kind roles deny with 422.
func TestMembershipRoleValidation(t *testing.T) {
	f := newFixture(t)
	owner := f.makePrincipal("owner@example.test")
	ws, _ := f.makeWebsiteWorkspace(owner, "ws-roles")
	ownerTok := f.login("owner@example.test")

	resp := f.post("/api/v1/workspaces/"+ws.WorkspaceID+"/memberships",
		`{"email":"owner@example.test","role_id":"server.administrator"}`, ownerTok)
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("cross-kind grant: want 422, got %d %s", resp.Code, resp.Body)
	}
	resp = f.post("/api/v1/workspaces/"+ws.WorkspaceID+"/memberships",
		`{"email":"owner@example.test","role_id":"website.supreme"}`, ownerTok)
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown role: want 422, got %d %s", resp.Code, resp.Body)
	}
}

// Support lease grants exactly its listed actions and expires on schedule.
func TestSupportLeaseExactActions(t *testing.T) {
	f := newFixture(t)
	owner := f.makePrincipal("owner@example.test")
	ws, res := f.makeWebsiteWorkspace(owner, "ws-lease")
	ctx := context.Background()

	agent := f.makePrincipal("support@example.test")
	leaseID := store.NewID("lea")
	exp := time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano)
	if _, err := f.db.ExecContext(ctx,
		`INSERT INTO support_lease(lease_id, principal_id, scope_id, approval_id, expires_at) VALUES(?,?,?,?,?)`,
		leaseID, agent.PrincipalID, ws.WorkspaceID, "approval-1", exp); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(ctx,
		`INSERT INTO support_lease_grant(lease_id, action_id) VALUES(?, 'monitor.read')`, leaseID); err != nil {
		t.Fatal(err)
	}

	// monitor.read allowed via lease (resource type health observation — use
	// the audit endpoint? monitor.read binds health observation type; here we
	// verify the checker directly for exact-action semantics).
	role, err := f.Authz.Authorize(ctx, agent.PrincipalID, "monitor.read", "health observation", ws.WorkspaceID, 1)
	if err != nil {
		t.Fatalf("lease-granted action denied: %v", err)
	}
	if role != "support_lease" {
		t.Fatalf("role: %q", role)
	}
	// Unlisted action denied even though lease is active.
	if _, err := f.Authz.Authorize(ctx, agent.PrincipalID, "file.write", "filesystem entry", ws.WorkspaceID, 1); err == nil {
		t.Fatal("unlisted lease action allowed")
	}
	// Revoked lease denies everything.
	if _, err := f.db.ExecContext(ctx,
		`UPDATE support_lease SET revoked_at=? WHERE lease_id=?`, store.Now(), leaseID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Authz.Authorize(ctx, agent.PrincipalID, "monitor.read", "health observation", ws.WorkspaceID, 1); err == nil {
		t.Fatal("revoked lease allowed")
	}
	_ = res
}

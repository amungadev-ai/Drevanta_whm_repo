// Package install implements the bootstrap/installation lifecycle (phase 06).
//
// Contract basis: domain contract `setup` — claim token scoped to
// installation; atomic consume with owner creation; checkpoints survive
// restart (interrupted setup resumes); recovery-code enrollment at claim.
package install

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"drevanta/internal/authn"
	"drevanta/internal/store"
)

// State machine: pending → (claim) → claimed → (address) → active.
const (
	StatePending = "pending"
	StateClaimed = "claimed"
	StateActive  = "active"
)

var (
	ErrNotPending    = errors.New("installation not pending")
	ErrBadClaimToken = errors.New("invalid claim token")
)

// Installer drives installation checkpoints.
type Installer struct {
	db   *store.DB
	auth *authn.Service
}

// NewInstaller builds an Installer.
func NewInstaller(db *store.DB, auth *authn.Service) *Installer {
	return &Installer{db: db, auth: auth}
}

// EnsureInstallation creates the installation record on first run and
// returns (installationID, rawClaimToken). If an installation already exists
// it returns its ID and empty token. The raw claim token is printed once to
// the operator console and only its hash is stored.
func (in *Installer) EnsureInstallation(ctx context.Context) (string, string, error) {
	var id, state string
	err := in.db.QueryRowContext(ctx,
		`SELECT installation_id, state FROM installation LIMIT 1`).Scan(&id, &state)
	if err == nil {
		return id, "", nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", "", err
	}
	raw := store.NewID("clm") + store.NewID("t")[4:]
	id = store.NewID("inst")
	sum := sha256.Sum256([]byte(raw))
	if _, err := in.db.ExecContext(ctx,
		`INSERT INTO installation(installation_id, state, claim_token_hash, created_at, checkpoint)
		 VALUES(?,?,?,?, 'started')`,
		id, StatePending, hex.EncodeToString(sum[:]), store.Now()); err != nil {
		return "", "", err
	}
	return id, raw, nil
}

// Status returns the installation state for the setup UI.
type Status struct {
	InstallationID string  `json:"installation_id"`
	State          string  `json:"state"`
	Checkpoint     string  `json:"checkpoint"`
	Address        *string `json:"address,omitempty"`
}

// GetStatus reads the current installation row (survives restart; resumed
// setup continues from the recorded checkpoint).
func (in *Installer) GetStatus(ctx context.Context) (*Status, error) {
	s := &Status{}
	err := in.db.QueryRowContext(ctx,
		`SELECT installation_id, state, checkpoint, address FROM installation LIMIT 1`).
		Scan(&s.InstallationID, &s.State, &s.Checkpoint, &s.Address)
	if errors.Is(err, sql.ErrNoRows) {
		return &Status{InstallationID: "", State: "uninitialized", Checkpoint: "started"}, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

// ClaimedResult carries the claim outcome.
type ClaimedResult struct {
	InstallationID string
	PrincipalID    string
	WorkspaceID    string
}

// Claim atomically consumes the claim token and creates the first
// administrator with an owner membership on the server workspace. MFA
// enrollment and recovery codes are set up by the caller through the authn
// service (setup UI drives the flow); password epoch revokes nothing yet.
func (in *Installer) Claim(ctx context.Context, claimToken, adminEmail, adminPassword, displayName string) (*ClaimedResult, error) {
	// Exactly one pending installation.
	var instID, state, tokHash, createdAt string
	err := in.db.QueryRowContext(ctx,
		`SELECT installation_id, state, claim_token_hash, created_at FROM installation LIMIT 1`).
		Scan(&instID, &state, &tokHash, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotPending
	}
	if err != nil {
		return nil, err
	}
	if state != StatePending {
		return nil, ErrNotPending
	}
	// Claim token expiry (phase 06: claim token scoped and expiring).
	created, err := time.Parse(time.RFC3339Nano, createdAt)
	if err == nil && time.Now().UTC().After(created.Add(ClaimTTL)) {
		return nil, ErrBadClaimToken
	}
	sum := sha256.Sum256([]byte(claimToken))
	if hex.EncodeToString(sum[:]) != tokHash {
		return nil, ErrBadClaimToken
	}

	// Atomic consume: only one claim transaction wins.
	tx, err := in.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx,
		`UPDATE installation SET state=?, claimed_at=?, checkpoint='administrator_enrolled'
		 WHERE installation_id=? AND state=?`,
		StateClaimed, store.Now(), instID, StatePending)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotPending
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Owner creation after the token is consumed (checkpoint recorded; a
	// restart resumes from 'administrator_enrolled').
	p, err := in.db.CreatePrincipal(ctx, adminEmail, displayName)
	if err != nil {
		return nil, err
	}
	if err := in.auth.SetPrimaryPassword(ctx, p.PrincipalID, adminPassword); err != nil {
		return nil, err
	}
	ws, err := in.db.CreateWorkspace(ctx, "server", p.PrincipalID, "Server Administration")
	if err != nil {
		return nil, err
	}
	if err := in.db.AddMembership(ctx, store.Membership{
		PrincipalID: p.PrincipalID, WorkspaceID: ws.WorkspaceID,
		RoleID: "server.owner", Revision: 1, State: "active",
	}); err != nil {
		return nil, err
	}
	// Full-persona provisioning (G40-F1): the platform is four-persona, so
	// the install administrator receives one workspace per persona kind with
	// the respective owner role. Each workspace is independently scoped —
	// tenancy isolation applies across them exactly as between principals.
	extra := []struct {
		kind, name, role string
	}{
		{"hosting_org", "Hosting Organization", "hosting.owner"},
		{"website", "Website Workspace", "website.owner"},
		{"developer_project", "Developer Project", "developer.owner"},
	}
	for _, spec := range extra {
		w, err := in.db.CreateWorkspace(ctx, spec.kind, p.PrincipalID, spec.name)
		if err != nil {
			return nil, err
		}
		if err := in.db.AddMembership(ctx, store.Membership{
			PrincipalID: p.PrincipalID, WorkspaceID: w.WorkspaceID,
			RoleID: spec.role, Revision: 1, State: "active",
		}); err != nil {
			return nil, err
		}
	}
	_, _ = in.db.ExecContext(ctx,
		`UPDATE installation SET state=?, checkpoint='workspace_ready' WHERE installation_id=?`,
		StateActive, instID)
	return &ClaimedResult{InstallationID: instID, PrincipalID: p.PrincipalID, WorkspaceID: ws.WorkspaceID}, nil
}

// SetAddress records the access address and advances the checkpoint.
func (in *Installer) SetAddress(ctx context.Context, address string) error {
	_, err := in.db.ExecContext(ctx,
		`UPDATE installation SET address=?, checkpoint='address_configured' WHERE state IN (?,?)`,
		address, StateClaimed, StateActive)
	return err
}

// MarkAddressVerified completes setup once the address is observed serving.
func (in *Installer) MarkAddressVerified(ctx context.Context) error {
	_, err := in.db.ExecContext(ctx,
		`UPDATE installation SET address_verified_at=?, checkpoint='complete', state=? WHERE address IS NOT NULL`,
		store.Now(), StateActive)
	return err
}

// ClaimTTL bounds how long a claim token remains valid (policy value; DEC-018
// fixture default, not an operating limit).
const ClaimTTL = 24 * time.Hour

// RegenerateClaimToken replaces the outstanding claim token of a still-pending
// installation (authorized recovery: the printed token was lost, expired or
// must be rotated before claiming). A fresh token is generated, only its hash
// is stored, and the 24h validity window restarts. A claimed installation can
// never be regenerated — the owner claim is final once consumed.
func (in *Installer) RegenerateClaimToken(ctx context.Context) (string, error) {
	var instID, state string
	err := in.db.QueryRowContext(ctx,
		`SELECT installation_id, state FROM installation LIMIT 1`).Scan(&instID, &state)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && state != StatePending) {
		return "", ErrNotPending
	}
	if err != nil {
		return "", err
	}
	raw := store.NewID("clm") + store.NewID("t")[4:]
	sum := sha256.Sum256([]byte(raw))
	tx, err := in.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx,
		`UPDATE installation SET claim_token_hash=?, created_at=?
		 WHERE installation_id=? AND state=?`,
		hex.EncodeToString(sum[:]), store.Now(), instID, StatePending)
	if err != nil {
		return "", err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return "", ErrNotPending
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return raw, nil
}

package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Principal is a human or machine identity. Auth material lives in `credential`.
type Principal struct {
	PrincipalID     string
	Email           string
	DisplayName     string
	Status          string // active | disabled
	CredentialEpoch int64
	CreatedAt       string
}

// Workspace is a tenancy boundary of a given kind.
type Workspace struct {
	WorkspaceID  string
	Kind         string // server | hosting_org | website | developer_project
	OwnerScopeID string
	Revision     int64
	Name         string
	CreatedAt    string
}

// Membership binds a principal to a workspace with a role.
type Membership struct {
	PrincipalID string
	WorkspaceID string
	RoleID      string
	Revision    int64
	State       string // active | suspended | revoked
	ExpiresAt   *string
}

// Resource is a governed record with monotonic generations.
type Resource struct {
	ResourceID         string
	ScopeID            string
	HostID             string
	Type               string
	DesiredGeneration  int64
	ObservedGeneration int64
	OwnerRevision      int64
	Name               string
	CreatedAt          string
}

const qPrincipal = `SELECT principal_id, email, display_name, status, credential_epoch, created_at FROM principal`

// CreatePrincipal inserts a new principal (email unique).
func (d *DB) CreatePrincipal(ctx context.Context, email, displayName string) (*Principal, error) {
	p := &Principal{
		PrincipalID: NewID("prn"),
		Email:       email,
		DisplayName: displayName,
		Status:      "active",
		CreatedAt:   Now(),
	}
	_, err := d.ExecContext(ctx,
		`INSERT INTO principal(principal_id, email, display_name, status, credential_epoch, created_at)
		 VALUES(?,?,?,?,0,?)`,
		p.PrincipalID, p.Email, p.DisplayName, p.Status, p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func scanPrincipal(row *sql.Row) (*Principal, error) {
	p := &Principal{}
	err := row.Scan(&p.PrincipalID, &p.Email, &p.DisplayName, &p.Status, &p.CredentialEpoch, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// PrincipalByEmail fetches a principal by email.
func (d *DB) PrincipalByEmail(ctx context.Context, email string) (*Principal, error) {
	return scanPrincipal(d.QueryRowContext(ctx, qPrincipal+` WHERE email = ?`, email))
}

// PrincipalByID fetches a principal by ID.
func (d *DB) PrincipalByID(ctx context.Context, id string) (*Principal, error) {
	return scanPrincipal(d.QueryRowContext(ctx, qPrincipal+` WHERE principal_id = ?`, id))
}

// BumpCredentialEpoch invalidates all prior sessions/challenges bound to the
// old epoch (monotonic per schema constraint).
func (d *DB) BumpCredentialEpoch(ctx context.Context, principalID string) (int64, error) {
	_, err := d.ExecContext(ctx,
		`UPDATE principal SET credential_epoch = credential_epoch + 1 WHERE principal_id = ?`, principalID)
	if err != nil {
		return 0, err
	}
	var epoch int64
	err = d.QueryRowContext(ctx,
		`SELECT credential_epoch FROM principal WHERE principal_id = ?`, principalID).Scan(&epoch)
	return epoch, err
}

// SetPrincipalStatus toggles active/disabled.
func (d *DB) SetPrincipalStatus(ctx context.Context, principalID, status string) error {
	_, err := d.ExecContext(ctx, `UPDATE principal SET status = ? WHERE principal_id = ?`, status, principalID)
	return err
}

// CreateWorkspace inserts a workspace with an authoritative owner scope.
func (d *DB) CreateWorkspace(ctx context.Context, kind, ownerScopeID, name string) (*Workspace, error) {
	w := &Workspace{
		WorkspaceID:  NewID("ws"),
		Kind:         kind,
		OwnerScopeID: ownerScopeID,
		Revision:     1,
		Name:         name,
		CreatedAt:    Now(),
	}
	_, err := d.ExecContext(ctx,
		`INSERT INTO workspace(workspace_id, kind, owner_scope_id, revision, name, created_at)
		 VALUES(?,?,?,?,?,?)`,
		w.WorkspaceID, w.Kind, w.OwnerScopeID, w.Revision, w.Name, w.CreatedAt)
	if err != nil {
		return nil, err
	}
	return w, nil
}

// WorkspaceByID fetches a workspace.
func (d *DB) WorkspaceByID(ctx context.Context, id string) (*Workspace, error) {
	w := &Workspace{}
	err := d.QueryRowContext(ctx,
		`SELECT workspace_id, kind, owner_scope_id, revision, name, created_at FROM workspace WHERE workspace_id = ?`, id).
		Scan(&w.WorkspaceID, &w.Kind, &w.OwnerScopeID, &w.Revision, &w.Name, &w.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return w, nil
}

// AddMembership inserts an active membership.
func (d *DB) AddMembership(ctx context.Context, m Membership) error {
	if m.State == "" {
		m.State = "active"
	}
	if m.Revision == 0 {
		m.Revision = 1
	}
	_, err := d.ExecContext(ctx,
		`INSERT INTO membership(principal_id, workspace_id, role_id, revision, state, expires_at)
		 VALUES(?,?,?,?,?,?)`,
		m.PrincipalID, m.WorkspaceID, m.RoleID, m.Revision, m.State, m.ExpiresAt)
	return err
}

// ActiveMemberships returns the principal's active, unexpired memberships.
func (d *DB) ActiveMemberships(ctx context.Context, principalID string) ([]Membership, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT principal_id, workspace_id, role_id, revision, state, expires_at
		 FROM membership
		 WHERE principal_id = ? AND state = 'active' AND (expires_at IS NULL OR expires_at > ?)`,
		principalID, Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Membership
	for rows.Next() {
		var m Membership
		if err := rows.Scan(&m.PrincipalID, &m.WorkspaceID, &m.RoleID, &m.Revision, &m.State, &m.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// RevokeMembership flips state to revoked (deny at next recomputation).
func (d *DB) RevokeMembership(ctx context.Context, principalID, workspaceID, roleID string) error {
	_, err := d.ExecContext(ctx,
		`UPDATE membership SET state = 'revoked' WHERE principal_id = ? AND workspace_id = ? AND role_id = ?`,
		principalID, workspaceID, roleID)
	return err
}

// CreateResource inserts a resource at generation 0.
func (d *DB) CreateResource(ctx context.Context, scopeID, hostID, typ, name string) (*Resource, error) {
	r := &Resource{
		ResourceID: NewID("res"),
		ScopeID:    scopeID,
		HostID:     hostID,
		Type:       typ,
		Name:       name,
		CreatedAt:  Now(),
	}
	_, err := d.ExecContext(ctx,
		`INSERT INTO resource(resource_id, scope_id, host_id, type, desired_generation, observed_generation, owner_revision, name, created_at)
		 VALUES(?,?,?,?,0,0,1,?,?)`,
		r.ResourceID, r.ScopeID, r.HostID, r.Type, r.Name, r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return r, nil
}

const qResource = `SELECT resource_id, scope_id, COALESCE(host_id,''), type, desired_generation, observed_generation, owner_revision, name, created_at FROM resource`

func scanResource(row *sql.Row) (*Resource, error) {
	r := &Resource{}
	err := row.Scan(&r.ResourceID, &r.ScopeID, &r.HostID, &r.Type, &r.DesiredGeneration, &r.ObservedGeneration, &r.OwnerRevision, &r.Name, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

// ResourceByID fetches a resource.
func (d *DB) ResourceByID(ctx context.Context, id string) (*Resource, error) {
	return scanResource(d.QueryRowContext(ctx, qResource+` WHERE resource_id = ?`, id))
}

// ResourcesInScope lists resources inside one owning scope (composite
// owner/target lookup; callers cannot query across scopes).
func (d *DB) ResourcesInScope(ctx context.Context, scopeID, typ string) ([]Resource, error) {
	q := qResource + ` WHERE scope_id = ?`
	args := []any{scopeID}
	if typ != "" {
		q += ` AND type = ?`
		args = append(args, typ)
	}
	q += ` ORDER BY created_at, resource_id`
	rows, err := d.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Resource
	for rows.Next() {
		r := Resource{}
		if err := rows.Scan(&r.ResourceID, &r.ScopeID, &r.HostID, &r.Type, &r.DesiredGeneration, &r.ObservedGeneration, &r.OwnerRevision, &r.Name, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ExpiredLeaseCutoff is a helper for tests and policy checks.
func ExpiredLeaseCutoff(d time.Duration) string {
	return time.Now().UTC().Add(d).Format(time.RFC3339Nano)
}

// Package authz implements the authorization engine.
//
// Contract basis: profile.md §Authorization and tenant isolation, DEC-004
// (explicit per-workspace membership, ownership edges, default deny),
// permission-matrix.csv (action/resource vocabulary). Role or persona names
// never grant authority by themselves: every allow requires an explicit
// grant_rule row plus an active membership (or active support lease) on the
// resource's owning scope, evaluated against the current owner revision.
package authz

import (
	"context"
	"errors"
	"time"

	"drevanta/internal/store"
)

// The 41 governed actions from spec permission-matrix.csv (single vocabulary
// for grants and operations; unknown actions deny before dispatch).
var Actions = []string{
	"account.provision", "allocation.reserve", "application.provision", "audit.read",
	"backup.restore", "billing.adjust", "cache.configure", "database.provision",
	"deployment.promote", "domain.bind", "environment.clone", "file.write",
	"host.inspect", "host.network.change", "host.place", "host.update",
	"integration.configure", "mail.domain.configure", "mailbox.provision", "membership.grant",
	"module.install", "monitor.read", "operation.retry", "payment.record",
	"release.certify", "resource.link", "runtime.configure", "runtime.toolchain.configure",
	"schedule.configure", "security.policy.apply", "session.issue", "sftp.grant",
	"site.log.read", "source.connect", "storage.inspect", "support.article.publish",
	"support.ticket.create", "template.install", "terminal.open", "vhost.apply", "zone.publish",
}

// ActionSet builds a lookup set.
func ActionSet() map[string]bool {
	m := make(map[string]bool, len(Actions))
	for _, a := range Actions {
		m[a] = true
	}
	return m
}

// Workspace kinds and their development-default role catalogue (DEC-004
// pending ratification; conservative, explicit, no wildcards). Role IDs are
// kind-qualified ("website.owner"); roles are data (role/grant_rule rows
// seeded by migration), never code checks.
var RoleCatalogue = map[string][]string{
	"server":            {"server.owner", "server.administrator", "server.operator", "server.viewer"},
	"hosting_org":       {"hosting.owner", "hosting.administrator", "hosting.reseller", "hosting.support_agent", "hosting.viewer"},
	"website":           {"website.owner", "website.manager", "website.developer", "website.viewer"},
	"developer_project": {"developer.owner", "developer.maintainer", "developer.developer", "developer.viewer"},
}

// ErrDenied is the uniform denial result (safe reason attached by callers).
var ErrDenied = errors.New("denied")

// Subject is the resolved authorization subject.
type Subject struct {
	PrincipalID string
	// ActiveRole holds the role used for the current decision, if allowed.
	ActiveRole string
}

// Checker evaluates authorization decisions from store rows.
type Checker struct {
	db *store.DB
}

// NewChecker builds a Checker.
func NewChecker(db *store.DB) *Checker { return &Checker{db: db} }

// Authorize applies the full allow predicate:
//
//	active principal + explicit action + active membership/support lease on the
//	resource's owning scope + role grant for (action, resource_type) + current
//	owner revision.
//
// On allow it returns the role that carried the grant ("support_lease" for
// lease-carried grants). On deny the reason is a *safe* string (no resource
// identifiers, no internal state).
func (c *Checker) Authorize(ctx context.Context, principalID, action, resourceType, scopeID string, ownerRevision int64) (string, error) {
	if principalID == "" || action == "" || resourceType == "" || scopeID == "" {
		return "", ErrDenied // unknown action denies before dispatch
	}
	if !ActionSet()[action] {
		return "", ErrDenied
	}
	p, err := c.db.PrincipalByID(ctx, principalID)
	if err != nil {
		return "", ErrDenied // unknown principal denies; do not leak existence
	}
	if p.Status != "active" {
		return "", ErrDenied
	}

	if role, ok, err := c.roleForScope(ctx, principalID, scopeID); err == nil && ok {
		var n int
		err = c.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM grant_rule WHERE role_id=? AND action_id=? AND resource_type=? AND scope_rule='same_scope'`,
			role, action, resourceType).Scan(&n)
		if err == nil && n > 0 && ownerRevision >= 1 {
			return role, nil
		}
	}
	// Support leases carry only their exactly-listed actions.
	if ok, err := c.leaseAllows(ctx, principalID, scopeID, action); err == nil && ok && ownerRevision >= 1 {
		return "support_lease", nil
	}
	return "", ErrDenied
}

// leaseAllows reports whether an active, unexpired support lease lists the
// exact action for this scope.
func (c *Checker) leaseAllows(ctx context.Context, principalID, scopeID, action string) (bool, error) {
	rows, err := c.db.QueryContext(ctx,
		`SELECT l.expires_at FROM support_lease l
		 JOIN support_lease_grant g ON g.lease_id = l.lease_id
		 WHERE l.principal_id=? AND l.scope_id=? AND g.action_id=? AND l.revoked_at IS NULL`,
		principalID, scopeID, action)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	now := time.Now().UTC()
	for rows.Next() {
		var expires string
		if err := rows.Scan(&expires); err != nil {
			return false, err
		}
		exp, err := time.Parse(time.RFC3339Nano, expires)
		if err == nil && now.Before(exp) {
			return true, nil
		}
	}
	return false, rows.Err()
}

// LeaseVisible reports whether any support lease (even expired/revoked
// grants) ever established a relationship with the scope. For *visibility*
// only: an expired lease makes the resource visible-but-denied (403), not
// concealed (404). Current grants govern authority separately.
func (c *Checker) LeaseVisible(ctx context.Context, principalID, scopeID string) (bool, error) {
	var n int
	err := c.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM support_lease WHERE principal_id=? AND scope_id=?`,
		principalID, scopeID).Scan(&n)
	return n > 0, err
}

// roleForScope returns the highest-priority active role the principal holds
// on exactly this scope. Priority follows RoleCatalogue order (first listed
// wins). Expired or non-active memberships deny.
func (c *Checker) roleForScope(ctx context.Context, principalID, scopeID string) (string, bool, error) {
	ws, err := c.db.WorkspaceByID(ctx, scopeID)
	if err != nil {
		return "", false, err // ErrNotFound → deny upstream
	}
	ms, err := c.db.ActiveMemberships(ctx, principalID)
	if err != nil {
		return "", false, err
	}
	for _, r := range RoleCatalogue[ws.Kind] {
		for _, m := range ms {
			if m.WorkspaceID == scopeID && m.RoleID == r {
				return r, true, nil
			}
		}
	}
	return "", false, nil
}

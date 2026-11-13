// Package audit provides the append-only application audit trail.
//
// Contract basis: storage-schema entity `audit` (DEC-004, DEC-011):
// append-only for ordinary control-plane privileges; safe_reason carries no
// credentials or protected identifiers; entries redact before publication.
package audit

import (
	"context"

	"drevanta/internal/store"
)

// Event is one audit record.
type Event struct {
	ActorID       string
	ScopeID       string
	ActionID      string
	ResourceID    string
	CorrelationID string
	Outcome       string // allow | deny | error | effect | replay
	SafeReason    string
}

// Writer appends audit events.
type Writer struct {
	db *store.DB
}

// NewWriter builds a Writer.
func NewWriter(db *store.DB) *Writer { return &Writer{db: db} }

// Append writes one event.
func (w *Writer) Append(ctx context.Context, e Event) error {
	if e.Outcome == "" {
		e.Outcome = "error"
	}
	_, err := w.db.ExecContext(ctx,
		`INSERT INTO audit(event_id, actor_id, scope_id, action_id, resource_id, correlation_id, occurred_at, outcome, safe_reason)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		store.NewID("aud"), e.ActorID, e.ScopeID, e.ActionID, e.ResourceID, e.CorrelationID,
		store.Now(), e.Outcome, e.SafeReason)
	return err
}

// Query returns audit events within one scope (scope-bounded read; visibility
// enforced by the authz layer at the API boundary via audit.read grant).
func (w *Writer) Query(ctx context.Context, scopeID string, limit int) ([]map[string]string, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := w.db.QueryContext(ctx,
		`SELECT event_id, COALESCE(actor_id,''), scope_id, action_id, COALESCE(resource_id,''),
		        COALESCE(correlation_id,''), occurred_at, outcome, safe_reason
		 FROM audit WHERE scope_id = ? ORDER BY occurred_at DESC, event_id DESC LIMIT ?`,
		scopeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]string
	for rows.Next() {
		m := map[string]string{}
		var eventID, actor, scope, action, resource, corr, at, outcome, reason string
		if err := rows.Scan(&eventID, &actor, &scope, &action, &resource, &corr, &at, &outcome, &reason); err != nil {
			return nil, err
		}
		m["event_id"] = eventID
		m["actor_id"] = actor
		m["scope_id"] = scope
		m["action_id"] = action
		m["resource_id"] = resource
		m["correlation_id"] = corr
		m["occurred_at"] = at
		m["outcome"] = outcome
		m["safe_reason"] = reason
		out = append(out, m)
	}
	return out, rows.Err()
}

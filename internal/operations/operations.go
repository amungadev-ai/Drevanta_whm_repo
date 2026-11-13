// Package operations implements the mutation intent pipeline.
//
// Contract basis: profile.md §API envelope and §Persistence, DEC-009/DEC-012.
// Order: authenticate → visibility/permission → idempotency replay check →
// body/precondition → atomic {intent, operation, outbox} commit → 202.
// Replay semantics per profile.md reference fixtures:
//   - same actor+scope+action+key, same digest  → original operation (stable replay)
//   - same namespace, changed digest            → 409 conflict (altered replay)
//   - grant revoked before replay               → denial, no operation ID (revoked replay)
package operations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"drevanta/internal/store"
)

// Intent is the canonical mutation intent. The wire form rejects unknown
// fields; the canonical digest is computed over the normalized struct
// (fixed field order, compact separators, sorted argument keys).
type Intent struct {
	ActionID              string         `json:"action_id"`
	ResourceType          string         `json:"resource_type"`
	ExpectedOwnerRevision int64          `json:"expected_owner_revision"`
	ExpectedGeneration    int64          `json:"expected_generation"`
	Arguments             map[string]any `json:"arguments,omitempty"`
}

// CanonicalDigest returns the deterministic SHA-256 of the intent.
// Map keys are marshaled sorted by encoding/json; numbers normalize via
// canonical re-marshal of a fixed struct.
func CanonicalDigest(i Intent) (string, error) {
	// Normalize arguments: round-trip through JSON to pin key order and types.
	if i.Arguments != nil {
		keys := make([]string, 0, len(i.Arguments))
		for k := range i.Arguments {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		norm := make(map[string]json.RawMessage, len(i.Arguments))
		for _, k := range keys {
			raw, err := json.Marshal(i.Arguments[k])
			if err != nil {
				return "", err
			}
			norm[k] = raw
		}
		raw, err := json.Marshal(norm)
		if err != nil {
			return "", err
		}
		i.Arguments = map[string]any{"__canonical": json.RawMessage(raw)}
	}
	b, err := json.Marshal(struct {
		ActionID              string         `json:"action_id"`
		ResourceType          string         `json:"resource_type"`
		ExpectedOwnerRevision int64          `json:"expected_owner_revision"`
		ExpectedGeneration    int64          `json:"expected_generation"`
		Arguments             map[string]any `json:"arguments,omitempty"`
	}{i.ActionID, i.ResourceType, i.ExpectedOwnerRevision, i.ExpectedGeneration, i.Arguments})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// Errors surfaced to the API layer (mapped to the status taxonomy there).
var (
	ErrConflict      = errors.New("conflict")           // 409
	ErrStale         = errors.New("stale_precondition") // 412
	ErrNotFound      = store.ErrNotFound                // 404
	ErrInvalidIntent = errors.New("invalid_input")      // 422
)

// Operation is the durable record of an accepted intent.
type Operation struct {
	OperationID        string
	ActorID            string
	ScopeID            string
	ResourceID         string
	ActionID           string
	State              string
	CorrelationID      string
	Digest             string
	Payload            string
	ResultJSON         string
	CreatedAt          string
	UpdatedAt          string
	ExpectedRevision   int64
	ExpectedGeneration int64
}

// Accepted is the 202 response body.
type Accepted struct {
	OperationID        string `json:"operation_id"`
	CorrelationID      string `json:"correlation_id"`
	State              string `json:"state"`
	OperationStatusURL string `json:"operation_status_url"`
	Replayed           bool   `json:"replayed"`
}

// Engine executes intent acceptance on top of the store.
type Engine struct {
	db *store.DB
}

// NewEngine builds an Engine.
func NewEngine(db *store.DB) *Engine { return &Engine{db: db} }

// Accept enforces the full envelope for one intent. Caller must have already
// authenticated the actor and authorized (action, resourceType, scope,
// ownerRevision) — revoked-replay denial happens at that layer before this.
func (e *Engine) Accept(ctx context.Context, actorID string, resource *store.Resource, intent Intent, idempotencyKey string) (*Accepted, error) {
	if intent.ActionID == "" || intent.ResourceType == "" {
		return nil, ErrInvalidIntent
	}
	if intent.ResourceType != resource.Type {
		return nil, ErrInvalidIntent
	}
	digest, err := CanonicalDigest(intent)
	if err != nil {
		return nil, ErrInvalidIntent
	}

	// Idempotency replay check (namespace actor+scope+action+key).
	if idempotencyKey != "" {
		var (
			storedDigest string
			opID         string
		)
		err := e.db.QueryRowContext(ctx,
			`SELECT intent_digest, operation_id FROM idempotency
			 WHERE actor_id=? AND scope_id=? AND action_id=? AND key=?`,
			actorID, resource.ScopeID, intent.ActionID, idempotencyKey).Scan(&storedDigest, &opID)
		if err == nil {
			if storedDigest != digest {
				return nil, ErrConflict // altered replay
			}
			// Stable replay: return the original operation if still readable.
			op, err := e.Get(ctx, opID)
			if err != nil {
				return nil, err
			}
			return &Accepted{
				OperationID:        op.OperationID,
				CorrelationID:      op.CorrelationID,
				State:              op.State,
				OperationStatusURL: "/api/v1/operations/" + op.OperationID,
				Replayed:           true,
			}, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}

	// Precondition: stale desired generation check (412 before acceptance).
	if intent.ExpectedGeneration != resource.DesiredGeneration {
		return nil, ErrStale
	}
	// Owner revision staleness check (412).
	if intent.ExpectedOwnerRevision != resource.OwnerRevision {
		return nil, ErrStale
	}

	payload, err := json.Marshal(intent)
	if err != nil {
		return nil, err
	}

	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	opID := store.NewID("op")
	corr := store.NewID("cor")
	now := store.Now()
	_, err = tx.ExecContext(ctx,
		`INSERT INTO operation(operation_id, actor_id, scope_id, resource_id, action_id, state,
		   expected_revision, expected_generation, intent_digest, payload, correlation_id, created_at, updated_at)
		 VALUES(?,?,?,?,?,'accepted',?,?,?,?,?,?,?)`,
		opID, actorID, resource.ScopeID, resource.ResourceID, intent.ActionID,
		intent.ExpectedOwnerRevision, intent.ExpectedGeneration, digest, string(payload), corr, now, now)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO outbox(event_id, operation_id, scope_id, type, occurred_at)
		 VALUES(?,?,?,?,?)`,
		store.NewID("evt"), opID, resource.ScopeID, "operation.accepted", now); err != nil {
		return nil, err
	}
	if idempotencyKey != "" {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO idempotency(actor_id, scope_id, action_id, key, intent_digest, operation_id, expires_at)
			 VALUES(?,?,?,?,?,?,?)`,
			actorID, resource.ScopeID, intent.ActionID, idempotencyKey, digest, opID,
			time.Now().UTC().Add(24*time.Hour).Format(time.RFC3339Nano))
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &Accepted{
		OperationID:        opID,
		CorrelationID:      corr,
		State:              "accepted",
		OperationStatusURL: "/api/v1/operations/" + opID,
		Replayed:           false,
	}, nil
}

// Get returns an operation (visibility enforced by callers).
func (e *Engine) Get(ctx context.Context, opID string) (*Operation, error) {
	o := &Operation{}
	err := e.db.QueryRowContext(ctx,
		`SELECT operation_id, actor_id, scope_id, resource_id, action_id, state,
		        expected_revision, expected_generation, intent_digest, payload,
		        COALESCE(result_json,''), correlation_id, created_at, updated_at
		 FROM operation WHERE operation_id = ?`, opID).
		Scan(&o.OperationID, &o.ActorID, &o.ScopeID, &o.ResourceID, &o.ActionID, &o.State,
			&o.ExpectedRevision, &o.ExpectedGeneration, &o.Digest, &o.Payload, &o.ResultJSON,
			&o.CorrelationID, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// Transition performs a compare-and-set state change (terminal or otherwise).
// Returns ErrConflict when the current state does not match.
func (e *Engine) Transition(ctx context.Context, opID, from, to string, resultJSON *string) error {
	var res any
	if resultJSON != nil {
		res = *resultJSON
	} else {
		res = sql.NullString{}
	}
	r, err := e.db.ExecContext(ctx,
		`UPDATE operation SET state=?, result_json=COALESCE(?, result_json), updated_at=?
		 WHERE operation_id=? AND state=?`,
		to, res, store.Now(), opID, from)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrConflict
	}
	return nil
}

// DispatchGuard rechecks authorization immediately before dispatch (phase 18:
// "recheck delayed work before dispatch"; a revoked actor's queued operation
// never reaches the agent). It moves an accepted/scheduled operation to
// dispatched and records the fenced attempt. The reauthorize callback must
// perform the full predicate (membership/grant/revision) against CURRENT
// state; a denial cancels the operation as an observed postcondition.
func (e *Engine) DispatchGuard(ctx context.Context, opID string, leaseUntil time.Time, reauthorize func(ctx context.Context, op *Operation) error) (int64, error) {
	op, err := e.Get(ctx, opID)
	if err != nil {
		return 0, err
	}
	// 'dispatched' is allowed: re-dispatch after lease expiry is the crash
	// reconciliation path (fence increments, effect idempotence required).
	if op.State != "accepted" && op.State != "scheduled" && op.State != "dispatched" {
		return 0, ErrConflict
	}
	if err := reauthorize(ctx, op); err != nil {
		_ = e.Transition(ctx, opID, op.State, "cancelled", nil)
		return 0, err
	}
	if err := e.Transition(ctx, opID, op.State, "dispatched", nil); err != nil {
		return 0, err
	}
	return e.FenceAttempt(ctx, opID, leaseUntil)
}

// FenceAttempt records a dispatch attempt with a monotonically increasing
// fence; stale fences (lower than max existing) are rejected before effects.
func (e *Engine) FenceAttempt(ctx context.Context, opID string, leaseUntil time.Time) (int64, error) {
	var maxFence int64
	err := e.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(fence),0) FROM operation_attempt WHERE operation_id=?`, opID).Scan(&maxFence)
	if err != nil {
		return 0, err
	}
	fence := maxFence + 1
	_, err = e.db.ExecContext(ctx,
		`INSERT INTO operation_attempt(operation_id, fence, lease_until, result) VALUES(?,?,?,'pending')`,
		opID, fence, leaseUntil.Format(time.RFC3339Nano))
	if err != nil {
		return 0, err
	}
	return fence, nil
}

// FormatErr renders an operations error for logging with correlation context.
func FormatErr(opID string, err error) string {
	return fmt.Sprintf("operation=%s err=%v", opID, err)
}

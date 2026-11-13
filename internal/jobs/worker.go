// Package jobs implements the durable job worker (phase 22, DEC-009).
//
// Contract basis: profile.md §Persistence/§VPS protocol — durable intent and
// transactional outbox; leases and monotonic fences separate stale-worker
// risks; observation before replay; ambiguity records uncertain state and
// blocks unsafe retry. Grants are rechecked at dispatch (phase 18).
package jobs

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"drevanta/internal/audit"
	"drevanta/internal/operations"
	"drevanta/internal/store"
)

// Executor performs the host/domain effect of one operation attempt. It MUST
// be idempotent per (operation, digest): a retried attempt that finds prior
// completion reports the journaled observation instead of re-effecting.
type Executor interface {
	Execute(ctx context.Context, op *operations.Operation, fence int64) (outcome string, resultJSON string, observedGen *int64, err error)
}

// Reauthorizer runs the full authorization predicate against CURRENT state
// immediately before dispatch (phase 18).
type Reauthorizer func(ctx context.Context, op *operations.Operation) error

// Worker claims, dispatches and reconciles operations.
type Worker struct {
	DB       *store.DB
	Ops      *operations.Engine
	Executor Executor
	Reauth   Reauthorizer
	LeaseTTL time.Duration
	Audit    *audit.Writer
	NowFn    func() time.Time
}

// NewWorker builds a Worker.
func NewWorker(db *store.DB, eng *operations.Engine, ex Executor, reauth Reauthorizer, aw *audit.Writer, leaseTTL time.Duration) *Worker {
	return &Worker{DB: db, Ops: eng, Executor: ex, Reauth: reauth, LeaseTTL: leaseTTL, Audit: aw, NowFn: time.Now}
}

var ErrDispatchDenied = errors.New("dispatch_denied")

// RunOnce performs one claim-dispatch-observe cycle over all due work.
// Returns the number of operations processed.
func (w *Worker) RunOnce(ctx context.Context) (int, error) {
	now := w.NowFn().UTC()
	nowS := now.Format(time.RFC3339Nano)

	// 1) Fresh work: accepted/scheduled operations without an active lease.
	// 2) Crash reconciliation: dispatched operations whose lease expired
	//    without terminal evidence (phase 22: worker crash reconciliation).
	rows, err := w.DB.QueryContext(ctx, `
		SELECT o.operation_id FROM operation o
		WHERE o.state IN ('accepted','scheduled')
		   OR (o.state = 'dispatched' AND EXISTS (
		        SELECT 1 FROM operation_attempt a
		        WHERE a.operation_id = o.operation_id
		          AND a.fence = (SELECT MAX(fence) FROM operation_attempt WHERE operation_id = o.operation_id)
		          AND (a.lease_until IS NULL OR a.lease_until < ?)
		   ))
		ORDER BY o.created_at, o.operation_id`, nowS)
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	processed := 0
	for _, id := range ids {
		if err := w.processOne(ctx, id, now); err != nil {
			// Denials cancel the operation inside DispatchGuard; execution
			// ambiguity records 'uncertain'. Neither aborts the batch.
			if !errors.Is(err, ErrDispatchDenied) && !errors.Is(err, operations.ErrConflict) {
				return processed, err
			}
		}
		processed++
	}
	return processed, nil
}

func (w *Worker) processOne(ctx context.Context, opID string, now time.Time) error {
	leaseUntil := now.Add(w.LeaseTTL)
	// DispatchGuard reauthorizes and fences the attempt; cancelled operations
	// never reach the executor.
	fence, err := w.Ops.DispatchGuard(ctx, opID, leaseUntil, w.Reauth)
	if err != nil {
		if w.Audit != nil {
			_ = w.Audit.Append(ctx, audit.Event{
				ScopeID: "worker", ActionID: "operation.retry",
				ResourceID: opID, Outcome: "deny", SafeReason: "dispatch recheck denied",
			})
		}
		return ErrDispatchDenied
	}

	op, err := w.Ops.Get(ctx, opID)
	if err != nil {
		return err
	}
	outcome, resultJSON, observedGen, execErr := w.Executor.Execute(ctx, op, fence)

	switch {
	case execErr == nil && outcome == "succeeded":
		if termErr := w.Ops.Transition(ctx, opID, "dispatched", "succeeded", &resultJSON); termErr != nil {
			// Lost the CAS race (another worker finished first): accept the
			// journaled outcome; duplicate terminal states are impossible by CAS.
			if errors.Is(termErr, operations.ErrConflict) {
				return nil
			}
			return termErr
		}
		return w.recordTerminal(ctx, op, "succeeded", observedGen)
	case execErr == nil && outcome == "failed":
		if err := w.Ops.Transition(ctx, opID, "dispatched", "failed", &resultJSON); err != nil {
			if errors.Is(err, operations.ErrConflict) {
				return nil
			}
			return err
		}
		return w.recordTerminal(ctx, op, "failed", nil)
	default:
		// Ambiguity (executor error or unknown postcondition): record
		// uncertain; automated retry is blocked pending observation.
		result := `{"safe_reason":"effect outcome unobserved"}`
		if resultJSON != "" {
			result = resultJSON
		}
		if err := w.Ops.Transition(ctx, opID, "dispatched", "uncertain", &result); err != nil {
			if errors.Is(err, operations.ErrConflict) {
				return nil
			}
			return err
		}
		return w.recordTerminal(ctx, op, "uncertain", nil)
	}
}

// recordTerminal appends the terminal outbox event and advances the resource
// observed generation when the effect was verified.
func (w *Worker) recordTerminal(ctx context.Context, op *operations.Operation, outcome string, observedGen *int64) error {
	if _, err := w.DB.ExecContext(ctx,
		`INSERT INTO outbox(event_id, operation_id, scope_id, type, occurred_at)
		 VALUES(?,?,?,?,?)`,
		store.NewID("evt"), op.OperationID, op.ScopeID, "operation."+outcome, store.Now()); err != nil {
		return err
	}
	if observedGen != nil {
		_, err := w.DB.ExecContext(ctx,
			`UPDATE resource SET observed_generation=? WHERE resource_id=? AND observed_generation < ?`,
			*observedGen, op.ResourceID, *observedGen)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	return nil
}

// ReconcileExpired re-queues dispatched operations whose lease expired —
// called at worker startup (phase 22: checkpoints survive restart). Claiming
// happens naturally on the next RunOnce; this helper exists for observability.
func (w *Worker) ReconcileExpired(ctx context.Context) (int, error) {
	now := w.NowFn().UTC().Format(time.RFC3339Nano)
	var n int
	err := w.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM operation o
		WHERE o.state = 'dispatched' AND EXISTS (
			SELECT 1 FROM operation_attempt a
			WHERE a.operation_id = o.operation_id
			  AND a.fence = (SELECT MAX(fence) FROM operation_attempt WHERE operation_id = o.operation_id)
			  AND (a.lease_until IS NULL OR a.lease_until < ?)
		)`, now).Scan(&n)
	return n, err
}

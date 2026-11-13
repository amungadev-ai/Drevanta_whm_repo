// Package jobs: API keys (phase 23 INT-01) and schedules (WEB-08 core).
package jobs

import (
	"context"
	cryptoRand "crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"drevanta/internal/operations"
	"drevanta/internal/store"
)

// ---- API keys ----

// IssueAPIKey creates a key bound to one principal+scope; the raw key is
// returned once and only its hash is stored.
func IssueAPIKey(ctx context.Context, db *store.DB, principalID, scopeID, name string, expiresAt *time.Time) (string, error) {
	var b [32]byte
	if err := randFill(b[:]); err != nil {
		return "", err
	}
	raw := "dak_" + hex.EncodeToString(b[:])
	var exp any
	if expiresAt != nil {
		exp = expiresAt.UTC().Format(time.RFC3339Nano)
	} else {
		exp = sql.NullString{}
	}
	_, err := db.ExecContext(ctx,
		`INSERT INTO api_key(api_key_id, principal_id, scope_id, token_hash, name, created_at, expires_at)
		 VALUES(?,?,?,?,?,?,?)`,
		store.NewID("key"), principalID, scopeID, hashKey(raw), name, store.Now(), exp)
	if err != nil {
		return "", err
	}
	return raw, nil
}

// ResolveAPIKey returns (principalID, scopeID) for a raw key, enforcing
// revocation and expiry. Uniform deny on any failure.
func ResolveAPIKey(ctx context.Context, db *store.DB, raw string) (string, string, error) {
	var principalID, scopeID string
	var expires *string
	var revoked *string
	err := db.QueryRowContext(ctx,
		`SELECT principal_id, scope_id, expires_at, revoked_at FROM api_key WHERE token_hash = ?`,
		hashKey(raw)).Scan(&principalID, &scopeID, &expires, &revoked)
	if err != nil {
		return "", "", store.ErrNotFound
	}
	if revoked != nil {
		return "", "", store.ErrNotFound
	}
	if expires != nil {
		expT, err := time.Parse(time.RFC3339Nano, *expires)
		if err != nil || time.Now().UTC().After(expT) {
			return "", "", store.ErrNotFound
		}
	}
	_, _ = db.ExecContext(ctx, `UPDATE api_key SET last_used_at=? WHERE token_hash=?`, store.Now(), hashKey(raw))
	return principalID, scopeID, nil
}

// RevokeAPIKey revokes one key owned by the principal.
func RevokeAPIKey(ctx context.Context, db *store.DB, principalID, keyID string) error {
	res, err := db.ExecContext(ctx,
		`UPDATE api_key SET revoked_at=? WHERE api_key_id=? AND principal_id=? AND revoked_at IS NULL`,
		store.Now(), keyID, principalID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

// ---- Schedules (WEB-08 core: interval schedules; cron expressions land
// with the WEB-08 contract freeze) ----

// ArmSchedule creates an armed interval schedule; the first run is due after
// one interval.
func ArmSchedule(ctx context.Context, db *store.DB, scopeID, principalID, resourceID, actionID string, everySeconds int, args map[string]any) (string, error) {
	if everySeconds < 1 {
		return "", errors.New("interval must be >= 1s")
	}
	argsJSON := "{}"
	if args != nil {
		if b, err := json.Marshal(args); err == nil {
			argsJSON = string(b)
		}
	}
	id := store.NewID("sch")
	next := time.Now().UTC().Add(time.Duration(everySeconds) * time.Second).Format(time.RFC3339Nano)
	_, err := db.ExecContext(ctx,
		`INSERT INTO schedule(schedule_id, scope_id, principal_id, resource_id, action_id, every_seconds, args_json, state, next_run_at, created_at)
		 VALUES(?,?,?,?,?,?,?,'armed',?,?)`,
		id, scopeID, principalID, resourceID, actionID, everySeconds, argsJSON, next, store.Now())
	if err != nil {
		return "", err
	}
	return id, nil
}

// PauseSchedule disarms a schedule.
func PauseSchedule(ctx context.Context, db *store.DB, scheduleID string) error {
	_, err := db.ExecContext(ctx, `UPDATE schedule SET state='paused' WHERE schedule_id=?`, scheduleID)
	return err
}

// TickDueSchedules fires all due schedules exactly once each, creating one
// accepted operation per schedule (executed as the schedule's owning
// principal). The schedule row is advanced before the operation is accepted
// so missed ticks coalesce and never double-fire.
func TickDueSchedules(ctx context.Context, db *store.DB, eng *operations.Engine) ([]string, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	rows, err := db.QueryContext(ctx, `
		SELECT schedule_id, scope_id, principal_id, resource_id, action_id, every_seconds, args_json
		FROM schedule WHERE state='armed' AND next_run_at <= ?`, now)
	if err != nil {
		return nil, err
	}
	type due struct {
		id, scope, principal, resource, action, args string
		every                                        int
	}
	var dueList []due
	for rows.Next() {
		var d due
		if err := rows.Scan(&d.id, &d.scope, &d.principal, &d.resource, &d.action, &d.every, &d.args); err != nil {
			rows.Close()
			return nil, err
		}
		dueList = append(dueList, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var opIDs []string
	for _, d := range dueList {
		next := time.Now().UTC().Add(time.Duration(d.every) * time.Second).Format(time.RFC3339Nano)
		res, err := db.ExecContext(ctx,
			`UPDATE schedule SET next_run_at=?, last_run_at=? WHERE schedule_id=? AND state='armed' AND next_run_at <= ?`,
			next, store.Now(), d.id, now)
		if err != nil {
			return opIDs, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			continue // raced with pause or another tick
		}
		res2, err := db.ResourceByID(ctx, d.resource)
		if err != nil {
			continue // resource vanished; tick skipped, schedule remains armed
		}
		var args map[string]any
		_ = json.Unmarshal([]byte(d.args), &args)
		intent := operations.Intent{
			ActionID:              d.action,
			ResourceType:          res2.Type,
			ExpectedOwnerRevision: res2.OwnerRevision,
			ExpectedGeneration:    res2.DesiredGeneration,
			Arguments:             args,
		}
		acc, err := eng.Accept(ctx, d.principal, res2, intent, "")
		if err != nil {
			continue // stale generation etc.; the schedule tick itself is recorded
		}
		opIDs = append(opIDs, acc.OperationID)
	}
	return opIDs, nil
}

func hashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func randFill(b []byte) error {
	_, err := cryptoRand.Read(b)
	return err
}

package helper

// SQLite-backed host journal (DEC-003: host journals persist accepted
// attempts; replays return journaled observations).

import (
	"database/sql"
	"encoding/json"
	"errors"

	"drevanta/internal/store"
)

// SQLiteJournal implements JournalStore over the helper database.
type SQLiteJournal struct {
	DB *store.DB
}

// Append records one attempt.
func (j *SQLiteJournal) Append(hostID, operationID, digest string, fence int64, verb, outcome string, result json.RawMessage) (string, error) {
	id := store.NewID("hj")
	res := "{}"
	if result != nil {
		res = string(result)
	}
	_, err := j.DB.Exec(`INSERT INTO host_journal(journal_id, host_id, operation_id, digest, fence, verb, outcome, result_json, occurred_at)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		id, hostID, operationID, digest, fence, verb, outcome, res, store.Now())
	if err != nil {
		return "", err
	}
	return id, nil
}

// Lookup returns the journaled outcome for an operation+digest pair.
func (j *SQLiteJournal) Lookup(operationID, digest string) (string, string, string, int64, error) {
	var outcome, result, jid string
	var fence int64
	err := j.DB.QueryRow(
		`SELECT outcome, COALESCE(result_json,'{}'), journal_id, fence FROM host_journal
		 WHERE operation_id=? AND digest=? ORDER BY fence DESC LIMIT 1`,
		operationID, digest).Scan(&outcome, &result, &jid, &fence)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", "", 0, ErrNotFound
	}
	if err != nil {
		return "", "", "", 0, err
	}
	return outcome, result, jid, fence, nil
}

// MaxFence returns the highest recorded fence for the operation (0 if none).
func (j *SQLiteJournal) MaxFence(operationID string) (int64, error) {
	var max int64
	err := j.DB.QueryRow(
		`SELECT COALESCE(MAX(fence),0) FROM host_journal WHERE operation_id=?`,
		operationID).Scan(&max)
	return max, err
}

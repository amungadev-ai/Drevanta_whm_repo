// Package store provides the SQLite-backed persistence layer.
//
// Contract basis: spec/specification/contracts/profile.md §Persistence and
// spec/specification/registers/storage-schema.csv. Opaque primary IDs, UTC
// timestamps, authoritative owner references, monotonic generations/revisions.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when a row lookup fails.
var ErrNotFound = errors.New("not found")

// DB wraps the sql.DB handle with SQLite-specific pragmas applied.
type DB struct {
	*sql.DB
}

// Open opens (creating if necessary) the SQLite database at path with WAL
// durability settings per DEC-002 (WAL, FULL synchronous).
func Open(path string) (*DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite WAL: one writer; serialize writes through a single connection,
	// keeping reads fast and avoiding SQLITE_BUSY under concurrency.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return &DB{db}, nil
}

// Migrate applies all pending forward-only migrations (*.sql in dir, lexical
// order) exactly once, each within its own transaction.
func (d *DB) Migrate(ctx context.Context, dir string) error {
	if _, err := d.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migration (
		name TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("create schema_migration: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	applied := map[string]bool{}
	rows, err := d.QueryContext(ctx, `SELECT name FROM schema_migration`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return err
		}
		applied[n] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, name := range names {
		if applied[name] {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		tx, err := d.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migration(name, applied_at) VALUES(?,?)`,
			name, Now()); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// NewID returns an opaque prefixed identifier (crypto/rand, 128 bits).
func NewID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}

// Now returns the current UTC time in RFC3339Nano (our canonical TEXT format).
func Now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// ParseTime parses a stored timestamp.
func ParseTime(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }

// ValidID reports whether s is a well-formed opaque ID with the given prefix.
func ValidID(prefix, s string) bool {
	if !strings.HasPrefix(s, prefix+"_") {
		return false
	}
	rest := s[len(prefix)+1:]
	if len(rest) != 32 {
		return false
	}
	_, err := hex.DecodeString(rest)
	return err == nil
}

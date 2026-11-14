package install

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"drevanta/internal/authn"
	"drevanta/internal/store"
)

func newTestInstaller(t *testing.T) (*Installer, *store.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "install.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(context.Background(), "../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewInstaller(db, authn.NewService(db, 24*time.Hour, 30*time.Minute)), db
}

// Regeneration replaces the pending token (hash-only storage; the 24h window
// restarts), invalidates the previous token, and is refused once the
// installation has been claimed — the owner claim is final.
func TestRegenerateClaimToken(t *testing.T) {
	in, db := newTestInstaller(t)
	ctx := context.Background()
	_, raw1, err := in.EnsureInstallation(ctx)
	if err != nil || raw1 == "" {
		t.Fatalf("ensure: %v", err)
	}
	raw2, err := in.RegenerateClaimToken(ctx)
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if raw2 == raw1 {
		t.Fatal("regenerated token must differ from the previous token")
	}
	var hash, created string
	if err := db.QueryRowContext(ctx,
		`SELECT claim_token_hash, created_at FROM installation LIMIT 1`).Scan(&hash, &created); err != nil {
		t.Fatalf("read installation: %v", err)
	}
	sum := sha256.Sum256([]byte(raw2))
	if hash != hex.EncodeToString(sum[:]) {
		t.Fatal("stored hash must match the regenerated token (plaintext never stored)")
	}
	created1, err1 := time.Parse(time.RFC3339Nano, created)
	if err1 != nil || time.Since(created1) > time.Minute {
		t.Fatal("regeneration must restart the validity window")
	}
	// The previous token is invalid after regeneration.
	if _, err := in.Claim(ctx, raw1, "owner@example.test", "AdminPassphrase-1!", "Owner"); !errors.Is(err, ErrBadClaimToken) {
		t.Fatalf("old token must be rejected after regeneration, got %v", err)
	}
	// The fresh token claims atomically and creates the owner.
	res, err := in.Claim(ctx, raw2, "owner@example.test", "AdminPassphrase-1!", "Owner")
	if err != nil {
		t.Fatalf("claim with regenerated token: %v", err)
	}
	if res.InstallationID == "" || res.PrincipalID == "" {
		t.Fatal("claim result missing ids")
	}
	// A claimed installation refuses regeneration.
	if _, err := in.RegenerateClaimToken(ctx); !errors.Is(err, ErrNotPending) {
		t.Fatalf("claimed installation must refuse regeneration, got %v", err)
	}
}

// Concurrent claim attempts must be serialized: exactly one wins, every
// other attempt is rejected (atomic consume, replay-safe).
func TestConcurrentClaimAttempts(t *testing.T) {
	in, _ := newTestInstaller(t)
	ctx := context.Background()
	_, raw, err := in.EnsureInstallation(ctx)
	if err != nil || raw == "" {
		t.Fatalf("ensure: %v", err)
	}
	const racers = 4
	type outcome struct {
		ok  bool
		err error
	}
	results := make(chan outcome, racers)
	for i := 0; i < racers; i++ {
		go func(n int) {
			res, err := in.Claim(ctx, raw, "owner@example.test", "AdminPassphrase-1!", "Owner")
			results <- outcome{ok: err == nil && res != nil, err: err}
		}(i)
	}
	wins := 0
	for i := 0; i < racers; i++ {
		r := <-results
		if r.ok {
			wins++
		} else if r.err == nil {
			t.Fatal("loser returned nil error without a result")
		}
	}
	if wins != 1 {
		t.Fatalf("exactly one concurrent claim must win, got %d (errors: %v)", wins, results)
	}
}

package jobs

// Build runner integration tests (G40-F2): real git execution against a
// file:// repository, success digest stability, failure mapping, and the
// promotion gate opening after a real build succeeds.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"drevanta/internal/domains"
	"drevanta/internal/store"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t.test",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t.test")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main", dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "seed")
}

func TestBuildRunnerRealGitSuccess(t *testing.T) {
	f := newJobsFixture(t)
	ctx := context.Background()
	src := t.TempDir()
	initGitRepo(t, src)

	// file:// URLs are inserted directly: ConnectSource enforces https for
	// user input (product rule), while the runner itself must support local
	// fixtures for real-execution testing.
	p := domains.NewPipeline(f.db)
	connID := seedConnection(t, f.db, "file://"+src)
	buildID, err := p.QueueBuild(ctx, "ws_x", connID, "main")
	if err != nil {
		t.Fatal(err)
	}

	br := NewBuildRunner(f.db, &GitBuildExecutor{Timeout: 30 * time.Second})
	n, err := br.RunOnce(ctx)
	if err != nil || n != 1 {
		t.Fatalf("run once: n=%d err=%v", n, err)
	}
	var state, digest string
	var size int64
	if err := f.db.QueryRow(`SELECT state, artifact_digest, artifact_size FROM build WHERE build_id=?`, buildID).
		Scan(&state, &digest, &size); err != nil {
		t.Fatal(err)
	}
	if state != "succeeded" || len(digest) != len("sha256:")+64 {
		t.Fatalf("build state=%s digest=%q", state, digest)
	}
	if size <= 0 {
		t.Fatalf("artifact size %d", size)
	}

	// Deterministic: a second identical build produces the same digest.
	build2, _ := p.QueueBuild(ctx, "ws_x", connID, "main")
	if _, err := br.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	var d2 string
	_ = f.db.QueryRow(`SELECT artifact_digest FROM build WHERE build_id=?`, build2).Scan(&d2)
	if d2 != digest {
		t.Fatalf("digest not deterministic: %q vs %q", d2, digest)
	}

	// The promotion gate opens only for succeeded builds with digests.
	envID, _ := p.CreateEnvironment(ctx, "ws_x", "production", "production")
	if _, err := p.PromoteRelease(ctx, envID, buildID); err != nil {
		t.Fatalf("promote after real build: %v", err)
	}
}

func TestBuildRunnerRealGitFailure(t *testing.T) {
	f := newJobsFixture(t)
	ctx := context.Background()
	p := domains.NewPipeline(f.db)
	connID := seedConnection(t, f.db, "https://git.invalid/nonexistent.git")
	buildID, _ := p.QueueBuild(ctx, "ws_x", connID, "main")

	br := NewBuildRunner(f.db, &GitBuildExecutor{Timeout: 10 * time.Second})
	if _, err := br.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	var state string
	_ = f.db.QueryRow(`SELECT state FROM build WHERE build_id=?`, buildID).Scan(&state)
	if state != "failed" {
		t.Fatalf("unreachable repo must fail the build, got %s", state)
	}
}

func TestBuildRunnerClaimRacesSafe(t *testing.T) {
	f := newJobsFixture(t)
	ctx := context.Background()
	src := t.TempDir()
	initGitRepo(t, src)
	p := domains.NewPipeline(f.db)
	connID := seedConnection(t, f.db, "file://"+src)
	if _, err := p.QueueBuild(ctx, "ws_x", connID, "main"); err != nil {
		t.Fatal(err)
	}
	br := NewBuildRunner(f.db, &GitBuildExecutor{Timeout: 30 * time.Second})
	// First claims and executes; the second finds nothing queued.
	if n, _ := br.RunOnce(ctx); n != 1 {
		t.Fatalf("first run n=%d", n)
	}
	if n, _ := br.RunOnce(ctx); n != 0 {
		t.Fatalf("second run n=%d (double execution)", n)
	}
}

// seedConnection inserts a source connection row directly (bypassing the
// user-facing https-only validation for local test fixtures).
func seedConnection(t *testing.T, db *store.DB, repoURL string) string {
	t.Helper()
	id := store.NewID("src")
	if _, err := db.Exec(`INSERT INTO source_connection(connection_id, scope_id, principal_id, repo_url, default_branch, state, created_at)
		VALUES(?,?,?,?,?, 'connected', ?)`, id, "ws_x", "prn_x", repoURL, "main", store.Now()); err != nil {
		t.Fatal(err)
	}
	return id
}

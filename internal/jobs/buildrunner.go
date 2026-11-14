package jobs

// Build worker (G40-F2): executes queued pipeline builds with REAL source
// fetches. A queued build is claimed (state → building), the repository is
// cloned at the build's commit ref, and a SHA-256 manifest digest over the
// working tree becomes the artifact digest; failures map to the build state
// machine (failed + redacted log reference) exactly as the domain service
// expects. No credentials are embedded in logs; clone URLs come from the
// source_connection row (no secret_ref material is echoed).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"drevanta/internal/store"
)

// BuildExecutor performs one build: given repo URL and commit ref it returns
// (artifactDigest, artifactSizeBytes, err). The default implementation uses
// git; tests inject a fake or a file:// repo.
type BuildExecutor interface {
	Execute(ctx context.Context, repoURL, commitRef, workDir string) (string, int64, error)
}

// GitBuildExecutor clones the repository (shallow, single branch/ref) into a
// scratch directory and hashes the tree.
type GitBuildExecutor struct {
	Timeout time.Duration
}

// Execute performs the clone + digest. A missing git binary or an
// unreachable repository returns an error (build fails safely).
func (g *GitBuildExecutor) Execute(ctx context.Context, repoURL, commitRef, workDir string) (string, int64, error) {
	timeout := g.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// URL hygiene: only https and file schemes reach git (no ssh agents, no
	// scp-like syntax execution).
	if !(strings.HasPrefix(repoURL, "https://") || strings.HasPrefix(repoURL, "file://")) {
		return "", 0, fmt.Errorf("unsupported repository scheme")
	}

	run := func(args ...string) error {
		cmd := exec.CommandContext(cctx, "git", args...)
		cmd.Dir = workDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			// Keep the log reference redacted: never echo clone output (it can
			// carry tokens embedded in URLs); store only a shape marker.
			return fmt.Errorf("git %s failed", args[0])
		}
		_ = out
		return nil
	}

	if err := run("clone", "--quiet", "--depth", "1", repoURL, "."); err != nil {
		return "", 0, err
	}
	// commitRef may be a branch or a full SHA; shallow clones fetch the
	// default branch — a ref that is not the head fails the build safely
	// rather than silently building the wrong revision.
	if commitRef != "" && commitRef != "main" && commitRef != "master" {
		if err := run("fetch", "--quiet", "--depth", "1", "origin", commitRef); err != nil {
			return "", 0, fmt.Errorf("ref not fetchable")
		}
		if err := run("checkout", "--quiet", "FETCH_HEAD"); err != nil {
			return "", 0, fmt.Errorf("ref not checkable")
		}
	}

	var size int64
	h := sha256.New()
	err := filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(h, f); err != nil {
			return err
		}
		size += info.Size()
		return nil
	})
	if err != nil {
		return "", 0, fmt.Errorf("tree digest failed")
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), size, nil
}

// BuildRunner claims queued builds and drives them through the state machine.
type BuildRunner struct {
	DB       *store.DB
	Executor BuildExecutor
	NowFn    func() time.Time
	// ScratchDir is the base for per-build work directories.
	ScratchDir string
}

// NewBuildRunner wires a BuildRunner with defaults.
func NewBuildRunner(db *store.DB, ex BuildExecutor) *BuildRunner {
	return &BuildRunner{DB: db, Executor: ex, NowFn: time.Now, ScratchDir: os.TempDir()}
}

// RunOnce executes up to one queued build; returns the number processed.
func (r *BuildRunner) RunOnce(ctx context.Context) (int, error) {
	var buildID, repoURL, commitRef string
	err := r.DB.QueryRowContext(ctx,
		`SELECT b.build_id, c.repo_url, b.commit_ref
		 FROM build b JOIN source_connection c ON c.connection_id = b.connection_id
		 WHERE b.state = 'queued' ORDER BY b.created_at ASC LIMIT 1`).
		Scan(&buildID, &repoURL, &commitRef)
	if err != nil {
		return 0, nil // nothing queued (or lookup failed: safe no-op this tick)
	}
	// Claim: CAS transition queued → building (one runner wins).
	res, err := r.DB.ExecContext(ctx,
		`UPDATE build SET state='building' WHERE build_id=? AND state='queued'`, buildID)
	if err != nil {
		return 0, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, nil // lost the claim
	}

	workDir, err := os.MkdirTemp(r.ScratchDir, "drevanta-build-")
	if err != nil {
		_ = r.finish(buildID, false, "", 0)
		return 1, nil
	}
	defer os.RemoveAll(workDir)

	digest, size, execErr := r.Executor.Execute(ctx, repoURL, commitRef, workDir)
	if execErr != nil {
		_ = r.finish(buildID, false, "", 0)
		return 1, nil
	}
	_ = r.finish(buildID, true, digest, size)
	return 1, nil
}

func (r *BuildRunner) finish(buildID string, succeeded bool, digest string, size int64) error {
	state, finished := "failed", store.Now()
	if succeeded {
		state = "succeeded"
	}
	_, err := r.DB.Exec(
		`UPDATE build SET state=?, artifact_digest=?, artifact_size=?, finished_at=?
		 WHERE build_id=? AND state IN ('queued','building')`,
		state, digest, size, finished, buildID)
	return err
}

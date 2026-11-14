package helper

// Isolated build execution (G30): developer builds run UNTRUSTED repository
// content in a verified isolation boundary or NOT AT ALL (fail closed).
//
// Two-phase design (network policy):
//   Phase A (privileged, no repo content executes): shallow bare fetch of
//   the source. git clone/checkout never runs hooks from the repo; hooks
//   are disabled and filters neutralized explicitly.
//   Phase B (isolated, no network): checkout + optional build command +
//   artifact digest under systemd-run with CPU/memory/task limits,
//   PrivateNetwork, NoNewPrivileges, non-root uid, minimal allowlisted env.
//
// Isolation is PROBED before every build (systemd-run --scope + true probe
// under the same properties). If the probe fails, the build refuses — the
// platform never falls back to running untrusted content unsandboxed.
// Concurrency is capped by a semaphore; timeouts kill the whole scope.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// BuildExecuteArgs is the typed argument for build.execute.
type BuildExecuteArgs struct {
	BuildID      string `json:"build_id"`
	RepoURL      string `json:"repo_url"`
	CommitRef    string `json:"commit_ref,omitempty"`
	ScratchRoot  string `json:"scratch_root"`            // root-allowlisted
	BuildCommand string `json:"build_command,omitempty"` // platform-configured, never from repo content
	TimeoutS     int    `json:"timeout_s,omitempty"`
}

var buildScratchRoots = map[string]bool{}
var buildSem = make(chan struct{}, 2) // max concurrent isolated builds

// LoadBuildBindings reads the root-owned scratch root bindings.
func LoadBuildBindings(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var dirs []string
	if err := json.Unmarshal(b, &dirs); err != nil {
		return err
	}
	for _, d := range dirs {
		buildScratchRoots[filepath.Clean(d)] = true
	}
	return nil
}

// isolationProbeFn executes a trivial command under the exact sandbox
// properties used for builds. Any failure refuses the build. Variable for
// test seam injection only; production always uses the real probe.
var isolationProbeFn = defaultIsolationProbe

func defaultIsolationProbe() error {
	// Service-mode transient unit (NOT --scope): namespace properties
	// (PrivateNetwork/NoNewPrivileges) are only valid for services.
	cmd := exec.Command("systemd-run", "--quiet", "--wait",
		"-p", "CPUQuota=5%", "-p", "MemoryMax=64M", "-p", "TasksMax=16",
		"-p", "PrivateNetwork=yes", "-p", "NoNewPrivileges=yes",
		"-E", "PATH=/usr/bin:/bin",
		"--uid=nobody", "true")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("isolation probe failed (refusing untrusted build)")
	}
	return nil
}

// sandboxRunner executes argv under the sandbox scope. Variable for test
// seam injection; production wraps systemd-run with the hardening flags.
var sandboxRunner = defaultSandboxRunner

func defaultSandboxRunner(ctx context.Context, argv []string, dir string, env []string) error {
	args := []string{"--quiet", "--wait",
		"-p", "CPUQuota=100%", "-p", "MemoryMax=1G", "-p", "TasksMax=256",
		"-p", "PrivateNetwork=yes", "-p", "NoNewPrivileges=yes",
		"--uid=nobody"}
	if dir != "" {
		args = append(args, "-p", "WorkingDirectory="+dir)
	}
	for _, e := range env {
		args = append(args, "-E", e) // allowlisted env only; never the host env
	}
	args = append(args, "--")
	args = append(args, argv...)
	cmd := exec.CommandContext(ctx, "systemd-run", args...)
	var stderr strings.Builder
	cmd.Stdout, cmd.Stderr = io.Discard, &stderr
	if err := cmd.Run(); err != nil {
		// Diagnosability without leaks: exit code + first stderr line shape.
		line := strings.SplitN(strings.TrimSpace(stderr.String()), "\n", 2)[0]
		if len(line) > 120 {
			line = line[:120]
		}
		return fmt.Errorf("sandboxed step failed (exit %v): %s", cmd.ProcessState.ExitCode(), line)
	}
	return nil
}

// buildEnv is the allowlisted environment for the isolated phase: no host
// secrets, no DREVANTA_* material, no proxy variables.
func buildEnv(scratch string) []string {
	return []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=" + scratch,
		"TMPDIR=" + scratch + "/tmp",
		"GIT_TERMINAL_PROMPT=0",
		"LC_ALL=C",
	}
}

// Sandbox identity: the non-root account the isolated phase runs as.
// nobody/nogroup (65534) exists on every Debian-family host.
const (
	sandboxUID = 65534
	sandboxGID = 65534
)

// chownTree recursively hands a directory tree to the sandbox user.
func chownTree(root string, uid, gid int) error {
	return filepath.Walk(root, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(path, uid, gid)
	})
}

// safeRepoURL allows only https and file schemes (no ssh agents, no scp-like
// remote execution syntax).
func safeRepoURL(u string) error {
	if !(strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "file://")) {
		return fmt.Errorf("unsupported repository scheme")
	}
	if strings.ContainsAny(u, "\x00\n\r") {
		return fmt.Errorf("invalid repository URL")
	}
	return nil
}

// BuildExecuteHandler performs the two-phase isolated build.
func BuildExecuteHandler() VerbHandler {
	return func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		dec := json.NewDecoder(strings.NewReader(string(args)))
		dec.DisallowUnknownFields()
		var a BuildExecuteArgs
		if err := dec.Decode(&a); err != nil {
			return nil, fmt.Errorf("invalid build.execute arguments")
		}
		if a.BuildID == "" || len(a.BuildID) > 80 || strings.ContainsAny(a.BuildID, "/\\\x00 \t") {
			return nil, fmt.Errorf("invalid build id")
		}
		if !buildScratchRoots[filepath.Clean(a.ScratchRoot)] {
			return nil, fmt.Errorf("scratch_root not allowlisted")
		}
		if err := safeRepoURL(a.RepoURL); err != nil {
			return nil, err
		}
		timeout := time.Duration(a.TimeoutS) * time.Second
		if timeout <= 0 {
			timeout = 10 * time.Minute
		}
		cctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		// Fail closed on isolation.
		if err := isolationProbeFn(); err != nil {
			return nil, err
		}

		buildSem <- struct{}{}
		defer func() { <-buildSem }()

		work := filepath.Join(a.ScratchRoot, "build-"+a.BuildID)
		_ = os.RemoveAll(work)
		if err := os.MkdirAll(filepath.Join(work, "tmp"), 0o700); err != nil {
			return nil, fmt.Errorf("scratch creation failed")
		}
		defer os.RemoveAll(work)

		// Phase A: privileged bare fetch (no repo content executes; hooks
		// and filters disabled by config).
		bare := filepath.Join(work, "src.git")
		fetch := exec.CommandContext(cctx, "git", "clone",
			"--quiet", "--depth", "1", "--bare", "--no-checkout",
			"-c", "core.hooksPath=/dev/null",
			"-c", "filter.lfs.smudge=cat", "-c", "filter.lfs.required=false",
			a.RepoURL, bare)
		fetch.Env = buildEnv(work)
		fetch.Stdout, fetch.Stderr = io.Discard, io.Discard
		if err := fetch.Run(); err != nil {
			return nil, fmt.Errorf("source fetch failed")
		}

		// Work tree must exist BEFORE the handover so the chown covers it.
		wt := filepath.Join(work, "tree")
		if err := os.MkdirAll(wt, 0o700); err != nil {
			return nil, fmt.Errorf("work tree creation failed")
		}

		// Hand the work area to the sandbox user: the isolated phase runs as
		// non-root and needs read/write on the bare repo and work tree.
		// (Only the privileged helper can hand over; non-root test runs skip.)
		if os.Geteuid() == 0 {
			if err := chownTree(work, sandboxUID, sandboxGID); err != nil {
				return nil, fmt.Errorf("work area handover failed")
			}
		}

		// Phase B: isolated checkout (+ optional platform build command) via
		// systemd-run scope. The work tree is the bare repo's working copy:
		// use git --git-dir with a work-tree inside the scratch.
		ref := a.CommitRef
		if ref == "" || ref == "main" || ref == "master" {
			ref = "HEAD"
		}
		gitArgs := []string{
			"git", "--git-dir", bare, "--work-tree", wt,
			"-c", "core.hooksPath=/dev/null",
			"-c", "filter.lfs.smudge=cat", "-c", "filter.lfs.required=false",
			"-c", "safe.directory=*",
			"checkout", "--quiet", "--force", ref}
		if err := sandboxRunner(cctx, gitArgs, "", buildEnv(work)); err != nil {
			return nil, fmt.Errorf("ref not checkable: %w", err)
		}

		if a.BuildCommand != "" {
			// The build command is platform-configured (from the pipeline
			// service, never from repository content); it runs under the
			// same scope with shell parsing restricted to the typed command.
			bcArgv := []string{"sh", "-c", a.BuildCommand}
			if err := sandboxRunner(cctx, bcArgv, wt, buildEnv(work)); err != nil {
				return nil, fmt.Errorf("build command failed: %w", err)
			}
		}

		// Artifact: sorted file-content digest over the built tree.
		digest, err := treeDigest(wt)
		if err != nil {
			return nil, fmt.Errorf("artifact digest failed")
		}
		var size int64
		_ = filepath.Walk(wt, func(_ string, fi os.FileInfo, err error) error {
			if err == nil && fi.Mode().IsRegular() {
				size += fi.Size()
			}
			return nil
		})
		sum := sha256.Sum256([]byte(a.BuildID))
		return json.Marshal(map[string]any{
			"build_id": a.BuildID, "state": "succeeded",
			"artifact_digest": "sha256:" + digest,
			"artifact_size":   size,
			"log_ref":         "build-" + hex.EncodeToString(sum[:4]), // redacted shape only
			"isolated":        true,
		})
	}
}

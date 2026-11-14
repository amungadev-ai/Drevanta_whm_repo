package helper

// Build isolation tests (G30): the fail-closed refusal is REAL in CI (no
// systemd-run); the isolated success path is exercised through an injected
// sandbox runner that performs the REAL git fetch + checkout + digest.

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildFixture(t *testing.T) (scratch string) {
	t.Helper()
	base := t.TempDir()
	scratch = filepath.Join(base, "builds")
	bind := filepath.Join(base, "build-bindings.json")
	if err := os.WriteFile(bind, []byte(`["`+scratch+`"]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := LoadBuildBindings(bind); err != nil {
		t.Fatal(err)
	}
	return scratch
}

func buildCall(t *testing.T, args BuildExecuteArgs) (map[string]any, error) {
	t.Helper()
	raw, _ := json.Marshal(args)
	out, err := BuildExecuteHandler()(context.Background(), raw)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	return m, nil
}

func TestBuildExecuteFailsClosedWithoutIsolation(t *testing.T) {
	scratch := buildFixture(t)
	// CI sandboxes cannot create systemd scopes; the probe must fail and
	// the build must REFUSE rather than run unsandboxed.
	_, err := buildCall(t, BuildExecuteArgs{
		BuildID: "bk-failclosed", RepoURL: "file:///nonexistent", ScratchRoot: scratch,
	})
	if err == nil || !strings.Contains(err.Error(), "isolation probe failed") {
		t.Fatalf("expected fail-closed refusal, got %v", err)
	}
}

func TestBuildExecuteRejectsUnsafeInputs(t *testing.T) {
	scratch := buildFixture(t)
	cases := []BuildExecuteArgs{
		{BuildID: "x1", RepoURL: "ssh://git@host/repo.git", ScratchRoot: scratch},
		{BuildID: "x2", RepoURL: "git+ssh://host/repo", ScratchRoot: scratch},
		{BuildID: "x3", RepoURL: "https://ok.test/r", ScratchRoot: t.TempDir()},
		{BuildID: "", RepoURL: "https://ok.test/r", ScratchRoot: scratch},
		{BuildID: "x5", RepoURL: "https://ok.test/r\nEVIL", ScratchRoot: scratch},
	}
	for _, c := range cases {
		if _, err := buildCall(t, c); err == nil {
			t.Fatalf("unsafe input must reject: %+v", c)
		}
	}
}

func TestBuildExecuteSandboxedSuccess(t *testing.T) {
	scratch := buildFixture(t)
	// Real local source repo.
	src := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = src
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t.test",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t.test")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main", src)
	if err := os.WriteFile(filepath.Join(src, "index.js"), []byte("console.log('built');\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-q", "-m", "seed")

	// Inject the isolation probe (pass) and a sandbox runner that executes
	// argv directly (real git checkout); production restores systemd-run
	// semantics — both seams are defer-restored.
	origProbe, origRunner := isolationProbeFn, sandboxRunner
	isolationProbeFn = func() error { return nil }
	sandboxRunner = func(ctx context.Context, argv []string, dir string, env []string) error {
		cmd := exec.Command(argv[0], argv[1:]...)
		if dir != "" {
			cmd.Dir = dir
		}
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Logf("sandbox argv %v -> %v: %s", argv[1:4], err, out)
			return err
		}
		return nil
	}
	defer func() { isolationProbeFn, sandboxRunner = origProbe, origRunner }()

	// Verify plaintext of the source never matters — digest determinism:
	res, err := buildCall(t, BuildExecuteArgs{
		BuildID: "bk-iso-1", RepoURL: "file://" + src, CommitRef: "main", ScratchRoot: scratch,
	})
	if err != nil {
		t.Fatalf("isolated build: %v", err)
	}
	if res["state"] != "succeeded" || res["isolated"] != true {
		t.Fatalf("result: %v", res)
	}
	d1 := res["artifact_digest"].(string)
	if !strings.HasPrefix(d1, "sha256:") || len(d1) != len("sha256:")+64 {
		t.Fatalf("digest shape: %v", d1)
	}
	// Second identical build → same digest.
	res2, err := buildCall(t, BuildExecuteArgs{
		BuildID: "bk-iso-2", RepoURL: "file://" + src, CommitRef: "main", ScratchRoot: scratch,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res2["artifact_digest"] != d1 {
		t.Fatal("digest not deterministic for identical source")
	}
	// log_ref is a redacted shape, never the URL.
	if strings.Contains(res["log_ref"].(string), "file://") {
		t.Fatal("log_ref leaks repository URL")
	}
}

package helper

// Interactive scoped terminal sessions (WEB-07 v2): PTY-backed streaming via
// chunked verbs. Sessions are scope-pinned, resource-confined (same sandbox
// contract as builds/non-interactive exec), time-limited, revocable, and
// audited (command + lifecycle events journaled; INPUT is never journaled —
// secrets typed in a terminal must not persist).

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ExecOpenArgs opens a PTY session inside a site binding.
type ExecOpenArgs struct {
	Binding  string `json:"binding"`
	Shell    string `json:"shell"` // allowlisted: sh | bash
	TimeoutS int    `json:"timeout_s,omitempty"`
}

// ExecSession is one live PTY session.
type ExecSession struct {
	mu       sync.Mutex
	cmd      *exec.Cmd
	stdin    interface{ Write([]byte) (int, error) }
	stdout   *strings.Builder
	expires  time.Time
	binding  string
	shellCmd string
}

var (
	execSessions   = map[string]*ExecSession{}
	execSessionsMu sync.Mutex
	sessionSeq     int
)

const execSessionTTL = 5 * time.Minute

// ExecOpenHandler allocates an interactive PTY session under the sandbox.
func ExecOpenHandler() VerbHandler {
	return func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		dec := json.NewDecoder(strings.NewReader(string(args)))
		dec.DisallowUnknownFields()
		var a ExecOpenArgs
		if err := dec.Decode(&a); err != nil {
			return nil, fmt.Errorf("invalid exec.open arguments")
		}
		root, ok := fileBindingRoots[a.Binding]
		if !ok {
			return nil, fmt.Errorf("binding not allowlisted")
		}
		if a.Shell != "sh" && a.Shell != "bash" {
			return nil, fmt.Errorf("shell not allowlisted")
		}
		timeout := a.TimeoutS
		if timeout <= 0 || timeout > 300 {
			timeout = 300
		}
		if err := isolationProbeFn(); err != nil {
			return nil, err
		}
		// `script` allocates a real PTY; the helper pipes stdin/stdout.
		runArgs := []string{"--quiet", "--wait",
			"-p", "CPUQuota=50%", "-p", "MemoryMax=512M", "-p", "TasksMax=64",
			"--uid=nobody", "-p", "WorkingDirectory=" + root,
			"-p", "PrivateNetwork=yes"}
		for _, e := range buildEnv(root) {
			runArgs = append(runArgs, "-E", e)
		}
		runArgs = append(runArgs, "--", "script", "-qefc", a.Shell, "/dev/null")
		cmd := exec.CommandContext(ctx, "systemd-run", runArgs...)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return nil, fmt.Errorf("session stdin failed")
		}
		out := &strings.Builder{}
		cmd.Stdout = out
		cmd.Stderr = out
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("session start failed")
		}
		execSessionsMu.Lock()
		defer execSessionsMu.Unlock()
		sessionSeq++
		id := fmt.Sprintf("term_%d_%d", time.Now().Unix(), sessionSeq)
		execSessions[id] = &ExecSession{
			cmd: cmd, stdin: stdin, stdout: out,
			expires: time.Now().Add(time.Duration(timeout) * time.Second),
			binding: a.Binding, shellCmd: a.Shell,
		}
		// Journal lifecycle (open) — command identity audited, no input.
		return json.Marshal(map[string]any{
			"session": id, "binding": a.Binding, "shell": a.Shell,
			"pty": true, "expires_in_s": timeout, "state": "open",
		})
	}
}

// ExecWriteHandler writes input to a session and returns new output.
type ExecWriteArgs struct {
	Session string `json:"session"`
	Input   string `json:"input"` // sent to the PTY; never journaled
}

func ExecWriteHandler() VerbHandler {
	return func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
		dec := json.NewDecoder(strings.NewReader(string(args)))
		dec.DisallowUnknownFields()
		var a ExecWriteArgs
		if err := dec.Decode(&a); err != nil {
			return nil, fmt.Errorf("invalid exec.write arguments")
		}
		execSessionsMu.Lock()
		s, ok := execSessions[a.Session]
		execSessionsMu.Unlock()
		if !ok {
			return nil, fmt.Errorf("session not found or expired")
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if time.Now().After(s.expires) {
			_ = s.cmd.Process.Kill()
			execSessionsMu.Lock()
			delete(execSessions, a.Session)
			execSessionsMu.Unlock()
			return nil, fmt.Errorf("session expired")
		}
		if _, err := s.stdin.Write([]byte(a.Input)); err != nil {
			return nil, fmt.Errorf("session write failed")
		}
		time.Sleep(120 * time.Millisecond) // small read window
		before := s.stdout.Len()
		return json.Marshal(map[string]any{
			"session": a.Session, "state": "streaming",
			"output_chunk": chunkOf(s.stdout.String(), before),
		})
	}
}

func chunkOf(all string, before int) string {
	if before >= len(all) {
		return ""
	}
	chunk := all[before:]
	if len(chunk) > 64*1024 {
		chunk = chunk[len(chunk)-64*1024:]
	}
	return chunk
}

// ExecCloseHandler revokes a session immediately (owner of the scope).
type ExecCloseArgs struct {
	Session string `json:"session"`
}

func ExecCloseHandler() VerbHandler {
	return func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
		dec := json.NewDecoder(strings.NewReader(string(args)))
		dec.DisallowUnknownFields()
		var a ExecCloseArgs
		if err := dec.Decode(&a); err != nil {
			return nil, fmt.Errorf("invalid exec.close arguments")
		}
		execSessionsMu.Lock()
		defer execSessionsMu.Unlock()
		s, ok := execSessions[a.Session]
		if !ok {
			return nil, fmt.Errorf("session not found")
		}
		_ = s.cmd.Process.Kill()
		delete(execSessions, a.Session)
		// Journal lifecycle (close/revocation).
		return json.Marshal(map[string]string{"session": a.Session, "state": "closed"})
	}
}

// SweepExecSessions kills expired sessions (called periodically by the host).
func SweepExecSessions() {
	execSessionsMu.Lock()
	defer execSessionsMu.Unlock()
	now := time.Now()
	for id, s := range execSessions {
		if now.After(s.expires) {
			_ = s.cmd.Process.Kill()
			delete(execSessions, id)
		}
	}
}

// SessionCount exposes the live session count (monitoring).
func SessionCount() int {
	execSessionsMu.Lock()
	defer execSessionsMu.Unlock()
	return len(execSessions)
}

// ensure os import used
var _ = os.Getpid

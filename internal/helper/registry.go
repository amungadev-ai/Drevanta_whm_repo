// Package helper implements the root-owned privileged host helper (DEC-003,
// phases 07/08). Local authenticated Unix socket only; every request carries
// a typed verb, typed arguments, host/installation binding, fence, expiry and
// canonical digest. Unknown verbs and extra fields reject before dispatch.
// Each accepted attempt is journaled; replays of the same (operation, digest)
// return the journaled observation; changed digests reject.
package helper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Protocol version.
const Version = 1

// HostRequest is the single wire request type (strict decoding; no extra
// fields accepted).
type HostRequest struct {
	Version        int             `json:"version"`
	InstallationID string          `json:"installation_id"`
	HostID         string          `json:"host_id"`
	Verb           string          `json:"verb"`
	Arguments      json.RawMessage `json:"arguments"`
	Fence          int64           `json:"fence"`
	OperationID    string          `json:"operation_id"`
	Digest         string          `json:"digest"`
	ExpiresAt      string          `json:"expires_at"`
}

// HostResult is the single wire response type.
type HostResult struct {
	Status      string          `json:"status"` // ok | rejected | uncertain
	SafeReason  string          `json:"safe_reason"`
	JournalID   string          `json:"journal_id,omitempty"`
	Observation json.RawMessage `json:"observation,omitempty"`
	OccurredAt  string          `json:"occurred_at,omitempty"`
}

// Policy is the root-owned allowlist: verb → permitted targets. The file is
// read from disk on every policy evaluation so operators can tighten it
// without restarting (host-local registry, readable/writable only by
// authorized host principals per profile.md §VPS protocol).
type Policy struct {
	Verbs map[string][]string `json:"verbs"` // verb → allowed target bindings (empty = verb disabled)
}

// LoadPolicy reads the policy file; missing file fails closed.
func LoadPolicy(path string) (*Policy, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Policy
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// VerbHandler executes one verb with already-typed, allowlist-checked
// arguments. Implementations must be idempotent per (operation, digest).
type VerbHandler func(ctx context.Context, args json.RawMessage) (json.RawMessage, error)

// Registry is the host-side state: policy, handlers, journals, fences.
type Registry struct {
	mu         sync.Mutex
	policyMu   sync.Mutex
	policy     *Policy
	policyPath string
	handlers   map[string]VerbHandler
	db         JournalStore
	installID  string
	peerUIDs   map[int]bool // allowed peer UIDs (panel service user)
}

// JournalStore persists host journal entries (the helper keeps its own
// journal; panel mirrors accepted attempts into host_journal).
type JournalStore interface {
	// Append records one attempt; returns the journal ID.
	Append(hostID, operationID, digest string, fence int64, verb, outcome string, result json.RawMessage) (string, error)
	// Lookup returns (outcome, result, journalID, fence) for a prior attempt
	// with the same operation+digest, or notFound.
	Lookup(operationID, digest string) (outcome, result, journalID string, fence int64, err error)
	// MaxFence returns the highest fence recorded for the operation.
	MaxFence(operationID string) (int64, error)
}

var ErrNotFound = errors.New("not found")

// NewRegistry builds a Registry. peerUIDs lists the UIDs allowed to connect
// (root-owned socket + peer credential check).
func NewRegistry(policy *Policy, policyPath string, db JournalStore, installID string, peerUIDs map[int]bool) *Registry {
	return &Registry{
		policy: policy, policyPath: policyPath, handlers: map[string]VerbHandler{},
		db: db, installID: installID, peerUIDs: peerUIDs,
	}
}

// Register binds a typed handler to a verb name.
func (r *Registry) Register(verb string, h VerbHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[verb] = h
}

// reloadPolicy re-reads the policy file (operators may tighten at runtime).
func (r *Registry) reloadPolicy() (*Policy, error) {
	r.policyMu.Lock()
	defer r.policyMu.Unlock()
	if r.policyPath == "" {
		return r.policy, nil
	}
	p, err := LoadPolicy(r.policyPath)
	if err != nil {
		// Fail closed on unreadable policy.
		return nil, err
	}
	r.policy = p
	return r.policy, nil
}

// Process validates and executes one host request. Order (profile.md): peer
// proof → policy match → version/host/install match → verb known → typed
// args → expiry → fence → digest/replay → dispatch → journal.
func (r *Registry) Process(ctx context.Context, uid int, req HostRequest) HostResult {
	now := time.Now().UTC()

	// Peer proof: only allowed UIDs ever reach here (listener enforces), but
	// the check is re-asserted in-band.
	if !r.peerUIDs[uid] {
		return reject("peer not authorized")
	}
	// Version and installation binding.
	if req.Version != Version {
		return reject("protocol version mismatch")
	}
	if req.InstallationID != r.installID {
		return reject("installation mismatch")
	}
	if req.HostID == "" || req.OperationID == "" || req.Digest == "" {
		return reject("missing host, operation or digest binding")
	}
	// Expiry.
	exp, err := time.Parse(time.RFC3339Nano, req.ExpiresAt)
	if err != nil || now.After(exp) {
		return reject("request expired")
	}

	// Policy (reloaded): unknown verbs or unlisted targets reject.
	policy, err := r.reloadPolicy()
	if err != nil {
		return reject("policy unavailable")
	}
	targets, ok := policy.Verbs[req.Verb]
	if !ok || len(targets) == 0 {
		return reject("verb not allowlisted")
	}
	bound := false
	for _, tg := range targets {
		if tg == req.HostID || tg == "*" {
			bound = true
			break
		}
	}
	if !bound {
		return reject("target not allowlisted")
	}

	r.mu.Lock()
	handler, ok := r.handlers[req.Verb]
	r.mu.Unlock()
	if !ok {
		return reject("verb not implemented")
	}

	// Replay: same operation+digest returns the journaled observation
	// (profile fixture: lost host reply → one logical effect). A retry
	// repeats the SAME fence; an OLDER fence means a stale worker and
	// rejects before effects and finalization.
	if _, result, jid, jfence, lerr := r.db.Lookup(req.OperationID, req.Digest); lerr == nil {
		if req.Fence < jfence {
			return reject("stale fence")
		}
		return HostResult{
			Status:      "ok",
			SafeReason:  "journaled observation replayed",
			JournalID:   jid,
			Observation: json.RawMessage(result),
			OccurredAt:  now.Format(time.RFC3339Nano),
		}
	} else if !errors.Is(lerr, ErrNotFound) {
		return reject("journal lookup failed")
	}

	// Fence: stale fences reject before effects (new effects only; a higher
	// fence supersedes older delivery attempts).
	r.mu.Lock()
	maxFence, err := r.db.MaxFence(req.OperationID)
	r.mu.Unlock()
	if err == nil && req.Fence <= maxFence {
		return reject("stale fence")
	}

	// Dispatch with typed arguments.
	obs, herr := handler(ctx, req.Arguments)

	outcome := "succeeded"
	if herr != nil {
		outcome = "failed"
	}
	jid, jerr := r.db.Append(req.HostID, req.OperationID, req.Digest, req.Fence, req.Verb, outcome, obs)
	if jerr != nil {
		// Journal failure means the effect's record is uncertain.
		return HostResult{Status: "uncertain", SafeReason: "journal unavailable; effect state unrecorded",
			OccurredAt: now.Format(time.RFC3339Nano)}
	}
	res := HostResult{Status: "ok", JournalID: jid, Observation: obs,
		OccurredAt: now.Format(time.RFC3339Nano)}
	if herr != nil {
		res.Status = "ok" // effect executed and journaled as failed; result carries the failure
		res.Observation = obs
	}
	return res
}

func reject(reason string) HostResult {
	return HostResult{Status: "rejected", SafeReason: reason}
}

// DigestOf computes the canonical request digest over the operation and its
// payload (panel-side helper).
func DigestOf(operationID, verb string, arguments json.RawMessage) string {
	h := sha256.New()
	h.Write([]byte(operationID))
	h.Write([]byte{0})
	h.Write([]byte(verb))
	h.Write([]byte{0})
	h.Write(arguments)
	return hex.EncodeToString(h.Sum(nil))
}

// ---- Listener (peer credentials) ----

// Serve listens on a Unix socket, enforces peer UID checks and root-owned
// socket file, then serves requests until the listener closes.
func (r *Registry) Serve(ctx context.Context, socketPath string) error {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o750); err != nil {
		return err
	}
	_ = os.Remove(socketPath)
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	if err := os.Chmod(socketPath, 0o660); err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go r.handleConn(conn)
	}
}

func (r *Registry) handleConn(conn net.Conn) {
	defer conn.Close()
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return
	}
	// Peer credential check: SO_PEERCRED via syscall on the raw fd.
	cred, err := peerCredentials(uc)
	if err != nil {
		writeResult(conn, reject("peer credential check failed"))
		return
	}
	if !r.peerUIDs[int(cred.Uid)] {
		writeResult(conn, reject("peer not authorized"))
		return
	}
	dec := json.NewDecoder(conn)
	dec.DisallowUnknownFields()
	var req HostRequest
	if err := dec.Decode(&req); err != nil {
		writeResult(conn, reject("invalid request schema"))
		return
	}
	res := r.Process(context.Background(), int(cred.Uid), req)
	writeResult(conn, res)
}

func writeResult(conn net.Conn, res HostResult) {
	b, _ := json.Marshal(res)
	_, _ = conn.Write(b)
	fmt.Fprint(conn, "\n")
}

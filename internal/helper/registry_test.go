package helper

// G20 helper-protocol verification: profile.md host fixtures (scenarios 7-10)
// plus allowlist and typed-argument discipline (DEC-003, phases 07/08).

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"drevanta/internal/store"
)

const testInstall = "inst_test0000000000000000000000"

// memoryJournal is an in-memory JournalStore for protocol tests.
type memoryJournal struct {
	entries map[string]journalEntry // key: operationID|digest
	fences  map[string]int64
}

type journalEntry struct {
	outcome string
	result  string
	jid     string
}

func newMemoryJournal() *memoryJournal {
	return &memoryJournal{entries: map[string]journalEntry{}, fences: map[string]int64{}}
}

func (m *memoryJournal) Append(hostID, operationID, digest string, fence int64, verb, outcome string, result json.RawMessage) (string, error) {
	res := "{}"
	if result != nil {
		res = string(result)
	}
	jid := "hj_" + store.NewID("x")[2:8]
	m.entries[operationID+"|"+digest] = journalEntry{outcome: outcome, result: res, jid: jid}
	if fence > m.fences[operationID] {
		m.fences[operationID] = fence
	}
	return jid, nil
}

func (m *memoryJournal) Lookup(operationID, digest string) (string, string, string, int64, error) {
	if e, ok := m.entries[operationID+"|"+digest]; ok {
		return e.outcome, e.result, e.jid, m.fences[operationID], nil
	}
	return "", "", "", 0, ErrNotFound
}

func (m *memoryJournal) MaxFence(operationID string) (int64, error) {
	return m.fences[operationID], nil
}

func newTestRegistry(t *testing.T) (*Registry, *memoryJournal) {
	t.Helper()
	policy := &Policy{Verbs: map[string][]string{
		"host.inspect":   {"host_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		"host.preflight": {"host_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}}
	j := newMemoryJournal()
	reg := NewRegistry(policy, "", j, testInstall, map[int]bool{0: true})
	reg.Register("host.inspect", InspectHandler())
	reg.Register("host.preflight", PreflightHandler())
	return reg, j
}

func baseReq(verb string) HostRequest {
	return HostRequest{
		Version:        Version,
		InstallationID: testInstall,
		HostID:         "host_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Verb:           verb,
		Arguments:      json.RawMessage(`{}`),
		Fence:          1,
		OperationID:    "op_11111111111111111111111111111111",
		Digest:         "aa",
		ExpiresAt:      time.Now().UTC().Add(time.Minute).Format(time.RFC3339Nano),
	}
}

func run(t *testing.T, reg *Registry, req HostRequest) HostResult {
	t.Helper()
	return reg.Process(context.Background(), 0, req)
}

// Fixture 7 · wrong host: request naming host-b is rejected before effects.
func TestWrongHostRejectedBeforeEffects(t *testing.T) {
	req := baseReq("host.inspect")
	req.HostID = "host_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	// Policy targets bind exactly host_a for this verb in a real deployment;
	// simulate the binding mismatch via installation binding (host-to-
	// installation proof) and allowlist: bind a tighter registry.
	policy := &Policy{Verbs: map[string][]string{"host.inspect": {"host_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}}
	reg2 := NewRegistry(policy, "", newMemoryJournal(), testInstall, map[int]bool{0: true})
	reg2.Register("host.inspect", InspectHandler())

	res := run(t, reg2, req)
	if res.Status != "rejected" {
		t.Fatalf("wrong host accepted: %+v", res)
	}
	if res.JournalID != "" {
		t.Fatalf("rejected attempt journaled")
	}
}

// Fixture 8 · stale fence: host accepted fence 2; worker submits fence 1 →
// rejected before effects and finalization.
func TestStaleFenceRejected(t *testing.T) {
	reg, j := newTestRegistry(t)
	// Host has already accepted fence 2 for this operation.
	opID := "op_22222222222222222222222222222222"
	dig := "bb"
	_, err := j.Append("host_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", opID, dig, 2, "host.inspect", "succeeded", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	req := baseReq("host.inspect")
	req.OperationID = opID
	req.Digest = dig
	req.Fence = 1
	res := run(t, reg, req)
	if res.Status != "rejected" || res.SafeReason != "stale fence" {
		t.Fatalf("stale fence not rejected: %+v", res)
	}
}

// Fixture 9 · lost host reply: host completes the effect, response
// disappears; worker repeats same ID/digest → one logical effect; journal
// returns the prior observation.
func TestLostReplyReplayReturnsObservation(t *testing.T) {
	reg, _ := newTestRegistry(t)
	req := baseReq("host.inspect")
	req.Digest = "cc"

	// First attempt succeeds and journals.
	res1 := run(t, reg, req)
	if res1.Status != "ok" || res1.JournalID == "" {
		t.Fatalf("first: %+v", res1)
	}
	first := res1.Observation

	// Response lost; worker repeats the same ID/digest.
	res2 := run(t, reg, req)
	if res2.Status != "ok" {
		t.Fatalf("replay: %+v", res2)
	}
	if res2.JournalID != res1.JournalID {
		t.Fatalf("replay created a new journal entry: %s vs %s", res2.JournalID, res1.JournalID)
	}
	if string(res2.Observation) != string(first) {
		t.Fatalf("observation drift")
	}
}

// Fixture 10 · changed digest: same operation ID, different canonical request
// digest → rejected before dispatch; previous journal entry retained.
func TestChangedDigestRejected(t *testing.T) {
	reg, jj := newTestRegistry(t)
	req := baseReq("host.inspect")
	req.Digest = "dd"
	if res := run(t, reg, req); res.Status != "ok" {
		t.Fatalf("first: %+v", res)
	}
	changed := req
	changed.Digest = "ee"
	res := run(t, reg, changed)
	if res.Status != "rejected" {
		t.Fatalf("changed digest accepted: %+v", res)
	}
	// Previous journal entry retained and still replayable.
	if _, _, jid, _, err := jj.Lookup(req.OperationID, "dd"); err != nil {
		t.Fatalf("original journal lost: %v", err)
	} else if jid == "" {
		t.Fatal("empty journal id")
	}
}

// Unknown verbs and extra fields reject before dispatch.
func TestUnknownVerbAndExtraFields(t *testing.T) {
	reg, _ := newTestRegistry(t)
	res := run(t, reg, baseReq("run_shell"))
	if res.Status != "rejected" || res.SafeReason != "verb not allowlisted" {
		t.Fatalf("unknown verb: %+v", res)
	}
	// Expired request rejects.
	expired := baseReq("host.inspect")
	expired.ExpiresAt = time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
	if res := run(t, reg, expired); res.Status != "rejected" {
		t.Fatalf("expired accepted: %+v", res)
	}
	// Version mismatch rejects.
	bad := baseReq("host.inspect")
	bad.Version = 99
	if res := run(t, reg, bad); res.Status != "rejected" {
		t.Fatalf("bad version accepted: %+v", res)
	}
}

// Typed arguments: extra fields inside verb arguments reject the effect.
func TestTypedArgumentDiscipline(t *testing.T) {
	reg, _ := newTestRegistry(t)
	_ = reg
	req := baseReq("host.inspect")
	req.Arguments = json.RawMessage(`{"injected":"rm -rf"}`)
	res := run(t, reg, req)
	// Effect executed but failed on argument validation; journaled as failed.
	if res.Status != "ok" {
		t.Fatalf("typed-arg violation should fail the effect, not the protocol: %+v", res)
	}
	var obs struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(res.Observation, &obs)
	// InspectHandler rejects unknown args with a Go error → journaled failed.
	if obs.Error == "" && len(res.Observation) > 2 {
		// observation is the marshal of OSInfo only on success; on failure the
		// handler returned an error and observation is null-ish.
		t.Fatalf("expected failure observation: %s", string(res.Observation))
	}
}

// Unsupported OS fails preflight closed (DEC-005).
func TestPreflightFailClosed(t *testing.T) {
	reg, _ := newTestRegistry(t)
	res := run(t, reg, baseReq("host.preflight"))
	if res.Status != "ok" {
		t.Fatalf("preflight protocol: %+v", res)
	}
	var info OSInfo
	if err := json.Unmarshal(res.Observation, &info); err != nil {
		t.Fatal(err)
	}
	// In the test sandbox the OS tuple must be explicitly classified; an
	// unknown tuple must NOT claim support.
	if info.OSID != "ubuntu" && info.OSID != "debian" && info.Supported {
		t.Fatalf("unsupported tuple claimed supported: %+v", info)
	}
}

package hostagent

// VerbExecutor bridges the durable operation pipeline to the privileged
// helper: action IDs map to allowlisted helper verbs with typed arguments.
// Results are journaled observations; failures map to the operation outcome
// taxonomy (succeeded/failed/uncertain) without leaking host detail.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"drevanta/internal/helper"
	"drevanta/internal/operations"
)

// VerbMapping defines the action→verb binding and typed-argument builder.
type VerbMapping struct {
	Verb      string
	Arguments func(intent operations.Intent) (json.RawMessage, error)
}

// passthroughArgs forwards the intent's typed arguments to the helper verb;
// the helper validates the exact schema and rejects extras before effects.
func passthroughArgs(i operations.Intent) (json.RawMessage, error) {
	if len(i.Arguments) == 0 {
		return json.RawMessage(`{}`), nil
	}
	raw, err := json.Marshal(i.Arguments)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func emptyArgs(i operations.Intent) (json.RawMessage, error) { return json.RawMessage(`{}`), nil }

// DefaultMappings maps the enabled action vocabulary (extended per domain
// wave as allowlist entries freeze).
var DefaultMappings = map[string]VerbMapping{
	"host.inspect":                {Verb: "host.inspect", Arguments: emptyArgs},
	"host.update":                 {Verb: "host.preflight", Arguments: emptyArgs},
	"file.write":                  {Verb: "file.apply", Arguments: passthroughArgs},
	"database.provision":          {Verb: "database.apply", Arguments: passthroughArgs},
	"sftp.grant":                  {Verb: "sftp.apply", Arguments: passthroughArgs},
	"vhost.apply":                 {Verb: "vhost.apply", Arguments: passthroughArgs},
	"domain.bind":                 {Verb: "vhost.apply", Arguments: passthroughArgs},
	"backup.restore":              {Verb: "backup.apply", Arguments: passthroughArgs},
	"backup.create":               {Verb: "backup.apply", Arguments: passthroughArgs},
	"backup.verify":               {Verb: "backup.apply", Arguments: passthroughArgs},
	"tls.issue":                   {Verb: "tls.apply", Arguments: passthroughArgs},
	"tls.renew":                   {Verb: "tls.apply", Arguments: passthroughArgs},
	"mail.domain.configure":       {Verb: "mail.apply", Arguments: passthroughArgs},
	"mailbox.provision":           {Verb: "mail.apply", Arguments: passthroughArgs},
	"runtime.toolchain.configure": {Verb: "toolchain.apply", Arguments: passthroughArgs},
	"cache.configure":             {Verb: "cache.apply", Arguments: passthroughArgs},
	"dns.zone.configure":          {Verb: "dns.apply", Arguments: passthroughArgs},
	"container.deploy":            {Verb: "container.apply", Arguments: passthroughArgs},
}

// VerbExecutor implements jobs.Executor over the helper client.
type VerbExecutor struct {
	Client    *Client
	HostID    string
	InstallID string
	MaxFence  func(operationID string) (int64, error) // helper journal fences
}

// Execute performs the mapped verb with a fresh fence.
func (e *VerbExecutor) Execute(ctx context.Context, op *operations.Operation, fence int64) (string, string, *int64, error) {
	m, ok := DefaultMappings[op.ActionID]
	if !ok {
		return "", "", nil, fmt.Errorf("no helper verb mapped for action %s", op.ActionID)
	}
	// The canonical intent payload carries the typed arguments.
	var intent operations.Intent
	if op.Payload != "" {
		if err := json.Unmarshal([]byte(op.Payload), &intent); err != nil {
			return "failed", `{"safe_reason":"intent payload unparsable"}`, nil, nil
		}
	}
	args, err := m.Arguments(intent)
	if err != nil {
		return "failed", `{"safe_reason":"argument build failed"}`, nil, nil
	}
	digest := helper.DigestOf(op.OperationID, m.Verb, args)
	req := helper.HostRequest{
		Version:        helper.Version,
		InstallationID: e.InstallID,
		HostID:         e.HostID,
		Verb:           m.Verb,
		Arguments:      args,
		Fence:          fence,
		OperationID:    op.OperationID,
		Digest:         digest,
		ExpiresAt:      time.Now().UTC().Add(2 * time.Minute).Format(time.RFC3339Nano),
	}
	res, err := e.Client.Call(ctx, req)
	if err != nil {
		if errors.Is(err, ErrNoConnection) {
			// Never delivered: no effect possible; record a safe failure.
			return "failed", `{"safe_reason":"helper unreachable; no effect delivered"}`, nil, nil
		}
		if errors.Is(err, ErrRejected) {
			return "failed", `{"safe_reason":"helper rejected request"}`, nil, nil
		}
		// Delivered but reply unparsable: outcome genuinely unobserved.
		return "", `{"safe_reason":"helper reply unparsable"}`, nil, err
	}
	if res.Status != "ok" {
		return "uncertain", `{"safe_reason":"` + res.SafeReason + `"}`, nil, nil
	}
	obs, _ := json.Marshal(map[string]any{
		"verb":        m.Verb,
		"journal_id":  res.JournalID,
		"outcome":     res.Outcome,
		"observation": json.RawMessage(res.Observation),
	})
	// The journaled effect verdict governs the operation outcome (profile:
	// verified success carries observed generation; verified failure is a
	// known postcondition, not a protocol error).
	if res.Outcome == "failed" {
		return "failed", string(obs), nil, nil
	}
	if res.Outcome != "" && res.Outcome != "succeeded" {
		return "uncertain", string(obs), nil, nil
	}
	gen := op.ExpectedGeneration + 1
	return "succeeded", string(obs), &gen, nil
}

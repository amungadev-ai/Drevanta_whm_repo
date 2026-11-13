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

// DefaultMappings maps the currently enabled action vocabulary (extended per
// domain wave as allowlist entries freeze).
var DefaultMappings = map[string]VerbMapping{
	"host.inspect": {
		Verb: "host.inspect",
		Arguments: func(i operations.Intent) (json.RawMessage, error) {
			return json.RawMessage(`{}`), nil
		},
	},
	"host.update": {
		Verb: "host.preflight",
		Arguments: func(i operations.Intent) (json.RawMessage, error) {
			return json.RawMessage(`{}`), nil
		},
	},
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
	args, err := m.Arguments(operations.Intent{ActionID: op.ActionID})
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
		"observation": json.RawMessage(res.Observation),
	})
	gen := op.ExpectedGeneration + 1
	return "succeeded", string(obs), &gen, nil
}

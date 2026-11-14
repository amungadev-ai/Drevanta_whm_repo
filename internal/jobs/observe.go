package jobs

// Observation derivation (G40-F2): successful host.inspect operations become
// real health observations. The inspect result JSON is produced by the
// privileged helper; whitelisted scalar facts (kind=health) are recorded, and nothing
// from unparseable results is invented.

import (
	"context"
	"encoding/json"
	"time"

	"drevanta/internal/domains"
	"drevanta/internal/operations"
)

// HostInspectObserver records whitelisted host facts from successful
// host.inspect results into the health_observation store.
type HostInspectObserver struct {
	Service *domains.ObserveService
}

// Observe implements the Worker.Observer seam.
func (h *HostInspectObserver) Observe(ctx context.Context, op *operations.Operation, outcome, resultJSON string) error {
	if outcome != "succeeded" || op.ActionID != "host.inspect" || resultJSON == "" {
		return nil
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(resultJSON), &res); err != nil {
		return nil // unparseable: record nothing, invent nothing
	}
	// Flatten one level; record only scalars under whitelisted fact names.
	allowed := map[string]bool{
		"os": true, "os_family": true, "arch": true, "kernel": true,
		"hostname": true, "cpu_cores": true, "memory_total_mb": true,
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_ = now
	var recErr error
	for k, v := range res {
		if !allowed[k] {
			continue
		}
		switch val := v.(type) {
		case string:
			_, recErr = h.Service.Record(ctx, op.ScopeID, op.ResourceID, "health", k, nil, val)
		case float64:
			_, recErr = h.Service.Record(ctx, op.ScopeID, op.ResourceID, "health", k, &val, "")
		default:
			continue
		}
		if recErr != nil {
			return recErr
		}
	}
	return nil
}

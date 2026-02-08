package ml

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ollama/ollama/logutil"
)

// RunnerStats contains best-effort, point-in-time stats about a runner.
//
// This is an internal API used by the Ollama server for observability.
// Not all runners support these fields.
type RunnerStats struct {
	ContextMax int `json:"context_max"`
	// ContextUsed is a cached-context watermark. It is the maximum number of
	// tokens currently stored in any cache slot (active or inactive).
	ContextUsed int `json:"context_used"`
	// ContextActive is the maximum number of tokens currently stored in any
	// active (in-use) cache slot.
	ContextActive int `json:"context_active"`
	// ContextAllocated is the current allocation-based context capacity per slot.
	// For the causal cache this is derived from allocated cache cells.
	ContextAllocated int `json:"context_allocated"`
	// ContextInitial is the initial allocation-based context capacity per slot
	// at cache creation time, before any dynamic growth.
	ContextInitial int `json:"context_initial"`
	Slots          int `json:"slots"`
	SlotsInUse     int `json:"slots_in_use"`
}

// GetRunnerStatsFromRunner queries a runner's /stats endpoint.
//
// Returns (nil, error) if the runner doesn't support stats or if the request
// cannot be completed within the provided context deadline.
func GetRunnerStatsFromRunner(ctx context.Context, runner BaseRunner) (*RunnerStats, error) {
	port := runner.GetPort()
	tick := time.Tick(10 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("failed to finish runner stats query before timeout")
		case <-tick:
			r, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/stats", port), nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create request: %w", err)
			}
			r.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(r)
			if err != nil {
				if runner.HasExited() {
					return nil, fmt.Errorf("runner crashed")
				}
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				return nil, fmt.Errorf("runner stats reporting not supported")
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				continue
			}
			if resp.StatusCode != http.StatusOK {
				logutil.Trace("runner failed to report stats", "status", resp.StatusCode, "response", body)
				return nil, fmt.Errorf("runner error: %s", string(body))
			}

			var stats RunnerStats
			if err := json.Unmarshal(body, &stats); err != nil {
				continue
			}
			return &stats, nil
		}
	}
}

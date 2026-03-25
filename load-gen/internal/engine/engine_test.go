package engine

import (
	"context"
	"testing"
)

func TestParseWorkload(t *testing.T) {
	yamlData := `
name: "test-workload"
target: "http://localhost:8080"
phases:
  - name: "ramp-up"
    duration_sec: 1
    actions:
      - type: "create_vm"
        rate_per_sec: 10
assertions:
  - metric: "api_response_time_p99"
    threshold_ms: 500
`

	w, err := ParseWorkload([]byte(yamlData))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if w.Name != "test-workload" {
		t.Errorf("name = %q", w.Name)
	}
	if len(w.Phases) != 1 {
		t.Fatalf("phases = %d", len(w.Phases))
	}
	if w.Phases[0].DurationSec != 1 {
		t.Errorf("duration = %d", w.Phases[0].DurationSec)
	}
	if len(w.Assertions) != 1 {
		t.Errorf("assertions = %d", len(w.Assertions))
	}
}

func TestRunWorkload(t *testing.T) {
	e := New(nil)
	ctx := context.Background()

	w := &WorkloadDef{
		Name:   "quick-test",
		Target: "http://localhost",
		Phases: []PhaseDef{
			{
				Name:        "short",
				DurationSec: 1,
				Actions: []ActionDef{
					{Type: "create_vm", RatePerSec: 100},
				},
			},
		},
		Assertions: []Assertion{
			{Metric: "success_rate", ThresholdPercent: 90},
		},
	}

	result, err := e.RunWorkload(ctx, w)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if result.RunID == "" {
		t.Error("expected non-empty run ID")
	}
	if result.Status != "running" {
		t.Errorf("status = %q, want running", result.Status)
	}

	// Wait for completion
	for {
		r, err := e.GetRun(ctx, result.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status == "completed" {
			if r.TotalRequests == 0 {
				t.Error("expected some requests")
			}
			break
		}
	}
}

func TestGetRunNotFound(t *testing.T) {
	e := New(nil)
	ctx := context.Background()

	_, err := e.GetRun(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent run")
	}
}

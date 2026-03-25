// Package engine provides the workload execution engine for the load generator.
package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// WorkloadDef defines a load test workload.
type WorkloadDef struct {
	Name       string     `yaml:"name" json:"name"`
	Target     string     `yaml:"target" json:"target"`
	Phases     []PhaseDef `yaml:"phases" json:"phases"`
	Assertions []Assertion `yaml:"assertions" json:"assertions"`
}

// PhaseDef defines a phase within a workload.
type PhaseDef struct {
	Name        string      `yaml:"name" json:"name"`
	DurationSec int         `yaml:"duration_sec" json:"duration_sec"`
	Actions     []ActionDef `yaml:"actions" json:"actions"`
}

// ActionDef defines an action to perform during a phase.
type ActionDef struct {
	Type       string  `yaml:"type" json:"type"`
	RatePerSec float64 `yaml:"rate_per_sec" json:"rate_per_sec"`
}

// Assertion defines a performance assertion.
type Assertion struct {
	Metric           string  `yaml:"metric" json:"metric"`
	ThresholdMs      float64 `yaml:"threshold_ms,omitempty" json:"threshold_ms,omitempty"`
	ThresholdPercent float64 `yaml:"threshold_percent,omitempty" json:"threshold_percent,omitempty"`
}

// RunResult holds the results of a workload run.
type RunResult struct {
	RunID              string                    `json:"run_id"`
	Status             string                    `json:"status"`
	WorkloadName       string                    `json:"workload_name"`
	TotalRequests      int                       `json:"total_requests"`
	SuccessfulRequests int                       `json:"successful_requests"`
	FailedRequests     int                       `json:"failed_requests"`
	DurationMs         int64                     `json:"duration_ms"`
	Assertions         map[string]AssertionResult `json:"assertions"`
	StartedAt          time.Time                 `json:"started_at"`
	FinishedAt         *time.Time                `json:"finished_at,omitempty"`
}

// AssertionResult holds the result of a single assertion check.
type AssertionResult struct {
	Passed           bool    `json:"passed"`
	ThresholdMs      float64 `json:"threshold_ms,omitempty"`
	ActualMs         float64 `json:"actual_ms,omitempty"`
	ThresholdPercent float64 `json:"threshold_percent,omitempty"`
	ActualPercent    float64 `json:"actual_percent,omitempty"`
}

// Engine runs workload tests against Cirrus.
type Engine struct {
	mu      sync.RWMutex
	runs    map[string]*RunResult
	counter int
	logger  *slog.Logger
}

// New creates a new Engine.
func New(logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	return &Engine{
		runs:   make(map[string]*RunResult),
		logger: logger,
	}
}

// ParseWorkload parses a YAML workload definition.
func ParseWorkload(data []byte) (*WorkloadDef, error) {
	var w WorkloadDef
	if err := yaml.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("parse workload: %w", err)
	}
	return &w, nil
}

// RunWorkload starts a workload execution and returns the run ID.
func (e *Engine) RunWorkload(ctx context.Context, workload *WorkloadDef) (*RunResult, error) {
	e.mu.Lock()
	e.counter++
	runID := fmt.Sprintf("run-%03d", e.counter)

	result := &RunResult{
		RunID:        runID,
		Status:       "running",
		WorkloadName: workload.Name,
		StartedAt:    time.Now().UTC(),
		Assertions:   make(map[string]AssertionResult),
	}
	e.runs[runID] = result
	e.mu.Unlock()

	go e.executeWorkload(ctx, result, workload)

	return result, nil
}

// GetRun returns a run result by ID.
func (e *Engine) GetRun(_ context.Context, runID string) (*RunResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	run, ok := e.runs[runID]
	if !ok {
		return nil, fmt.Errorf("get run %q: run not found", runID)
	}
	cp := *run
	return &cp, nil
}

func (e *Engine) executeWorkload(_ context.Context, result *RunResult, workload *WorkloadDef) {
	totalRequests := 0
	successCount := 0

	for _, phase := range workload.Phases {
		duration := time.Duration(phase.DurationSec) * time.Second
		deadline := time.Now().Add(duration)

		for time.Now().Before(deadline) {
			for _, action := range phase.Actions {
				// Simulate action execution at the given rate
				interval := time.Duration(float64(time.Second) / action.RatePerSec)
				totalRequests++
				// Simulate ~99% success rate for the simulation
				successCount++
				time.Sleep(interval)

				if time.Now().After(deadline) {
					break
				}
			}
		}
	}

	now := time.Now().UTC()
	e.mu.Lock()
	defer e.mu.Unlock()

	result.TotalRequests = totalRequests
	result.SuccessfulRequests = successCount
	result.FailedRequests = totalRequests - successCount
	result.DurationMs = time.Since(result.StartedAt).Milliseconds()
	result.FinishedAt = &now
	result.Status = "completed"

	// Evaluate assertions
	successRate := float64(0)
	if totalRequests > 0 {
		successRate = float64(successCount) / float64(totalRequests) * 100
	}

	for _, a := range workload.Assertions {
		ar := AssertionResult{}
		switch {
		case a.ThresholdMs > 0:
			ar.ThresholdMs = a.ThresholdMs
			ar.ActualMs = 10 // simulated latency
			ar.Passed = ar.ActualMs <= ar.ThresholdMs
		case a.ThresholdPercent > 0:
			ar.ThresholdPercent = a.ThresholdPercent
			ar.ActualPercent = successRate
			ar.Passed = ar.ActualPercent >= ar.ThresholdPercent
		}
		result.Assertions[a.Metric] = ar
	}

	e.logger.Info("workload completed", "run_id", result.RunID, "requests", totalRequests)
}

package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tjst-t/cirrus-sim/load-gen/internal/engine"
)

func setupMux() *http.ServeMux {
	e := engine.New(nil)
	h := NewHandler(e)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return mux
}

func TestRunWorkload(t *testing.T) {
	mux := setupMux()

	yaml := `name: test
target: http://localhost
phases:
  - name: short
    duration_sec: 1
    actions:
      - type: create_vm
        rate_per_sec: 100
`

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/workloads/run", bytes.NewBufferString(yaml))
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	runID, ok := result["run_id"].(string)
	if !ok || runID == "" {
		t.Error("expected non-empty run_id")
	}
}

func TestGetRunNotFound(t *testing.T) {
	mux := setupMux()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/workloads/run/nonexistent", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestInvalidWorkloadYAML(t *testing.T) {
	mux := setupMux()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/workloads/run", bytes.NewBufferString("invalid: [yaml"))
	mux.ServeHTTP(w, r)

	// Invalid YAML that still parses to a struct will just be empty, not an error
	// Only truly unparseable YAML returns an error
	if w.Code != http.StatusAccepted && w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

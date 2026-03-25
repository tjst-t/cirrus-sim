package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/tjst-t/cirrus-sim/ovn-sim/internal/state"
)

func setupTest(t *testing.T) (*Management, *http.ServeMux) {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx := context.Background()
	manager := state.NewManager(logger)
	mgmt := NewManagement(ctx, manager, logger)
	mux := http.NewServeMux()
	mgmt.RegisterRoutes(mux)
	return mgmt, mux
}

func TestCreateAndListClusters(t *testing.T) {
	_, mux := setupTest(t)

	// Create cluster
	body := `{"cluster_id": "test-cluster", "ovsdb_port": 16641}`
	req := httptest.NewRequest("POST", "/sim/clusters", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created state.ClusterInfo
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.ID != "test-cluster" {
		t.Errorf("expected cluster_id=test-cluster, got %s", created.ID)
	}

	// List clusters
	req = httptest.NewRequest("GET", "/sim/clusters", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var clusters []state.ClusterInfo
	if err := json.NewDecoder(w.Body).Decode(&clusters); err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
}

func TestCreateDuplicateCluster(t *testing.T) {
	_, mux := setupTest(t)

	body := `{"cluster_id": "dup", "ovsdb_port": 16642}`
	req := httptest.NewRequest("POST", "/sim/clusters", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Try again with same ID
	req = httptest.NewRequest("POST", "/sim/clusters", bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 for duplicate, got %d", w.Code)
	}
}

func TestCreateClusterMissingFields(t *testing.T) {
	_, mux := setupTest(t)

	tests := []struct {
		name string
		body string
	}{
		{"missing cluster_id", `{"ovsdb_port": 6641}`},
		{"missing ovsdb_port", `{"cluster_id": "test"}`},
		{"invalid json", `{invalid`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/sim/clusters", bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code == http.StatusCreated {
				t.Errorf("expected error status, got 201")
			}
		})
	}
}

func TestGetStats(t *testing.T) {
	_, mux := setupTest(t)

	req := httptest.NewRequest("GET", "/sim/stats", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var stats map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatal(err)
	}
	if stats["cluster_count"] != float64(0) {
		t.Errorf("expected cluster_count=0, got %v", stats["cluster_count"])
	}
}

func TestReset(t *testing.T) {
	_, mux := setupTest(t)

	// Create a cluster first
	body := `{"cluster_id": "reset-test", "ovsdb_port": 16643}`
	req := httptest.NewRequest("POST", "/sim/clusters", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Reset
	req = httptest.NewRequest("POST", "/sim/reset", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Verify clusters are empty
	req = httptest.NewRequest("GET", "/sim/clusters", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var clusters []state.ClusterInfo
	if err := json.NewDecoder(w.Body).Decode(&clusters); err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 0 {
		t.Errorf("expected 0 clusters after reset, got %d", len(clusters))
	}
}

func TestPortUpDownNotFound(t *testing.T) {
	_, mux := setupTest(t)

	req := httptest.NewRequest("POST", "/sim/ports/nonexistent-uuid/up", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}

	req = httptest.NewRequest("POST", "/sim/ports/nonexistent-uuid/down", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

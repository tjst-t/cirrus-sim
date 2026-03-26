package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tjst-t/cirrus-sim/netbox-sim/internal/state"
)

func setupMux() *http.ServeMux {
	store := state.NewStore()
	h := NewHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return mux
}

func TestBulkLoadAndListSites(t *testing.T) {
	mux := setupMux()

	body := `{
		"sites": [{
			"name": "site-tokyo",
			"locations": [{
				"name": "row-A",
				"racks": [{
					"name": "rack-A01",
					"tor_switch": "tor-a01",
					"power_circuit": "pdu-a01-1",
					"devices": [
						{"name": "host-001", "position": 40, "cirrus_host_id": "host-001"},
						{"name": "host-002", "position": 38, "cirrus_host_id": "host-002"}
					]
				}]
			}]
		}]
	}`

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/sim/bulk-load", bytes.NewBufferString(body))
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("bulk-load status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/api/dcim/sites/", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("sites status = %d", w.Code)
	}

	var resp struct {
		Count   int              `json:"count"`
		Results []map[string]any `json:"results"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 1 {
		t.Errorf("site count = %d, want 1", resp.Count)
	}
}

func TestBulkLoadHierarchicalLocations(t *testing.T) {
	mux := setupMux()

	body := `{
		"sites": [{
			"name": "site-tokyo",
			"locations": [{
				"name": "floor-1",
				"power_feed": "pf-main",
				"locations": [{
					"name": "hall-A",
					"locations": [{
						"name": "row-A1",
						"racks": [{
							"name": "rack-A1-01",
							"tor_switch": "tor-a1-01",
							"power_circuit": "pdu-a1-01",
							"devices": [{"name": "host-001", "position": 40, "cirrus_host_id": "host-001"}]
						}]
					}]
				}]
			}]
		}]
	}`

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/sim/bulk-load", bytes.NewBufferString(body))
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d; body: %s", w.Code, w.Body.String())
	}

	var stats map[string]int
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatal(err)
	}
	if stats["location_count"] != 3 {
		t.Errorf("location_count = %d, want 3", stats["location_count"])
	}
	if stats["rack_count"] != 1 {
		t.Errorf("rack_count = %d, want 1", stats["rack_count"])
	}

	// List all locations
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/api/dcim/locations/", nil)
	mux.ServeHTTP(w, r)

	var locResp struct {
		Count   int              `json:"count"`
		Results []map[string]any `json:"results"`
	}
	if err := json.NewDecoder(w.Body).Decode(&locResp); err != nil {
		t.Fatal(err)
	}
	if locResp.Count != 3 {
		t.Errorf("all locations = %d, want 3", locResp.Count)
	}

	// List top-level locations only (parent_id=0)
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/api/dcim/locations/?parent_id=0", nil)
	mux.ServeHTTP(w, r)

	if err := json.NewDecoder(w.Body).Decode(&locResp); err != nil {
		t.Fatal(err)
	}
	if locResp.Count != 1 {
		t.Errorf("top-level locations = %d, want 1", locResp.Count)
	}

	// Verify depth is set
	if locResp.Results[0]["name"] != "floor-1" {
		t.Errorf("top-level name = %v, want floor-1", locResp.Results[0]["name"])
	}
	depth := locResp.Results[0]["_depth"]
	if depth != float64(0) {
		t.Errorf("floor depth = %v, want 0", depth)
	}

	// Verify floor-1 custom_fields has power_feed
	cf, ok := locResp.Results[0]["custom_fields"].(map[string]any)
	if !ok {
		t.Fatal("custom_fields not a map")
	}
	if cf["power_feed"] != "pf-main" {
		t.Errorf("power_feed = %v, want pf-main", cf["power_feed"])
	}
}

func TestListRacksWithFilter(t *testing.T) {
	mux := setupMux()

	body := `{"sites":[{"name":"s1","locations":[{"name":"r1","racks":[{"name":"rack1","devices":[]}]}]},{"name":"s2","locations":[{"name":"r2","racks":[{"name":"rack2","devices":[]}]}]}]}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/sim/bulk-load", bytes.NewBufferString(body))
	mux.ServeHTTP(w, r)

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/api/dcim/racks/", nil)
	mux.ServeHTTP(w, r)

	var resp struct{ Count int }
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 {
		t.Errorf("all racks = %d, want 2", resp.Count)
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/api/dcim/racks/?site_id=1", nil)
	mux.ServeHTTP(w, r)

	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 1 {
		t.Errorf("filtered racks = %d, want 1", resp.Count)
	}
}

func TestListDevicesWithFilter(t *testing.T) {
	mux := setupMux()

	body := `{"sites":[{"name":"s1","locations":[{"name":"r1","racks":[{"name":"rack1","devices":[{"name":"h1","position":1,"cirrus_host_id":"h1"},{"name":"h2","position":2,"cirrus_host_id":"h2"}]}]}]}]}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/sim/bulk-load", bytes.NewBufferString(body))
	mux.ServeHTTP(w, r)

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/api/dcim/devices/", nil)
	mux.ServeHTTP(w, r)

	var resp struct{ Count int }
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 {
		t.Errorf("device count = %d, want 2", resp.Count)
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/api/dcim/devices/?role=server", nil)
	mux.ServeHTTP(w, r)

	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 {
		t.Errorf("server count = %d, want 2", resp.Count)
	}
}

func TestStatsAndReset(t *testing.T) {
	mux := setupMux()

	body := `{"sites":[{"name":"s1","locations":[{"name":"r1","racks":[{"name":"rack1","devices":[{"name":"h1","position":1,"cirrus_host_id":"h1"}]}]}]}]}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/sim/bulk-load", bytes.NewBufferString(body))
	mux.ServeHTTP(w, r)

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/sim/stats", nil)
	mux.ServeHTTP(w, r)

	var stats map[string]int
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatal(err)
	}
	if stats["site_count"] != 1 {
		t.Errorf("site_count = %d, want 1", stats["site_count"])
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/sim/reset", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("reset status = %d", w.Code)
	}
}

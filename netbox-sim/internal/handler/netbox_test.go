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

	// List sites
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/api/dcim/sites/", nil)
	mux.ServeHTTP(w, r)

	var resp struct {
		Count    int              `json:"count"`
		Next     *string          `json:"next"`
		Previous *string          `json:"previous"`
		Results  []map[string]any `json:"results"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 1 {
		t.Errorf("site count = %d, want 1", resp.Count)
	}
	if resp.Next != nil {
		t.Errorf("next should be nil")
	}
	if resp.Previous != nil {
		t.Errorf("previous should be nil")
	}

	site := resp.Results[0]
	// Check required fields exist
	for _, field := range []string{"id", "url", "display_url", "display", "name", "slug", "status", "custom_fields", "created", "last_updated", "rack_count", "device_count"} {
		if _, ok := site[field]; !ok {
			t.Errorf("site missing field %q", field)
		}
	}

	// Check status has value and label
	status := site["status"].(map[string]any)
	if status["value"] != "active" {
		t.Errorf("status.value = %v, want active", status["value"])
	}
	if status["label"] != "Active" {
		t.Errorf("status.label = %v, want Active", status["label"])
	}

	// Check slug
	if site["slug"] != "site-tokyo" {
		t.Errorf("slug = %v, want site-tokyo", site["slug"])
	}

	// Check counts
	if site["rack_count"] != float64(1) {
		t.Errorf("rack_count = %v, want 1", site["rack_count"])
	}
	if site["device_count"] != float64(2) {
		t.Errorf("device_count = %v, want 2", site["device_count"])
	}
}

func TestDeviceResponseV4Format(t *testing.T) {
	mux := setupMux()

	body := `{"sites":[{"name":"s1","locations":[{"name":"row-1","racks":[{"name":"rack1","devices":[{"name":"h1","position":1,"cirrus_host_id":"h1"}]}]}]}]}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/sim/bulk-load", bytes.NewBufferString(body))
	mux.ServeHTTP(w, r)

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/api/dcim/devices/", nil)
	mux.ServeHTTP(w, r)

	var resp struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	dev := resp.Results[0]

	// Must use "role" not "device_role" (v4)
	if _, ok := dev["device_role"]; ok {
		t.Error("response should not have device_role (v4 uses role)")
	}
	role, ok := dev["role"].(map[string]any)
	if !ok {
		t.Fatal("role field missing or not an object")
	}
	if role["name"] != "server" {
		t.Errorf("role.name = %v, want server", role["name"])
	}
	if role["slug"] != "server" {
		t.Errorf("role.slug = %v, want server", role["slug"])
	}

	// Must have url, display_url, display
	for _, field := range []string{"url", "display_url", "display", "created", "last_updated"} {
		if _, ok := dev[field]; !ok {
			t.Errorf("device missing field %q", field)
		}
	}

	// Must have location reference
	if dev["location"] == nil {
		t.Error("device should have location reference")
	}

	// Must have face
	face, ok := dev["face"].(map[string]any)
	if !ok {
		t.Fatal("face missing")
	}
	if face["value"] != "front" {
		t.Errorf("face.value = %v, want front", face["value"])
	}

	// Rack should be a nested ref with name and device_count
	rack, ok := dev["rack"].(map[string]any)
	if !ok {
		t.Fatal("rack missing")
	}
	if _, ok := rack["name"]; !ok {
		t.Error("rack nested ref missing name")
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

	// Check location has all v4 fields
	for _, loc := range locResp.Results {
		for _, field := range []string{"id", "url", "display_url", "display", "name", "slug", "site", "status", "created", "last_updated", "rack_count", "device_count", "_depth"} {
			if _, ok := loc[field]; !ok {
				t.Errorf("location %v missing field %q", loc["name"], field)
			}
		}
	}

	// List top-level only
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/api/dcim/locations/?parent_id=0", nil)
	mux.ServeHTTP(w, r)

	if err := json.NewDecoder(w.Body).Decode(&locResp); err != nil {
		t.Fatal(err)
	}
	if locResp.Count != 1 {
		t.Errorf("top-level locations = %d, want 1", locResp.Count)
	}

	// Verify depth
	if locResp.Results[0]["_depth"] != float64(0) {
		t.Errorf("floor depth = %v, want 0", locResp.Results[0]["_depth"])
	}

	// Verify parent field is null for top-level
	if locResp.Results[0]["parent"] != nil {
		t.Errorf("top-level parent should be null")
	}
}

func TestListRacksWithFilter(t *testing.T) {
	mux := setupMux()

	body := `{"sites":[{"name":"s1","locations":[{"name":"r1","racks":[{"name":"rack1","devices":[]}]}]},{"name":"s2","locations":[{"name":"r2","racks":[{"name":"rack2","devices":[]}]}]}]}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/sim/bulk-load", bytes.NewBufferString(body))
	mux.ServeHTTP(w, r)

	// All racks
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

	// Filtered by site
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

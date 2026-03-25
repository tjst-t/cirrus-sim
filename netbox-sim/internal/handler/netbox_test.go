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

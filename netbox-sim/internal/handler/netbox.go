// Package handler provides HTTP handlers for the NetBox simulator API.
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/tjst-t/cirrus-sim/netbox-sim/internal/state"
)

// Handler provides NetBox REST API handlers.
type Handler struct {
	store *state.Store
}

// NewHandler creates a new Handler backed by the given store.
func NewHandler(store *state.Store) *Handler {
	return &Handler{store: store}
}

// RegisterRoutes registers all NetBox API routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/dcim/sites/", h.listSites)
	mux.HandleFunc("GET /api/dcim/racks/", h.listRacks)
	mux.HandleFunc("GET /api/dcim/devices/", h.listDevices)
	mux.HandleFunc("POST /sim/bulk-load", h.bulkLoad)
	mux.HandleFunc("GET /sim/stats", h.getStats)
	mux.HandleFunc("POST /sim/reset", h.resetState)
}

func (h *Handler) listSites(w http.ResponseWriter, _ *http.Request) {
	sites := h.store.ListSites()
	results := make([]siteResponse, 0, len(sites))
	for _, s := range sites {
		results = append(results, toSiteResponse(s))
	}
	writeJSON(w, http.StatusOK, listResponse{Count: len(results), Results: results})
}

func (h *Handler) listRacks(w http.ResponseWriter, r *http.Request) {
	var siteID int
	if v := r.URL.Query().Get("site_id"); v != "" {
		var err error
		siteID, err = strconv.Atoi(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid site_id")
			return
		}
	}

	racks := h.store.ListRacks(siteID)
	results := make([]rackResponse, 0, len(racks))
	for _, rk := range racks {
		results = append(results, h.toRackResponse(rk))
	}
	writeJSON(w, http.StatusOK, listResponse{Count: len(results), Results: results})
}

func (h *Handler) listDevices(w http.ResponseWriter, r *http.Request) {
	var rackID int
	if v := r.URL.Query().Get("rack_id"); v != "" {
		var err error
		rackID, err = strconv.Atoi(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid rack_id")
			return
		}
	}
	role := r.URL.Query().Get("role")

	devices := h.store.ListDevices(rackID, role)
	results := make([]deviceResponse, 0, len(devices))
	for _, d := range devices {
		results = append(results, h.toDeviceResponse(d))
	}
	writeJSON(w, http.StatusOK, listResponse{Count: len(results), Results: results})
}

type bulkLoadRequest struct {
	Sites []bulkSite `json:"sites"`
}

type bulkSite struct {
	Name      string         `json:"name"`
	Locations []bulkLocation `json:"locations"`
}

type bulkLocation struct {
	Name  string     `json:"name"`
	Racks []bulkRack `json:"racks"`
}

type bulkRack struct {
	Name         string       `json:"name"`
	TorSwitch    string       `json:"tor_switch"`
	PowerCircuit string       `json:"power_circuit"`
	Devices      []bulkDevice `json:"devices"`
}

type bulkDevice struct {
	Name         string `json:"name"`
	Position     int    `json:"position"`
	CirrusHostID string `json:"cirrus_host_id"`
}

func (h *Handler) bulkLoad(w http.ResponseWriter, r *http.Request) {
	var req bulkLoadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	for _, site := range req.Sites {
		siteID := h.store.AddSite(site.Name, "active", nil, nil)
		for _, loc := range site.Locations {
			locID := h.store.AddLocation(loc.Name, siteID)
			for _, rack := range loc.Racks {
				cf := map[string]string{}
				if rack.TorSwitch != "" {
					cf["tor_switch"] = rack.TorSwitch
				}
				if rack.PowerCircuit != "" {
					cf["power_circuit"] = rack.PowerCircuit
				}
				rackID := h.store.AddRack(rack.Name, siteID, locID, "active", cf)
				for _, dev := range rack.Devices {
					dcf := map[string]string{}
					if dev.CirrusHostID != "" {
						dcf["cirrus_host_id"] = dev.CirrusHostID
					}
					h.store.AddDevice(dev.Name, "server", siteID, rackID, dev.Position, "active", dcf)
				}
			}
		}
	}

	stats := h.store.GetStats()
	writeJSON(w, http.StatusCreated, stats)
}

func (h *Handler) getStats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.store.GetStats())
}

func (h *Handler) resetState(w http.ResponseWriter, _ *http.Request) {
	h.store.Reset()
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

// Response types matching NetBox API format.

type listResponse struct {
	Count   int         `json:"count"`
	Results interface{} `json:"results"`
}

type statusValue struct {
	Value string `json:"value"`
}

type siteResponse struct {
	ID           int               `json:"id"`
	Name         string            `json:"name"`
	Status       statusValue       `json:"status"`
	Region       *state.NamedRef   `json:"region"`
	CustomFields map[string]string `json:"custom_fields"`
}

func toSiteResponse(s *state.Site) siteResponse {
	return siteResponse{
		ID:           s.ID,
		Name:         s.Name,
		Status:       statusValue{Value: s.Status},
		Region:       s.Region,
		CustomFields: s.CustomFields,
	}
}

type rackResponse struct {
	ID           int               `json:"id"`
	Name         string            `json:"name"`
	Site         state.NamedRef    `json:"site"`
	Location     *state.NamedRef   `json:"location"`
	Status       statusValue       `json:"status"`
	CustomFields map[string]string `json:"custom_fields"`
}

func (h *Handler) toRackResponse(rk *state.Rack) rackResponse {
	resp := rackResponse{
		ID:           rk.ID,
		Name:         rk.Name,
		Status:       statusValue{Value: rk.Status},
		CustomFields: rk.CustomFields,
	}
	if site := h.store.GetSite(rk.SiteID); site != nil {
		resp.Site = state.NamedRef{ID: site.ID, Name: site.Name}
	}
	if loc := h.store.GetLocation(rk.LocationID); loc != nil {
		resp.Location = &state.NamedRef{ID: loc.ID, Name: loc.Name}
	}
	return resp
}

type deviceResponse struct {
	ID           int               `json:"id"`
	Name         string            `json:"name"`
	DeviceRole   state.NamedRef    `json:"device_role"`
	Site         state.NamedRef    `json:"site"`
	Rack         state.NamedRef    `json:"rack"`
	Position     int               `json:"position"`
	Status       statusValue       `json:"status"`
	CustomFields map[string]string `json:"custom_fields"`
}

func (h *Handler) toDeviceResponse(d *state.Device) deviceResponse {
	resp := deviceResponse{
		ID:           d.ID,
		Name:         d.Name,
		DeviceRole:   state.NamedRef{Name: d.DeviceRole},
		Position:     d.Position,
		Status:       statusValue{Value: d.Status},
		CustomFields: d.CustomFields,
	}
	if site := h.store.GetSite(d.SiteID); site != nil {
		resp.Site = state.NamedRef{ID: site.ID, Name: site.Name}
	}
	if rk := h.store.GetRack(d.RackID); rk != nil {
		resp.Rack = state.NamedRef{ID: rk.ID, Name: rk.Name}
	}
	return resp
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("failed to write response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		slog.Warn("failed to write error response", "error", err)
	}
}

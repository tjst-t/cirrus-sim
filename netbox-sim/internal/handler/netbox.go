// Package handler provides HTTP handlers for the NetBox simulator API.
package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tjst-t/cirrus-sim/netbox-sim/internal/state"
)

// Handler provides NetBox REST API handlers.
type Handler struct {
	store   *state.Store
	baseURL string
}

// NewHandler creates a new Handler backed by the given store.
func NewHandler(store *state.Store) *Handler {
	return &Handler{store: store, baseURL: "/api"}
}

// RegisterRoutes registers all NetBox API routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/dcim/sites/", h.listSites)
	mux.HandleFunc("GET /api/dcim/locations/", h.listLocations)
	mux.HandleFunc("GET /api/dcim/racks/", h.listRacks)
	mux.HandleFunc("GET /api/dcim/devices/", h.listDevices)
	mux.HandleFunc("POST /sim/bulk-load", h.bulkLoad)
	mux.HandleFunc("GET /sim/stats", h.getStats)
	mux.HandleFunc("POST /sim/reset", h.resetState)
}

// ── List handlers ──

func (h *Handler) listSites(w http.ResponseWriter, _ *http.Request) {
	sites := h.store.ListSites()
	results := make([]siteResponse, 0, len(sites))
	for _, s := range sites {
		results = append(results, h.toSiteResponse(s))
	}
	writeJSON(w, http.StatusOK, listResponse{Count: len(results), Results: results})
}

func (h *Handler) listLocations(w http.ResponseWriter, r *http.Request) {
	siteID := queryInt(r, "site_id", 0)
	parentID := queryInt(r, "parent_id", -1)

	locs := h.store.ListLocations(siteID, parentID)
	results := make([]locationResponse, 0, len(locs))
	for _, loc := range locs {
		results = append(results, h.toLocationResponse(loc))
	}
	writeJSON(w, http.StatusOK, listResponse{Count: len(results), Results: results})
}

func (h *Handler) listRacks(w http.ResponseWriter, r *http.Request) {
	siteID := queryInt(r, "site_id", 0)
	racks := h.store.ListRacks(siteID)
	results := make([]rackResponse, 0, len(racks))
	for _, rk := range racks {
		results = append(results, h.toRackResponse(rk))
	}
	writeJSON(w, http.StatusOK, listResponse{Count: len(results), Results: results})
}

func (h *Handler) listDevices(w http.ResponseWriter, r *http.Request) {
	rackID := queryInt(r, "rack_id", 0)
	role := r.URL.Query().Get("role")
	devices := h.store.ListDevices(rackID, role)
	results := make([]deviceResponse, 0, len(devices))
	for _, d := range devices {
		results = append(results, h.toDeviceResponse(d))
	}
	writeJSON(w, http.StatusOK, listResponse{Count: len(results), Results: results})
}

// ── Bulk load ──

type bulkLoadRequest struct {
	Sites []bulkSite `json:"sites"`
}

type bulkSite struct {
	Name      string         `json:"name"`
	Locations []bulkLocation `json:"locations"`
}

type bulkLocation struct {
	Name      string         `json:"name"`
	PowerFeed string         `json:"power_feed,omitempty"`
	Locations []bulkLocation `json:"locations,omitempty"`
	Racks     []bulkRack     `json:"racks,omitempty"`
}

type bulkRack struct {
	Name         string       `json:"name"`
	TorSwitch    string       `json:"tor_switch"`
	PowerCircuit string       `json:"power_circuit"`
	Devices      []bulkDevice `json:"devices"`
}

type bulkDevice struct {
	Name         string `json:"name"`
	DeviceRole   string `json:"device_role,omitempty"`
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
			h.loadLocation(siteID, 0, loc)
		}
	}

	stats := h.store.GetStats()
	writeJSON(w, http.StatusCreated, stats)
}

func (h *Handler) loadLocation(siteID, parentID int, loc bulkLocation) {
	locCF := map[string]string{}
	if loc.PowerFeed != "" {
		locCF["power_feed"] = loc.PowerFeed
	}
	locID := h.store.AddLocation(loc.Name, siteID, parentID, locCF)

	for _, child := range loc.Locations {
		h.loadLocation(siteID, locID, child)
	}

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
			role := dev.DeviceRole
			if role == "" {
				role = "server"
			}
			h.store.AddDevice(dev.Name, role, siteID, locID, rackID, dev.Position, "active", dcf)
		}
	}
}

func (h *Handler) getStats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.store.GetStats())
}

func (h *Handler) resetState(w http.ResponseWriter, _ *http.Request) {
	h.store.Reset()
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

// ── Response types (NetBox v4 compatible) ──

type listResponse struct {
	Count    int         `json:"count"`
	Next     *string     `json:"next"`
	Previous *string     `json:"previous"`
	Results  interface{} `json:"results"`
}

type statusValue struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

func makeStatus(value string) statusValue {
	label := strings.ToUpper(value[:1]) + value[1:]
	return statusValue{Value: value, Label: label}
}

// ── Nested references ──

type nestedRef struct {
	ID         int    `json:"id"`
	URL        string `json:"url"`
	DisplayURL string `json:"display_url"`
	Display    string `json:"display"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
}

type nestedLocationRef struct {
	ID         int    `json:"id"`
	URL        string `json:"url"`
	DisplayURL string `json:"display_url"`
	Display    string `json:"display"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	RackCount  int    `json:"rack_count"`
	Depth      int    `json:"_depth"`
}

type nestedRoleRef struct {
	ID         int    `json:"id"`
	URL        string `json:"url"`
	DisplayURL string `json:"display_url"`
	Display    string `json:"display"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
}

func (h *Handler) siteRef(siteID int) *nestedRef {
	site := h.store.GetSite(siteID)
	if site == nil {
		return nil
	}
	return &nestedRef{
		ID:         site.ID,
		URL:        fmt.Sprintf("%s/dcim/sites/%d/", h.baseURL, site.ID),
		DisplayURL: fmt.Sprintf("%s/dcim/sites/%d/", h.baseURL, site.ID),
		Display:    site.Name,
		Name:       site.Name,
		Slug:       site.Slug,
	}
}

func (h *Handler) locationRef(locID int) *nestedLocationRef {
	loc := h.store.GetLocation(locID)
	if loc == nil {
		return nil
	}
	depth := len(h.store.LocationAncestors(loc.ID)) - 1
	return &nestedLocationRef{
		ID:         loc.ID,
		URL:        fmt.Sprintf("%s/dcim/locations/%d/", h.baseURL, loc.ID),
		DisplayURL: fmt.Sprintf("%s/dcim/locations/%d/", h.baseURL, loc.ID),
		Display:    loc.Name,
		Name:       loc.Name,
		Slug:       loc.Slug,
		RackCount:  h.store.CountRacksInLocation(loc.ID),
		Depth:      depth,
	}
}

func (h *Handler) rackRef(rackID int) *nestedRef {
	rk := h.store.GetRack(rackID)
	if rk == nil {
		return nil
	}
	return &nestedRef{
		ID:         rk.ID,
		URL:        fmt.Sprintf("%s/dcim/racks/%d/", h.baseURL, rk.ID),
		DisplayURL: fmt.Sprintf("%s/dcim/racks/%d/", h.baseURL, rk.ID),
		Display:    rk.Name,
		Name:       rk.Name,
		Slug:       rk.Slug,
	}
}

func roleRef(name string) nestedRoleRef {
	slug := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	return nestedRoleRef{
		ID:         0,
		URL:        "",
		DisplayURL: "",
		Display:    name,
		Name:       name,
		Slug:       slug,
	}
}

// ── Site response ──

type siteResponse struct {
	ID           int               `json:"id"`
	URL          string            `json:"url"`
	DisplayURL   string            `json:"display_url"`
	Display      string            `json:"display"`
	Name         string            `json:"name"`
	Slug         string            `json:"slug"`
	Status       statusValue       `json:"status"`
	Region       *state.NamedRef   `json:"region"`
	Tenant       *interface{}      `json:"tenant"`
	Facility     string            `json:"facility"`
	Description  string            `json:"description"`
	Tags         []interface{}     `json:"tags"`
	CustomFields map[string]string `json:"custom_fields"`
	Created      string            `json:"created"`
	LastUpdated  string            `json:"last_updated"`
	RackCount    int               `json:"rack_count"`
	DeviceCount  int               `json:"device_count"`
}

func (h *Handler) toSiteResponse(s *state.Site) siteResponse {
	return siteResponse{
		ID:           s.ID,
		URL:          fmt.Sprintf("%s/dcim/sites/%d/", h.baseURL, s.ID),
		DisplayURL:   fmt.Sprintf("%s/dcim/sites/%d/", h.baseURL, s.ID),
		Display:      s.Name,
		Name:         s.Name,
		Slug:         s.Slug,
		Status:       makeStatus(s.Status),
		Region:       s.Region,
		Facility:     "",
		Description:  s.Description,
		Tags:         s.Tags,
		CustomFields: s.CustomFields,
		Created:      s.CreatedAt.Format(time.DateOnly),
		LastUpdated:  s.LastUpdated.Format(time.RFC3339),
		RackCount:    h.store.CountRacksInSite(s.ID),
		DeviceCount:  h.store.CountDevicesInSite(s.ID),
	}
}

// ── Location response ──

type locationResponse struct {
	ID           int                `json:"id"`
	URL          string             `json:"url"`
	DisplayURL   string             `json:"display_url"`
	Display      string             `json:"display"`
	Name         string             `json:"name"`
	Slug         string             `json:"slug"`
	Site         *nestedRef         `json:"site"`
	Parent       *nestedLocationRef `json:"parent"`
	Status       statusValue        `json:"status"`
	Tenant       *interface{}       `json:"tenant"`
	Description  string             `json:"description"`
	Tags         []interface{}      `json:"tags"`
	CustomFields map[string]string  `json:"custom_fields"`
	Created      string             `json:"created"`
	LastUpdated  string             `json:"last_updated"`
	RackCount    int                `json:"rack_count"`
	DeviceCount  int                `json:"device_count"`
	Depth        int                `json:"_depth"`
}

func (h *Handler) toLocationResponse(loc *state.Location) locationResponse {
	var parent *nestedLocationRef
	if loc.ParentID != 0 {
		parent = h.locationRef(loc.ParentID)
	}
	depth := len(h.store.LocationAncestors(loc.ID)) - 1
	return locationResponse{
		ID:           loc.ID,
		URL:          fmt.Sprintf("%s/dcim/locations/%d/", h.baseURL, loc.ID),
		DisplayURL:   fmt.Sprintf("%s/dcim/locations/%d/", h.baseURL, loc.ID),
		Display:      loc.Name,
		Name:         loc.Name,
		Slug:         loc.Slug,
		Site:         h.siteRef(loc.SiteID),
		Parent:       parent,
		Status:       makeStatus(loc.Status),
		Description:  loc.Description,
		Tags:         loc.Tags,
		CustomFields: loc.CustomFields,
		Created:      loc.CreatedAt.Format(time.DateOnly),
		LastUpdated:  loc.LastUpdated.Format(time.RFC3339),
		RackCount:    h.store.CountRacksInLocation(loc.ID),
		DeviceCount:  h.store.CountDevicesInLocation(loc.ID),
		Depth:        depth,
	}
}

// ── Rack response ──

type rackResponse struct {
	ID           int                `json:"id"`
	URL          string             `json:"url"`
	DisplayURL   string             `json:"display_url"`
	Display      string             `json:"display"`
	Name         string             `json:"name"`
	Site         *nestedRef         `json:"site"`
	Location     *nestedLocationRef `json:"location"`
	Status       statusValue        `json:"status"`
	Tenant       *interface{}       `json:"tenant"`
	UHeight      int                `json:"u_height"`
	Description  string             `json:"description"`
	Tags         []interface{}      `json:"tags"`
	CustomFields map[string]string  `json:"custom_fields"`
	Created      string             `json:"created"`
	LastUpdated  string             `json:"last_updated"`
	DeviceCount  int                `json:"device_count"`
}

func (h *Handler) toRackResponse(rk *state.Rack) rackResponse {
	var loc *nestedLocationRef
	if rk.LocationID != 0 {
		loc = h.locationRef(rk.LocationID)
	}
	return rackResponse{
		ID:           rk.ID,
		URL:          fmt.Sprintf("%s/dcim/racks/%d/", h.baseURL, rk.ID),
		DisplayURL:   fmt.Sprintf("%s/dcim/racks/%d/", h.baseURL, rk.ID),
		Display:      rk.Name,
		Name:         rk.Name,
		Site:         h.siteRef(rk.SiteID),
		Location:     loc,
		Status:       makeStatus(rk.Status),
		UHeight:      rk.UHeight,
		Description:  rk.Description,
		Tags:         rk.Tags,
		CustomFields: rk.CustomFields,
		Created:      rk.CreatedAt.Format(time.DateOnly),
		LastUpdated:  rk.LastUpdated.Format(time.RFC3339),
		DeviceCount:  h.store.CountDevicesInRack(rk.ID),
	}
}

// ── Device response ──

type deviceResponse struct {
	ID           int                `json:"id"`
	URL          string             `json:"url"`
	DisplayURL   string             `json:"display_url"`
	Display      string             `json:"display"`
	Name         string             `json:"name"`
	Role         nestedRoleRef      `json:"role"`
	Site         *nestedRef         `json:"site"`
	Location     *nestedLocationRef `json:"location"`
	Rack         *nestedRef         `json:"rack"`
	Position     *int               `json:"position"`
	Face         statusValue        `json:"face"`
	Status       statusValue        `json:"status"`
	Tenant       *interface{}       `json:"tenant"`
	Description  string             `json:"description"`
	Tags         []interface{}      `json:"tags"`
	CustomFields map[string]string  `json:"custom_fields"`
	Created      string             `json:"created"`
	LastUpdated  string             `json:"last_updated"`
}

func (h *Handler) toDeviceResponse(d *state.Device) deviceResponse {
	var loc *nestedLocationRef
	if d.LocationID != 0 {
		loc = h.locationRef(d.LocationID)
	}
	var pos *int
	if d.Position > 0 {
		pos = &d.Position
	}
	return deviceResponse{
		ID:           d.ID,
		URL:          fmt.Sprintf("%s/dcim/devices/%d/", h.baseURL, d.ID),
		DisplayURL:   fmt.Sprintf("%s/dcim/devices/%d/", h.baseURL, d.ID),
		Display:      d.Name,
		Name:         d.Name,
		Role:         roleRef(d.Role),
		Site:         h.siteRef(d.SiteID),
		Location:     loc,
		Rack:         h.rackRef(d.RackID),
		Position:     pos,
		Face:         makeStatus(d.Face),
		Status:       makeStatus(d.Status),
		Description:  d.Description,
		Tags:         d.Tags,
		CustomFields: d.CustomFields,
		Created:      d.CreatedAt.Format(time.DateOnly),
		LastUpdated:  d.LastUpdated.Format(time.RFC3339),
	}
}

// ── Helpers ──

func queryInt(r *http.Request, key string, defaultVal int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
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

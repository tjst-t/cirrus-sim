// Package handler provides the management REST API for ovn-sim.
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/tjst-t/cirrus-sim/ovn-sim/internal/state"
)

// Management handles the /sim/ management API endpoints.
type Management struct {
	manager *state.Manager
	ctx     context.Context
	logger  *slog.Logger
}

// NewManagement creates a new Management handler.
func NewManagement(ctx context.Context, manager *state.Manager, logger *slog.Logger) *Management {
	return &Management{
		manager: manager,
		ctx:     ctx,
		logger:  logger,
	}
}

// RegisterRoutes registers all management API routes on the given mux.
func (m *Management) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /sim/clusters", m.createCluster)
	mux.HandleFunc("GET /sim/clusters", m.listClusters)
	mux.HandleFunc("POST /sim/ports/{uuid}/up", m.setPortUp)
	mux.HandleFunc("POST /sim/ports/{uuid}/down", m.setPortDown)
	mux.HandleFunc("GET /sim/stats", m.getStats)
	mux.HandleFunc("POST /sim/reset", m.reset)
}

type createClusterRequest struct {
	ClusterID string `json:"cluster_id"`
	OVSDBPort int    `json:"ovsdb_port"`
}

func (m *Management) createCluster(w http.ResponseWriter, r *http.Request) {
	var req createClusterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	if req.ClusterID == "" {
		writeError(w, http.StatusBadRequest, "cluster_id is required")
		return
	}
	if req.OVSDBPort == 0 {
		writeError(w, http.StatusBadRequest, "ovsdb_port is required")
		return
	}

	cluster, err := m.manager.CreateCluster(m.ctx, req.ClusterID, req.OVSDBPort)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, state.ClusterInfo{
		ID:   cluster.ID,
		Port: cluster.Port,
	})
}

func (m *Management) listClusters(w http.ResponseWriter, _ *http.Request) {
	clusters := m.manager.ListClusters()
	writeJSON(w, http.StatusOK, clusters)
}

func (m *Management) setPortUp(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	if uuid == "" {
		writeError(w, http.StatusBadRequest, "port uuid is required")
		return
	}

	if !m.manager.SetPortUp(uuid) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("port %s not found", uuid))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "up", "uuid": uuid})
}

func (m *Management) setPortDown(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	if uuid == "" {
		writeError(w, http.StatusBadRequest, "port uuid is required")
		return
	}

	if !m.manager.SetPortDown(uuid) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("port %s not found", uuid))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "down", "uuid": uuid})
}

func (m *Management) getStats(w http.ResponseWriter, _ *http.Request) {
	stats := m.manager.Stats()
	writeJSON(w, http.StatusOK, stats)
}

func (m *Management) reset(w http.ResponseWriter, _ *http.Request) {
	m.manager.Reset()
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Warn("failed to write JSON response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

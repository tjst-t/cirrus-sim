// Package handler provides HTTP handlers for the load generator API.
package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/tjst-t/cirrus-sim/load-gen/internal/engine"
)

// Handler provides load generator API handlers.
type Handler struct {
	engine *engine.Engine
}

// NewHandler creates a new Handler.
func NewHandler(e *engine.Engine) *Handler {
	return &Handler{engine: e}
}

// RegisterRoutes registers all load generator routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/workloads/run", h.runWorkload)
	mux.HandleFunc("GET /api/v1/workloads/run/{run_id}", h.getRun)
}

func (h *Handler) runWorkload(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	workload, err := engine.ParseWorkload(data)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.engine.RunWorkload(r.Context(), workload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, result)
}

func (h *Handler) getRun(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("run_id")

	result, err := h.engine.GetRun(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
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

// Package handler provides HTTP handlers for the Cirrus Storage API.
package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/tjst-t/cirrus-sim/storage-sim/internal/state"
)

// StorageHandler handles Cirrus Storage API requests under /api/v1/.
type StorageHandler struct {
	store  *state.Store
	logger *slog.Logger
}

// NewStorageHandler creates a new StorageHandler.
func NewStorageHandler(store *state.Store, logger *slog.Logger) *StorageHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &StorageHandler{store: store, logger: logger}
}

// RegisterRoutes registers all storage API routes on the given mux.
func (h *StorageHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/backend/info", h.handleBackendInfo)
	mux.HandleFunc("GET /api/v1/backend/health", h.handleBackendHealth)
	mux.HandleFunc("POST /api/v1/volumes", h.handleCreateVolume)
	mux.HandleFunc("GET /api/v1/volumes/{volume_id}", h.handleGetVolume)
	mux.HandleFunc("GET /api/v1/volumes", h.handleListVolumes)
	mux.HandleFunc("DELETE /api/v1/volumes/{volume_id}", h.handleDeleteVolume)
	mux.HandleFunc("PUT /api/v1/volumes/{volume_id}/extend", h.handleExtendVolume)
	mux.HandleFunc("POST /api/v1/volumes/{volume_id}/export", h.handleExportVolume)
	mux.HandleFunc("DELETE /api/v1/volumes/{volume_id}/export", h.handleUnexportVolume)
}

func (h *StorageHandler) handleBackendInfo(w http.ResponseWriter, r *http.Request) {
	backendID := r.Header.Get("X-Backend-Id")
	if backendID == "" {
		writeError(w, http.StatusBadRequest, "X-Backend-Id header is required")
		return
	}

	b, err := h.store.GetBackend(r.Context(), backendID)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("backend not found: %s", backendID))
		return
	}

	writeJSON(w, http.StatusOK, b)
}

func (h *StorageHandler) handleBackendHealth(w http.ResponseWriter, r *http.Request) {
	backendID := r.Header.Get("X-Backend-Id")
	if backendID == "" {
		writeError(w, http.StatusBadRequest, "X-Backend-Id header is required")
		return
	}

	_, err := h.store.GetBackend(r.Context(), backendID)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("backend not found: %s", backendID))
		return
	}

	cfg := h.store.GetConfig(r.Context())

	resp := map[string]any{
		"healthy":    true,
		"latency_ms": cfg.DefaultLatencyMs,
		"last_check": time.Now().UTC().Format(time.RFC3339),
	}
	writeJSON(w, http.StatusOK, resp)
}

type createVolumeRequest struct {
	VolumeID        string            `json:"volume_id"`
	SizeGB          int64             `json:"size_gb"`
	ThinProvisioned bool              `json:"thin_provisioned"`
	QoSPolicy       *state.QoSPolicy  `json:"qos_policy,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

func (h *StorageHandler) handleCreateVolume(w http.ResponseWriter, r *http.Request) {
	backendID := r.Header.Get("X-Backend-Id")
	if backendID == "" {
		writeError(w, http.StatusBadRequest, "X-Backend-Id header is required")
		return
	}

	var req createVolumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	vol := state.Volume{
		VolumeID:        req.VolumeID,
		BackendID:       backendID,
		SizeGB:          req.SizeGB,
		ThinProvisioned: req.ThinProvisioned,
		QoSPolicy:       req.QoSPolicy,
		Metadata:        req.Metadata,
	}

	created, err := h.store.CreateVolume(r.Context(), vol)
	if err != nil {
		code := errorToHTTPStatus(err)
		writeError(w, code, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

func (h *StorageHandler) handleGetVolume(w http.ResponseWriter, r *http.Request) {
	volumeID := r.PathValue("volume_id")
	v, err := h.store.GetVolume(r.Context(), volumeID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h *StorageHandler) handleListVolumes(w http.ResponseWriter, r *http.Request) {
	backendID := r.Header.Get("X-Backend-Id")
	stateFilter := state.VolumeState(r.URL.Query().Get("state"))

	volumes := h.store.ListVolumes(r.Context(), backendID, stateFilter)
	writeJSON(w, http.StatusOK, volumes)
}

func (h *StorageHandler) handleDeleteVolume(w http.ResponseWriter, r *http.Request) {
	volumeID := r.PathValue("volume_id")
	err := h.store.DeleteVolume(r.Context(), volumeID)
	if err != nil {
		code := errorToHTTPStatus(err)
		writeError(w, code, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type extendVolumeRequest struct {
	NewSizeGB int64 `json:"new_size_gb"`
}

func (h *StorageHandler) handleExtendVolume(w http.ResponseWriter, r *http.Request) {
	volumeID := r.PathValue("volume_id")

	var req extendVolumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	v, err := h.store.ExtendVolume(r.Context(), volumeID, req.NewSizeGB)
	if err != nil {
		code := errorToHTTPStatus(err)
		writeError(w, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

type exportVolumeRequest struct {
	HostID   string `json:"host_id"`
	Protocol string `json:"protocol"`
}

func (h *StorageHandler) handleExportVolume(w http.ResponseWriter, r *http.Request) {
	volumeID := r.PathValue("volume_id")

	var req exportVolumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	v, err := h.store.ExportVolume(r.Context(), volumeID, req.HostID, req.Protocol)
	if err != nil {
		code := errorToHTTPStatus(err)
		writeError(w, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h *StorageHandler) handleUnexportVolume(w http.ResponseWriter, r *http.Request) {
	volumeID := r.PathValue("volume_id")

	v, err := h.store.UnexportVolume(r.Context(), volumeID)
	if err != nil {
		code := errorToHTTPStatus(err)
		writeError(w, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// errorToHTTPStatus maps state errors to HTTP status codes.
func errorToHTTPStatus(err error) int {
	switch {
	case containsErr(err, state.ErrVolumeNotFound), containsErr(err, state.ErrBackendNotFound):
		return http.StatusNotFound
	case containsErr(err, state.ErrVolumeInUse):
		return http.StatusNotAcceptable // 406
	case containsErr(err, state.ErrVolumeHasSnapshots):
		return http.StatusConflict // 409
	case containsErr(err, state.ErrVolumeAlreadyExported), containsErr(err, state.ErrVolumeNotExported):
		return http.StatusConflict
	case containsErr(err, state.ErrShrinkNotAllowed):
		return http.StatusBadRequest
	case containsErr(err, state.ErrInsufficientCapacity):
		return http.StatusInsufficientStorage // 507
	case containsErr(err, state.ErrBackendNotActive):
		return http.StatusServiceUnavailable // 503
	case containsErr(err, state.ErrVolumeExists), containsErr(err, state.ErrBackendExists):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func containsErr(err, target error) bool {
	for e := err; e != nil; e = unwrapSingle(e) {
		if e == target {
			return true
		}
	}
	return false
}

func unwrapSingle(err error) error {
	u, ok := err.(interface{ Unwrap() error })
	if !ok {
		return nil
	}
	return u.Unwrap()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Default().Error("failed to encode JSON response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		slog.Default().Error("failed to encode error response", "error", err)
	}
}

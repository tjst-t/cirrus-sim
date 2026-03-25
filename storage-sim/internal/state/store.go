// Package state provides in-memory storage for backends and volumes.
package state

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// BackendState represents the operational state of a storage backend.
type BackendState string

const (
	// BackendActive indicates the backend is fully operational.
	BackendActive BackendState = "active"
	// BackendDraining indicates the backend is draining (no new volumes).
	BackendDraining BackendState = "draining"
	// BackendReadOnly indicates the backend is read-only.
	BackendReadOnly BackendState = "read_only"
	// BackendRetired indicates the backend is retired.
	BackendRetired BackendState = "retired"
)

// VolumeState represents the state of a volume.
type VolumeState string

const (
	// VolumeAvailable indicates the volume is available for use.
	VolumeAvailable VolumeState = "available"
	// VolumeInUse indicates the volume is currently exported.
	VolumeInUse VolumeState = "in_use"
)

// QoSPolicy defines quality of service limits for a volume.
type QoSPolicy struct {
	MaxIOPS         int `json:"max_iops,omitempty"`
	MaxBandwidthMBs int `json:"max_bandwidth_mbps,omitempty"`
}

// Backend represents a storage backend.
type Backend struct {
	BackendID          string       `json:"backend_id"`
	TotalCapacityGB    int64        `json:"total_capacity_gb"`
	UsedCapacityGB     int64        `json:"used_capacity_gb"`
	AllocatedCapacityGB int64       `json:"allocated_capacity_gb"`
	TotalIOPS          int64        `json:"total_iops"`
	UsedIOPSEstimate   int64        `json:"used_iops_estimate"`
	Capabilities       []string     `json:"capabilities"`
	State              BackendState `json:"state"`
	OverprovisionRatio float64      `json:"overprovision_ratio"`
}

// Volume represents a storage volume.
type Volume struct {
	VolumeID        string            `json:"volume_id"`
	BackendID       string            `json:"backend_id"`
	SizeGB          int64             `json:"size_gb"`
	ConsumedGB      int64             `json:"consumed_gb"`
	ThinProvisioned bool              `json:"thin_provisioned"`
	State           VolumeState       `json:"state"`
	QoSPolicy       *QoSPolicy        `json:"qos_policy,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	ExportInfo      *ExportInfo       `json:"export_info,omitempty"`
	Snapshots       []string          `json:"snapshots,omitempty"`
}

// ExportInfo holds details about an exported volume.
type ExportInfo struct {
	HostID   string `json:"host_id"`
	Protocol string `json:"protocol"`
}

// SimConfig holds simulation configuration.
type SimConfig struct {
	DefaultLatencyMs int `json:"default_latency_ms"`
}

// Stats holds overall simulation statistics.
type Stats struct {
	BackendCount int `json:"backend_count"`
	VolumeCount  int `json:"volume_count"`
	ExportCount  int `json:"export_count"`
}

// Store is a thread-safe in-memory state store for storage simulation.
type Store struct {
	mu       sync.RWMutex
	backends map[string]*Backend
	volumes  map[string]*Volume
	config   SimConfig
	logger   *slog.Logger
}

// NewStore creates a new empty Store.
func NewStore(logger *slog.Logger) *Store {
	if logger == nil {
		logger = slog.Default()
	}
	return &Store{
		backends: make(map[string]*Backend),
		volumes:  make(map[string]*Volume),
		config:   SimConfig{DefaultLatencyMs: 2},
		logger:   logger,
	}
}

// AddBackend registers a new storage backend.
func (s *Store) AddBackend(_ context.Context, b Backend) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if b.BackendID == "" {
		return fmt.Errorf("add backend: %w", ErrEmptyBackendID)
	}
	if _, exists := s.backends[b.BackendID]; exists {
		return fmt.Errorf("add backend %q: %w", b.BackendID, ErrBackendExists)
	}
	if b.State == "" {
		b.State = BackendActive
	}
	if b.OverprovisionRatio == 0 {
		b.OverprovisionRatio = 1.0
	}
	stored := b
	s.backends[b.BackendID] = &stored
	s.logger.Info("backend registered", "backend_id", b.BackendID)
	return nil
}

// GetBackend returns a backend by ID.
func (s *Store) GetBackend(_ context.Context, id string) (*Backend, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, ok := s.backends[id]
	if !ok {
		return nil, fmt.Errorf("get backend %q: %w", id, ErrBackendNotFound)
	}
	cp := *b
	return &cp, nil
}

// ListBackends returns all registered backends.
func (s *Store) ListBackends(_ context.Context) []Backend {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Backend, 0, len(s.backends))
	for _, b := range s.backends {
		result = append(result, *b)
	}
	return result
}

// SetBackendState changes the state of a backend.
func (s *Store) SetBackendState(_ context.Context, id string, state BackendState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.backends[id]
	if !ok {
		return fmt.Errorf("set backend state %q: %w", id, ErrBackendNotFound)
	}
	b.State = state
	s.logger.Info("backend state changed", "backend_id", id, "state", state)
	return nil
}

// CreateVolume creates a new volume on the specified backend.
func (s *Store) CreateVolume(_ context.Context, v Volume) (*Volume, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if v.VolumeID == "" {
		return nil, fmt.Errorf("create volume: %w", ErrEmptyVolumeID)
	}
	if _, exists := s.volumes[v.VolumeID]; exists {
		return nil, fmt.Errorf("create volume %q: %w", v.VolumeID, ErrVolumeExists)
	}

	b, ok := s.backends[v.BackendID]
	if !ok {
		return nil, fmt.Errorf("create volume on backend %q: %w", v.BackendID, ErrBackendNotFound)
	}

	if b.State != BackendActive {
		return nil, fmt.Errorf("create volume on backend %q (state %s): %w", v.BackendID, b.State, ErrBackendNotActive)
	}

	// Capacity check
	if v.ThinProvisioned {
		maxAlloc := int64(float64(b.TotalCapacityGB) * b.OverprovisionRatio)
		if b.AllocatedCapacityGB+v.SizeGB > maxAlloc {
			return nil, fmt.Errorf("create volume %q: %w", v.VolumeID, ErrInsufficientCapacity)
		}
		b.AllocatedCapacityGB += v.SizeGB
	} else {
		if b.UsedCapacityGB+v.SizeGB > b.TotalCapacityGB {
			return nil, fmt.Errorf("create volume %q: %w", v.VolumeID, ErrInsufficientCapacity)
		}
		b.UsedCapacityGB += v.SizeGB
		b.AllocatedCapacityGB += v.SizeGB
	}

	v.State = VolumeAvailable
	v.ConsumedGB = 0
	v.CreatedAt = time.Now()
	stored := v
	s.volumes[v.VolumeID] = &stored
	s.logger.Info("volume created", "volume_id", v.VolumeID, "backend_id", v.BackendID)
	cp := stored
	return &cp, nil
}

// GetVolume returns a volume by ID.
func (s *Store) GetVolume(_ context.Context, id string) (*Volume, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, ok := s.volumes[id]
	if !ok {
		return nil, fmt.Errorf("get volume %q: %w", id, ErrVolumeNotFound)
	}
	cp := *v
	return &cp, nil
}

// ListVolumes returns volumes, optionally filtered by state and/or backend.
func (s *Store) ListVolumes(_ context.Context, backendID string, stateFilter VolumeState) []Volume {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Volume, 0)
	for _, v := range s.volumes {
		if backendID != "" && v.BackendID != backendID {
			continue
		}
		if stateFilter != "" && v.State != stateFilter {
			continue
		}
		result = append(result, *v)
	}
	return result
}

// DeleteVolume removes a volume if it is available and has no snapshots.
func (s *Store) DeleteVolume(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.volumes[id]
	if !ok {
		return fmt.Errorf("delete volume %q: %w", id, ErrVolumeNotFound)
	}
	if v.State == VolumeInUse {
		return fmt.Errorf("delete volume %q: %w", id, ErrVolumeInUse)
	}
	if len(v.Snapshots) > 0 {
		return fmt.Errorf("delete volume %q: %w", id, ErrVolumeHasSnapshots)
	}

	// Return capacity to backend
	if b, ok := s.backends[v.BackendID]; ok {
		b.AllocatedCapacityGB -= v.SizeGB
		if !v.ThinProvisioned {
			b.UsedCapacityGB -= v.SizeGB
		}
	}

	delete(s.volumes, id)
	s.logger.Info("volume deleted", "volume_id", id)
	return nil
}

// ExtendVolume increases the size of a volume.
func (s *Store) ExtendVolume(_ context.Context, id string, newSizeGB int64) (*Volume, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.volumes[id]
	if !ok {
		return nil, fmt.Errorf("extend volume %q: %w", id, ErrVolumeNotFound)
	}
	if newSizeGB <= v.SizeGB {
		return nil, fmt.Errorf("extend volume %q: new size %d <= current size %d: %w", id, newSizeGB, v.SizeGB, ErrShrinkNotAllowed)
	}

	b, ok := s.backends[v.BackendID]
	if !ok {
		return nil, fmt.Errorf("extend volume %q: backend %q: %w", id, v.BackendID, ErrBackendNotFound)
	}

	diff := newSizeGB - v.SizeGB

	if v.ThinProvisioned {
		maxAlloc := int64(float64(b.TotalCapacityGB) * b.OverprovisionRatio)
		if b.AllocatedCapacityGB+diff > maxAlloc {
			return nil, fmt.Errorf("extend volume %q: %w", id, ErrInsufficientCapacity)
		}
		b.AllocatedCapacityGB += diff
	} else {
		if b.UsedCapacityGB+diff > b.TotalCapacityGB {
			return nil, fmt.Errorf("extend volume %q: %w", id, ErrInsufficientCapacity)
		}
		b.UsedCapacityGB += diff
		b.AllocatedCapacityGB += diff
	}

	v.SizeGB = newSizeGB
	s.logger.Info("volume extended", "volume_id", id, "new_size_gb", newSizeGB)
	cp := *v
	return &cp, nil
}

// ExportVolume marks a volume as exported to a host.
func (s *Store) ExportVolume(_ context.Context, id string, hostID string, protocol string) (*Volume, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.volumes[id]
	if !ok {
		return nil, fmt.Errorf("export volume %q: %w", id, ErrVolumeNotFound)
	}
	if v.State == VolumeInUse {
		return nil, fmt.Errorf("export volume %q: %w", id, ErrVolumeAlreadyExported)
	}

	v.State = VolumeInUse
	v.ExportInfo = &ExportInfo{
		HostID:   hostID,
		Protocol: protocol,
	}
	s.logger.Info("volume exported", "volume_id", id, "host_id", hostID)
	cp := *v
	return &cp, nil
}

// UnexportVolume removes an export from a volume.
func (s *Store) UnexportVolume(_ context.Context, id string) (*Volume, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.volumes[id]
	if !ok {
		return nil, fmt.Errorf("unexport volume %q: %w", id, ErrVolumeNotFound)
	}
	if v.State != VolumeInUse {
		return nil, fmt.Errorf("unexport volume %q: %w", id, ErrVolumeNotExported)
	}

	v.State = VolumeAvailable
	v.ExportInfo = nil
	s.logger.Info("volume unexported", "volume_id", id)
	cp := *v
	return &cp, nil
}

// SetConfig updates the simulation configuration.
func (s *Store) SetConfig(_ context.Context, cfg SimConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
}

// GetConfig returns the current simulation configuration.
func (s *Store) GetConfig(_ context.Context) SimConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// GetStats returns overall simulation statistics.
func (s *Store) GetStats(_ context.Context) Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	exportCount := 0
	for _, v := range s.volumes {
		if v.State == VolumeInUse {
			exportCount++
		}
	}
	return Stats{
		BackendCount: len(s.backends),
		VolumeCount:  len(s.volumes),
		ExportCount:  exportCount,
	}
}

// Reset clears all state.
func (s *Store) Reset(_ context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backends = make(map[string]*Backend)
	s.volumes = make(map[string]*Volume)
	s.config = SimConfig{DefaultLatencyMs: 2}
	s.logger.Info("state reset")
}

// Sentinel errors for the state package.
var (
	ErrEmptyBackendID       = fmt.Errorf("backend_id is required")
	ErrBackendExists        = fmt.Errorf("backend already exists")
	ErrBackendNotFound      = fmt.Errorf("backend not found")
	ErrBackendNotActive     = fmt.Errorf("backend is not active")
	ErrEmptyVolumeID        = fmt.Errorf("volume_id is required")
	ErrVolumeExists         = fmt.Errorf("volume already exists")
	ErrVolumeNotFound       = fmt.Errorf("volume not found")
	ErrVolumeInUse          = fmt.Errorf("volume is in use")
	ErrVolumeHasSnapshots   = fmt.Errorf("volume has snapshots")
	ErrVolumeAlreadyExported = fmt.Errorf("volume is already exported")
	ErrVolumeNotExported    = fmt.Errorf("volume is not exported")
	ErrInsufficientCapacity = fmt.Errorf("insufficient storage capacity")
	ErrShrinkNotAllowed     = fmt.Errorf("shrinking volumes is not allowed")
)

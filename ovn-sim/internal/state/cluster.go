// Package state manages cluster state for ovn-sim.
package state

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/tjst-t/cirrus-sim/ovn-sim/internal/ovsdb"
)

// Cluster represents a single OVN cluster with its own OVSDB instance.
type Cluster struct {
	ID       string       `json:"cluster_id"`
	Port     int          `json:"ovsdb_port"`
	Store    *ovsdb.Store `json:"-"`
	Server   *ovsdb.Server `json:"-"`
}

// ClusterInfo is the JSON-serializable info for a cluster.
type ClusterInfo struct {
	ID   string `json:"cluster_id"`
	Port int    `json:"ovsdb_port"`
}

// Manager manages multiple OVN clusters.
type Manager struct {
	mu       sync.RWMutex
	clusters map[string]*Cluster
	logger   *slog.Logger
}

// NewManager creates a new cluster Manager.
func NewManager(logger *slog.Logger) *Manager {
	return &Manager{
		clusters: make(map[string]*Cluster),
		logger:   logger,
	}
}

// CreateCluster creates and starts a new OVN cluster.
func (m *Manager) CreateCluster(ctx context.Context, id string, port int) (*Cluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clusters[id]; exists {
		return nil, fmt.Errorf("create cluster failed: cluster %q already exists", id)
	}

	store := ovsdb.NewStore(ovsdb.OVNNBTables)
	server := ovsdb.NewServer(store, m.logger.With("cluster", id))

	addr := fmt.Sprintf(":%d", port)
	if err := server.Listen(ctx, id, addr); err != nil {
		return nil, fmt.Errorf("create cluster failed: %w", err)
	}

	cluster := &Cluster{
		ID:     id,
		Port:   port,
		Store:  store,
		Server: server,
	}
	m.clusters[id] = cluster

	m.logger.Info("cluster created", "cluster_id", id, "port", port)
	return cluster, nil
}

// GetCluster returns a cluster by ID.
func (m *Manager) GetCluster(id string) (*Cluster, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.clusters[id]
	return c, ok
}

// ListClusters returns info for all clusters.
func (m *Manager) ListClusters() []ClusterInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []ClusterInfo
	for _, c := range m.clusters {
		result = append(result, ClusterInfo{
			ID:   c.ID,
			Port: c.Port,
		})
	}
	if result == nil {
		result = []ClusterInfo{}
	}
	return result
}

// SetPortUp sets the "up" field of a Logical_Switch_Port to true in all clusters.
// Returns true if the port was found in any cluster.
func (m *Manager) SetPortUp(portUUID string) bool {
	return m.setPortState(portUUID, true)
}

// SetPortDown sets the "up" field of a Logical_Switch_Port to false in all clusters.
// Returns true if the port was found in any cluster.
func (m *Manager) SetPortDown(portUUID string) bool {
	return m.setPortState(portUUID, false)
}

func (m *Manager) setPortState(portUUID string, up bool) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	found := false
	for _, c := range m.clusters {
		oldRow, ok := c.Store.GetRow("Logical_Switch_Port", portUUID)
		if !ok {
			continue
		}
		found = true

		_, err := c.Store.Update("Logical_Switch_Port",
			[]interface{}{[]interface{}{"_uuid", "==", []interface{}{"uuid", portUUID}}},
			ovsdb.Row{"up": up},
		)
		if err != nil {
			m.logger.Warn("failed to set port state", "port", portUUID, "error", err)
			continue
		}

		newRow, _ := c.Store.GetRow("Logical_Switch_Port", portUUID)
		c.Server.Monitors().NotifyUpdate("Logical_Switch_Port", portUUID, oldRow, newRow)
	}
	return found
}

// Stats returns aggregate statistics across all clusters.
func (m *Manager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clusterStats := make(map[string]interface{})
	for id, c := range m.clusters {
		clusterStats[id] = c.Store.Stats()
	}
	return map[string]interface{}{
		"cluster_count": len(m.clusters),
		"clusters":      clusterStats,
	}
}

// ICConnection represents an OVN IC connection configuration.
type ICConnection struct {
	ICClusterID string   `json:"ic_cluster_id"`
	Connects    []string `json:"connects"`
	Port        int      `json:"port"`
}

// CreateICCluster creates an OVN IC Northbound cluster with IC schema.
func (m *Manager) CreateICCluster(ctx context.Context, id string, port int, connects []string) (*Cluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clusters[id]; exists {
		return nil, fmt.Errorf("create IC cluster: cluster %q already exists", id)
	}

	store := ovsdb.NewStore(ovsdb.OVNICTables)
	server := ovsdb.NewServer(store, m.logger.With("cluster", id, "type", "ic"))

	addr := fmt.Sprintf(":%d", port)
	if err := server.Listen(ctx, id, addr); err != nil {
		return nil, fmt.Errorf("create IC cluster: %w", err)
	}

	cluster := &Cluster{
		ID:     id,
		Port:   port,
		Store:  store,
		Server: server,
	}
	m.clusters[id] = cluster

	// Create Availability_Zone entries for connected clusters
	for _, clusterID := range connects {
		if _, err := store.Insert("Availability_Zone", ovsdb.Row{"name": clusterID}); err != nil {
			m.logger.Warn("failed to create AZ", "cluster", clusterID, "error", err)
		}
	}

	m.logger.Info("IC cluster created", "cluster_id", id, "port", port, "connects", connects)
	return cluster, nil
}

// Reset stops all clusters and clears all state.
func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, c := range m.clusters {
		c.Server.StopAll()
		c.Server.Wait()
		delete(m.clusters, id)
	}

	m.logger.Info("all clusters reset")
}

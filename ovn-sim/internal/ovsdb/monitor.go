package ovsdb

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
)

// MonitorRequest describes which tables/columns a client wants to monitor.
type MonitorRequest struct {
	Columns []string `json:"columns,omitempty"`
}

// MonitorRegistration tracks a single monitor for a client.
type MonitorRegistration struct {
	ID     string
	Tables map[string]MonitorRequest
}

// UpdateNotification represents a row change for a monitor update.
type UpdateNotification struct {
	Old map[string]interface{} `json:"old,omitempty"`
	New map[string]interface{} `json:"new,omitempty"`
}

// ClientNotifier is an interface for sending notifications to a client.
type ClientNotifier interface {
	// SendNotification sends a JSON-RPC notification to the client.
	SendNotification(method string, params interface{}) error
}

// MonitorManager manages monitor registrations and sends update notifications.
type MonitorManager struct {
	mu       sync.RWMutex
	monitors map[string]*clientMonitor // key: client-specific key
	logger   *slog.Logger
}

type clientMonitor struct {
	notifier ClientNotifier
	regs     map[string]*MonitorRegistration // key: monitor ID
}

// NewMonitorManager creates a new MonitorManager.
func NewMonitorManager(logger *slog.Logger) *MonitorManager {
	return &MonitorManager{
		monitors: make(map[string]*clientMonitor),
		logger:   logger,
	}
}

// Register registers a monitor for the given client. Returns the initial table dump.
func (mm *MonitorManager) Register(clientKey string, notifier ClientNotifier, monitorID string, requests map[string]MonitorRequest, store *Store) (map[string]interface{}, error) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	cm, ok := mm.monitors[clientKey]
	if !ok {
		cm = &clientMonitor{
			notifier: notifier,
			regs:     make(map[string]*MonitorRegistration),
		}
		mm.monitors[clientKey] = cm
	}

	cm.regs[monitorID] = &MonitorRegistration{
		ID:     monitorID,
		Tables: requests,
	}

	// Build initial dump
	result := make(map[string]interface{})
	for tableName, req := range requests {
		rows, err := store.AllRows(tableName)
		if err != nil {
			return nil, fmt.Errorf("monitor register failed: %w", err)
		}
		tableUpdates := make(map[string]interface{})
		for uuid, row := range rows {
			filtered := filterColumns(row, req.Columns)
			tableUpdates[uuid] = map[string]interface{}{
				"new": filtered,
			}
		}
		if len(tableUpdates) > 0 {
			result[tableName] = tableUpdates
		}
	}

	return result, nil
}

// Cancel cancels a monitor registration.
func (mm *MonitorManager) Cancel(clientKey, monitorID string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	cm, ok := mm.monitors[clientKey]
	if !ok {
		return fmt.Errorf("monitor cancel failed: unknown client %q", clientKey)
	}

	if _, ok := cm.regs[monitorID]; !ok {
		return fmt.Errorf("monitor cancel failed: unknown monitor %q", monitorID)
	}

	delete(cm.regs, monitorID)
	if len(cm.regs) == 0 {
		delete(mm.monitors, clientKey)
	}
	return nil
}

// RemoveClient removes all monitors for a client (e.g., on disconnect).
func (mm *MonitorManager) RemoveClient(clientKey string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	delete(mm.monitors, clientKey)
}

// NotifyInsert sends update notifications for a row insert.
func (mm *MonitorManager) NotifyInsert(table, uuid string, row Row) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	for _, cm := range mm.monitors {
		for monID, reg := range cm.regs {
			req, ok := reg.Tables[table]
			if !ok {
				continue
			}
			filtered := filterColumns(row, req.Columns)
			update := map[string]interface{}{
				table: map[string]interface{}{
					uuid: map[string]interface{}{
						"new": filtered,
					},
				},
			}
			if err := cm.notifier.SendNotification("update", []interface{}{monID, update}); err != nil {
				mm.logger.Warn("failed to send monitor notification", "error", err)
			}
		}
	}
}

// NotifyUpdate sends update notifications for a row update.
func (mm *MonitorManager) NotifyUpdate(table, uuid string, oldRow, newRow Row) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	for _, cm := range mm.monitors {
		for monID, reg := range cm.regs {
			req, ok := reg.Tables[table]
			if !ok {
				continue
			}
			oldFiltered := filterColumns(oldRow, req.Columns)
			newFiltered := filterColumns(newRow, req.Columns)
			update := map[string]interface{}{
				table: map[string]interface{}{
					uuid: map[string]interface{}{
						"old": oldFiltered,
						"new": newFiltered,
					},
				},
			}
			if err := cm.notifier.SendNotification("update", []interface{}{monID, update}); err != nil {
				mm.logger.Warn("failed to send monitor notification", "error", err)
			}
		}
	}
}

// NotifyDelete sends update notifications for a row delete.
func (mm *MonitorManager) NotifyDelete(table, uuid string, row Row) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	for _, cm := range mm.monitors {
		for monID, reg := range cm.regs {
			req, ok := reg.Tables[table]
			if !ok {
				continue
			}
			filtered := filterColumns(row, req.Columns)
			update := map[string]interface{}{
				table: map[string]interface{}{
					uuid: map[string]interface{}{
						"old": filtered,
					},
				},
			}
			if err := cm.notifier.SendNotification("update", []interface{}{monID, update}); err != nil {
				mm.logger.Warn("failed to send monitor notification", "error", err)
			}
		}
	}
}

func filterColumns(row Row, columns []string) map[string]interface{} {
	if len(columns) == 0 {
		// Return all columns
		result := make(map[string]interface{}, len(row))
		for k, v := range row {
			result[k] = v
		}
		return result
	}
	result := make(map[string]interface{}, len(columns))
	for _, col := range columns {
		if v, ok := row[col]; ok {
			result[col] = v
		}
	}
	// Always include _uuid
	if v, ok := row["_uuid"]; ok {
		result["_uuid"] = v
	}
	return result
}

// ParseMonitorParams parses the params array from a monitor RPC call.
// params: ["OVN_Northbound", "monitor-id", {table: {columns: [...]}, ...}]
func ParseMonitorParams(params json.RawMessage) (db string, monitorID string, requests map[string]MonitorRequest, err error) {
	var raw []json.RawMessage
	if err = json.Unmarshal(params, &raw); err != nil {
		return "", "", nil, fmt.Errorf("parse monitor params failed: %w", err)
	}
	if len(raw) < 3 {
		return "", "", nil, fmt.Errorf("parse monitor params failed: expected at least 3 params, got %d", len(raw))
	}

	if err = json.Unmarshal(raw[0], &db); err != nil {
		return "", "", nil, fmt.Errorf("parse monitor params failed: invalid db name: %w", err)
	}
	if err = json.Unmarshal(raw[1], &monitorID); err != nil {
		return "", "", nil, fmt.Errorf("parse monitor params failed: invalid monitor id: %w", err)
	}

	// raw[2] is map of table name -> monitor request
	var tableReqs map[string]json.RawMessage
	if err = json.Unmarshal(raw[2], &tableReqs); err != nil {
		return "", "", nil, fmt.Errorf("parse monitor params failed: invalid table requests: %w", err)
	}

	requests = make(map[string]MonitorRequest, len(tableReqs))
	for tName, raw := range tableReqs {
		var req MonitorRequest
		if err = json.Unmarshal(raw, &req); err != nil {
			// Try as array of monitor requests (some clients send [{columns: [...]}])
			var reqs []MonitorRequest
			if err2 := json.Unmarshal(raw, &reqs); err2 == nil && len(reqs) > 0 {
				req = reqs[0]
			}
			// If still failed, just use empty (monitor all columns)
		}
		requests[tName] = req
	}

	return db, monitorID, requests, nil
}

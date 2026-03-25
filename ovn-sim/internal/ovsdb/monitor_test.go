package ovsdb

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"testing"
)

type mockNotifier struct {
	mu      sync.Mutex
	updates []interface{}
}

func (m *mockNotifier) SendNotification(method string, params interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updates = append(m.updates, params)
	return nil
}

func (m *mockNotifier) getUpdates() []interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]interface{}, len(m.updates))
	copy(cp, m.updates)
	return cp
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func TestMonitorRegisterAndInitialDump(t *testing.T) {
	store := newTestStore()
	if _, err := store.Insert("Logical_Switch", Row{"name": "ls1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Insert("Logical_Switch", Row{"name": "ls2"}); err != nil {
		t.Fatal(err)
	}

	mm := NewMonitorManager(testLogger())
	notifier := &mockNotifier{}

	requests := map[string]MonitorRequest{
		"Logical_Switch": {Columns: []string{"name"}},
	}

	result, err := mm.Register("client1", notifier, "mon1", requests, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lsUpdates, ok := result["Logical_Switch"]
	if !ok {
		t.Fatal("expected Logical_Switch in initial dump")
	}

	updates := lsUpdates.(map[string]interface{})
	if len(updates) != 2 {
		t.Fatalf("expected 2 initial rows, got %d", len(updates))
	}
}

func TestMonitorNotifyInsert(t *testing.T) {
	store := newTestStore()
	mm := NewMonitorManager(testLogger())
	notifier := &mockNotifier{}

	requests := map[string]MonitorRequest{
		"Logical_Switch": {},
	}
	if _, err := mm.Register("client1", notifier, "mon1", requests, store); err != nil {
		t.Fatal(err)
	}

	// Insert a row and notify
	mm.NotifyInsert("Logical_Switch", "test-uuid", Row{
		"_uuid": []interface{}{"uuid", "test-uuid"},
		"name":  "ls1",
	})

	updates := notifier.getUpdates()
	if len(updates) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(updates))
	}
}

func TestMonitorNotifyDelete(t *testing.T) {
	store := newTestStore()
	mm := NewMonitorManager(testLogger())
	notifier := &mockNotifier{}

	requests := map[string]MonitorRequest{
		"Logical_Switch": {},
	}
	if _, err := mm.Register("client1", notifier, "mon1", requests, store); err != nil {
		t.Fatal(err)
	}

	mm.NotifyDelete("Logical_Switch", "test-uuid", Row{
		"_uuid": []interface{}{"uuid", "test-uuid"},
		"name":  "ls1",
	})

	updates := notifier.getUpdates()
	if len(updates) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(updates))
	}
}

func TestMonitorCancel(t *testing.T) {
	store := newTestStore()
	mm := NewMonitorManager(testLogger())
	notifier := &mockNotifier{}

	requests := map[string]MonitorRequest{
		"Logical_Switch": {},
	}
	if _, err := mm.Register("client1", notifier, "mon1", requests, store); err != nil {
		t.Fatal(err)
	}

	err := mm.Cancel("client1", "mon1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After cancel, no notifications should be sent
	mm.NotifyInsert("Logical_Switch", "test-uuid", Row{"name": "ls1"})
	updates := notifier.getUpdates()
	if len(updates) != 0 {
		t.Errorf("expected 0 notifications after cancel, got %d", len(updates))
	}
}

func TestMonitorCancelUnknown(t *testing.T) {
	mm := NewMonitorManager(testLogger())

	err := mm.Cancel("nonexistent", "mon1")
	if err == nil {
		t.Error("expected error for unknown client")
	}
}

func TestMonitorNoNotificationForUnmonitoredTable(t *testing.T) {
	store := newTestStore()
	mm := NewMonitorManager(testLogger())
	notifier := &mockNotifier{}

	requests := map[string]MonitorRequest{
		"Logical_Router": {},
	}
	if _, err := mm.Register("client1", notifier, "mon1", requests, store); err != nil {
		t.Fatal(err)
	}

	// Insert into a different table
	mm.NotifyInsert("Logical_Switch", "test-uuid", Row{"name": "ls1"})
	updates := notifier.getUpdates()
	if len(updates) != 0 {
		t.Errorf("expected 0 notifications for unmonitored table, got %d", len(updates))
	}
}

func TestParseMonitorParams(t *testing.T) {
	raw := `["OVN_Northbound", "my-monitor", {"Logical_Switch": {"columns": ["name", "ports"]}}]`
	db, monID, requests, err := ParseMonitorParams(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if db != "OVN_Northbound" {
		t.Errorf("expected db=OVN_Northbound, got %s", db)
	}
	if monID != "my-monitor" {
		t.Errorf("expected monitor-id=my-monitor, got %s", monID)
	}
	req, ok := requests["Logical_Switch"]
	if !ok {
		t.Fatal("expected Logical_Switch in requests")
	}
	if len(req.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(req.Columns))
	}
}

package state

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/tjst-t/cirrus-sim/ovn-sim/internal/ovsdb"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func TestCreateCluster(t *testing.T) {
	m := NewManager(testLogger())
	ctx := context.Background()

	cluster, err := m.CreateCluster(ctx, "test-1", 16651)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cluster.ID != "test-1" {
		t.Errorf("expected id=test-1, got %s", cluster.ID)
	}
	if cluster.Port != 16651 {
		t.Errorf("expected port=16651, got %d", cluster.Port)
	}

	// Clean up
	cluster.Server.StopAll()
}

func TestCreateDuplicateCluster(t *testing.T) {
	m := NewManager(testLogger())
	ctx := context.Background()

	c, err := m.CreateCluster(ctx, "dup", 16652)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer c.Server.StopAll()

	_, err = m.CreateCluster(ctx, "dup", 16653)
	if err == nil {
		t.Error("expected error for duplicate cluster")
	}
}

func TestListClusters(t *testing.T) {
	m := NewManager(testLogger())
	ctx := context.Background()

	clusters := m.ListClusters()
	if len(clusters) != 0 {
		t.Fatalf("expected 0 clusters, got %d", len(clusters))
	}

	c, _ := m.CreateCluster(ctx, "c1", 16654)
	defer c.Server.StopAll()

	clusters = m.ListClusters()
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
}

func TestSetPortUpDown(t *testing.T) {
	m := NewManager(testLogger())
	ctx := context.Background()

	c, err := m.CreateCluster(ctx, "port-test", 16655)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer c.Server.StopAll()

	// Insert a port
	uuid, err := c.Store.Insert("Logical_Switch_Port", ovsdb.Row{
		"name": "test-port",
		"up":   false,
	})
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	// Set port up
	if !m.SetPortUp(uuid) {
		t.Error("expected SetPortUp to return true")
	}

	row, ok := c.Store.GetRow("Logical_Switch_Port", uuid)
	if !ok {
		t.Fatal("port not found")
	}
	if row["up"] != true {
		t.Errorf("expected up=true, got %v", row["up"])
	}

	// Set port down
	if !m.SetPortDown(uuid) {
		t.Error("expected SetPortDown to return true")
	}

	row, ok = c.Store.GetRow("Logical_Switch_Port", uuid)
	if !ok {
		t.Fatal("port not found")
	}
	if row["up"] != false {
		t.Errorf("expected up=false, got %v", row["up"])
	}
}

func TestSetPortNotFound(t *testing.T) {
	m := NewManager(testLogger())
	if m.SetPortUp("nonexistent") {
		t.Error("expected SetPortUp to return false for nonexistent port")
	}
}

func TestResetManager(t *testing.T) {
	m := NewManager(testLogger())
	ctx := context.Background()

	c, _ := m.CreateCluster(ctx, "reset-1", 16656)
	_ = c // will be cleaned up by Reset

	m.Reset()

	clusters := m.ListClusters()
	if len(clusters) != 0 {
		t.Errorf("expected 0 clusters after reset, got %d", len(clusters))
	}
}

func TestStats(t *testing.T) {
	m := NewManager(testLogger())
	ctx := context.Background()

	c, _ := m.CreateCluster(ctx, "stats-test", 16657)
	defer c.Server.StopAll()

	if _, err := c.Store.Insert("Logical_Switch", ovsdb.Row{"name": "ls1"}); err != nil {
		t.Fatal(err)
	}

	stats := m.Stats()
	if stats["cluster_count"] != 1 {
		t.Errorf("expected cluster_count=1, got %v", stats["cluster_count"])
	}
}

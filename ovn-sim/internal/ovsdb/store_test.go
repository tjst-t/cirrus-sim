package ovsdb

import (
	"testing"
)

func newTestStore() *Store {
	return NewStore(OVNNBTables)
}

func TestInsertAndSelect(t *testing.T) {
	s := newTestStore()

	tests := []struct {
		name    string
		table   string
		row     Row
		wantErr bool
	}{
		{
			name:  "insert logical switch",
			table: "Logical_Switch",
			row:   Row{"name": "ls1"},
		},
		{
			name:    "insert unknown table",
			table:   "Unknown_Table",
			row:     Row{"name": "x"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uuid, err := s.Insert(tt.table, tt.row)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if uuid == "" {
				t.Fatal("expected non-empty UUID")
			}

			// Select back
			rows, err := s.Select(tt.table, []interface{}{})
			if err != nil {
				t.Fatalf("select error: %v", err)
			}
			if len(rows) != 1 {
				t.Fatalf("expected 1 row, got %d", len(rows))
			}
			if rows[0]["name"] != tt.row["name"] {
				t.Errorf("expected name=%v, got %v", tt.row["name"], rows[0]["name"])
			}
		})
	}
}

func TestSelectWithWhere(t *testing.T) {
	s := newTestStore()

	if _, err := s.Insert("Logical_Switch", Row{"name": "ls1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Insert("Logical_Switch", Row{"name": "ls2"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Insert("Logical_Switch", Row{"name": "ls3"}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		where    []interface{}
		wantLen  int
		wantName string
	}{
		{
			name:    "no filter",
			where:   []interface{}{},
			wantLen: 3,
		},
		{
			name:     "filter by name ==",
			where:    []interface{}{[]interface{}{"name", "==", "ls2"}},
			wantLen:  1,
			wantName: "ls2",
		},
		{
			name:    "filter by name !=",
			where:   []interface{}{[]interface{}{"name", "!=", "ls1"}},
			wantLen: 2,
		},
		{
			name:    "no match",
			where:   []interface{}{[]interface{}{"name", "==", "nonexistent"}},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := s.Select("Logical_Switch", tt.where)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(rows) != tt.wantLen {
				t.Fatalf("expected %d rows, got %d", tt.wantLen, len(rows))
			}
			if tt.wantName != "" && rows[0]["name"] != tt.wantName {
				t.Errorf("expected name=%s, got %v", tt.wantName, rows[0]["name"])
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	s := newTestStore()
	uuid, _ := s.Insert("Logical_Switch", Row{"name": "ls1"})

	tests := []struct {
		name      string
		where     []interface{}
		values    Row
		wantCount int
	}{
		{
			name:      "update by uuid",
			where:     []interface{}{[]interface{}{"_uuid", "==", []interface{}{"uuid", uuid}}},
			values:    Row{"name": "ls1-updated"},
			wantCount: 1,
		},
		{
			name:      "update no match",
			where:     []interface{}{[]interface{}{"name", "==", "nonexistent"}},
			values:    Row{"name": "x"},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := s.Update("Logical_Switch", tt.where, tt.values)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if count != tt.wantCount {
				t.Fatalf("expected count=%d, got %d", tt.wantCount, count)
			}
		})
	}

	// Verify update was applied
	row, ok := s.GetRow("Logical_Switch", uuid)
	if !ok {
		t.Fatal("row not found after update")
	}
	if row["name"] != "ls1-updated" {
		t.Errorf("expected name=ls1-updated, got %v", row["name"])
	}
}

func TestDelete(t *testing.T) {
	s := newTestStore()
	if _, err := s.Insert("Logical_Switch", Row{"name": "ls1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Insert("Logical_Switch", Row{"name": "ls2"}); err != nil {
		t.Fatal(err)
	}

	count, err := s.Delete("Logical_Switch", []interface{}{[]interface{}{"name", "==", "ls1"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count=1, got %d", count)
	}

	rows, _ := s.Select("Logical_Switch", []interface{}{})
	if len(rows) != 1 {
		t.Fatalf("expected 1 row remaining, got %d", len(rows))
	}
	if rows[0]["name"] != "ls2" {
		t.Errorf("expected remaining row name=ls2, got %v", rows[0]["name"])
	}
}

func TestMutate(t *testing.T) {
	s := newTestStore()
	uuid, _ := s.Insert("Logical_Switch_Port", Row{"name": "port1", "tag": float64(100)})

	where := []interface{}{[]interface{}{"_uuid", "==", []interface{}{"uuid", uuid}}}

	tests := []struct {
		name      string
		mutations []interface{}
		wantTag   float64
	}{
		{
			name:      "increment tag",
			mutations: []interface{}{[]interface{}{"tag", "+=", float64(50)}},
			wantTag:   150,
		},
		{
			name:      "decrement tag",
			mutations: []interface{}{[]interface{}{"tag", "-=", float64(25)}},
			wantTag:   125,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := s.Mutate("Logical_Switch_Port", where, tt.mutations)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if count != 1 {
				t.Fatalf("expected count=1, got %d", count)
			}

			row, ok := s.GetRow("Logical_Switch_Port", uuid)
			if !ok {
				t.Fatal("row not found")
			}
			if row["tag"] != tt.wantTag {
				t.Errorf("expected tag=%v, got %v", tt.wantTag, row["tag"])
			}
		})
	}
}

func TestReset(t *testing.T) {
	s := newTestStore()
	if _, err := s.Insert("Logical_Switch", Row{"name": "ls1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Insert("Logical_Router", Row{"name": "lr1"}); err != nil {
		t.Fatal(err)
	}

	s.Reset()

	rows, _ := s.Select("Logical_Switch", []interface{}{})
	if len(rows) != 0 {
		t.Errorf("expected 0 rows after reset, got %d", len(rows))
	}
	rows, _ = s.Select("Logical_Router", []interface{}{})
	if len(rows) != 0 {
		t.Errorf("expected 0 rows after reset, got %d", len(rows))
	}
}

func TestStats(t *testing.T) {
	s := newTestStore()
	if _, err := s.Insert("Logical_Switch", Row{"name": "ls1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Insert("Logical_Switch", Row{"name": "ls2"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Insert("Logical_Router", Row{"name": "lr1"}); err != nil {
		t.Fatal(err)
	}

	stats := s.Stats()
	if stats["Logical_Switch"] != 2 {
		t.Errorf("expected 2 switches, got %d", stats["Logical_Switch"])
	}
	if stats["Logical_Router"] != 1 {
		t.Errorf("expected 1 router, got %d", stats["Logical_Router"])
	}
}

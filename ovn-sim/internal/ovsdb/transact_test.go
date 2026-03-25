package ovsdb

import (
	"encoding/json"
	"testing"
)

func TestTransactInsertAndSelect(t *testing.T) {
	s := newTestStore()

	ops := []TransactOp{
		{
			Op:       "insert",
			Table:    "Logical_Switch",
			Row:      map[string]interface{}{"name": "ls1"},
			UUIDName: "new_ls",
		},
		{
			Op:    "select",
			Table: "Logical_Switch",
			Where: []interface{}{[]interface{}{"name", "==", "ls1"}},
		},
	}

	results, changes, err := Transact(s, ops)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Check insert result
	insertResult, ok := results[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", results[0])
	}
	uuidArr, ok := insertResult["uuid"].([]interface{})
	if !ok || len(uuidArr) != 2 || uuidArr[0] != "uuid" {
		t.Fatalf("unexpected uuid format: %v", insertResult["uuid"])
	}

	// Check select result
	selectResult, ok := results[1].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", results[1])
	}
	rows, ok := selectResult["rows"].([]Row)
	if !ok || len(rows) != 1 {
		t.Fatalf("expected 1 row in select, got %v", selectResult["rows"])
	}
	if rows[0]["name"] != "ls1" {
		t.Errorf("expected name=ls1, got %v", rows[0]["name"])
	}

	// Check changes
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Type != "insert" {
		t.Errorf("expected insert change, got %s", changes[0].Type)
	}
}

func TestTransactNamedUUID(t *testing.T) {
	s := newTestStore()

	ops := []TransactOp{
		{
			Op:       "insert",
			Table:    "Logical_Switch_Port",
			Row:      map[string]interface{}{"name": "port1"},
			UUIDName: "new_port",
		},
		{
			Op:    "insert",
			Table: "Logical_Switch",
			Row: map[string]interface{}{
				"name":  "ls1",
				"ports": []interface{}{"set", []interface{}{[]interface{}{"named-uuid", "new_port"}}},
			},
		},
	}

	results, _, err := Transact(s, ops)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// The named-uuid should have been resolved
	portResult := results[0].(map[string]interface{})
	portUUID := portResult["uuid"].([]interface{})[1].(string)

	// Verify the switch has the port UUID in its ports set
	rows, _ := s.Select("Logical_Switch", []interface{}{[]interface{}{"name", "==", "ls1"}})
	if len(rows) != 1 {
		t.Fatalf("expected 1 switch, got %d", len(rows))
	}

	ports := rows[0]["ports"]
	portsArr, ok := ports.([]interface{})
	if !ok || len(portsArr) != 2 {
		t.Fatalf("expected set format, got %v", ports)
	}
	if portsArr[0] != "set" {
		t.Fatalf("expected 'set' tag, got %v", portsArr[0])
	}
	setElems := portsArr[1].([]interface{})
	if len(setElems) != 1 {
		t.Fatalf("expected 1 element in set, got %d", len(setElems))
	}
	uuidRef := setElems[0].([]interface{})
	if uuidRef[0] != "uuid" || uuidRef[1] != portUUID {
		t.Errorf("expected uuid ref to %s, got %v", portUUID, uuidRef)
	}
}

func TestTransactUpdateAndDelete(t *testing.T) {
	s := newTestStore()

	// First insert
	ops := []TransactOp{
		{Op: "insert", Table: "Logical_Switch", Row: map[string]interface{}{"name": "ls1"}},
	}
	results, _, _ := Transact(s, ops)
	insertResult := results[0].(map[string]interface{})
	uuid := insertResult["uuid"].([]interface{})[1].(string)

	// Update
	ops = []TransactOp{
		{
			Op:    "update",
			Table: "Logical_Switch",
			Where: []interface{}{[]interface{}{"_uuid", "==", []interface{}{"uuid", uuid}}},
			Row:   map[string]interface{}{"name": "ls1-updated"},
		},
	}
	results, changes, _ := Transact(s, ops)
	updateResult := results[0].(map[string]interface{})
	if updateResult["count"] != 1 {
		t.Errorf("expected count=1, got %v", updateResult["count"])
	}
	if len(changes) != 1 || changes[0].Type != "update" {
		t.Errorf("expected 1 update change, got %d changes", len(changes))
	}

	// Delete
	ops = []TransactOp{
		{
			Op:    "delete",
			Table: "Logical_Switch",
			Where: []interface{}{[]interface{}{"_uuid", "==", []interface{}{"uuid", uuid}}},
		},
	}
	results, changes, _ = Transact(s, ops)
	deleteResult := results[0].(map[string]interface{})
	if deleteResult["count"] != 1 {
		t.Errorf("expected count=1, got %v", deleteResult["count"])
	}
	if len(changes) != 1 || changes[0].Type != "delete" {
		t.Errorf("expected 1 delete change, got %d changes", len(changes))
	}
}

func TestTransactMutate(t *testing.T) {
	s := newTestStore()

	ops := []TransactOp{
		{Op: "insert", Table: "Logical_Switch_Port", Row: map[string]interface{}{"name": "p1", "tag": float64(100)}},
	}
	results, _, _ := Transact(s, ops)
	uuid := results[0].(map[string]interface{})["uuid"].([]interface{})[1].(string)

	ops = []TransactOp{
		{
			Op:        "mutate",
			Table:     "Logical_Switch_Port",
			Where:     []interface{}{[]interface{}{"_uuid", "==", []interface{}{"uuid", uuid}}},
			Mutations: []interface{}{[]interface{}{"tag", "+=", float64(50)}},
		},
	}
	results, _, _ = Transact(s, ops)
	mutResult := results[0].(map[string]interface{})
	if mutResult["count"] != 1 {
		t.Errorf("expected count=1, got %v", mutResult["count"])
	}

	row, _ := s.GetRow("Logical_Switch_Port", uuid)
	if row["tag"] != float64(150) {
		t.Errorf("expected tag=150, got %v", row["tag"])
	}
}

func TestTransactAtomicRollback(t *testing.T) {
	s := newTestStore()

	// Insert a row first
	if _, err := s.Insert("Logical_Switch", Row{"name": "existing"}); err != nil {
		t.Fatal(err)
	}

	// Try a transaction where second op fails (unknown table)
	ops := []TransactOp{
		{Op: "insert", Table: "Logical_Switch", Row: map[string]interface{}{"name": "new_row"}},
		{Op: "insert", Table: "Nonexistent_Table", Row: map[string]interface{}{"name": "fail"}},
	}

	results, _, _ := Transact(s, ops)

	// The first result should be empty (rolled back), second should have error
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	errResult, ok := results[1].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error result at index 1, got %T", results[1])
	}
	if errResult["error"] == nil {
		t.Error("expected error in result")
	}

	// Verify the first insert was rolled back
	rows, _ := s.Select("Logical_Switch", []interface{}{})
	if len(rows) != 1 {
		t.Errorf("expected 1 row (rollback), got %d", len(rows))
	}
	if rows[0]["name"] != "existing" {
		t.Errorf("expected existing row preserved, got %v", rows[0]["name"])
	}
}

func TestParseTransactParams(t *testing.T) {
	raw := `["OVN_Northbound", {"op": "insert", "table": "Logical_Switch", "row": {"name": "ls1"}, "uuid-name": "new_ls"}]`
	db, ops, err := ParseTransactParams(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if db != "OVN_Northbound" {
		t.Errorf("expected db=OVN_Northbound, got %s", db)
	}
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ops))
	}
	if ops[0].Op != "insert" {
		t.Errorf("expected op=insert, got %s", ops[0].Op)
	}
	if ops[0].Table != "Logical_Switch" {
		t.Errorf("expected table=Logical_Switch, got %s", ops[0].Table)
	}
	if ops[0].UUIDName != "new_ls" {
		t.Errorf("expected uuid-name=new_ls, got %s", ops[0].UUIDName)
	}
}

package ovsdb

import (
	"encoding/json"
	"fmt"
)

// Transact executes a list of OVSDB operations atomically.
// If any operation fails, none of the changes are applied.
// Returns the results array and any affected rows for monitor notifications.
func Transact(store *Store, ops []TransactOp) ([]interface{}, []Change, error) {
	// Phase 1: execute all operations on a snapshot, collecting results and changes
	namedUUIDs := make(map[string]string)
	var results []interface{}
	var changes []Change

	// Take a snapshot of affected tables for rollback
	snapshots := make(map[string]map[string]Row)
	for _, op := range ops {
		if _, ok := snapshots[op.Table]; !ok {
			rows, err := store.AllRows(op.Table)
			if err != nil {
				// Skip unknown tables in snapshot; the operation will fail at its index
				continue
			}
			snapshots[op.Table] = rows
		}
	}

	for i, op := range ops {
		// Resolve named-uuids in row values
		if op.Row != nil {
			op.Row = resolveNamedUUIDs(op.Row, namedUUIDs)
		}
		if op.Where != nil {
			op.Where = resolveNamedUUIDsInWhere(op.Where, namedUUIDs)
		}

		switch op.Op {
		case "insert":
			result, ch, err := execInsert(store, op)
			if err != nil {
				rollback(store, snapshots)
				return errorResults(len(ops), i, "error", err.Error()), nil, nil
			}
			results = append(results, result)
			changes = append(changes, ch...)
			// Track named UUID
			if op.UUIDName != "" {
				if uuidResult, ok := result.(map[string]interface{}); ok {
					if uuidArr, ok := uuidResult["uuid"].([]interface{}); ok && len(uuidArr) == 2 {
						namedUUIDs[op.UUIDName] = uuidArr[1].(string)
					}
				}
			}

		case "select":
			result, err := execSelect(store, op)
			if err != nil {
				rollback(store, snapshots)
				return errorResults(len(ops), i, "error", err.Error()), nil, nil
			}
			results = append(results, result)

		case "update":
			result, ch, err := execUpdate(store, op)
			if err != nil {
				rollback(store, snapshots)
				return errorResults(len(ops), i, "error", err.Error()), nil, nil
			}
			results = append(results, result)
			changes = append(changes, ch...)

		case "delete":
			result, ch, err := execDelete(store, op)
			if err != nil {
				rollback(store, snapshots)
				return errorResults(len(ops), i, "error", err.Error()), nil, nil
			}
			results = append(results, result)
			changes = append(changes, ch...)

		case "mutate":
			result, ch, err := execMutate(store, op)
			if err != nil {
				rollback(store, snapshots)
				return errorResults(len(ops), i, "error", err.Error()), nil, nil
			}
			results = append(results, result)
			changes = append(changes, ch...)

		default:
			rollback(store, snapshots)
			return errorResults(len(ops), i, "error", fmt.Sprintf("unknown operation: %s", op.Op)), nil, nil
		}
	}

	return results, changes, nil
}

// Change describes a single row change for monitor notifications.
type Change struct {
	Table  string
	UUID   string
	Type   string // "insert", "update", "delete"
	OldRow Row
	NewRow Row
}

func execInsert(store *Store, op TransactOp) (interface{}, []Change, error) {
	row := make(Row, len(op.Row))
	for k, v := range op.Row {
		row[k] = v
	}

	uuid, err := store.Insert(op.Table, row)
	if err != nil {
		return nil, nil, fmt.Errorf("insert into %s failed: %w", op.Table, err)
	}

	// Get the stored row (with _uuid) for notifications
	stored, _ := store.GetRow(op.Table, uuid)

	return map[string]interface{}{
		"uuid": []interface{}{"uuid", uuid},
	}, []Change{{
		Table:  op.Table,
		UUID:   uuid,
		Type:   "insert",
		NewRow: stored,
	}}, nil
}

func execSelect(store *Store, op TransactOp) (interface{}, error) {
	rows, err := store.Select(op.Table, op.Where)
	if err != nil {
		return nil, fmt.Errorf("select from %s failed: %w", op.Table, err)
	}

	// Filter columns if specified
	if len(op.Columns) > 0 {
		for i, row := range rows {
			filtered := make(Row, len(op.Columns)+1)
			for _, col := range op.Columns {
				if v, ok := row[col]; ok {
					filtered[col] = v
				}
			}
			if v, ok := row["_uuid"]; ok {
				filtered["_uuid"] = v
			}
			rows[i] = filtered
		}
	}

	return map[string]interface{}{"rows": rows}, nil
}

func execUpdate(store *Store, op TransactOp) (interface{}, []Change, error) {
	// Get old rows for notifications
	oldRows, err := store.Select(op.Table, op.Where)
	if err != nil {
		return nil, nil, fmt.Errorf("update %s failed: %w", op.Table, err)
	}

	count, err := store.Update(op.Table, op.Where, Row(op.Row))
	if err != nil {
		return nil, nil, fmt.Errorf("update %s failed: %w", op.Table, err)
	}

	var changes []Change
	for _, oldRow := range oldRows {
		uuid := extractUUID(oldRow)
		if uuid == "" {
			continue
		}
		newRow, _ := store.GetRow(op.Table, uuid)
		changes = append(changes, Change{
			Table:  op.Table,
			UUID:   uuid,
			Type:   "update",
			OldRow: oldRow,
			NewRow: newRow,
		})
	}

	return map[string]interface{}{"count": count}, changes, nil
}

func execDelete(store *Store, op TransactOp) (interface{}, []Change, error) {
	// Get rows before delete for notifications
	rows, err := store.Select(op.Table, op.Where)
	if err != nil {
		return nil, nil, fmt.Errorf("delete from %s failed: %w", op.Table, err)
	}

	count, err := store.Delete(op.Table, op.Where)
	if err != nil {
		return nil, nil, fmt.Errorf("delete from %s failed: %w", op.Table, err)
	}

	var changes []Change
	for _, row := range rows {
		uuid := extractUUID(row)
		changes = append(changes, Change{
			Table:  op.Table,
			UUID:   uuid,
			Type:   "delete",
			OldRow: row,
		})
	}

	return map[string]interface{}{"count": count}, changes, nil
}

func execMutate(store *Store, op TransactOp) (interface{}, []Change, error) {
	// Get old rows for notifications
	oldRows, err := store.Select(op.Table, op.Where)
	if err != nil {
		return nil, nil, fmt.Errorf("mutate %s failed: %w", op.Table, err)
	}

	count, err := store.Mutate(op.Table, op.Where, op.Mutations)
	if err != nil {
		return nil, nil, fmt.Errorf("mutate %s failed: %w", op.Table, err)
	}

	var changes []Change
	for _, oldRow := range oldRows {
		uuid := extractUUID(oldRow)
		if uuid == "" {
			continue
		}
		newRow, _ := store.GetRow(op.Table, uuid)
		changes = append(changes, Change{
			Table:  op.Table,
			UUID:   uuid,
			Type:   "update",
			OldRow: oldRow,
			NewRow: newRow,
		})
	}

	return map[string]interface{}{"count": count}, changes, nil
}

func extractUUID(row Row) string {
	uuidVal, ok := row["_uuid"]
	if !ok {
		return ""
	}
	arr, ok := uuidVal.([]interface{})
	if !ok || len(arr) != 2 {
		return ""
	}
	s, _ := arr[1].(string)
	return s
}

func resolveNamedUUIDs(row Row, named map[string]string) Row {
	resolved := make(Row, len(row))
	for k, v := range row {
		resolved[k] = resolveNamedUUIDValue(v, named)
	}
	return resolved
}

func resolveNamedUUIDValue(v interface{}, named map[string]string) interface{} {
	arr, ok := v.([]interface{})
	if !ok {
		return v
	}
	if len(arr) == 2 {
		if tag, ok := arr[0].(string); ok {
			if tag == "named-uuid" {
				if name, ok := arr[1].(string); ok {
					if uuid, ok := named[name]; ok {
						return []interface{}{"uuid", uuid}
					}
				}
			}
		}
		// Also resolve inside sets
		if tag, ok := arr[0].(string); ok && tag == "set" {
			if elems, ok := arr[1].([]interface{}); ok {
				newElems := make([]interface{}, len(elems))
				for i, e := range elems {
					newElems[i] = resolveNamedUUIDValue(e, named)
				}
				return []interface{}{"set", newElems}
			}
		}
	}
	return v
}

func resolveNamedUUIDsInWhere(where []interface{}, named map[string]string) []interface{} {
	resolved := make([]interface{}, len(where))
	for i, cond := range where {
		if condSlice, ok := cond.([]interface{}); ok && len(condSlice) == 3 {
			newCond := make([]interface{}, 3)
			newCond[0] = condSlice[0]
			newCond[1] = condSlice[1]
			newCond[2] = resolveNamedUUIDValue(condSlice[2], named)
			resolved[i] = newCond
		} else {
			resolved[i] = cond
		}
	}
	return resolved
}

func errorResults(total, failIdx int, errType, details string) []interface{} {
	results := make([]interface{}, total)
	for i := 0; i < failIdx; i++ {
		results[i] = map[string]interface{}{}
	}
	results[failIdx] = map[string]interface{}{
		"error":   errType,
		"details": details,
	}
	for i := failIdx + 1; i < total; i++ {
		results[i] = nil
	}
	return results
}

func rollback(store *Store, snapshots map[string]map[string]Row) {
	// Restore each table to its snapshot
	store.mu.Lock()
	defer store.mu.Unlock()

	for tableName, rows := range snapshots {
		t, ok := store.tables[tableName]
		if !ok {
			continue
		}
		t.Rows = make(map[string]Row, len(rows))
		for id, row := range rows {
			t.Rows[id] = copyRow(row)
		}
	}
}

// ParseTransactParams parses the params array from a transact RPC call.
func ParseTransactParams(params json.RawMessage) (string, []TransactOp, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(params, &raw); err != nil {
		return "", nil, fmt.Errorf("parse transact params failed: %w", err)
	}
	if len(raw) < 1 {
		return "", nil, fmt.Errorf("parse transact params failed: no database name")
	}

	var db string
	if err := json.Unmarshal(raw[0], &db); err != nil {
		return "", nil, fmt.Errorf("parse transact params failed: invalid db name: %w", err)
	}

	ops := make([]TransactOp, 0, len(raw)-1)
	for i := 1; i < len(raw); i++ {
		var op TransactOp
		if err := json.Unmarshal(raw[i], &op); err != nil {
			return "", nil, fmt.Errorf("parse transact params failed: invalid operation at index %d: %w", i-1, err)
		}
		ops = append(ops, op)
	}

	return db, ops, nil
}

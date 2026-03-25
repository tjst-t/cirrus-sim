package ovsdb

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// Row represents a single row in an OVSDB table.
type Row map[string]interface{}

// Table holds all rows for a single OVSDB table.
type Table struct {
	Rows map[string]Row // keyed by UUID string
}

// Store is an in-memory OVSDB table store for a single database.
type Store struct {
	mu     sync.RWMutex
	tables map[string]*Table
	schema map[string]TableSchema
}

// NewStore creates a new Store initialized with the given schema.
func NewStore(schema map[string]TableSchema) *Store {
	tables := make(map[string]*Table, len(schema))
	for name := range schema {
		tables[name] = &Table{Rows: make(map[string]Row)}
	}
	return &Store{
		tables: tables,
		schema: schema,
	}
}

// Insert adds a new row to the specified table and returns its UUID.
func (s *Store) Insert(table string, row Row) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tables[table]
	if !ok {
		return "", fmt.Errorf("insert failed: unknown table %q", table)
	}

	id := uuid.New().String()
	// Deep copy row to avoid external mutation
	stored := make(Row, len(row))
	for k, v := range row {
		stored[k] = v
	}
	stored["_uuid"] = []interface{}{"uuid", id}
	t.Rows[id] = stored
	return id, nil
}

// Select returns rows matching the given where conditions.
func (s *Store) Select(table string, where []interface{}) ([]Row, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tables[table]
	if !ok {
		return nil, fmt.Errorf("select failed: unknown table %q", table)
	}

	var results []Row
	for _, row := range t.Rows {
		if matchWhere(row, where) {
			results = append(results, copyRow(row))
		}
	}
	if results == nil {
		results = []Row{}
	}
	return results, nil
}

// Update modifies rows matching where with the given values. Returns count of updated rows.
func (s *Store) Update(table string, where []interface{}, values Row) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tables[table]
	if !ok {
		return 0, fmt.Errorf("update failed: unknown table %q", table)
	}

	count := 0
	for id, row := range t.Rows {
		if matchWhere(row, where) {
			for k, v := range values {
				row[k] = v
			}
			t.Rows[id] = row
			count++
		}
	}
	return count, nil
}

// Delete removes rows matching where. Returns count of deleted rows.
func (s *Store) Delete(table string, where []interface{}) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tables[table]
	if !ok {
		return 0, fmt.Errorf("delete failed: unknown table %q", table)
	}

	count := 0
	for id, row := range t.Rows {
		if matchWhere(row, where) {
			delete(t.Rows, id)
			count++
		}
	}
	return count, nil
}

// Mutate applies mutations to rows matching where. Returns count of mutated rows.
func (s *Store) Mutate(table string, where []interface{}, mutations []interface{}) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tables[table]
	if !ok {
		return 0, fmt.Errorf("mutate failed: unknown table %q", table)
	}

	count := 0
	for id, row := range t.Rows {
		if matchWhere(row, where) {
			for _, m := range mutations {
				mut, ok := m.([]interface{})
				if !ok || len(mut) != 3 {
					return 0, fmt.Errorf("mutate failed: invalid mutation format")
				}
				col, _ := mut[0].(string)
				op, _ := mut[1].(string)
				val := mut[2]
				if err := applyMutation(row, col, op, val); err != nil {
					return 0, fmt.Errorf("mutate failed: %w", err)
				}
			}
			t.Rows[id] = row
			count++
		}
	}
	return count, nil
}

// GetRow returns a single row by UUID from the given table.
func (s *Store) GetRow(table, uuid string) (Row, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tables[table]
	if !ok {
		return nil, false
	}
	row, ok := t.Rows[uuid]
	if !ok {
		return nil, false
	}
	return copyRow(row), true
}

// AllRows returns all rows in the specified table.
func (s *Store) AllRows(table string) (map[string]Row, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tables[table]
	if !ok {
		return nil, fmt.Errorf("allrows failed: unknown table %q", table)
	}

	result := make(map[string]Row, len(t.Rows))
	for id, row := range t.Rows {
		result[id] = copyRow(row)
	}
	return result, nil
}

// Reset clears all data from all tables.
func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for name := range s.tables {
		s.tables[name] = &Table{Rows: make(map[string]Row)}
	}
}

// RowExists checks whether a row with the given UUID exists in the given table.
func (s *Store) RowExists(table, uuid string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tables[table]
	if !ok {
		return false
	}
	_, ok = t.Rows[uuid]
	return ok
}

// Stats returns a map of table name to row count.
func (s *Store) Stats() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]int, len(s.tables))
	for name, t := range s.tables {
		stats[name] = len(t.Rows)
	}
	return stats
}

func copyRow(r Row) Row {
	c := make(Row, len(r))
	for k, v := range r {
		c[k] = v
	}
	return c
}

// matchWhere evaluates a list of OVSDB where conditions against a row.
// An empty where list matches all rows.
func matchWhere(row Row, where []interface{}) bool {
	if len(where) == 0 {
		return true
	}
	for _, cond := range where {
		condSlice, ok := cond.([]interface{})
		if !ok || len(condSlice) != 3 {
			return false
		}
		col, _ := condSlice[0].(string)
		fn, _ := condSlice[1].(string)
		expected := condSlice[2]

		actual, exists := row[col]
		if !exists {
			// For == against a non-existent column, no match
			if fn == "==" {
				return false
			}
			if fn == "!=" {
				continue
			}
			return false
		}

		if !evalCondition(actual, fn, expected) {
			return false
		}
	}
	return true
}

func evalCondition(actual interface{}, fn string, expected interface{}) bool {
	// Normalize UUID comparison: if expected is ["uuid", "xxx"], extract "xxx"
	// and compare against actual which may be ["uuid", "xxx"]
	actualStr := normalizeForCompare(actual)
	expectedStr := normalizeForCompare(expected)

	switch fn {
	case "==":
		return compareValues(actualStr, expectedStr) == 0
	case "!=":
		return compareValues(actualStr, expectedStr) != 0
	case "<":
		return compareValues(actualStr, expectedStr) < 0
	case ">":
		return compareValues(actualStr, expectedStr) > 0
	case "<=":
		return compareValues(actualStr, expectedStr) <= 0
	case ">=":
		return compareValues(actualStr, expectedStr) >= 0
	case "includes":
		return setIncludes(actual, expected)
	case "excludes":
		return !setIncludes(actual, expected)
	default:
		return false
	}
}

func normalizeForCompare(v interface{}) interface{} {
	// If v is ["uuid", "xxx"], return "xxx" for comparison
	if arr, ok := v.([]interface{}); ok && len(arr) == 2 {
		if tag, ok := arr[0].(string); ok && tag == "uuid" {
			return arr[1]
		}
	}
	return v
}

func compareValues(a, b interface{}) int {
	// Try numeric comparison
	af, aOk := toFloat64(a)
	bf, bOk := toFloat64(b)
	if aOk && bOk {
		if af < bf {
			return -1
		}
		if af > bf {
			return 1
		}
		return 0
	}

	// String comparison
	as := fmt.Sprintf("%v", a)
	bs := fmt.Sprintf("%v", b)
	if as < bs {
		return -1
	}
	if as > bs {
		return 1
	}
	return 0
}

func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func setIncludes(actual, expected interface{}) bool {
	// Check if actual set/value includes the expected value
	as := fmt.Sprintf("%v", normalizeForCompare(expected))
	// If actual is a set ["set", [...]], check membership
	if arr, ok := actual.([]interface{}); ok && len(arr) == 2 {
		if tag, ok := arr[0].(string); ok && tag == "set" {
			if elems, ok := arr[1].([]interface{}); ok {
				for _, e := range elems {
					if fmt.Sprintf("%v", normalizeForCompare(e)) == as {
						return true
					}
				}
				return false
			}
		}
	}
	return fmt.Sprintf("%v", normalizeForCompare(actual)) == as
}

func applyMutation(row Row, col, op string, val interface{}) error {
	current := row[col]

	switch op {
	case "+=":
		// Numeric addition
		cf, cOk := toFloat64(current)
		vf, vOk := toFloat64(val)
		if cOk && vOk {
			row[col] = cf + vf
			return nil
		}
		// Set insert
		return mutateSetInsert(row, col, current, val)
	case "-=":
		cf, cOk := toFloat64(current)
		vf, vOk := toFloat64(val)
		if cOk && vOk {
			row[col] = cf - vf
			return nil
		}
		// Set delete
		return mutateSetDelete(row, col, current, val)
	case "*=":
		cf, cOk := toFloat64(current)
		vf, vOk := toFloat64(val)
		if cOk && vOk {
			row[col] = cf * vf
			return nil
		}
		return fmt.Errorf("cannot multiply non-numeric column %q", col)
	case "/=":
		cf, cOk := toFloat64(current)
		vf, vOk := toFloat64(val)
		if cOk && vOk {
			if vf == 0 {
				return fmt.Errorf("division by zero on column %q", col)
			}
			row[col] = cf / vf
			return nil
		}
		return fmt.Errorf("cannot divide non-numeric column %q", col)
	case "%=":
		cf, cOk := toFloat64(current)
		vf, vOk := toFloat64(val)
		if cOk && vOk {
			if vf == 0 {
				return fmt.Errorf("modulo by zero on column %q", col)
			}
			row[col] = float64(int64(cf) % int64(vf))
			return nil
		}
		return fmt.Errorf("cannot modulo non-numeric column %q", col)
	case "insert":
		return mutateSetInsert(row, col, current, val)
	case "delete":
		return mutateSetDelete(row, col, current, val)
	default:
		return fmt.Errorf("unknown mutation operator %q", op)
	}
}

func mutateSetInsert(row Row, col string, current, val interface{}) error {
	// If current is a set, add val to it
	if arr, ok := current.([]interface{}); ok && len(arr) == 2 {
		if tag, ok := arr[0].(string); ok && tag == "set" {
			if elems, ok := arr[1].([]interface{}); ok {
				row[col] = []interface{}{"set", append(elems, val)}
				return nil
			}
		}
	}
	// If current is nil or not a set, create a new set
	if current == nil {
		row[col] = []interface{}{"set", []interface{}{val}}
		return nil
	}
	// Current is a single value, make it a set
	row[col] = []interface{}{"set", []interface{}{current, val}}
	return nil
}

func mutateSetDelete(row Row, col string, current, val interface{}) error {
	if arr, ok := current.([]interface{}); ok && len(arr) == 2 {
		if tag, ok := arr[0].(string); ok && tag == "set" {
			if elems, ok := arr[1].([]interface{}); ok {
				valStr := fmt.Sprintf("%v", normalizeForCompare(val))
				var newElems []interface{}
				for _, e := range elems {
					if fmt.Sprintf("%v", normalizeForCompare(e)) != valStr {
						newElems = append(newElems, e)
					}
				}
				if newElems == nil {
					newElems = []interface{}{}
				}
				row[col] = []interface{}{"set", newElems}
				return nil
			}
		}
	}
	return nil
}

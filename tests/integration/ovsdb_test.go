//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"
)

// JSON-RPC types for OVSDB protocol testing.
type ovsdbRequest struct {
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
	ID     int           `json:"id"`
}

type ovsdbResponse struct {
	Result json.RawMessage `json:"result"`
	Error  interface{}     `json:"error"`
	ID     int             `json:"id"`
}

type ovsdbClient struct {
	conn   net.Conn
	enc    *json.Encoder
	dec    *json.Decoder
	nextID int
}

const ovsdbAddr = "127.0.0.1:6641"

func dialOVSDB(t *testing.T) *ovsdbClient {
	t.Helper()
	conn, err := net.DialTimeout("tcp", ovsdbAddr, 5*time.Second)
	if err != nil {
		t.Fatalf("TCP connect to %s failed: %v", ovsdbAddr, err)
	}
	c := &ovsdbClient{conn: conn, enc: json.NewEncoder(conn), dec: json.NewDecoder(conn), nextID: 1}
	t.Cleanup(func() { conn.Close() })
	return c
}

func (c *ovsdbClient) call(t *testing.T, method string, params ...interface{}) json.RawMessage {
	t.Helper()
	id := c.nextID
	c.nextID++
	req := ovsdbRequest{Method: method, Params: params, ID: id}
	if err := c.enc.Encode(req); err != nil {
		t.Fatalf("encode %s request: %v", method, err)
	}
	var resp ovsdbResponse
	if err := c.dec.Decode(&resp); err != nil {
		t.Fatalf("decode %s response: %v", method, err)
	}
	if resp.Error != nil {
		t.Fatalf("%s returned error: %v", method, resp.Error)
	}
	return resp.Result
}

func (c *ovsdbClient) callMayFail(method string, params ...interface{}) (json.RawMessage, error) {
	id := c.nextID
	c.nextID++
	req := ovsdbRequest{Method: method, Params: params, ID: id}
	if err := c.enc.Encode(req); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	var resp ovsdbResponse
	if err := c.dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("rpc error: %v", resp.Error)
	}
	return resp.Result, nil
}

func extractUUID(t *testing.T, result json.RawMessage, index int) string {
	t.Helper()
	var results []map[string]interface{}
	if err := json.Unmarshal(result, &results); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if index >= len(results) {
		t.Fatalf("result index %d out of range (len=%d)", index, len(results))
	}
	uuidArr, ok := results[index]["uuid"].([]interface{})
	if !ok || len(uuidArr) != 2 {
		t.Fatalf("expected [uuid, value], got %v", results[index]["uuid"])
	}
	return fmt.Sprint(uuidArr[1])
}

func selectRows(t *testing.T, result json.RawMessage) []interface{} {
	t.Helper()
	var results []map[string]interface{}
	if err := json.Unmarshal(result, &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) == 0 {
		return nil
	}
	rows, _ := results[0]["rows"].([]interface{})
	return rows
}

func updateCount(t *testing.T, result json.RawMessage) int {
	t.Helper()
	var results []map[string]interface{}
	if err := json.Unmarshal(result, &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) == 0 {
		return 0
	}
	count, _ := results[0]["count"].(float64)
	return int(count)
}

// ── Tests ──

func TestOVSDBEcho(t *testing.T) {
	c := dialOVSDB(t)
	result := c.call(t, "echo", "hello", "world")
	var params []interface{}
	if err := json.Unmarshal(result, &params); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(params) < 1 {
		t.Error("echo should return params")
	}
}

func TestOVSDBListDBs(t *testing.T) {
	c := dialOVSDB(t)
	result := c.call(t, "list_dbs")
	var dbs []string
	if err := json.Unmarshal(result, &dbs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	found := false
	for _, db := range dbs {
		if db == "OVN_Northbound" {
			found = true
		}
	}
	if !found {
		t.Errorf("list_dbs = %v, want OVN_Northbound", dbs)
	}
}

func TestOVSDBGetSchema(t *testing.T) {
	c := dialOVSDB(t)
	result := c.call(t, "get_schema", "OVN_Northbound")

	var schema map[string]interface{}
	if err := json.Unmarshal(result, &schema); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if schema["name"] != "OVN_Northbound" {
		t.Errorf("schema name = %v", schema["name"])
	}

	tables, _ := schema["tables"].(map[string]interface{})
	requiredTables := []string{
		"Logical_Switch", "Logical_Switch_Port", "Logical_Router",
		"Logical_Router_Port", "ACL", "DHCP_Options", "NAT",
		"DNS", "Load_Balancer", "Address_Set", "Port_Group", "Gateway_Chassis",
	}
	for _, tbl := range requiredTables {
		if tables[tbl] == nil {
			t.Errorf("missing table %s in schema", tbl)
		}
	}
}

func TestOVSDBGetSchemaUnknownDB(t *testing.T) {
	c := dialOVSDB(t)
	_, err := c.callMayFail("get_schema", "NoSuchDB")
	if err == nil {
		t.Error("get_schema for unknown DB should return error")
	}
}

func TestOVSDBTransactInsertSelectDeleteLogicalSwitch(t *testing.T) {
	c := dialOVSDB(t)

	// Insert
	result := c.call(t, "transact", "OVN_Northbound",
		map[string]interface{}{
			"op": "insert", "table": "Logical_Switch",
			"row":       map[string]interface{}{"name": "test-ls", "external_ids": []interface{}{"map", []interface{}{[]interface{}{"key", "val"}}}},
			"uuid-name": "new_ls",
		},
	)
	uuid := extractUUID(t, result, 0)
	if uuid == "" {
		t.Fatal("expected UUID")
	}

	// Select by name
	result = c.call(t, "transact", "OVN_Northbound",
		map[string]interface{}{
			"op": "select", "table": "Logical_Switch",
			"where": []interface{}{[]interface{}{"name", "==", "test-ls"}},
		},
	)
	rows := selectRows(t, result)
	if len(rows) != 1 {
		t.Errorf("select got %d rows, want 1", len(rows))
	}

	// Delete
	result = c.call(t, "transact", "OVN_Northbound",
		map[string]interface{}{
			"op": "delete", "table": "Logical_Switch",
			"where": []interface{}{[]interface{}{"name", "==", "test-ls"}},
		},
	)
	if n := updateCount(t, result); n != 1 {
		t.Errorf("delete count = %d, want 1", n)
	}

	// Verify deleted
	result = c.call(t, "transact", "OVN_Northbound",
		map[string]interface{}{
			"op": "select", "table": "Logical_Switch",
			"where": []interface{}{[]interface{}{"name", "==", "test-ls"}},
		},
	)
	rows = selectRows(t, result)
	if len(rows) != 0 {
		t.Errorf("after delete got %d rows, want 0", len(rows))
	}
}

func TestOVSDBTransactInsertPort(t *testing.T) {
	c := dialOVSDB(t)

	result := c.call(t, "transact", "OVN_Northbound",
		map[string]interface{}{
			"op": "insert", "table": "Logical_Switch_Port",
			"row": map[string]interface{}{
				"name":      "test-port",
				"addresses": []interface{}{"set", []interface{}{"02:ac:10:ff:00:01 192.168.1.10"}},
				"external_ids": []interface{}{"map", []interface{}{
					[]interface{}{"cirrus_port_id", "port-001"},
				}},
			},
		},
	)
	uuid := extractUUID(t, result, 0)
	if uuid == "" {
		t.Fatal("expected port UUID")
	}

	// Cleanup
	c.call(t, "transact", "OVN_Northbound",
		map[string]interface{}{"op": "delete", "table": "Logical_Switch_Port", "where": []interface{}{[]interface{}{"name", "==", "test-port"}}},
	)
}

func TestOVSDBTransactUpdate(t *testing.T) {
	c := dialOVSDB(t)

	// Insert
	c.call(t, "transact", "OVN_Northbound",
		map[string]interface{}{"op": "insert", "table": "Logical_Switch_Port", "row": map[string]interface{}{"name": "upd-port"}},
	)

	// Update
	result := c.call(t, "transact", "OVN_Northbound",
		map[string]interface{}{
			"op": "update", "table": "Logical_Switch_Port",
			"where": []interface{}{[]interface{}{"name", "==", "upd-port"}},
			"row":   map[string]interface{}{"up": true},
		},
	)
	if n := updateCount(t, result); n != 1 {
		t.Errorf("update count = %d, want 1", n)
	}

	// Cleanup
	c.call(t, "transact", "OVN_Northbound",
		map[string]interface{}{"op": "delete", "table": "Logical_Switch_Port", "where": []interface{}{[]interface{}{"name", "==", "upd-port"}}},
	)
}

func TestOVSDBTransactMutate(t *testing.T) {
	c := dialOVSDB(t)

	c.call(t, "transact", "OVN_Northbound",
		map[string]interface{}{"op": "insert", "table": "Logical_Switch_Port", "row": map[string]interface{}{"name": "mut-port"}},
	)

	result := c.call(t, "transact", "OVN_Northbound",
		map[string]interface{}{
			"op": "mutate", "table": "Logical_Switch_Port",
			"where":     []interface{}{[]interface{}{"name", "==", "mut-port"}},
			"mutations": []interface{}{[]interface{}{"tag", "+=", 42}},
		},
	)
	if n := updateCount(t, result); n != 1 {
		t.Errorf("mutate count = %d, want 1", n)
	}

	c.call(t, "transact", "OVN_Northbound",
		map[string]interface{}{"op": "delete", "table": "Logical_Switch_Port", "where": []interface{}{[]interface{}{"name", "==", "mut-port"}}},
	)
}

func TestOVSDBTransactMultiOp(t *testing.T) {
	c := dialOVSDB(t)

	result := c.call(t, "transact", "OVN_Northbound",
		map[string]interface{}{"op": "insert", "table": "Address_Set", "row": map[string]interface{}{"name": "as-multi"}},
		map[string]interface{}{"op": "insert", "table": "Port_Group", "row": map[string]interface{}{"name": "pg-multi"}},
	)

	var results []map[string]interface{}
	if err := json.Unmarshal(result, &results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("multi-op got %d results, want 2", len(results))
	}

	// Cleanup
	c.call(t, "transact", "OVN_Northbound",
		map[string]interface{}{"op": "delete", "table": "Address_Set", "where": []interface{}{[]interface{}{"name", "==", "as-multi"}}},
		map[string]interface{}{"op": "delete", "table": "Port_Group", "where": []interface{}{[]interface{}{"name", "==", "pg-multi"}}},
	)
}

func TestOVSDBTransactInsertAllTables(t *testing.T) {
	c := dialOVSDB(t)

	tables := []struct {
		table string
		row   map[string]interface{}
	}{
		{"Logical_Switch", map[string]interface{}{"name": "tbl-ls"}},
		{"Logical_Switch_Port", map[string]interface{}{"name": "tbl-lsp"}},
		{"Logical_Router", map[string]interface{}{"name": "tbl-lr"}},
		{"Logical_Router_Port", map[string]interface{}{"name": "tbl-lrp", "mac": "02:00:00:00:00:01"}},
		{"Logical_Router_Static_Route", map[string]interface{}{"ip_prefix": "10.0.0.0/8", "nexthop": "192.168.1.1"}},
		{"ACL", map[string]interface{}{"action": "allow", "direction": "from-lport", "match": "1", "priority": 100}},
		{"NAT", map[string]interface{}{"type": "snat", "external_ip": "1.2.3.4", "logical_ip": "10.0.0.0/24"}},
		{"DHCP_Options", map[string]interface{}{"cidr": "10.0.0.0/24"}},
		{"DNS", map[string]interface{}{"records": []interface{}{"map", []interface{}{[]interface{}{"host.example.com", "10.0.0.1"}}}}},
		{"Load_Balancer", map[string]interface{}{"name": "tbl-lb"}},
		{"Address_Set", map[string]interface{}{"name": "tbl-as"}},
		{"Port_Group", map[string]interface{}{"name": "tbl-pg"}},
		{"Gateway_Chassis", map[string]interface{}{"name": "tbl-gc", "chassis_name": "ch1", "priority": 10}},
	}

	for _, tt := range tables {
		t.Run(tt.table, func(t *testing.T) {
			result := c.call(t, "transact", "OVN_Northbound",
				map[string]interface{}{"op": "insert", "table": tt.table, "row": tt.row},
			)
			uuid := extractUUID(t, result, 0)
			if uuid == "" {
				t.Errorf("insert into %s: no UUID returned", tt.table)
			}
		})
	}
}

func TestOVSDBMonitor(t *testing.T) {
	c := dialOVSDB(t)

	// Insert a switch first
	c.call(t, "transact", "OVN_Northbound",
		map[string]interface{}{"op": "insert", "table": "Logical_Switch", "row": map[string]interface{}{"name": "mon-ls"}},
	)

	// Monitor
	mc := dialOVSDB(t)
	result := mc.call(t, "monitor", "OVN_Northbound", "mon-test",
		map[string]interface{}{
			"Logical_Switch": map[string]interface{}{
				"columns": []interface{}{"name", "external_ids"},
			},
		},
	)

	var monResult map[string]interface{}
	if err := json.Unmarshal(result, &monResult); err != nil {
		t.Fatalf("unmarshal monitor result: %v", err)
	}
	lsData, _ := monResult["Logical_Switch"].(map[string]interface{})
	if len(lsData) == 0 {
		t.Error("monitor should return initial data for Logical_Switch")
	}

	// Cancel
	mc.call(t, "monitor_cancel", "mon-test")

	// Cleanup
	c.call(t, "transact", "OVN_Northbound",
		map[string]interface{}{"op": "delete", "table": "Logical_Switch", "where": []interface{}{[]interface{}{"name", "==", "mon-ls"}}},
	)
}

package ovsdb

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestServerListDBsAndEcho(t *testing.T) {
	store := newTestStore()
	logger := testLogger()
	srv := NewServer(store, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Listen on random port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	if err := srv.Listen(ctx, "test", addr); err != nil {
		t.Fatalf("server listen failed: %v", err)
	}
	defer srv.StopAll()

	// Connect client
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)

	// Test list_dbs
	id := json.RawMessage(`1`)
	err = enc.Encode(Request{Method: "list_dbs", Params: json.RawMessage(`[]`), ID: &id})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	var resp Response
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	dbs, ok := resp.Result.([]interface{})
	if !ok || len(dbs) != 1 || dbs[0] != "OVN_Northbound" {
		t.Errorf("unexpected list_dbs result: %v", resp.Result)
	}

	// Test echo
	id2 := json.RawMessage(`2`)
	err = enc.Encode(Request{Method: "echo", Params: json.RawMessage(`["hello", "world"]`), ID: &id2})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	echoResult, ok := resp.Result.([]interface{})
	if !ok || len(echoResult) != 2 {
		t.Errorf("unexpected echo result: %v", resp.Result)
	}
}

func TestServerTransact(t *testing.T) {
	store := newTestStore()
	logger := testLogger()
	srv := NewServer(store, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	if err := srv.Listen(ctx, "test", addr); err != nil {
		t.Fatalf("server listen failed: %v", err)
	}
	defer srv.StopAll()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)

	// Insert via transact
	id := json.RawMessage(`10`)
	params := `["OVN_Northbound", {"op": "insert", "table": "Logical_Switch", "row": {"name": "test-ls"}}]`
	err = enc.Encode(Request{Method: "transact", Params: json.RawMessage(params), ID: &id})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	var resp Response
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	results, ok := resp.Result.([]interface{})
	if !ok || len(results) != 1 {
		t.Fatalf("unexpected transact result: %v", resp.Result)
	}

	insertResult, ok := results[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", results[0])
	}
	if insertResult["uuid"] == nil {
		t.Error("expected uuid in insert result")
	}

	// Select back
	id2 := json.RawMessage(`11`)
	params2 := `["OVN_Northbound", {"op": "select", "table": "Logical_Switch", "where": [["name", "==", "test-ls"]]}]`
	err = enc.Encode(Request{Method: "transact", Params: json.RawMessage(params2), ID: &id2})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	results, ok = resp.Result.([]interface{})
	if !ok || len(results) != 1 {
		t.Fatalf("unexpected select result: %v", resp.Result)
	}

	selectResult, ok := results[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", results[0])
	}
	rows, ok := selectResult["rows"].([]interface{})
	if !ok || len(rows) != 1 {
		t.Errorf("expected 1 row, got %v", selectResult["rows"])
	}
}

func TestServerGetSchema(t *testing.T) {
	store := newTestStore()
	logger := testLogger()
	srv := NewServer(store, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	if err := srv.Listen(ctx, "test", addr); err != nil {
		t.Fatalf("server listen failed: %v", err)
	}
	defer srv.StopAll()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)

	id := json.RawMessage(`3`)
	err = enc.Encode(Request{Method: "get_schema", Params: json.RawMessage(`["OVN_Northbound"]`), ID: &id})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	var resp Response
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	schema, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	if schema["name"] != "OVN_Northbound" {
		t.Errorf("expected name=OVN_Northbound, got %v", schema["name"])
	}
	tables, ok := schema["tables"].(map[string]interface{})
	if !ok {
		t.Fatal("expected tables map")
	}
	if _, ok := tables["Logical_Switch"]; !ok {
		t.Error("expected Logical_Switch table in schema")
	}
}

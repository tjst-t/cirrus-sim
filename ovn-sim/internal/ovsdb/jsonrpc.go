// Package ovsdb implements the OVSDB protocol (RFC 7047) server.
package ovsdb

import "encoding/json"

// Request represents a JSON-RPC 1.0 request.
type Request struct {
	Method string            `json:"method"`
	Params json.RawMessage   `json:"params"`
	ID     *json.RawMessage  `json:"id"`
}

// Response represents a JSON-RPC 1.0 response.
type Response struct {
	Result interface{}      `json:"result"`
	Error  interface{}      `json:"error"`
	ID     *json.RawMessage `json:"id"`
}

// Notification represents a JSON-RPC 1.0 notification (id is null).
type Notification struct {
	Method string      `json:"method"`
	Params interface{} `json:"params"`
	ID     interface{} `json:"id"` // always nil/null
}

// TransactOp represents a single operation within a transact request.
type TransactOp struct {
	Op        string              `json:"op"`
	Table     string              `json:"table"`
	Row       map[string]interface{} `json:"row,omitempty"`
	Where     []interface{}       `json:"where,omitempty"`
	UUIDName  string              `json:"uuid-name,omitempty"`
	Mutations []interface{}       `json:"mutations,omitempty"`
	Columns   []string            `json:"columns,omitempty"`
}

// OpResult represents the result of a single transact operation.
type OpResult struct {
	UUID    interface{}          `json:"uuid,omitempty"`
	Rows    []map[string]interface{} `json:"rows,omitempty"`
	Count   *int                 `json:"count,omitempty"`
	Error   string               `json:"error,omitempty"`
	Details string               `json:"details,omitempty"`
}

package ovsdb

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
)

// Server is a TCP server that speaks the OVSDB JSON-RPC protocol.
type Server struct {
	store    *Store
	monitors *MonitorManager
	logger   *slog.Logger

	mu        sync.Mutex
	listeners map[string]net.Listener
	wg        sync.WaitGroup
	clients   map[string]*clientConn
	clientSeq int
}

// clientConn tracks a connected client.
type clientConn struct {
	key    string
	conn   net.Conn
	enc    *json.Encoder
	encMu  sync.Mutex
	server *Server
}

// SendNotification implements ClientNotifier.
func (cc *clientConn) SendNotification(method string, params interface{}) error {
	cc.encMu.Lock()
	defer cc.encMu.Unlock()

	return cc.enc.Encode(Notification{
		Method: method,
		Params: params,
		ID:     nil,
	})
}

// NewServer creates a new OVSDB protocol server.
func NewServer(store *Store, logger *slog.Logger) *Server {
	return &Server{
		store:     store,
		monitors:  NewMonitorManager(logger),
		logger:    logger,
		listeners: make(map[string]net.Listener),
		clients:   make(map[string]*clientConn),
	}
}

// Store returns the server's underlying store.
func (s *Server) Store() *Store {
	return s.store
}

// Monitors returns the server's monitor manager.
func (s *Server) Monitors() *MonitorManager {
	return s.monitors
}

// Listen starts listening on the given address and accepting connections.
func (s *Server) Listen(ctx context.Context, id, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("ovsdb server listen failed: %w", err)
	}

	s.mu.Lock()
	s.listeners[id] = ln
	s.mu.Unlock()

	s.logger.Info("OVSDB server listening", "id", id, "addr", addr)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.acceptLoop(ctx, ln)
	}()

	return nil
}

// Stop stops the listener identified by id.
func (s *Server) Stop(id string) {
	s.mu.Lock()
	ln, ok := s.listeners[id]
	if ok {
		delete(s.listeners, id)
	}
	s.mu.Unlock()

	if ok {
		ln.Close()
	}
}

// StopAll stops all listeners.
func (s *Server) StopAll() {
	s.mu.Lock()
	for id, ln := range s.listeners {
		ln.Close()
		delete(s.listeners, id)
	}
	// Close all client connections
	for key, cc := range s.clients {
		cc.conn.Close()
		delete(s.clients, key)
	}
	s.mu.Unlock()
}

// Wait waits for all goroutines to finish.
func (s *Server) Wait() {
	s.wg.Wait()
}

func (s *Server) acceptLoop(ctx context.Context, ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				// Check if listener was closed
				if opErr, ok := err.(*net.OpError); ok && !opErr.Temporary() {
					return
				}
				s.logger.Warn("accept failed", "error", err)
				continue
			}
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(ctx, conn)
		}()
	}
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	s.mu.Lock()
	s.clientSeq++
	clientKey := fmt.Sprintf("client-%d", s.clientSeq)
	cc := &clientConn{
		key:    clientKey,
		conn:   conn,
		enc:    json.NewEncoder(conn),
		server: s,
	}
	s.clients[clientKey] = cc
	s.mu.Unlock()

	defer func() {
		s.monitors.RemoveClient(clientKey)
		s.mu.Lock()
		delete(s.clients, clientKey)
		s.mu.Unlock()
	}()

	s.logger.Info("client connected", "client", clientKey, "remote", conn.RemoteAddr())

	dec := json.NewDecoder(conn)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var req Request
		if err := dec.Decode(&req); err != nil {
			s.logger.Debug("client disconnected", "client", clientKey, "error", err)
			return
		}

		resp := s.handleRequest(cc, &req)
		if resp != nil {
			cc.encMu.Lock()
			if err := cc.enc.Encode(resp); err != nil {
				cc.encMu.Unlock()
				s.logger.Warn("failed to send response", "client", clientKey, "error", err)
				return
			}
			cc.encMu.Unlock()
		}
	}
}

func (s *Server) handleRequest(cc *clientConn, req *Request) *Response {
	s.logger.Debug("handling request", "method", req.Method)

	switch req.Method {
	case "list_dbs":
		return &Response{
			Result: []string{"OVN_Northbound"},
			Error:  nil,
			ID:     req.ID,
		}

	case "get_schema":
		return s.handleGetSchema(req)

	case "echo":
		return s.handleEcho(req)

	case "transact":
		return s.handleTransact(cc, req)

	case "monitor":
		return s.handleMonitor(cc, req)

	case "monitor_cancel":
		return s.handleMonitorCancel(cc, req)

	default:
		return &Response{
			Result: nil,
			Error:  map[string]string{"error": "unknown method", "details": req.Method},
			ID:     req.ID,
		}
	}
}

func (s *Server) handleGetSchema(req *Request) *Response {
	var params []string
	if err := json.Unmarshal(req.Params, &params); err != nil || len(params) == 0 {
		return &Response{
			Error: map[string]string{"error": "invalid params"},
			ID:    req.ID,
		}
	}

	if params[0] != "OVN_Northbound" {
		return &Response{
			Error: map[string]string{"error": "unknown database", "details": params[0]},
			ID:    req.ID,
		}
	}

	return &Response{
		Result: SchemaJSON(),
		ID:     req.ID,
	}
}

func (s *Server) handleEcho(req *Request) *Response {
	var params []interface{}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &Response{
			Result: []interface{}{},
			ID:     req.ID,
		}
	}
	return &Response{
		Result: params,
		ID:     req.ID,
	}
}

func (s *Server) handleTransact(cc *clientConn, req *Request) *Response {
	db, ops, err := ParseTransactParams(req.Params)
	if err != nil {
		return &Response{
			Error: map[string]string{"error": "invalid params", "details": err.Error()},
			ID:    req.ID,
		}
	}

	if db != "OVN_Northbound" {
		return &Response{
			Error: map[string]string{"error": "unknown database", "details": db},
			ID:    req.ID,
		}
	}

	results, changes, err := Transact(s.store, ops)
	if err != nil {
		return &Response{
			Error: map[string]string{"error": "transaction failed", "details": err.Error()},
			ID:    req.ID,
		}
	}

	// Send monitor notifications for changes
	for _, ch := range changes {
		switch ch.Type {
		case "insert":
			s.monitors.NotifyInsert(ch.Table, ch.UUID, ch.NewRow)
		case "update":
			s.monitors.NotifyUpdate(ch.Table, ch.UUID, ch.OldRow, ch.NewRow)
		case "delete":
			s.monitors.NotifyDelete(ch.Table, ch.UUID, ch.OldRow)
		}
	}

	return &Response{
		Result: results,
		ID:     req.ID,
	}
}

func (s *Server) handleMonitor(cc *clientConn, req *Request) *Response {
	db, monitorID, requests, err := ParseMonitorParams(req.Params)
	if err != nil {
		return &Response{
			Error: map[string]string{"error": "invalid params", "details": err.Error()},
			ID:    req.ID,
		}
	}

	if db != "OVN_Northbound" {
		return &Response{
			Error: map[string]string{"error": "unknown database", "details": db},
			ID:    req.ID,
		}
	}

	result, err := s.monitors.Register(cc.key, cc, monitorID, requests, s.store)
	if err != nil {
		return &Response{
			Error: map[string]string{"error": "monitor failed", "details": err.Error()},
			ID:    req.ID,
		}
	}

	return &Response{
		Result: result,
		ID:     req.ID,
	}
}

func (s *Server) handleMonitorCancel(cc *clientConn, req *Request) *Response {
	var params []string
	if err := json.Unmarshal(req.Params, &params); err != nil || len(params) == 0 {
		return &Response{
			Error: map[string]string{"error": "invalid params"},
			ID:    req.ID,
		}
	}

	if err := s.monitors.Cancel(cc.key, params[0]); err != nil {
		return &Response{
			Error: map[string]string{"error": "monitor_cancel failed", "details": err.Error()},
			ID:    req.ID,
		}
	}

	return &Response{
		Result: map[string]interface{}{},
		ID:     req.ID,
	}
}

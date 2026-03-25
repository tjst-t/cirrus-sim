// Package libvirtsim provides the libvirt RPC protocol simulator.
package libvirtsim

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/tjst-t/cirrus-sim/libvirt-sim/internal/handler"
	"github.com/tjst-t/cirrus-sim/libvirt-sim/internal/rpc"
	"github.com/tjst-t/cirrus-sim/libvirt-sim/internal/state"
)

// Server is the libvirt-sim server instance.
type Server struct {
	httpServer *http.Server
	rpcServer  *rpc.Server
	logger     *slog.Logger
}

// New creates a new libvirt-sim Server.
func New(port string, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	store := state.NewStore()
	rpcServer := rpc.NewServer(store, logger)
	mgmt := handler.NewManagement(store, rpcServer, logger)

	mux := http.NewServeMux()
	mgmt.RegisterRoutes(mux)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	return &Server{
		httpServer: &http.Server{
			Addr:              fmt.Sprintf(":%s", port),
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		},
		rpcServer: rpcServer,
		logger:    logger,
	}
}

// Start starts the management API server in a goroutine.
// Per-host libvirt RPC listeners are started when hosts are registered via the management API.
func (s *Server) Start() {
	go func() {
		s.logger.Info("libvirt-sim starting", "addr", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("libvirt-sim server failed", "error", err)
		}
	}()
}

// Shutdown gracefully shuts down the management API and all RPC listeners.
func (s *Server) Shutdown(ctx context.Context) error {
	s.rpcServer.StopAll()
	s.rpcServer.Wait()
	return s.httpServer.Shutdown(ctx)
}

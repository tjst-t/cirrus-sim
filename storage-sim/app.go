// Package storagesim provides the Cirrus Storage API simulator.
package storagesim

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/tjst-t/cirrus-sim/storage-sim/internal/handler"
	"github.com/tjst-t/cirrus-sim/storage-sim/internal/sim"
	"github.com/tjst-t/cirrus-sim/storage-sim/internal/state"
)

// Server is the storage-sim server instance.
type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

// New creates a new storage-sim Server.
func New(port string, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	store := state.NewStore(logger)
	mux := http.NewServeMux()

	storageHandler := handler.NewStorageHandler(store, logger)
	storageHandler.RegisterRoutes(mux)

	mgmtHandler := sim.NewManagementHandler(store, logger)
	mgmtHandler.RegisterRoutes(mux)

	return &Server{
		httpServer: &http.Server{
			Addr:              fmt.Sprintf(":%s", port),
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// Start starts the server in a goroutine. Call Shutdown to stop.
func (s *Server) Start() {
	go func() {
		s.logger.Info("storage-sim starting", "addr", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("storage-sim server failed", "error", err)
		}
	}()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// Package netboxsim provides the NetBox REST API simulator.
package netboxsim

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/tjst-t/cirrus-sim/netbox-sim/internal/handler"
	"github.com/tjst-t/cirrus-sim/netbox-sim/internal/state"
)

// Server is the netbox-sim server instance.
type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

// New creates a new netbox-sim Server.
func New(port string, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	store := state.NewStore()
	h := handler.NewHandler(store)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	return &Server{
		httpServer: &http.Server{
			Addr:              fmt.Sprintf(":%s", port),
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// Start starts the server in a goroutine.
func (s *Server) Start() {
	go func() {
		s.logger.Info("netbox-sim starting", "addr", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("netbox-sim server failed", "error", err)
		}
	}()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

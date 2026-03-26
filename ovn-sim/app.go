// Package ovnsim provides the OVN Northbound DB simulator (OVSDB protocol).
package ovnsim

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/tjst-t/cirrus-sim/ovn-sim/internal/handler"
	"github.com/tjst-t/cirrus-sim/ovn-sim/internal/state"
)

// Server is the ovn-sim server instance.
type Server struct {
	httpServer *http.Server
	manager    *state.Manager
	ctx        context.Context
	cancel     context.CancelFunc
	logger     *slog.Logger
}

// New creates a new ovn-sim Server.
func New(port string, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())
	manager := state.NewManager(logger)
	mgmt := handler.NewManagement(ctx, manager, logger)

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
		manager: manager,
		ctx:     ctx,
		cancel:  cancel,
		logger:  logger,
	}
}

// Start starts the management API server in a goroutine.
func (s *Server) Start() {
	go func() {
		s.logger.Info("ovn-sim starting", "addr", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("ovn-sim server failed", "error", err)
		}
	}()
}

// SeedCluster creates an OVN cluster and starts its OVSDB listener.
func (s *Server) SeedCluster(clusterID string, port int) error {
	_, err := s.manager.CreateCluster(s.ctx, clusterID, port)
	if err != nil {
		return fmt.Errorf("create cluster %s: %w", clusterID, err)
	}
	return nil
}

// Shutdown gracefully shuts down the server and all OVSDB listeners.
func (s *Server) Shutdown(ctx context.Context) error {
	s.cancel()
	s.manager.Reset()
	return s.httpServer.Shutdown(ctx)
}

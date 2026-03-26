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
	store      *state.Store
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
		store:     store,
		logger:    logger,
	}
}

// SeedHost registers a host and starts its RPC listener.
func (s *Server) SeedHost(ctx context.Context, cfg HostConfig) error {
	host := &state.Host{
		HostID:             cfg.HostID,
		LibvirtPort:        cfg.LibvirtPort,
		CPUModel:           cfg.CPUModel,
		CPUSockets:         cfg.CPUSockets,
		CoresPerSocket:     cfg.CoresPerSocket,
		ThreadsPerCore:     cfg.ThreadsPerCore,
		MemoryMB:           cfg.MemoryMB,
		CPUOvercommitRatio: cfg.CPUOvercommitRatio,
		MemOvercommitRatio: cfg.MemOvercommitRatio,
	}
	if err := s.store.AddHost(host); err != nil {
		return fmt.Errorf("add host %s: %w", cfg.HostID, err)
	}
	if err := s.rpcServer.StartListener(ctx, cfg.HostID, cfg.LibvirtPort); err != nil {
		_ = s.store.RemoveHost(cfg.HostID)
		return fmt.Errorf("start listener for host %s: %w", cfg.HostID, err)
	}
	return nil
}

// HostConfig holds the configuration for seeding a host.
type HostConfig struct {
	HostID             string
	LibvirtPort        int
	CPUModel           string
	CPUSockets         int
	CoresPerSocket     int
	ThreadsPerCore     int
	MemoryMB           int64
	CPUOvercommitRatio float64
	MemOvercommitRatio float64
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

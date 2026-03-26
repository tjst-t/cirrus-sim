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
	store      *state.Store
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
		store:  store,
		logger: logger,
	}
}

// SeedSite adds a site and returns its ID.
func (s *Server) SeedSite(name string) int {
	return s.store.AddSite(name, "active", nil, nil)
}

// SeedLocation adds a location under a site with an optional parent. Returns its ID.
func (s *Server) SeedLocation(name string, siteID, parentID int, customFields map[string]string) int {
	return s.store.AddLocation(name, siteID, parentID, customFields)
}

// SeedRack adds a rack under a location. Returns its ID.
func (s *Server) SeedRack(name string, siteID, locationID int, customFields map[string]string) int {
	return s.store.AddRack(name, siteID, locationID, "active", customFields)
}

// SeedDevice adds a device in a rack.
func (s *Server) SeedDevice(name, role string, siteID, locationID, rackID, position int, customFields map[string]string) {
	s.store.AddDevice(name, role, siteID, locationID, rackID, position, "active", customFields)
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

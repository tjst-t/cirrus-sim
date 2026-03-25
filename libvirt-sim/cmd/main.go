// Package main is the entry point for libvirt-sim.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tjst-t/cirrus-sim/libvirt-sim/internal/handler"
	"github.com/tjst-t/cirrus-sim/libvirt-sim/internal/rpc"
	"github.com/tjst-t/cirrus-sim/libvirt-sim/internal/state"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	store := state.NewStore()
	rpcServer := rpc.NewServer(store, logger)
	mgmt := handler.NewManagement(store, rpcServer, logger)

	mux := http.NewServeMux()
	mgmt.RegisterRoutes(mux)

	// Health check endpoint
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	port := os.Getenv("MANAGEMENT_PORT")
	if port == "" {
		port = "8100"
	}

	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Info("received signal, shutting down", "signal", sig)
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("HTTP server shutdown failed", "error", err)
		}
	}()

	_ = ctx // Used by RPC server listeners implicitly through cancel

	logger.Info("starting libvirt-sim management API", "port", port)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}

	// Stop all RPC listeners
	rpcServer.StopAll()
	rpcServer.Wait()
	logger.Info("libvirt-sim stopped")
}

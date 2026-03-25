// Package main is the entry point for storage-sim, the Cirrus Storage API simulator.
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

	"github.com/tjst-t/cirrus-sim/storage-sim/internal/handler"
	"github.com/tjst-t/cirrus-sim/storage-sim/internal/sim"
	"github.com/tjst-t/cirrus-sim/storage-sim/internal/state"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	port := os.Getenv("STORAGE_SIM_PORT")
	if port == "" {
		port = "8500"
	}

	store := state.NewStore(logger)
	mux := http.NewServeMux()

	storageHandler := handler.NewStorageHandler(store, logger)
	storageHandler.RegisterRoutes(mux)

	mgmtHandler := sim.NewManagementHandler(store, logger)
	mgmtHandler.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("storage-sim starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down storage-sim")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown failed", "error", err)
	}
}

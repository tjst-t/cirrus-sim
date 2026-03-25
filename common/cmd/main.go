// Package main provides the entry point for the common event log API server.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tjst-t/cirrus-sim/common/internal/handler"
	"github.com/tjst-t/cirrus-sim/common/pkg/eventlog"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	el := eventlog.New()

	mux := http.NewServeMux()
	eventsHandler := handler.NewEventsHandler(el)
	eventsHandler.Register(mux)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {
		slog.Info("starting event log API server", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}

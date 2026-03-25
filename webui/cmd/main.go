// Package main provides the entry point for the cirrus-sim dashboard web UI.
package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//go:embed static
var staticFiles embed.FS

// simulatorEndpoints maps simulator names to their base URLs.
var simulatorEndpoints = map[string]string{
	"libvirt-sim": "http://localhost:8100",
	"storage-sim": "http://localhost:8500",
	"ovn-sim":     "http://localhost:8200",
	"awx-sim":     "http://localhost:8300",
	"netbox-sim":  "http://localhost:8400",
	"common":      "http://localhost:8000",
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Allow overriding endpoints via env vars
	for name := range simulatorEndpoints {
		envKey := fmt.Sprintf("%s_URL", envName(name))
		if v := os.Getenv(envKey); v != "" {
			simulatorEndpoints[name] = v
		}
	}

	mux := http.NewServeMux()

	// Serve static files
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := staticFiles.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, writeErr := w.Write(data); writeErr != nil {
			slog.Warn("failed to write response", "error", writeErr)
		}
	})

	// Proxy endpoints for each simulator
	for name, baseURL := range simulatorEndpoints {
		proxyPath := fmt.Sprintf("/proxy/%s/", name)
		mux.Handle(proxyPath, newProxy(proxyPath, baseURL))
	}

	// Aggregated status endpoint
	mux.HandleFunc("GET /api/status", handleAggregatedStatus)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("cirrus-sim dashboard starting", "port", port, "url", fmt.Sprintf("http://localhost:%s", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown failed", "error", err)
	}
}

func envName(s string) string {
	result := make([]byte, 0, len(s))
	for _, c := range s {
		if c == '-' {
			result = append(result, '_')
		} else if c >= 'a' && c <= 'z' {
			result = append(result, byte(c-'a'+'A'))
		} else {
			result = append(result, byte(c))
		}
	}
	return string(result)
}

type proxy struct {
	prefix  string
	baseURL string
	client  *http.Client
}

func newProxy(prefix, baseURL string) *proxy {
	return &proxy{
		prefix:  prefix,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	targetPath := r.URL.Path[len(p.prefix)-1:] // keep leading slash
	targetURL := p.baseURL + targetPath
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("proxy error: %v", err), http.StatusBadGateway)
		return
	}
	proxyReq.Header = r.Header.Clone()

	resp, err := p.client.Do(proxyReq)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, `{"error":"service unavailable","detail":"%s"}`, err.Error())
		return
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Warn("failed to close proxy response", "error", closeErr)
		}
	}()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	if _, copyErr := io.Copy(w, resp.Body); copyErr != nil {
		slog.Warn("failed to copy proxy response", "error", copyErr)
	}
}

func handleAggregatedStatus(w http.ResponseWriter, r *http.Request) {
	client := &http.Client{Timeout: 2 * time.Second}
	type simStatus struct {
		Name   string `json:"name"`
		URL    string `json:"url"`
		Status string `json:"status"`
	}

	results := make([]simStatus, 0, len(simulatorEndpoints))

	for name, baseURL := range simulatorEndpoints {
		s := simStatus{Name: name, URL: baseURL, Status: "down"}

		statsURL := baseURL + "/sim/stats"
		if name == "common" {
			statsURL = baseURL + "/api/v1/events?limit=1"
		}

		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, statsURL, nil)
		if err != nil {
			results = append(results, s)
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			results = append(results, s)
			continue
		}
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Warn("failed to close status response", "error", closeErr)
		}

		if resp.StatusCode == http.StatusOK {
			s.Status = "up"
		}
		results = append(results, s)
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"simulators":[`)
	for i, s := range results {
		if i > 0 {
			fmt.Fprint(w, ",")
		}
		fmt.Fprintf(w, `{"name":"%s","url":"%s","status":"%s"}`, s.Name, s.URL, s.Status)
	}
	fmt.Fprint(w, `]}`)
}

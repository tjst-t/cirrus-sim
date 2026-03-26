// Package main provides the unified cirrus-sim binary that starts all simulators in one process.
//
// Usage:
//
//	cirrus-sim                    # start all with default ports
//	cirrus-sim -common=8000 ...  # override individual ports
//
// Default ports:
//
//	common (event log, faults)  :8000
//	dashboard (web UI)          :8080
//	libvirt-sim (management)    :8100
//	ovn-sim (management)        :8200
//	awx-sim                     :8300
//	netbox-sim                  :8400
//	storage-sim                 :8500
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	awxsim "github.com/tjst-t/cirrus-sim/awx-sim"
	common "github.com/tjst-t/cirrus-sim/common"
	libvirtsim "github.com/tjst-t/cirrus-sim/libvirt-sim"
	netboxsim "github.com/tjst-t/cirrus-sim/netbox-sim"
	ovnsim "github.com/tjst-t/cirrus-sim/ovn-sim"
	storagesim "github.com/tjst-t/cirrus-sim/storage-sim"
	"github.com/tjst-t/cirrus-sim/webui"
)

// Shutdowner is implemented by all simulator servers.
type Shutdowner interface {
	Start()
	Shutdown(ctx context.Context) error
}

func main() {
	commonPort := flag.String("common", envOrDefault("COMMON_PORT", "8000"), "common API port")
	libvirtPort := flag.String("libvirt", envOrDefault("LIBVIRT_SIM_PORT", "8100"), "libvirt-sim management port")
	ovnPort := flag.String("ovn", envOrDefault("OVN_SIM_PORT", "8200"), "ovn-sim management port")
	awxPort := flag.String("awx", envOrDefault("AWX_SIM_PORT", "8300"), "awx-sim port")
	netboxPort := flag.String("netbox", envOrDefault("NETBOX_SIM_PORT", "8400"), "netbox-sim port")
	storagePort := flag.String("storage", envOrDefault("STORAGE_SIM_PORT", "8500"), "storage-sim port")
	dashboardPort := flag.String("dashboard", envOrDefault("DASHBOARD_PORT", "8080"), "dashboard web UI port")
	envFile := flag.String("env", envOrDefault("CIRRUS_SIM_ENV", ""), "environment YAML file to seed on startup")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// Build endpoint map for dashboard proxy
	endpoints := webui.Endpoints{
		"common":      fmt.Sprintf("http://localhost:%s", *commonPort),
		"libvirt-sim": fmt.Sprintf("http://localhost:%s", *libvirtPort),
		"ovn-sim":     fmt.Sprintf("http://localhost:%s", *ovnPort),
		"awx-sim":     fmt.Sprintf("http://localhost:%s", *awxPort),
		"netbox-sim":  fmt.Sprintf("http://localhost:%s", *netboxPort),
		"storage-sim": fmt.Sprintf("http://localhost:%s", *storagePort),
	}

	// Create simulator instances
	libvirtSim := libvirtsim.New(*libvirtPort, logger.With("sim", "libvirt-sim"))
	ovnSim := ovnsim.New(*ovnPort, logger.With("sim", "ovn-sim"))
	storageSim := storagesim.New(*storagePort, logger.With("sim", "storage-sim"))

	sims := []struct {
		name string
		srv  Shutdowner
	}{
		{"common", common.New(*commonPort, logger.With("sim", "common"))},
		{"libvirt-sim", libvirtSim},
		{"ovn-sim", ovnSim},
		{"awx-sim", awxsim.New(*awxPort, logger.With("sim", "awx-sim"))},
		{"netbox-sim", netboxsim.New(*netboxPort, logger.With("sim", "netbox-sim"))},
		{"storage-sim", storageSim},
		{"dashboard", webui.New(*dashboardPort, endpoints, logger.With("sim", "dashboard"))},
	}

	logger.Info("starting cirrus-sim (unified)",
		"common", *commonPort,
		"libvirt-sim", *libvirtPort,
		"ovn-sim", *ovnPort,
		"awx-sim", *awxPort,
		"netbox-sim", *netboxPort,
		"storage-sim", *storagePort,
		"dashboard", *dashboardPort,
	)

	for _, s := range sims {
		s.srv.Start()
	}

	// Seed environment if specified
	if *envFile != "" {
		ctx := context.Background()
		if err := seedFromEnvFile(ctx, *envFile, libvirtSim, ovnSim, storageSim, logger); err != nil {
			logger.Error("environment seeding failed", "file", *envFile, "error", err)
			os.Exit(1)
		}
	}

	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  cirrus-sim is running\n")
	fmt.Fprintf(os.Stderr, "  ─────────────────────────────────────────\n")
	fmt.Fprintf(os.Stderr, "  Dashboard                http://localhost:%s\n", *dashboardPort)
	fmt.Fprintf(os.Stderr, "  ─────────────────────────────────────────\n")
	fmt.Fprintf(os.Stderr, "  common (events/faults)   http://localhost:%s\n", *commonPort)
	fmt.Fprintf(os.Stderr, "  libvirt-sim (management) http://localhost:%s\n", *libvirtPort)
	fmt.Fprintf(os.Stderr, "  ovn-sim (management)     http://localhost:%s\n", *ovnPort)
	fmt.Fprintf(os.Stderr, "  awx-sim                  http://localhost:%s\n", *awxPort)
	fmt.Fprintf(os.Stderr, "  netbox-sim               http://localhost:%s\n", *netboxPort)
	fmt.Fprintf(os.Stderr, "  storage-sim              http://localhost:%s\n", *storagePort)
	fmt.Fprintf(os.Stderr, "  ─────────────────────────────────────────\n")
	if *envFile != "" {
		fmt.Fprintf(os.Stderr, "  Environment              %s\n", *envFile)
		fmt.Fprintf(os.Stderr, "  ─────────────────────────────────────────\n")
	}
	fmt.Fprintf(os.Stderr, "  Press Ctrl+C to stop\n\n")

	// Wait for signal
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	logger.Info("shutting down all simulators")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for i := len(sims) - 1; i >= 0; i-- {
		s := sims[i]
		if err := s.srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown failed", "sim", s.name, "error", err)
		} else {
			logger.Info("stopped", "sim", s.name)
		}
	}
	logger.Info("cirrus-sim stopped")
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

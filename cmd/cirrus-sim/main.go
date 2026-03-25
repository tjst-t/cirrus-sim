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
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	sims := []struct {
		name string
		srv  Shutdowner
	}{
		{"common", common.New(*commonPort, logger.With("sim", "common"))},
		{"libvirt-sim", libvirtsim.New(*libvirtPort, logger.With("sim", "libvirt-sim"))},
		{"ovn-sim", ovnsim.New(*ovnPort, logger.With("sim", "ovn-sim"))},
		{"awx-sim", awxsim.New(*awxPort, logger.With("sim", "awx-sim"))},
		{"netbox-sim", netboxsim.New(*netboxPort, logger.With("sim", "netbox-sim"))},
		{"storage-sim", storagesim.New(*storagePort, logger.With("sim", "storage-sim"))},
	}

	logger.Info("starting cirrus-sim (unified)",
		"common", *commonPort,
		"libvirt-sim", *libvirtPort,
		"ovn-sim", *ovnPort,
		"awx-sim", *awxPort,
		"netbox-sim", *netboxPort,
		"storage-sim", *storagePort,
	)

	for _, s := range sims {
		s.srv.Start()
	}

	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  cirrus-sim is running\n")
	fmt.Fprintf(os.Stderr, "  ─────────────────────────────────────────\n")
	fmt.Fprintf(os.Stderr, "  common (events/faults)   http://localhost:%s\n", *commonPort)
	fmt.Fprintf(os.Stderr, "  libvirt-sim (management) http://localhost:%s\n", *libvirtPort)
	fmt.Fprintf(os.Stderr, "  ovn-sim (management)     http://localhost:%s\n", *ovnPort)
	fmt.Fprintf(os.Stderr, "  awx-sim                  http://localhost:%s\n", *awxPort)
	fmt.Fprintf(os.Stderr, "  netbox-sim               http://localhost:%s\n", *netboxPort)
	fmt.Fprintf(os.Stderr, "  storage-sim              http://localhost:%s\n", *storagePort)
	fmt.Fprintf(os.Stderr, "  ─────────────────────────────────────────\n")
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

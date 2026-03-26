package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"

	"github.com/tjst-t/cirrus-sim/common/pkg/datagen"
	libvirtsim "github.com/tjst-t/cirrus-sim/libvirt-sim"
	netboxsim "github.com/tjst-t/cirrus-sim/netbox-sim"
	ovnsim "github.com/tjst-t/cirrus-sim/ovn-sim"
	storagesim "github.com/tjst-t/cirrus-sim/storage-sim"
)

// portRange holds a start-end port range from portman.
type portRange struct {
	start int
	end   int
}

// getPortRange reads PORT_START/PORT_END env vars for a given name.
// Returns zero range if not set (fallback to defaults).
func getPortRange(envPrefix string) portRange {
	startStr := os.Getenv(envPrefix + "_PORT_START")
	endStr := os.Getenv(envPrefix + "_PORT_END")
	if startStr == "" || endStr == "" {
		return portRange{}
	}
	start, err1 := strconv.Atoi(startStr)
	end, err2 := strconv.Atoi(endStr)
	if err1 != nil || err2 != nil {
		return portRange{}
	}
	return portRange{start: start, end: end}
}

// seedFromEnvFile loads an environment YAML file and seeds all simulators with the generated data.
func seedFromEnvFile(
	ctx context.Context,
	path string,
	libvirt *libvirtsim.Server,
	ovn *ovnsim.Server,
	storage *storagesim.Server,
	netbox *netboxsim.Server,
	logger *slog.Logger,
) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read environment file: %w", err)
	}

	// Parse YAML for OVN cluster and topology definitions
	var envDef datagen.EnvironmentDef
	if err := yaml.Unmarshal(data, &envDef); err != nil {
		return fmt.Errorf("parse environment YAML: %w", err)
	}

	// Check for portman-managed port ranges
	libvirtRange := getPortRange("LIBVIRT_HOSTS")
	ovnRange := getPortRange("OVN_CLUSTERS")

	// Generate hosts and backends
	gen := datagen.New()
	opts := datagen.GenerateOptions{
		LibvirtBasePort: libvirtRange.start,
	}
	result, err := gen.GenerateWithOptions(ctx, data, opts)
	if err != nil {
		return fmt.Errorf("generate environment: %w", err)
	}

	logger.Info("seeding environment", "name", result.Name, "hosts", len(result.Hosts), "backends", len(result.Backends))

	if libvirtRange.start > 0 {
		logger.Info("using portman range for libvirt hosts", "start", libvirtRange.start, "end", libvirtRange.end)
	}

	// Seed libvirt-sim hosts
	for _, h := range result.Hosts {
		cfg := libvirtsim.HostConfig{
			HostID:             h.HostID,
			LibvirtPort:        h.LibvirtPort,
			CPUModel:           h.CPUModel,
			CPUSockets:         h.CPUSockets,
			CoresPerSocket:     h.CoresPerSocket,
			ThreadsPerCore:     h.ThreadsPerCore,
			MemoryMB:           int64(h.MemoryMB),
			CPUOvercommitRatio: 4.0,
			MemOvercommitRatio: 1.5,
		}
		if err := libvirt.SeedHost(ctx, cfg); err != nil {
			return fmt.Errorf("seed host %s: %w", h.HostID, err)
		}
	}
	logger.Info("seeded libvirt-sim", "hosts", len(result.Hosts))

	// Seed OVN clusters
	ovnBasePort := 6641
	if ovnRange.start > 0 {
		ovnBasePort = ovnRange.start
		logger.Info("using portman range for OVN clusters", "start", ovnRange.start, "end", ovnRange.end)
	}
	for i, cluster := range envDef.Environment.OVNClusters {
		port := ovnBasePort + i
		if err := ovn.SeedCluster(cluster.Name, port); err != nil {
			return fmt.Errorf("seed OVN cluster %s: %w", cluster.Name, err)
		}
	}
	if len(envDef.Environment.OVNClusters) > 0 {
		logger.Info("seeded ovn-sim", "clusters", len(envDef.Environment.OVNClusters))
	}

	// Seed storage backends
	for _, b := range result.Backends {
		cfg := storagesim.BackendConfig{
			BackendID:          b.BackendID,
			TotalCapacityGB:    b.TotalCapacityGB,
			TotalIOPS:          b.TotalIOPS,
			Capabilities:       b.Capabilities,
			OverprovisionRatio: 1.5,
		}
		if err := storage.SeedBackend(cfg); err != nil {
			return fmt.Errorf("seed backend %s: %w", b.BackendID, err)
		}
	}
	if len(result.Backends) > 0 {
		logger.Info("seeded storage-sim", "backends", len(result.Backends))
	}

	// Seed netbox-sim with physical topology
	seedNetboxTopology(netbox, envDef.Environment, result)
	logger.Info("seeded netbox-sim",
		"sites", len(envDef.Environment.Sites),
		"devices", len(result.Hosts),
	)

	logger.Info("environment seeding complete", "name", result.Name)
	return nil
}

// seedNetboxTopology populates netbox-sim with the physical topology derived from the environment.
func seedNetboxTopology(netbox *netboxsim.Server, env datagen.Environment, result *datagen.GenerateResult) {
	// Build a host lookup by ID for mapping generated hosts to netbox devices
	hostIdx := 0

	for _, site := range env.Sites {
		siteID := netbox.SeedSite(site.Name)

		for row := 1; row <= site.RackRows; row++ {
			rowName := fmt.Sprintf("row-%s-%d", site.Name, row)
			rowID := netbox.SeedLocation(rowName, siteID, 0, nil)

			for rack := 1; rack <= site.RacksPerRow; rack++ {
				rackName := fmt.Sprintf("rack-%s-%d-%d", site.Name, row, rack)
				torSwitch := fmt.Sprintf("tor-%s-%d-%d", site.Name, row, rack)
				powerCircuit := fmt.Sprintf("pdu-%s-%d-%d", site.Name, row, rack)

				rackID := netbox.SeedRack(rackName, siteID, rowID, map[string]string{
					"tor_switch":    torSwitch,
					"power_circuit": powerCircuit,
				})

				for h := 1; h <= site.HostsPerRack; h++ {
					if hostIdx >= len(result.Hosts) {
						break
					}
					host := result.Hosts[hostIdx]
					hostIdx++

					netbox.SeedDevice(host.HostID, "server", siteID, rowID, rackID, h, map[string]string{
						"cirrus_host_id": host.HostID,
					})
				}
			}
		}
	}
}

//go:build integration

// Package integration provides end-to-end tests against running simulators.
//
// Prerequisites:
//  1. Start libvirt-sim: cd libvirt-sim && go run ./cmd/
//  2. Register a host:
//     curl -X POST http://localhost:8100/sim/hosts \
//     -H 'Content-Type: application/json' \
//     -d '{"host_id":"host-001","libvirt_port":16510,"cpu_model":"Intel Xeon Gold 6348",
//     "cpu_sockets":2,"cores_per_socket":28,"threads_per_core":2,"memory_mb":524288,
//     "cpu_overcommit_ratio":4.0,"memory_overcommit_ratio":1.5}'
//  3. Run: go test -tags=integration -v ./...
package integration

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	libvirt "github.com/digitalocean/go-libvirt"
)

const (
	libvirtSimAddr = "127.0.0.1:16510"
	testHostname   = "host-001"
)

func connect(t *testing.T) *libvirt.Libvirt {
	t.Helper()
	conn, err := net.DialTimeout("tcp", libvirtSimAddr, 5*time.Second)
	if err != nil {
		t.Fatalf("TCP connect to %s failed: %v", libvirtSimAddr, err)
	}
	l := libvirt.New(conn)
	if err := l.Connect(); err != nil {
		t.Fatalf("libvirt Connect failed: %v", err)
	}
	t.Cleanup(func() { _ = l.Disconnect() })
	return l
}

func defineDomain(t *testing.T, l *libvirt.Libvirt, name, uuid string, memKiB int, vcpus int) libvirt.Domain {
	t.Helper()
	xml := fmt.Sprintf(`<domain type="kvm">
  <name>%s</name>
  <uuid>%s</uuid>
  <memory unit="KiB">%d</memory>
  <vcpu>%d</vcpu>
  <os><type>hvm</type></os>
</domain>`, name, uuid, memKiB, vcpus)

	dom, err := l.DomainDefineXMLFlags(xml, 0)
	if err != nil {
		t.Fatalf("DomainDefineXMLFlags failed: %v", err)
	}
	t.Cleanup(func() {
		_ = l.DomainDestroyFlags(dom, 0)
		_ = l.DomainUndefineFlags(dom, 0)
	})
	return dom
}

// ── Connection Management ──

func TestConnectGetHostname(t *testing.T) {
	l := connect(t)
	hostname, err := l.ConnectGetHostname()
	if err != nil {
		t.Fatalf("ConnectGetHostname: %v", err)
	}
	if hostname != testHostname {
		t.Errorf("hostname = %q, want %q", hostname, testHostname)
	}
}

func TestConnectGetVersion(t *testing.T) {
	l := connect(t)
	ver, err := l.ConnectGetVersion()
	if err != nil {
		t.Fatalf("ConnectGetVersion: %v", err)
	}
	if ver == 0 {
		t.Error("version should be non-zero")
	}
}

func TestConnectGetCapabilities(t *testing.T) {
	l := connect(t)
	caps, err := l.ConnectGetCapabilities()
	if err != nil {
		t.Fatalf("ConnectGetCapabilities: %v", err)
	}
	if len(caps) == 0 {
		t.Error("capabilities should be non-empty")
	}
}

// ── Host Info ──

func TestNodeGetInfo(t *testing.T) {
	l := connect(t)
	rModel, rMemory, rCpus, _, _, rSockets, rCores, rThreads, err := l.NodeGetInfo()
	if err != nil {
		t.Fatalf("NodeGetInfo: %v", err)
	}

	model := int8ToString(rModel[:])
	tests := []struct {
		name string
		got  any
		want any
	}{
		{"model contains Xeon", strings.Contains(model, "Xeon"), true},
		{"memory (MB)", rMemory / 1024, uint64(524288)},
		{"cpus", rCpus, int32(112)},
		{"sockets", rSockets, int32(2)},
		{"cores", rCores, int32(28)},
		{"threads", rThreads, int32(2)},
	}
	for _, tt := range tests {
		if fmt.Sprint(tt.got) != fmt.Sprint(tt.want) {
			t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
}

func TestNodeGetFreeMemory(t *testing.T) {
	l := connect(t)
	freeMem, err := l.NodeGetFreeMemory()
	if err != nil {
		t.Fatalf("NodeGetFreeMemory: %v", err)
	}
	if freeMem == 0 {
		t.Error("free memory should be non-zero")
	}
}

func TestNodeGetCPUStats(t *testing.T) {
	l := connect(t)
	stats, nparams, err := l.NodeGetCPUStats(-1, 0, 0)
	if err != nil {
		t.Fatalf("NodeGetCPUStats: %v", err)
	}
	if nparams != 4 {
		t.Errorf("nparams = %d, want 4", nparams)
	}
	if len(stats) != 4 {
		t.Errorf("stats count = %d, want 4", len(stats))
	}
}

func TestNodeGetMemoryStats(t *testing.T) {
	l := connect(t)
	stats, nparams, err := l.NodeGetMemoryStats(0, -1, 0)
	if err != nil {
		t.Fatalf("NodeGetMemoryStats: %v", err)
	}
	if nparams != 4 {
		t.Errorf("nparams = %d, want 4", nparams)
	}
	if len(stats) != 4 {
		t.Errorf("stats count = %d, want 4", len(stats))
	}
}

// ── Domain Lifecycle ──

func TestDomainFullLifecycle(t *testing.T) {
	l := connect(t)
	dom := defineDomain(t, l, "lifecycle-vm", "11111111-1111-1111-1111-111111111111", 4194304, 4)

	// shutoff
	state, _, err := l.DomainGetState(dom, 0)
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	if state != 5 {
		t.Fatalf("initial state = %d, want 5 (SHUTOFF)", state)
	}

	// start
	if _, err := l.DomainCreateWithFlags(dom, 0); err != nil {
		t.Fatalf("CreateWithFlags: %v", err)
	}
	state, _, _ = l.DomainGetState(dom, 0)
	if state != 1 {
		t.Errorf("after start state = %d, want 1 (RUNNING)", state)
	}

	// suspend
	if err := l.DomainSuspend(dom); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	state, _, _ = l.DomainGetState(dom, 0)
	if state != 3 {
		t.Errorf("after suspend state = %d, want 3 (PAUSED)", state)
	}

	// resume
	if err := l.DomainResume(dom); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	state, _, _ = l.DomainGetState(dom, 0)
	if state != 1 {
		t.Errorf("after resume state = %d, want 1 (RUNNING)", state)
	}

	// reboot
	if err := l.DomainReboot(dom, 0); err != nil {
		t.Fatalf("Reboot: %v", err)
	}
	state, _, _ = l.DomainGetState(dom, 0)
	if state != 1 {
		t.Errorf("after reboot state = %d, want 1 (RUNNING)", state)
	}

	// destroy
	if err := l.DomainDestroyFlags(dom, 0); err != nil {
		t.Fatalf("DestroyFlags: %v", err)
	}
	state, _, _ = l.DomainGetState(dom, 0)
	if state != 5 {
		t.Errorf("after destroy state = %d, want 5 (SHUTOFF)", state)
	}

	// start + shutdown
	if _, err := l.DomainCreateWithFlags(dom, 0); err != nil {
		t.Fatalf("CreateWithFlags (2nd): %v", err)
	}
	if err := l.DomainShutdownFlags(dom, 0); err != nil {
		t.Fatalf("ShutdownFlags: %v", err)
	}
	state, _, _ = l.DomainGetState(dom, 0)
	if state != 5 {
		t.Errorf("after shutdown state = %d, want 5 (SHUTOFF)", state)
	}
}

func TestDomainGetInfo(t *testing.T) {
	l := connect(t)
	dom := defineDomain(t, l, "info-vm", "22222222-2222-2222-2222-222222222222", 4194304, 4)
	if _, err := l.DomainCreateWithFlags(dom, 0); err != nil {
		t.Fatalf("start: %v", err)
	}

	rState, rMaxMem, _, rNrVCPU, _, err := l.DomainGetInfo(dom)
	if err != nil {
		t.Fatalf("DomainGetInfo: %v", err)
	}
	if rState != 1 {
		t.Errorf("state = %d, want 1", rState)
	}
	if rMaxMem != 4194304 {
		t.Errorf("maxMem = %d, want 4194304", rMaxMem)
	}
	if rNrVCPU != 4 {
		t.Errorf("nrVirtCpu = %d, want 4", rNrVCPU)
	}
}

func TestDomainGetXMLDesc(t *testing.T) {
	l := connect(t)
	dom := defineDomain(t, l, "xml-vm", "33333333-3333-3333-3333-333333333333", 2097152, 2)

	xml, err := l.DomainGetXMLDesc(dom, 0)
	if err != nil {
		t.Fatalf("GetXMLDesc: %v", err)
	}
	if !strings.Contains(xml, "xml-vm") {
		t.Errorf("XML should contain domain name, got %d bytes", len(xml))
	}
}

func TestConnectListAllDomains(t *testing.T) {
	l := connect(t)
	defineDomain(t, l, "list-vm", "44444444-4444-4444-4444-444444444444", 1048576, 1)

	domains, _, err := l.ConnectListAllDomains(1, 0)
	if err != nil {
		t.Fatalf("ListAllDomains: %v", err)
	}
	if len(domains) < 1 {
		t.Errorf("expected at least 1 domain, got %d", len(domains))
	}
}

func TestDomainLookupByName(t *testing.T) {
	l := connect(t)
	defineDomain(t, l, "lookup-name-vm", "55555555-5555-5555-5555-555555555555", 1048576, 1)

	found, err := l.DomainLookupByName("lookup-name-vm")
	if err != nil {
		t.Fatalf("LookupByName: %v", err)
	}
	if found.Name != "lookup-name-vm" {
		t.Errorf("name = %q, want %q", found.Name, "lookup-name-vm")
	}
}

func TestDomainLookupByUUID(t *testing.T) {
	l := connect(t)
	defineDomain(t, l, "lookup-uuid-vm", "66666666-6666-6666-6666-666666666666", 1048576, 1)

	uuid := libvirt.UUID{0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x66}
	found, err := l.DomainLookupByUUID(uuid)
	if err != nil {
		t.Fatalf("LookupByUUID: %v", err)
	}
	if found.Name != "lookup-uuid-vm" {
		t.Errorf("name = %q, want %q", found.Name, "lookup-uuid-vm")
	}
}

func TestConnectGetAllDomainStats(t *testing.T) {
	l := connect(t)
	dom := defineDomain(t, l, "stats-vm", "77777777-7777-7777-7777-777777777777", 1048576, 1)
	if _, err := l.DomainCreateWithFlags(dom, 0); err != nil {
		t.Fatalf("start: %v", err)
	}

	allStats, err := l.ConnectGetAllDomainStats(nil, 0, 0)
	if err != nil {
		t.Fatalf("GetAllDomainStats: %v", err)
	}
	if len(allStats) < 1 {
		t.Errorf("expected at least 1 stat record, got %d", len(allStats))
	}
}

// ── Migration Speed ──

func TestDomainMigrateSpeed(t *testing.T) {
	l := connect(t)
	dom := defineDomain(t, l, "speed-vm", "88888888-8888-8888-8888-888888888888", 1048576, 1)
	if _, err := l.DomainCreateWithFlags(dom, 0); err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := l.DomainMigrateSetMaxSpeed(dom, 5000, 0); err != nil {
		t.Fatalf("SetMaxSpeed: %v", err)
	}

	speed, err := l.DomainMigrateGetMaxSpeed(dom, 0)
	if err != nil {
		t.Fatalf("GetMaxSpeed: %v", err)
	}
	if speed != 5000 {
		t.Errorf("speed = %d, want 5000", speed)
	}
}

// ── Event Registration ──

func TestDomainEventRegisterDeregister(t *testing.T) {
	l := connect(t)

	if err := l.ConnectDomainEventRegisterAny(0); err != nil {
		t.Fatalf("RegisterAny: %v", err)
	}

	if err := l.ConnectDomainEventDeregisterAny(0); err != nil {
		t.Fatalf("DeregisterAny: %v", err)
	}
}

// ── Error Cases ──

func TestDomainInvalidStateTransitions(t *testing.T) {
	l := connect(t)
	dom := defineDomain(t, l, "error-vm", "99999999-9999-9999-9999-999999999999", 1048576, 1)

	// Suspend a shutoff domain should fail
	if err := l.DomainSuspend(dom); err == nil {
		t.Error("Suspend of shutoff domain should return error")
	}

	// Resume a shutoff domain should fail
	if err := l.DomainResume(dom); err == nil {
		t.Error("Resume of shutoff domain should return error")
	}
}

func TestDomainLookupNotFound(t *testing.T) {
	l := connect(t)

	if _, err := l.DomainLookupByName("nonexistent-vm"); err == nil {
		t.Error("LookupByName for nonexistent domain should return error")
	}

	bogusUUID := libvirt.UUID{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	if _, err := l.DomainLookupByUUID(bogusUUID); err == nil {
		t.Error("LookupByUUID for nonexistent domain should return error")
	}
}

// ── Helpers ──

func int8ToString(arr []int8) string {
	b := make([]byte, 0, len(arr))
	for _, v := range arr {
		if v == 0 {
			break
		}
		b = append(b, byte(v))
	}
	return string(b)
}

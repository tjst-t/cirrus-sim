package state

import (
	"errors"
	"testing"
)

func TestAddHost(t *testing.T) {
	tests := []struct {
		name    string
		hosts   []*Host
		wantErr error
	}{
		{
			name: "add single host",
			hosts: []*Host{
				{HostID: "h1", LibvirtPort: 16509, CPUSockets: 2, CoresPerSocket: 4, ThreadsPerCore: 2, MemoryMB: 32768},
			},
			wantErr: nil,
		},
		{
			name: "duplicate host ID",
			hosts: []*Host{
				{HostID: "h1", LibvirtPort: 16509},
				{HostID: "h1", LibvirtPort: 16510},
			},
			wantErr: ErrHostExists,
		},
		{
			name: "duplicate port",
			hosts: []*Host{
				{HostID: "h1", LibvirtPort: 16509},
				{HostID: "h2", LibvirtPort: 16509},
			},
			wantErr: ErrPortInUse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStore()
			var lastErr error
			for _, h := range tt.hosts {
				lastErr = s.AddHost(h)
			}
			if tt.wantErr != nil {
				if !errors.Is(lastErr, tt.wantErr) {
					t.Errorf("got error %v, want %v", lastErr, tt.wantErr)
				}
			} else if lastErr != nil {
				t.Errorf("unexpected error: %v", lastErr)
			}
		})
	}
}

func makeTestHost(id string, port int) *Host {
	return &Host{
		HostID:             id,
		LibvirtPort:        port,
		CPUModel:           "Test CPU",
		CPUSockets:         2,
		CoresPerSocket:     4,
		ThreadsPerCore:     2,
		MemoryMB:           32768,
		CPUOvercommitRatio: 4.0,
		MemOvercommitRatio: 1.5,
	}
}

func makeTestDomain(name string, uuid [16]byte, vcpus int, memKiB int64) *Domain {
	return &Domain{
		Name:      name,
		UUID:      uuid,
		VCPUs:     vcpus,
		MemoryKiB: memKiB,
		XML:       "<domain><name>" + name + "</name></domain>",
	}
}

func TestDomainStateTransitions(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(s *Store, hostID, uuid string)
		action    func(s *Store, hostID, uuid string) error
		wantState DomainState
		wantErr   error
	}{
		{
			name:  "start from shutoff",
			setup: func(s *Store, hostID, uuid string) {},
			action: func(s *Store, hostID, uuid string) error {
				return s.StartDomain(hostID, uuid)
			},
			wantState: DomainStateRunning,
		},
		{
			name: "destroy from running",
			setup: func(s *Store, hostID, uuid string) {
				_ = s.StartDomain(hostID, uuid)
			},
			action: func(s *Store, hostID, uuid string) error {
				return s.DestroyDomain(hostID, uuid)
			},
			wantState: DomainStateShutoff,
		},
		{
			name: "suspend from running",
			setup: func(s *Store, hostID, uuid string) {
				_ = s.StartDomain(hostID, uuid)
			},
			action: func(s *Store, hostID, uuid string) error {
				return s.SuspendDomain(hostID, uuid)
			},
			wantState: DomainStatePaused,
		},
		{
			name: "resume from paused",
			setup: func(s *Store, hostID, uuid string) {
				_ = s.StartDomain(hostID, uuid)
				_ = s.SuspendDomain(hostID, uuid)
			},
			action: func(s *Store, hostID, uuid string) error {
				return s.ResumeDomain(hostID, uuid)
			},
			wantState: DomainStateRunning,
		},
		{
			name: "shutdown from running",
			setup: func(s *Store, hostID, uuid string) {
				_ = s.StartDomain(hostID, uuid)
			},
			action: func(s *Store, hostID, uuid string) error {
				return s.ShutdownDomain(hostID, uuid)
			},
			wantState: DomainStateShutoff,
		},
		{
			name:  "suspend from shutoff fails",
			setup: func(s *Store, hostID, uuid string) {},
			action: func(s *Store, hostID, uuid string) error {
				return s.SuspendDomain(hostID, uuid)
			},
			wantErr: ErrOperationInvalid,
		},
		{
			name:  "resume from shutoff fails",
			setup: func(s *Store, hostID, uuid string) {},
			action: func(s *Store, hostID, uuid string) error {
				return s.ResumeDomain(hostID, uuid)
			},
			wantErr: ErrOperationInvalid,
		},
		{
			name: "start from running fails",
			setup: func(s *Store, hostID, uuid string) {
				_ = s.StartDomain(hostID, uuid)
			},
			action: func(s *Store, hostID, uuid string) error {
				return s.StartDomain(hostID, uuid)
			},
			wantErr: ErrOperationInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStore()
			h := makeTestHost("h1", 16509)
			if err := s.AddHost(h); err != nil {
				t.Fatal(err)
			}

			uuid := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
			d := makeTestDomain("vm1", uuid, 2, 4194304) // 4 GiB
			if err := s.DefineDomain("h1", d); err != nil {
				t.Fatal(err)
			}

			uuidStr := d.UUIDString()
			tt.setup(s, "h1", uuidStr)

			err := tt.action(s, "h1", uuidStr)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("got error %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			dom, err := s.GetDomain("h1", uuidStr)
			if err != nil {
				t.Fatal(err)
			}
			if dom.State != tt.wantState {
				t.Errorf("got state %d, want %d", dom.State, tt.wantState)
			}
		})
	}
}

func TestResourceTracking(t *testing.T) {
	s := NewStore()
	h := &Host{
		HostID:             "h1",
		LibvirtPort:        16509,
		CPUSockets:         1,
		CoresPerSocket:     2,
		ThreadsPerCore:     1,
		MemoryMB:           8192,
		CPUOvercommitRatio: 2.0,   // 4 available vCPUs
		MemOvercommitRatio: 1.0,   // 8192 MB available
	}
	if err := s.AddHost(h); err != nil {
		t.Fatal(err)
	}

	// Define a domain using all available resources
	uuid1 := [16]byte{1}
	d1 := makeTestDomain("vm1", uuid1, 4, 8388608) // 4 vCPUs, 8192 MiB
	if err := s.DefineDomain("h1", d1); err != nil {
		t.Fatal(err)
	}
	if err := s.StartDomain("h1", d1.UUIDString()); err != nil {
		t.Fatal(err)
	}

	// Try to start another domain - should fail
	uuid2 := [16]byte{2}
	d2 := makeTestDomain("vm2", uuid2, 1, 1048576) // 1 vCPU, 1024 MiB
	if err := s.DefineDomain("h1", d2); err != nil {
		t.Fatal(err)
	}
	err := s.StartDomain("h1", d2.UUIDString())
	if !errors.Is(err, ErrOperationDenied) {
		t.Errorf("expected ErrOperationDenied, got %v", err)
	}

	// Destroy first domain, then second should start
	if err := s.DestroyDomain("h1", d1.UUIDString()); err != nil {
		t.Fatal(err)
	}
	if err := s.StartDomain("h1", d2.UUIDString()); err != nil {
		t.Errorf("expected start to succeed after freeing resources: %v", err)
	}
}

func TestUndefine(t *testing.T) {
	s := NewStore()
	h := makeTestHost("h1", 16509)
	if err := s.AddHost(h); err != nil {
		t.Fatal(err)
	}

	uuid := [16]byte{1}
	d := makeTestDomain("vm1", uuid, 2, 4194304)
	if err := s.DefineDomain("h1", d); err != nil {
		t.Fatal(err)
	}

	// Start, then try to undefine - should fail
	if err := s.StartDomain("h1", d.UUIDString()); err != nil {
		t.Fatal(err)
	}
	err := s.UndefineDomain("h1", d.UUIDString())
	if !errors.Is(err, ErrOperationInvalid) {
		t.Errorf("expected ErrOperationInvalid, got %v", err)
	}

	// Destroy, then undefine should work
	if err := s.DestroyDomain("h1", d.UUIDString()); err != nil {
		t.Fatal(err)
	}
	if err := s.UndefineDomain("h1", d.UUIDString()); err != nil {
		t.Errorf("expected undefine to succeed: %v", err)
	}

	// Verify domain is gone
	_, err = s.GetDomain("h1", d.UUIDString())
	if !errors.Is(err, ErrNoDomain) {
		t.Errorf("expected ErrNoDomain, got %v", err)
	}
}

func TestGetStats(t *testing.T) {
	s := NewStore()
	h := makeTestHost("h1", 16509)
	if err := s.AddHost(h); err != nil {
		t.Fatal(err)
	}

	uuid := [16]byte{1}
	d := makeTestDomain("vm1", uuid, 2, 2097152) // 2 vCPUs, 2048 MiB
	if err := s.DefineDomain("h1", d); err != nil {
		t.Fatal(err)
	}
	if err := s.StartDomain("h1", d.UUIDString()); err != nil {
		t.Fatal(err)
	}

	stats := s.GetStats()
	if stats.TotalHosts != 1 {
		t.Errorf("TotalHosts = %d, want 1", stats.TotalHosts)
	}
	if stats.OnlineHosts != 1 {
		t.Errorf("OnlineHosts = %d, want 1", stats.OnlineHosts)
	}
	if stats.TotalDomains != 1 {
		t.Errorf("TotalDomains = %d, want 1", stats.TotalDomains)
	}
	if stats.RunningDomains != 1 {
		t.Errorf("RunningDomains = %d, want 1", stats.RunningDomains)
	}
	if stats.TotalVCPUsUsed != 2 {
		t.Errorf("TotalVCPUsUsed = %d, want 2", stats.TotalVCPUsUsed)
	}
	if stats.TotalMemoryUsed != 2048 {
		t.Errorf("TotalMemoryUsed = %d, want 2048", stats.TotalMemoryUsed)
	}
}

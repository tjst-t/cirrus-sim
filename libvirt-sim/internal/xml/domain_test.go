package xml

import (
	"testing"
)

func TestParseDomainXML(t *testing.T) {
	tests := []struct {
		name      string
		xml       string
		wantName  string
		wantUUID  string
		wantVCPU  int
		wantMemKiB int64
		wantErr   bool
	}{
		{
			name: "basic domain",
			xml: `<domain type="kvm">
  <name>vm-001</name>
  <uuid>12345678-1234-1234-1234-123456789abc</uuid>
  <memory unit="KiB">8388608</memory>
  <vcpu>4</vcpu>
  <devices>
    <disk type="network">
      <source protocol="rbd" name="cirrus-volumes/vol-001"/>
    </disk>
    <interface type="bridge">
      <target dev="ovn-abc123"/>
    </interface>
  </devices>
</domain>`,
			wantName:   "vm-001",
			wantUUID:   "12345678-1234-1234-1234-123456789abc",
			wantVCPU:   4,
			wantMemKiB: 8388608,
		},
		{
			name: "memory in GiB",
			xml: `<domain type="kvm">
  <name>vm-002</name>
  <uuid>aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee</uuid>
  <memory unit="GiB">8</memory>
  <vcpu>2</vcpu>
</domain>`,
			wantName:   "vm-002",
			wantUUID:   "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			wantVCPU:   2,
			wantMemKiB: 8 * 1024 * 1024,
		},
		{
			name:    "invalid XML",
			xml:     "<not-valid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dom, err := ParseDomainXML(tt.xml)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if dom.Name != tt.wantName {
				t.Errorf("name = %q, want %q", dom.Name, tt.wantName)
			}
			if dom.UUID != tt.wantUUID {
				t.Errorf("uuid = %q, want %q", dom.UUID, tt.wantUUID)
			}
			if dom.VCPU != tt.wantVCPU {
				t.Errorf("vcpu = %d, want %d", dom.VCPU, tt.wantVCPU)
			}
			if dom.MemoryKiB() != tt.wantMemKiB {
				t.Errorf("memory = %d KiB, want %d", dom.MemoryKiB(), tt.wantMemKiB)
			}
		})
	}
}

func TestParseUUID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    [16]byte
		wantErr bool
	}{
		{
			name:  "valid UUID with dashes",
			input: "12345678-1234-1234-1234-123456789abc",
			want:  [16]byte{0x12, 0x34, 0x56, 0x78, 0x12, 0x34, 0x12, 0x34, 0x12, 0x34, 0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc},
		},
		{
			name:    "invalid hex",
			input:   "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz",
			wantErr: true,
		},
		{
			name:    "too short",
			input:   "1234",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseUUID(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDomainDevices(t *testing.T) {
	xmlStr := `<domain type="kvm">
  <name>vm-dev</name>
  <uuid>11111111-1111-1111-1111-111111111111</uuid>
  <memory unit="KiB">4194304</memory>
  <vcpu>2</vcpu>
  <devices>
    <disk type="network">
      <source protocol="rbd" name="cirrus-volumes/vol-001"/>
    </disk>
    <disk type="network">
      <source protocol="rbd" name="cirrus-volumes/vol-002"/>
    </disk>
    <interface type="bridge">
      <target dev="ovn-abc123"/>
    </interface>
    <hostdev mode="subsystem" type="pci">
      <source><address domain="0x0000" bus="0x3b" slot="0x00" function="0x0"/></source>
    </hostdev>
  </devices>
</domain>`

	dom, err := ParseDomainXML(xmlStr)
	if err != nil {
		t.Fatal(err)
	}
	if len(dom.Devices.Disks) != 2 {
		t.Errorf("disks = %d, want 2", len(dom.Devices.Disks))
	}
	if len(dom.Devices.Interfaces) != 1 {
		t.Errorf("interfaces = %d, want 1", len(dom.Devices.Interfaces))
	}
	if len(dom.Devices.HostDevs) != 1 {
		t.Errorf("hostdevs = %d, want 1", len(dom.Devices.HostDevs))
	}
	if dom.Devices.Disks[0].Source.Name != "cirrus-volumes/vol-001" {
		t.Errorf("disk source = %q", dom.Devices.Disks[0].Source.Name)
	}
}

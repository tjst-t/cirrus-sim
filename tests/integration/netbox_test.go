//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	netbox "github.com/netbox-community/go-netbox/v4"
)

func netboxBaseURL(t *testing.T) string {
	t.Helper()
	port := os.Getenv("NETBOX_SIM_PORT")
	if port == "" {
		port = "8400"
	}
	return fmt.Sprintf("http://localhost:%s", port)
}

func seedNetboxData(t *testing.T, baseURL string) {
	t.Helper()

	payload := `{
		"sites": [{
			"name": "site-tokyo",
			"locations": [{
				"name": "floor-1",
				"power_feed": "pf-main",
				"locations": [{
					"name": "row-A",
					"racks": [{
						"name": "rack-A01",
						"tor_switch": "tor-a01",
						"power_circuit": "pdu-a01",
						"devices": [
							{"name": "host-001", "position": 40, "cirrus_host_id": "host-001"},
							{"name": "host-002", "position": 38, "cirrus_host_id": "host-002"}
						]
					},{
						"name": "rack-A02",
						"tor_switch": "tor-a02",
						"power_circuit": "pdu-a02",
						"devices": [
							{"name": "host-003", "position": 40, "cirrus_host_id": "host-003"}
						]
					}]
				}]
			}]
		},{
			"name": "site-osaka",
			"locations": [{
				"name": "row-B",
				"racks": [{
					"name": "rack-B01",
					"tor_switch": "tor-b01",
					"power_circuit": "pdu-b01",
					"devices": [
						{"name": "host-004", "position": 40, "cirrus_host_id": "host-004"}
					]
				}]
			}]
		}]
	}`

	// Reset first
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/sim/reset", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("reset failed: %v", err)
	}
	resp.Body.Close()

	// Bulk load
	req, _ = http.NewRequest(http.MethodPost, baseURL+"/sim/bulk-load", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("bulk-load failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("bulk-load status = %d, body: %s", resp.StatusCode, body)
	}

	var stats map[string]int
	json.NewDecoder(resp.Body).Decode(&stats)
	t.Logf("Seeded: %+v", stats)
}

func newNetboxClient(t *testing.T) *netbox.APIClient {
	t.Helper()
	cfg := netbox.NewConfiguration()
	cfg.Servers = netbox.ServerConfigurations{
		{URL: netboxBaseURL(t)},
	}
	// Add a dummy token header (netbox-sim doesn't check auth)
	cfg.DefaultHeader["Authorization"] = "Token dummy"
	return netbox.NewAPIClient(cfg)
}

func TestNetboxSitesListWithGoNetbox(t *testing.T) {
	baseURL := netboxBaseURL(t)
	seedNetboxData(t, baseURL)

	client := newNetboxClient(t)
	ctx := context.Background()

	result, resp, err := client.DcimAPI.DcimSitesList(ctx).Execute()
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			t.Logf("Response body: %s", body)
		}
		t.Fatalf("DcimSitesList failed: %v", err)
	}

	if result.Count != 2 {
		t.Errorf("site count = %d, want 2", result.Count)
	}

	for _, site := range result.Results {
		t.Logf("Site: id=%d name=%q slug=%q", site.Id, site.Name, site.Slug)

		if site.Url == "" {
			t.Error("site url is empty")
		}
		if site.Display == "" {
			t.Error("site display is empty")
		}
		if site.Slug == "" {
			t.Error("site slug is empty")
		}
		if site.Status != nil {
			if site.Status.Value == nil || *site.Status.Value == "" {
				t.Error("site status.value is empty")
			}
			if site.Status.Label == nil || *site.Status.Label == "" {
				t.Error("site status.label is empty")
			}
		}
		if site.RackCount != nil {
			t.Logf("  rack_count=%d device_count=%d", *site.RackCount, *site.DeviceCount)
		}
	}
}

func TestNetboxLocationsListWithGoNetbox(t *testing.T) {
	baseURL := netboxBaseURL(t)
	seedNetboxData(t, baseURL)

	client := newNetboxClient(t)
	ctx := context.Background()

	result, resp, err := client.DcimAPI.DcimLocationsList(ctx).Execute()
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			t.Logf("Response body: %s", body)
		}
		t.Fatalf("DcimLocationsList failed: %v", err)
	}

	// 3 locations: floor-1, row-A (under floor-1), row-B
	if result.Count != 3 {
		t.Errorf("location count = %d, want 3", result.Count)
	}

	for _, loc := range result.Results {
		t.Logf("Location: id=%d name=%q slug=%q depth=%d", loc.Id, loc.Name, loc.Slug, loc.Depth)

		if loc.Url == "" {
			t.Error("location url is empty")
		}
		if loc.Slug == "" {
			t.Errorf("location %q slug is empty", loc.Name)
		}
		if loc.Site.Name == "" {
			t.Errorf("location %q site.name is empty", loc.Name)
		}
	}
}

func TestNetboxRacksListWithGoNetbox(t *testing.T) {
	baseURL := netboxBaseURL(t)
	seedNetboxData(t, baseURL)

	client := newNetboxClient(t)
	ctx := context.Background()

	result, resp, err := client.DcimAPI.DcimRacksList(ctx).Execute()
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			t.Logf("Response body: %s", body)
		}
		t.Fatalf("DcimRacksList failed: %v", err)
	}

	if result.Count != 3 {
		t.Errorf("rack count = %d, want 3", result.Count)
	}

	for _, rack := range result.Results {
		t.Logf("Rack: id=%d name=%q site=%q", rack.Id, rack.Name, rack.Site.Name)

		if rack.Url == "" {
			t.Error("rack url is empty")
		}
		if rack.Site.Slug == "" {
			t.Errorf("rack %q site.slug is empty", rack.Name)
		}
	}
}

func TestNetboxDevicesListWithGoNetbox(t *testing.T) {
	baseURL := netboxBaseURL(t)
	seedNetboxData(t, baseURL)

	client := newNetboxClient(t)
	ctx := context.Background()

	result, resp, err := client.DcimAPI.DcimDevicesList(ctx).Execute()
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			t.Logf("Response body: %s", body)
		}
		t.Fatalf("DcimDevicesList failed: %v", err)
	}

	if result.Count != 4 {
		t.Errorf("device count = %d, want 4", result.Count)
	}

	for _, dev := range result.Results {
		name := ""
		if dev.Name.IsSet() {
			name = *dev.Name.Get()
		}
		t.Logf("Device: id=%d name=%q role=%q site=%q", dev.Id, name, dev.Role.Name, dev.Site.Name)

		if dev.Url == "" {
			t.Error("device url is empty")
		}
		if dev.Role.Slug == "" {
			t.Errorf("device %q role.slug is empty", name)
		}
		if dev.DeviceType.Slug == "" {
			t.Errorf("device %q device_type.slug is empty", name)
		}
		if dev.Site.Slug == "" {
			t.Errorf("device %q site.slug is empty", name)
		}

		// Check location is populated
		if dev.Location.IsSet() && dev.Location.Get() != nil {
			loc := dev.Location.Get()
			t.Logf("  location=%q depth=%d", loc.Name, loc.Depth)
		}

		// Check custom_fields
		if dev.CustomFields != nil {
			if hostID, ok := dev.CustomFields["cirrus_host_id"]; ok {
				t.Logf("  cirrus_host_id=%v", hostID)
			}
		}
	}
}

func TestNetboxSitesFilterBySiteId(t *testing.T) {
	baseURL := netboxBaseURL(t)
	seedNetboxData(t, baseURL)

	client := newNetboxClient(t)
	ctx := context.Background()

	// Filter racks by site_id=1
	result, _, err := client.DcimAPI.DcimRacksList(ctx).SiteId([]int32{1}).Execute()
	if err != nil {
		t.Fatalf("DcimRacksList with filter failed: %v", err)
	}

	if result.Count != 2 {
		t.Errorf("filtered rack count = %d, want 2 (site-tokyo has 2 racks)", result.Count)
	}
}

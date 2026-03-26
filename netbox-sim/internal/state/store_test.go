package state

import (
	"testing"
)

func TestAddAndListSites(t *testing.T) {
	s := NewStore()
	id := s.AddSite("site-tokyo", "active", &NamedRef{ID: 1, Name: "japan", Display: "japan", Slug: "japan"}, nil)
	if id != 1 {
		t.Errorf("id = %d, want 1", id)
	}

	sites := s.ListSites()
	if len(sites) != 1 {
		t.Fatalf("len = %d, want 1", len(sites))
	}
	if sites[0].Name != "site-tokyo" {
		t.Errorf("name = %q, want %q", sites[0].Name, "site-tokyo")
	}
	if sites[0].Slug != "site-tokyo" {
		t.Errorf("slug = %q, want %q", sites[0].Slug, "site-tokyo")
	}
}

func TestAddAndListRacks(t *testing.T) {
	s := NewStore()
	siteID := s.AddSite("site-1", "", nil, nil)
	s.AddRack("rack-1", siteID, 0, "", map[string]string{"tor_switch": "tor-1"})
	s.AddRack("rack-2", siteID, 0, "", nil)
	s.AddRack("rack-3", siteID+1, 0, "", nil) // different site

	racks := s.ListRacks(siteID)
	if len(racks) != 2 {
		t.Errorf("filtered count = %d, want 2", len(racks))
	}

	all := s.ListRacks(0)
	if len(all) != 3 {
		t.Errorf("all count = %d, want 3", len(all))
	}
}

func TestAddAndListDevices(t *testing.T) {
	s := NewStore()
	siteID := s.AddSite("site-1", "", nil, nil)
	locID := s.AddLocation("row-1", siteID, 0, nil)
	rackID := s.AddRack("rack-1", siteID, locID, "", nil)
	s.AddDevice("host-001", "server", siteID, locID, rackID, 40, "", map[string]string{"cirrus_host_id": "host-001"})
	s.AddDevice("host-002", "server", siteID, locID, rackID, 38, "", nil)
	s.AddDevice("switch-001", "switch", siteID, locID, rackID, 1, "", nil)

	servers := s.ListDevices(0, "server")
	if len(servers) != 2 {
		t.Errorf("server count = %d, want 2", len(servers))
	}

	rackDevices := s.ListDevices(rackID, "")
	if len(rackDevices) != 3 {
		t.Errorf("rack device count = %d, want 3", len(rackDevices))
	}

	// Verify role field
	if servers[0].Role != "server" {
		t.Errorf("role = %q, want server", servers[0].Role)
	}
}

func TestHierarchicalLocations(t *testing.T) {
	s := NewStore()
	siteID := s.AddSite("site-tokyo", "", nil, nil)

	floorID := s.AddLocation("floor-1", siteID, 0, map[string]string{"power_feed": "pf-a"})
	rowID := s.AddLocation("row-A", siteID, floorID, nil)

	floor := s.GetLocation(floorID)
	if floor.ParentID != 0 {
		t.Errorf("floor parentID = %d, want 0", floor.ParentID)
	}
	if floor.CustomFields["power_feed"] != "pf-a" {
		t.Errorf("floor power_feed = %q, want %q", floor.CustomFields["power_feed"], "pf-a")
	}
	if floor.Slug != "floor-1" {
		t.Errorf("floor slug = %q, want floor-1", floor.Slug)
	}

	row := s.GetLocation(rowID)
	if row.ParentID != floorID {
		t.Errorf("row parentID = %d, want %d", row.ParentID, floorID)
	}

	topLevel := s.ListLocations(siteID, 0)
	if len(topLevel) != 1 {
		t.Errorf("top-level count = %d, want 1", len(topLevel))
	}

	children := s.ListLocations(siteID, floorID)
	if len(children) != 1 {
		t.Errorf("children count = %d, want 1", len(children))
	}

	all := s.ListLocations(0, -1)
	if len(all) != 2 {
		t.Errorf("all count = %d, want 2", len(all))
	}
}

func TestLocationAncestors(t *testing.T) {
	s := NewStore()
	siteID := s.AddSite("site-1", "", nil, nil)

	floorID := s.AddLocation("floor-1", siteID, 0, nil)
	hallID := s.AddLocation("hall-A", siteID, floorID, nil)
	rowID := s.AddLocation("row-1", siteID, hallID, nil)

	ancestors := s.LocationAncestors(rowID)
	if len(ancestors) != 3 {
		t.Fatalf("ancestors count = %d, want 3", len(ancestors))
	}
	if ancestors[0].Name != "row-1" {
		t.Errorf("ancestors[0] = %q, want row-1", ancestors[0].Name)
	}
	if ancestors[1].Name != "hall-A" {
		t.Errorf("ancestors[1] = %q, want hall-A", ancestors[1].Name)
	}
	if ancestors[2].Name != "floor-1" {
		t.Errorf("ancestors[2] = %q, want floor-1", ancestors[2].Name)
	}
}

func TestCountMethods(t *testing.T) {
	s := NewStore()
	siteID := s.AddSite("s1", "", nil, nil)
	locID := s.AddLocation("row-1", siteID, 0, nil)
	rackID := s.AddRack("r1", siteID, locID, "", nil)
	s.AddDevice("h1", "server", siteID, locID, rackID, 1, "", nil)
	s.AddDevice("h2", "server", siteID, locID, rackID, 2, "", nil)

	if c := s.CountRacksInSite(siteID); c != 1 {
		t.Errorf("racks in site = %d, want 1", c)
	}
	if c := s.CountDevicesInSite(siteID); c != 2 {
		t.Errorf("devices in site = %d, want 2", c)
	}
	if c := s.CountRacksInLocation(locID); c != 1 {
		t.Errorf("racks in location = %d, want 1", c)
	}
	if c := s.CountDevicesInRack(rackID); c != 2 {
		t.Errorf("devices in rack = %d, want 2", c)
	}
}

func TestStatsAndReset(t *testing.T) {
	s := NewStore()
	siteID := s.AddSite("s1", "", nil, nil)
	locID := s.AddLocation("row-A", siteID, 0, nil)
	rackID := s.AddRack("r1", siteID, locID, "", nil)
	s.AddDevice("h1", "server", siteID, locID, rackID, 1, "", nil)

	stats := s.GetStats()
	if stats.SiteCount != 1 || stats.RackCount != 1 || stats.DeviceCount != 1 || stats.LocationCount != 1 {
		t.Errorf("unexpected stats: %+v", stats)
	}

	s.Reset()
	stats = s.GetStats()
	if stats.SiteCount != 0 || stats.DeviceCount != 0 {
		t.Errorf("expected all zeros after reset: %+v", stats)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"site-tokyo", "site-tokyo"},
		{"Site Tokyo", "site-tokyo"},
		{"rack_row_1", "rack-row-1"},
		{"Floor 3 (Main)", "floor-3-main"},
	}
	for _, tt := range tests {
		got := slugify(tt.input)
		if got != tt.want {
			t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// Package state provides in-memory storage for NetBox simulator entities.
package state

import "sync"

// NamedRef represents a reference to a named entity with an ID.
type NamedRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Site represents a NetBox site.
type Site struct {
	ID           int               `json:"id"`
	Name         string            `json:"name"`
	Status       string            `json:"status"`
	Region       *NamedRef         `json:"region"`
	CustomFields map[string]string `json:"custom_fields"`
}

// Location represents a hierarchical location within a site (floor, hall, rack row, etc.).
// Locations can be nested via ParentID to form a tree: Site → Floor → Rack Row.
type Location struct {
	ID           int               `json:"id"`
	Name         string            `json:"name"`
	SiteID       int               `json:"site_id"`
	ParentID     int               `json:"parent_id,omitempty"`
	CustomFields map[string]string `json:"custom_fields"`
}

// Rack represents a NetBox rack.
type Rack struct {
	ID           int               `json:"id"`
	Name         string            `json:"name"`
	SiteID       int               `json:"site_id"`
	LocationID   int               `json:"location_id"`
	Status       string            `json:"status"`
	CustomFields map[string]string `json:"custom_fields"`
}

// Device represents a NetBox device.
type Device struct {
	ID           int               `json:"id"`
	Name         string            `json:"name"`
	DeviceRole   string            `json:"device_role"`
	SiteID       int               `json:"site_id"`
	RackID       int               `json:"rack_id"`
	Position     int               `json:"position"`
	Status       string            `json:"status"`
	CustomFields map[string]string `json:"custom_fields"`
}

// Store holds all in-memory state for the NetBox simulator.
type Store struct {
	mu sync.RWMutex

	sites     map[int]*Site
	locations map[int]*Location
	racks     map[int]*Rack
	devices   map[int]*Device

	nextSiteID     int
	nextLocationID int
	nextRackID     int
	nextDeviceID   int
}

// NewStore creates a new empty Store.
func NewStore() *Store {
	return &Store{
		sites:          make(map[int]*Site),
		locations:      make(map[int]*Location),
		racks:          make(map[int]*Rack),
		devices:        make(map[int]*Device),
		nextSiteID:     1,
		nextLocationID: 1,
		nextRackID:     1,
		nextDeviceID:   1,
	}
}

// AddSite adds a site and returns its assigned ID.
func (s *Store) AddSite(name, status string, region *NamedRef, customFields map[string]string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextSiteID
	s.nextSiteID++

	if status == "" {
		status = "active"
	}
	if customFields == nil {
		customFields = make(map[string]string)
	}

	s.sites[id] = &Site{
		ID:           id,
		Name:         name,
		Status:       status,
		Region:       region,
		CustomFields: customFields,
	}
	return id
}

// AddLocation adds a location and returns its assigned ID.
// parentID of 0 means a top-level location under the site.
func (s *Store) AddLocation(name string, siteID, parentID int, customFields map[string]string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextLocationID
	s.nextLocationID++

	if customFields == nil {
		customFields = make(map[string]string)
	}

	s.locations[id] = &Location{
		ID:           id,
		Name:         name,
		SiteID:       siteID,
		ParentID:     parentID,
		CustomFields: customFields,
	}
	return id
}

// AddRack adds a rack and returns its assigned ID.
func (s *Store) AddRack(name string, siteID, locationID int, status string, customFields map[string]string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextRackID
	s.nextRackID++

	if status == "" {
		status = "active"
	}
	if customFields == nil {
		customFields = make(map[string]string)
	}

	s.racks[id] = &Rack{
		ID:           id,
		Name:         name,
		SiteID:       siteID,
		LocationID:   locationID,
		Status:       status,
		CustomFields: customFields,
	}
	return id
}

// AddDevice adds a device and returns its assigned ID.
func (s *Store) AddDevice(name, deviceRole string, siteID, rackID, position int, status string, customFields map[string]string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextDeviceID
	s.nextDeviceID++

	if status == "" {
		status = "active"
	}
	if deviceRole == "" {
		deviceRole = "server"
	}
	if customFields == nil {
		customFields = make(map[string]string)
	}

	s.devices[id] = &Device{
		ID:           id,
		Name:         name,
		DeviceRole:   deviceRole,
		SiteID:       siteID,
		RackID:       rackID,
		Position:     position,
		Status:       status,
		CustomFields: customFields,
	}
	return id
}

// ListSites returns all sites.
func (s *Store) ListSites() []*Site {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sites := make([]*Site, 0, len(s.sites))
	for _, site := range s.sites {
		sites = append(sites, site)
	}
	return sites
}

// ListLocations returns locations, optionally filtered by siteID and/or parentID.
// If siteID is 0, no site filter is applied.
// If parentID is -1, no parent filter is applied. If 0, returns top-level locations only.
func (s *Store) ListLocations(siteID, parentID int) []*Location {
	s.mu.RLock()
	defer s.mu.RUnlock()

	locs := make([]*Location, 0, len(s.locations))
	for _, loc := range s.locations {
		if siteID != 0 && loc.SiteID != siteID {
			continue
		}
		if parentID != -1 && loc.ParentID != parentID {
			continue
		}
		locs = append(locs, loc)
	}
	return locs
}

// ListRacks returns all racks, optionally filtered by siteID.
// If siteID is 0, all racks are returned.
func (s *Store) ListRacks(siteID int) []*Rack {
	s.mu.RLock()
	defer s.mu.RUnlock()

	racks := make([]*Rack, 0, len(s.racks))
	for _, rack := range s.racks {
		if siteID != 0 && rack.SiteID != siteID {
			continue
		}
		racks = append(racks, rack)
	}
	return racks
}

// ListDevices returns all devices, optionally filtered by rackID and/or role.
// If rackID is 0, the rack filter is not applied.
// If role is empty, the role filter is not applied.
func (s *Store) ListDevices(rackID int, role string) []*Device {
	s.mu.RLock()
	defer s.mu.RUnlock()

	devices := make([]*Device, 0, len(s.devices))
	for _, device := range s.devices {
		if rackID != 0 && device.RackID != rackID {
			continue
		}
		if role != "" && device.DeviceRole != role {
			continue
		}
		devices = append(devices, device)
	}
	return devices
}

// GetSite returns a site by ID, or nil if not found.
func (s *Store) GetSite(id int) *Site {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sites[id]
}

// GetLocation returns a location by ID, or nil if not found.
func (s *Store) GetLocation(id int) *Location {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.locations[id]
}

// GetRack returns a rack by ID, or nil if not found.
func (s *Store) GetRack(id int) *Rack {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.racks[id]
}

// LocationAncestors returns the chain of parent locations from the given location up to the site.
// The result is ordered from the given location to the root (site-level).
func (s *Store) LocationAncestors(locationID int) []*Location {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var chain []*Location
	seen := make(map[int]bool)
	for id := locationID; id != 0; {
		if seen[id] {
			break // prevent infinite loop
		}
		seen[id] = true
		loc, ok := s.locations[id]
		if !ok {
			break
		}
		chain = append(chain, loc)
		id = loc.ParentID
	}
	return chain
}

// Stats holds counts for all entity types.
type Stats struct {
	SiteCount     int `json:"site_count"`
	RackCount     int `json:"rack_count"`
	DeviceCount   int `json:"device_count"`
	LocationCount int `json:"location_count"`
}

// GetStats returns counts for all entity types.
func (s *Store) GetStats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return Stats{
		SiteCount:     len(s.sites),
		RackCount:     len(s.racks),
		DeviceCount:   len(s.devices),
		LocationCount: len(s.locations),
	}
}

// Reset clears all state and resets ID counters.
func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sites = make(map[int]*Site)
	s.locations = make(map[int]*Location)
	s.racks = make(map[int]*Rack)
	s.devices = make(map[int]*Device)
	s.nextSiteID = 1
	s.nextLocationID = 1
	s.nextRackID = 1
	s.nextDeviceID = 1
}

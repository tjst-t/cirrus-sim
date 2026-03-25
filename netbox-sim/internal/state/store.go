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

// Location represents a row or location within a site.
type Location struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	SiteID int    `json:"site_id"`
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
func (s *Store) AddLocation(name string, siteID int) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextLocationID
	s.nextLocationID++

	s.locations[id] = &Location{
		ID:     id,
		Name:   name,
		SiteID: siteID,
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

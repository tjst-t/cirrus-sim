// Package state provides in-memory storage for NetBox simulator entities.
package state

import (
	"strings"
	"sync"
	"time"
)

// slugify converts a name to a URL-friendly slug.
func slugify(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	// Remove any characters that aren't alphanumeric or hyphens
	var b strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// NamedRef represents a brief nested reference to a named entity.
type NamedRef struct {
	ID      int    `json:"id"`
	URL     string `json:"url"`
	Display string `json:"display"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
}

// Site represents a NetBox site.
type Site struct {
	ID           int               `json:"id"`
	Name         string            `json:"name"`
	Slug         string            `json:"slug"`
	Status       string            `json:"status"`
	Region       *NamedRef         `json:"region"`
	Description  string            `json:"description"`
	Tags         []interface{}     `json:"tags"`
	CustomFields map[string]string `json:"custom_fields"`
	CreatedAt    time.Time         `json:"created"`
	LastUpdated  time.Time         `json:"last_updated"`
}

// Location represents a hierarchical location within a site (floor, hall, rack row, etc.).
type Location struct {
	ID           int               `json:"id"`
	Name         string            `json:"name"`
	Slug         string            `json:"slug"`
	SiteID       int               `json:"site_id"`
	ParentID     int               `json:"parent_id,omitempty"`
	Status       string            `json:"status"`
	Description  string            `json:"description"`
	Tags         []interface{}     `json:"tags"`
	CustomFields map[string]string `json:"custom_fields"`
	CreatedAt    time.Time         `json:"created"`
	LastUpdated  time.Time         `json:"last_updated"`
}

// Rack represents a NetBox rack.
type Rack struct {
	ID           int               `json:"id"`
	Name         string            `json:"name"`
	Slug         string            `json:"slug"`
	SiteID       int               `json:"site_id"`
	LocationID   int               `json:"location_id"`
	Status       string            `json:"status"`
	Description  string            `json:"description"`
	UHeight      int               `json:"u_height"`
	Tags         []interface{}     `json:"tags"`
	CustomFields map[string]string `json:"custom_fields"`
	CreatedAt    time.Time         `json:"created"`
	LastUpdated  time.Time         `json:"last_updated"`
}

// Device represents a NetBox device.
type Device struct {
	ID           int               `json:"id"`
	Name         string            `json:"name"`
	Role         string            `json:"role"`
	SiteID       int               `json:"site_id"`
	LocationID   int               `json:"location_id"`
	RackID       int               `json:"rack_id"`
	Position     int               `json:"position"`
	Face         string            `json:"face"`
	Status       string            `json:"status"`
	Description  string            `json:"description"`
	Tags         []interface{}     `json:"tags"`
	CustomFields map[string]string `json:"custom_fields"`
	CreatedAt    time.Time         `json:"created"`
	LastUpdated  time.Time         `json:"last_updated"`
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
	now := time.Now().UTC()

	s.sites[id] = &Site{
		ID:           id,
		Name:         name,
		Slug:         slugify(name),
		Status:       status,
		Region:       region,
		Tags:         []interface{}{},
		CustomFields: customFields,
		CreatedAt:    now,
		LastUpdated:  now,
	}
	return id
}

// AddLocation adds a location and returns its assigned ID.
func (s *Store) AddLocation(name string, siteID, parentID int, customFields map[string]string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextLocationID
	s.nextLocationID++

	if customFields == nil {
		customFields = make(map[string]string)
	}
	now := time.Now().UTC()

	s.locations[id] = &Location{
		ID:           id,
		Name:         name,
		Slug:         slugify(name),
		SiteID:       siteID,
		ParentID:     parentID,
		Status:       "active",
		Tags:         []interface{}{},
		CustomFields: customFields,
		CreatedAt:    now,
		LastUpdated:  now,
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
	now := time.Now().UTC()

	s.racks[id] = &Rack{
		ID:           id,
		Name:         name,
		Slug:         slugify(name),
		SiteID:       siteID,
		LocationID:   locationID,
		Status:       status,
		UHeight:      42,
		Tags:         []interface{}{},
		CustomFields: customFields,
		CreatedAt:    now,
		LastUpdated:  now,
	}
	return id
}

// AddDevice adds a device and returns its assigned ID.
func (s *Store) AddDevice(name, role string, siteID, locationID, rackID, position int, status string, customFields map[string]string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextDeviceID
	s.nextDeviceID++

	if status == "" {
		status = "active"
	}
	if role == "" {
		role = "server"
	}
	if customFields == nil {
		customFields = make(map[string]string)
	}
	now := time.Now().UTC()

	s.devices[id] = &Device{
		ID:           id,
		Name:         name,
		Role:         role,
		SiteID:       siteID,
		LocationID:   locationID,
		RackID:       rackID,
		Position:     position,
		Face:         "front",
		Status:       status,
		Tags:         []interface{}{},
		CustomFields: customFields,
		CreatedAt:    now,
		LastUpdated:  now,
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
func (s *Store) ListDevices(rackID int, role string) []*Device {
	s.mu.RLock()
	defer s.mu.RUnlock()

	devices := make([]*Device, 0, len(s.devices))
	for _, device := range s.devices {
		if rackID != 0 && device.RackID != rackID {
			continue
		}
		if role != "" && device.Role != role {
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

// LocationAncestors returns the chain of parent locations from the given location up to the root.
func (s *Store) LocationAncestors(locationID int) []*Location {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var chain []*Location
	seen := make(map[int]bool)
	for id := locationID; id != 0; {
		if seen[id] {
			break
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

// CountDevicesInRack returns the number of devices in a rack.
func (s *Store) CountDevicesInRack(rackID int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, d := range s.devices {
		if d.RackID == rackID {
			count++
		}
	}
	return count
}

// CountRacksInLocation returns the number of racks in a location.
func (s *Store) CountRacksInLocation(locationID int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, r := range s.racks {
		if r.LocationID == locationID {
			count++
		}
	}
	return count
}

// CountDevicesInLocation returns the number of devices in a location.
func (s *Store) CountDevicesInLocation(locationID int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, d := range s.devices {
		if d.LocationID == locationID {
			count++
		}
	}
	return count
}

// CountRacksInSite returns the number of racks in a site.
func (s *Store) CountRacksInSite(siteID int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, r := range s.racks {
		if r.SiteID == siteID {
			count++
		}
	}
	return count
}

// CountDevicesInSite returns the number of devices in a site.
func (s *Store) CountDevicesInSite(siteID int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, d := range s.devices {
		if d.SiteID == siteID {
			count++
		}
	}
	return count
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

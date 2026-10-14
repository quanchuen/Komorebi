// internal/domain/environment/venue.go
package environment

// Venue is a point-of-interest sourced from OSM (environment schema).
type Venue struct {
	ID       string // UUID
	OsmID    int64
	Name     string
	Category string
	Brand    string
	Lat      float64
	Lon      float64
	OsmTags  map[string]string
}

// VenueTagMapping links a hashtag to an OSM filter for venue matching.
type VenueTagMapping struct {
	Hashtag     string
	OSMFilter   map[string]string
	Description string
	IsBrand     bool
}

// VenueTag represents a hashtag → OSM filter mapping from venue_tag_mapping.
type VenueTag struct {
	Hashtag     string
	Description string
	IsBrand     bool
}

// AlongRouteParams holds parameters for the venues-along-route query.
type AlongRouteParams struct {
	RouteID  string
	Category string  // optional: filter by venue category (maps to hashtag lookup)
	BufferM  float64 // distance from route geometry in metres; default 200
}

// NearestAlongLineParams holds parameters for finding the nearest venue along a route line.
type NearestAlongLineParams struct {
	// RouteWKT is the LINESTRING WKT of the current route geometry (SRID 4326).
	RouteWKT string
	// OSMFilter maps OSM tag keys to expected values (all must match).
	OSMFilter map[string]string
	// IsBrand when true uses ILIKE matching on brand field values.
	IsBrand bool
	// BufferM is the search corridor in metres; defaults to 200.
	BufferM float64
}

// VenueRepository defines persistence operations for venue reads.
type VenueRepository interface {
	// AlongRoute returns venues within BufferM metres of the named route geometry.
	// Category is an optional filter (e.g. "convenience", "cafe").
	AlongRoute(params AlongRouteParams) ([]Venue, error)

	// ListTags returns all hashtag → filter mappings from venue_tag_mapping.
	ListTags() ([]VenueTag, error)

	// GetTagMapping returns the full VenueTagMapping for the given hashtag, or nil if not found.
	GetTagMapping(hashtag string) (*VenueTagMapping, error)

	// NearestAlongLine returns the single nearest venue matching the OSM filter
	// within BufferM metres of the route geometry WKT.
	// Returns nil, nil when no matching venue exists within the corridor.
	NearestAlongLine(params NearestAlongLineParams) (*Venue, error)
}

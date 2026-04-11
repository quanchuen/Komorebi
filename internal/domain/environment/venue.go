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

// VenueRepository defines persistence operations for venue reads.
type VenueRepository interface {
	// AlongRoute returns venues within BufferM metres of the named route geometry.
	// Category is an optional filter (e.g. "convenience", "cafe").
	AlongRoute(params AlongRouteParams) ([]Venue, error)

	// ListTags returns all hashtag → filter mappings from venue_tag_mapping.
	ListTags() ([]VenueTag, error)
}

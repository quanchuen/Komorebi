package environment

// Venue represents a point-of-interest mapped from OpenStreetMap.
type Venue struct {
	ID       string
	OSMID    int64
	Lat      float64
	Lon      float64
	Name     string
	Category string
	Brand    string
	OSMTags  map[string]string
}

// VenueTagMapping links a hashtag to an OSM filter for venue matching.
type VenueTagMapping struct {
	Hashtag     string
	OSMFilter   map[string]string
	Description string
	IsBrand     bool
}

package environment

// GreeneryIndex holds greenery scoring for an OSM way.
type GreeneryIndex struct {
	OSMWayID      int64
	GreeneryScore float64
	TreeLined     bool
	ParkAdjacent  bool
}

// RouteGreeneryParams carries the parameters for a greenery query along a route.
type RouteGreeneryParams struct {
	// RouteID is the UUID of the route whose geometry is used for the spatial join.
	RouteID string
	// BufferDeg is the planar-degree buffer around the route geometry used to
	// match greenery_edge rows via their osm_way_id → osm.roads geometry.
	// Defaults to 0.00009 (~10 m) if zero.
	BufferDeg float64
}

// RouteGreeneryResult summarises greenery along a route.
type RouteGreeneryResult struct {
	// AvgScore is the mean greenery_score across all matched edges (0.0–1.0).
	AvgScore float64
	// EdgeCount is the number of OSM way edges matched.
	EdgeCount int
}

// GreeneryRepository is the read-side port for greenery_edge data.
type GreeneryRepository interface {
	// ScoreAlongRoute returns the average greenery score for OSM edges that
	// spatially overlap the named route within BufferDeg degrees.
	ScoreAlongRoute(params RouteGreeneryParams) (RouteGreeneryResult, error)
}

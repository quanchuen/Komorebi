package environment

// ShadowGrid stores shade coverage for a map cell at a given time slot.
type ShadowGrid struct {
	ID            string
	CellGeometry  [][2]float64
	HourSlot      int
	Month         int
	ShadeCoverage float64
}

// ShadowParams identifies the time slot and route geometry for a shadow query.
type ShadowParams struct {
	// RouteGeometryWKT is a WKT LINESTRING in EPSG:4326.
	RouteGeometryWKT string
	// BufferM is the buffer radius around the route in metres for the spatial join.
	// Defaults to 50 m (half grid cell) if zero.
	BufferM  float64
	HourSlot int
	Month    int
}

// ShadowRepository retrieves precomputed shade coverage from the shadow grid.
type ShadowRepository interface {
	// ForRoute returns all shadow grid cells that intersect a buffered route
	// geometry at the given hour slot and month. Cells are returned in no
	// guaranteed order.
	ForRoute(params ShadowParams) ([]ShadowGrid, error)
}

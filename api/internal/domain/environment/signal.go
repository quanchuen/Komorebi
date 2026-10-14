package environment

// TrafficSignal represents a traffic light node from OpenStreetMap.
type TrafficSignal struct {
	OSMNodeID int64
	Lat       float64
	Lon       float64
}

// RouteSignalParams carries parameters for counting signals along a route.
type RouteSignalParams struct {
	// RouteID is the UUID of the route.
	RouteID string
	// BufferM is the corridor width in metres. Defaults to 30 if zero.
	BufferM float64
}

// SegmentSignalCount holds the signal count for one route segment.
type SegmentSignalCount struct {
	// SegmentOrder matches routes.route_segment.segment_order.
	SegmentOrder int
	// Count is the number of traffic signals within BufferM of this segment.
	Count int
}

// SignalRepository is the read-side port for traffic signal queries.
type SignalRepository interface {
	// CountAlongRoute returns per-segment signal counts for the named route.
	// Signals within BufferM metres of each segment geometry are counted.
	CountAlongRoute(params RouteSignalParams) ([]SegmentSignalCount, error)

	// TotalAlongRoute returns the total signal count within a corridor around
	// the full route geometry. Used for route-level speed model penalties.
	TotalAlongRoute(params RouteSignalParams) (int, error)
}

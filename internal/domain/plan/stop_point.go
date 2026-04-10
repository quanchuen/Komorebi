package plan

// StopType classifies how a stop was added to a plan.
type StopType string

const (
	StopManual        StopType = "manual"
	StopVenueResolved StopType = "venue_resolved"
	StopWaypoint      StopType = "waypoint"
)

// StopPoint is a location the cyclist intends to visit.
type StopPoint struct {
	ID           string
	Lat          float64
	Lon          float64
	Type         StopType
	SortOrder    int
	VenueID      string
	ResolvedName string
}

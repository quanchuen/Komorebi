package route

// WaypointType classifies the kind of waypoint along a route.
type WaypointType string

const (
	WaypointViewpoint WaypointType = "viewpoint"
	WaypointRestStop  WaypointType = "rest_stop"
	WaypointWater     WaypointType = "water"
	WaypointShrine    WaypointType = "shrine"
	WaypointKonbini   WaypointType = "konbini"
	WaypointOther     WaypointType = "other"
)

// Waypoint is a notable point along the route.
type Waypoint struct {
	ID        string
	Name      string
	Type      WaypointType
	Lat       float64
	Lon       float64
	SortOrder int
}

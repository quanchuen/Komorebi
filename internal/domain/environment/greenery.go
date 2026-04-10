package environment

// GreeneryIndex holds greenery scoring for an OSM way.
type GreeneryIndex struct {
	OSMWayID     int64
	GreeneryScore float64
	TreeLined    bool
	ParkAdjacent bool
}

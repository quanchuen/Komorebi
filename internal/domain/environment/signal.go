package environment

// TrafficSignal represents a traffic light node from OpenStreetMap.
type TrafficSignal struct {
	OSMNodeID int64
	Lat       float64
	Lon       float64
}

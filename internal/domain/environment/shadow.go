package environment

// ShadowGrid stores shade coverage for a map cell at a given time slot.
type ShadowGrid struct {
	ID           string
	CellGeometry [][2]float64
	HourSlot     int
	Month        int
	ShadeCoverage float64
}

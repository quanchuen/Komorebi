package route

// SurfaceType describes the road surface of a segment.
type SurfaceType string

const (
	SurfacePaved      SurfaceType = "paved"
	SurfaceGravel     SurfaceType = "gravel"
	SurfaceDirt       SurfaceType = "dirt"
	SurfaceCobbleston SurfaceType = "cobblestone"
)

// Segment represents a contiguous stretch of the route with uniform surface conditions.
type Segment struct {
	ID           string
	Geometry     [][3]float64
	SurfaceType  SurfaceType
	GradePercent float64
	SegmentOrder int
}

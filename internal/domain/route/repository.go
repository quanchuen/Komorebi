package route

// ListParams specifies filters for listing routes.
type ListParams struct {
	BBox       [4]float64 // [minLon, minLat, maxLon, maxLat]
	Difficulty Difficulty
	Surface    SurfaceType
	Tags       []string
	MinDistM   float64
	MaxDistM   float64
	Cursor     string
	Limit      int
}

// ListResult holds a page of routes plus the next pagination cursor.
type ListResult struct {
	Routes     []*Route
	NextCursor string
}

// Repository defines persistence operations for Route aggregates.
type Repository interface {
	Create(r *Route) error
	GetByID(id string) (*Route, error)
	Update(r *Route) error
	List(params ListParams) (ListResult, error)
	Delete(id string) error
}

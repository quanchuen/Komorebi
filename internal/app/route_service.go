package app

import (
	"github.com/cyclist-map/cyclist-map/internal/domain/route"
)

// RouteService provides application-level operations on Routes.
type RouteService struct {
	repo route.Repository
}

// NewRouteService creates a RouteService backed by the given repository.
func NewRouteService(repo route.Repository) *RouteService {
	return &RouteService{repo: repo}
}

// CreateRoute builds a new Route aggregate and persists it.
func (s *RouteService) CreateRoute(
	name, description string,
	difficulty route.Difficulty,
	creatorID string,
	geometry [][3]float64,
	distanceM, elevGainM, elevLossM float64,
	waypoints []route.Waypoint,
	segments []route.Segment,
	tags []string,
) (*route.Route, error) {
	rt, err := route.NewRoute(name, description, difficulty, creatorID)
	if err != nil {
		return nil, err
	}
	if len(geometry) > 0 {
		rt.SetGeometry(geometry, distanceM, elevGainM, elevLossM)
	}
	for _, wp := range waypoints {
		rt.AddWaypoint(wp)
	}
	for _, seg := range segments {
		rt.AddSegment(seg)
	}
	if len(tags) > 0 {
		rt.SetTags(tags)
	}
	if err := s.repo.Create(rt); err != nil {
		return nil, err
	}
	return rt, nil
}

// GetRoute retrieves a route by ID.
func (s *RouteService) GetRoute(id string) (*route.Route, error) {
	return s.repo.GetByID(id)
}

// UpdateRoute replaces name, description, difficulty, and tags on an existing route.
func (s *RouteService) UpdateRoute(
	id, name, description string,
	difficulty route.Difficulty,
	tags []string,
) (*route.Route, error) {
	rt, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if err := rt.UpdateMetadata(name, description, difficulty, tags); err != nil {
		return nil, err
	}
	if err := s.repo.Update(rt); err != nil {
		return nil, err
	}
	return rt, nil
}

// ListRoutes returns a filtered, paginated list of routes.
func (s *RouteService) ListRoutes(params route.ListParams) (route.ListResult, error) {
	return s.repo.List(params)
}

// ArchiveRoute transitions a Published route to Archived.
func (s *RouteService) ArchiveRoute(id string) (*route.Route, error) {
	rt, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if err := rt.Archive(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(rt); err != nil {
		return nil, err
	}
	return rt, nil
}

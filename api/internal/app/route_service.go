package app

import (
	"komorebi/internal/domain/route"
)

// RouteService provides application-level read operations on Routes.
type RouteService struct {
	repo       route.Repository
	elevations ElevationSampler
}

// NewRouteService creates a RouteService backed by the given repository.
// elevations may be nil; when set, routes stored without elevation are
// backfilled on read (see backfillElevation).
func NewRouteService(repo route.Repository, elevations ElevationSampler) *RouteService {
	return &RouteService{repo: repo, elevations: elevations}
}

// GetRoute retrieves a route by ID, backfilling elevation for routes that
// were stored without it.
func (s *RouteService) GetRoute(id string) (*route.Route, error) {
	rt, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	s.backfillElevation(rt)
	return rt, nil
}

// ListRoutes returns a filtered, paginated list of routes.
func (s *RouteService) ListRoutes(params route.ListParams) (route.ListResult, error) {
	return s.repo.List(params)
}

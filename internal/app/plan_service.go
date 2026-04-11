// internal/app/plan_service.go
package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/domain/plan"
	routedomain "github.com/cyclist-map/cyclist-map/internal/domain/route"
)

// PlanRepository is the persistence interface used by PlanService.
// Satisfied by *postgres.PlanRepo.
type PlanRepository interface {
	Create(p *plan.RoutePlan) error
	GetByID(id string) (*plan.RoutePlan, error)
	Update(p *plan.RoutePlan) error
	Delete(id string) error
}

// RouteGeometryReader can load a Route's waypoints for plan seeding.
type RouteGeometryReader interface {
	GetByID(id string) (*routedomain.Route, error)
}

// CreatePlanRequest holds the inputs for creating a new plan.
type CreatePlanRequest struct {
	UserID      string
	DepartureAt time.Time
	SpeedModel  plan.SpeedModel
	Preferences plan.Preferences
	// SourceRouteID when non-empty seeds the plan from a curated route's waypoints.
	SourceRouteID string
}

// AddStopRequest holds inputs for adding a stop.
type AddStopRequest struct {
	PlanID string
	Stop   plan.StopPoint
}

// AddTaskRequest holds inputs for adding a task.
type AddTaskRequest struct {
	PlanID      string
	Description string
	Hashtag     string
}

// ErrPlanNotFound is returned when the requested plan does not exist.
var ErrPlanNotFound = errors.New("plan not found")

// PlanService orchestrates the RoutePlan lifecycle.
type PlanService struct {
	repo       PlanRepository
	routeRepo  RouteGeometryReader
	routing    *RoutingService
	resolution *VenueResolutionService
}

// NewPlanService creates a PlanService.
func NewPlanService(
	repo PlanRepository,
	routeRepo RouteGeometryReader,
	routing *RoutingService,
	resolution *VenueResolutionService,
) *PlanService {
	return &PlanService{
		repo:       repo,
		routeRepo:  routeRepo,
		routing:    routing,
		resolution: resolution,
	}
}

// CreatePlan builds a new RoutePlan, optionally seeded from a curated route.
func (s *PlanService) CreatePlan(req CreatePlanRequest) (*plan.RoutePlan, error) {
	p := plan.NewRoutePlan(req.UserID)
	p.DepartureAt = req.DepartureAt
	p.SpeedModel = req.SpeedModel
	p.Preferences = req.Preferences

	if req.SourceRouteID != "" {
		rt, err := s.routeRepo.GetByID(req.SourceRouteID)
		if err != nil {
			return nil, fmt.Errorf("load source route: %w", err)
		}
		for i, wp := range rt.Waypoints {
			p.AddStop(plan.StopPoint{
				ID:           plan.NewStopID(),
				Lat:          wp.Lat,
				Lon:          wp.Lon,
				Type:         plan.StopWaypoint,
				SortOrder:    i,
				ResolvedName: wp.Name,
			})
		}
		if len(p.Stops) >= 2 {
			if err := s.reroute(p); err != nil {
				// Non-fatal: plan is valid even without an initial route line.
				p.RouteWKT = ""
			}
		}
	}

	if err := s.repo.Create(p); err != nil {
		return nil, fmt.Errorf("persist plan: %w", err)
	}
	return p, nil
}

// GetPlan returns a RoutePlan with resolved stops and tasks.
func (s *PlanService) GetPlan(id string) (*plan.RoutePlan, error) {
	p, err := s.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrPlanNotFound
		}
		return nil, fmt.Errorf("get plan: %w", err)
	}
	return p, nil
}

// AddStop appends a stop to the plan and triggers a re-route.
func (s *PlanService) AddStop(req AddStopRequest) (*plan.RoutePlan, error) {
	p, err := s.repo.GetByID(req.PlanID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrPlanNotFound
		}
		return nil, fmt.Errorf("load plan for AddStop: %w", err)
	}

	stop := req.Stop
	if stop.ID == "" {
		stop.ID = plan.NewStopID()
	}
	stop.SortOrder = len(p.Stops)
	p.AddStop(stop)

	if len(p.Stops) >= 2 {
		_ = s.reroute(p) // non-fatal
	}

	if err := s.repo.Update(p); err != nil {
		return nil, fmt.Errorf("persist plan after AddStop: %w", err)
	}
	return p, nil
}

// RemoveStop removes a stop from the plan, renumbers sort_order, and triggers re-route.
func (s *PlanService) RemoveStop(planID, stopID string) (*plan.RoutePlan, error) {
	p, err := s.repo.GetByID(planID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrPlanNotFound
		}
		return nil, fmt.Errorf("load plan for RemoveStop: %w", err)
	}

	p.RemoveStop(stopID)
	renumberStops(p)

	if len(p.Stops) >= 2 {
		_ = s.reroute(p) // non-fatal
	}

	if err := s.repo.Update(p); err != nil {
		return nil, fmt.Errorf("persist plan after RemoveStop: %w", err)
	}
	return p, nil
}

// AddTask appends a task, resolves its hashtag if present, and persists.
func (s *PlanService) AddTask(req AddTaskRequest) (*plan.RoutePlan, error) {
	p, err := s.repo.GetByID(req.PlanID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrPlanNotFound
		}
		return nil, fmt.Errorf("load plan for AddTask: %w", err)
	}

	hashtag := req.Hashtag
	if hashtag == "" {
		hashtag = extractHashtag(req.Description)
	}
	t := plan.PlanTask{
		ID:          plan.NewTaskID(),
		Description: req.Description,
		Hashtag:     hashtag,
		Status:      plan.TaskUnresolved,
	}

	if p.RouteWKT != "" {
		t, _ = s.resolution.ResolveTask(t, p.RouteWKT) // resolution failure is non-fatal
	}

	p.AddTask(t)

	if err := s.repo.Update(p); err != nil {
		return nil, fmt.Errorf("persist plan after AddTask: %w", err)
	}
	return p, nil
}

// --- internal helpers ---

// reroute calls Valhalla with the suggested profile and stores the computed WKT.
func (s *PlanService) reroute(p *plan.RoutePlan) error {
	result, err := s.routing.GetSingleDirections(DirectionsRequest{
		Stops:       p.Stops,
		DepartureAt: p.DepartureAt,
		SpeedModel:  p.SpeedModel,
		Preferences: p.Preferences,
	}, "suggested")
	if err != nil {
		return err
	}
	p.RouteWKT = geoJSONToWKT(result.GeoJSON)
	return nil
}

// geoJSONToWKT converts a GeoJSONLineString to a LINESTRING WKT string.
// Format: LINESTRING(lon lat, lon lat, ...)
func geoJSONToWKT(g GeoJSONLineString) string {
	if len(g.Coordinates) == 0 {
		return ""
	}
	pts := make([]string, len(g.Coordinates))
	for i, c := range g.Coordinates {
		pts[i] = fmt.Sprintf("%f %f", c[0], c[1])
	}
	return "LINESTRING(" + strings.Join(pts, ", ") + ")"
}

// renumberStops reassigns SortOrder 0..N-1 after a stop is removed.
func renumberStops(p *plan.RoutePlan) {
	for i := range p.Stops {
		p.Stops[i].SortOrder = i
	}
}

// _ ensures PlanService satisfies PlanDirector at compile time.
// (checked in plan_handler.go via the PlanDirector interface)
var _ = time.Now // keep time import

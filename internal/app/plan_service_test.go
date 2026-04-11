// internal/app/plan_service_test.go
package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/cyclist-map/cyclist-map/internal/domain/plan"
	routedomain "github.com/cyclist-map/cyclist-map/internal/domain/route"
	"github.com/cyclist-map/cyclist-map/internal/infra/valhalla"
)

// --- stubs ---

type stubPlanRepo struct {
	plans  map[string]*plan.RoutePlan
	saveErr error
}

func newStubPlanRepo() *stubPlanRepo {
	return &stubPlanRepo{plans: map[string]*plan.RoutePlan{}}
}

func (s *stubPlanRepo) Create(p *plan.RoutePlan) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	clone := *p
	s.plans[p.ID] = &clone
	return nil
}

func (s *stubPlanRepo) GetByID(id string) (*plan.RoutePlan, error) {
	p, ok := s.plans[id]
	if !ok {
		return nil, app.ErrNotFound
	}
	clone := *p
	return &clone, nil
}

func (s *stubPlanRepo) Update(p *plan.RoutePlan) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	clone := *p
	s.plans[p.ID] = &clone
	return nil
}

func (s *stubPlanRepo) Delete(id string) error {
	delete(s.plans, id)
	return nil
}

type stubRouteReader struct {
	route *routedomain.Route
	err   error
}

func (s *stubRouteReader) GetByID(_ string) (*routedomain.Route, error) {
	return s.route, s.err
}

type stubRouter struct {
	result *valhalla.RouteResult
	err    error
}

func (s *stubRouter) Route(_ []valhalla.Location, _ valhalla.RouteProfile) (*valhalla.RouteResult, error) {
	return s.result, s.err
}

func noOpResolutionSvc() *app.VenueResolutionService {
	return app.NewVenueResolutionService(&stubVenueNearby{}, &stubTagLookup{})
}

func noOpRoutingService() *app.RoutingService {
	// Returns error so reroute is non-fatal.
	return app.NewRoutingService(&stubRouter{err: errors.New("valhalla not running")})
}

// --- tests ---

func TestPlanService_CreatePlan_Empty(t *testing.T) {
	repo := newStubPlanRepo()
	svc := app.NewPlanService(repo, &stubRouteReader{}, noOpRoutingService(), noOpResolutionSvc())

	p, err := svc.CreatePlan(app.CreatePlanRequest{
		UserID:      "user-1",
		DepartureAt: time.Now(),
		SpeedModel:  plan.SpeedModelFlat,
	})
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	if p.ID == "" {
		t.Error("expected non-empty ID")
	}
	if p.UserID != "user-1" {
		t.Errorf("expected user-1, got %q", p.UserID)
	}
}

func TestPlanService_GetPlan_NotFound(t *testing.T) {
	repo := newStubPlanRepo()
	svc := app.NewPlanService(repo, &stubRouteReader{}, noOpRoutingService(), noOpResolutionSvc())

	_, err := svc.GetPlan("nonexistent-id")
	if !errors.Is(err, app.ErrPlanNotFound) {
		t.Errorf("expected ErrPlanNotFound, got %v", err)
	}
}

func TestPlanService_AddStop_TriggersUpdate(t *testing.T) {
	repo := newStubPlanRepo()
	svc := app.NewPlanService(repo, &stubRouteReader{}, noOpRoutingService(), noOpResolutionSvc())

	// Create a plan first.
	p, err := svc.CreatePlan(app.CreatePlanRequest{
		UserID:     "user-1",
		SpeedModel: plan.SpeedModelFlat,
	})
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	// Add first stop.
	_, err = svc.AddStop(app.AddStopRequest{
		PlanID: p.ID,
		Stop:   plan.StopPoint{Lat: 35.6762, Lon: 139.6503, Type: plan.StopManual},
	})
	if err != nil {
		t.Fatalf("AddStop 1: %v", err)
	}

	// Add second stop (triggers reroute attempt, non-fatal).
	updated, err := svc.AddStop(app.AddStopRequest{
		PlanID: p.ID,
		Stop:   plan.StopPoint{Lat: 35.6895, Lon: 139.6917, Type: plan.StopManual},
	})
	if err != nil {
		t.Fatalf("AddStop 2: %v", err)
	}
	if len(updated.Stops) != 2 {
		t.Errorf("expected 2 stops, got %d", len(updated.Stops))
	}
}

func TestPlanService_RemoveStop(t *testing.T) {
	repo := newStubPlanRepo()
	svc := app.NewPlanService(repo, &stubRouteReader{}, noOpRoutingService(), noOpResolutionSvc())

	p, _ := svc.CreatePlan(app.CreatePlanRequest{UserID: "user-1", SpeedModel: plan.SpeedModelFlat})

	p, _ = svc.AddStop(app.AddStopRequest{
		PlanID: p.ID,
		Stop:   plan.StopPoint{Lat: 35.6762, Lon: 139.6503, Type: plan.StopManual},
	})
	stopID := p.Stops[0].ID

	p, _ = svc.AddStop(app.AddStopRequest{
		PlanID: p.ID,
		Stop:   plan.StopPoint{Lat: 35.6895, Lon: 139.6917, Type: plan.StopManual},
	})

	result, err := svc.RemoveStop(p.ID, stopID)
	if err != nil {
		t.Fatalf("RemoveStop: %v", err)
	}
	if len(result.Stops) != 1 {
		t.Errorf("expected 1 stop after removal, got %d", len(result.Stops))
	}
	// Verify SortOrder renumbered.
	if result.Stops[0].SortOrder != 0 {
		t.Errorf("expected SortOrder 0, got %d", result.Stops[0].SortOrder)
	}
}

func TestPlanService_AddTask_NoRoute_StaysUnresolved(t *testing.T) {
	repo := newStubPlanRepo()
	svc := app.NewPlanService(repo, &stubRouteReader{}, noOpRoutingService(), noOpResolutionSvc())

	p, _ := svc.CreatePlan(app.CreatePlanRequest{UserID: "user-1", SpeedModel: plan.SpeedModelFlat})

	result, err := svc.AddTask(app.AddTaskRequest{
		PlanID:      p.ID,
		Description: "Stop at #cafe",
	})
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	if len(result.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result.Tasks))
	}
	// No RouteWKT means resolution is skipped, task stays unresolved.
	if result.Tasks[0].Status != plan.TaskUnresolved {
		t.Errorf("expected unresolved, got %q", result.Tasks[0].Status)
	}
	if result.Tasks[0].Hashtag != "#cafe" {
		t.Errorf("expected #cafe, got %q", result.Tasks[0].Hashtag)
	}
}

func TestPlanService_CreatePlan_FromRoute_FailsNonFatal(t *testing.T) {
	// Route reader returns error — should still create an empty plan.
	repo := newStubPlanRepo()
	svc := app.NewPlanService(repo,
		&stubRouteReader{err: errors.New("route not found")},
		noOpRoutingService(),
		noOpResolutionSvc(),
	)

	_, err := svc.CreatePlan(app.CreatePlanRequest{
		UserID:        "user-1",
		SourceRouteID: "route-123",
	})
	// Should return error because the route load fails (not non-fatal).
	if err == nil {
		t.Fatal("expected error when source route not found, got nil")
	}
}

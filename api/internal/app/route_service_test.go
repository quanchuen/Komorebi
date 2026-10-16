package app_test

import (
	"errors"
	"testing"

	"komorebi/internal/app"
	"komorebi/internal/domain/route"
)

// fakeRepo is an in-memory implementation of route.Repository for unit tests.
type fakeRepo struct {
	routes map[string]*route.Route
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{routes: map[string]*route.Route{}}
}

// Create seeds the fake; the real repo has no API write path.
func (f *fakeRepo) Create(r *route.Route) error {
	cp := *r
	f.routes[r.ID] = &cp
	return nil
}

func (f *fakeRepo) add(name string, difficulty route.Difficulty) *route.Route {
	rt, _ := route.NewRoute(name, "", difficulty, "")
	_ = f.Create(rt)
	return rt
}

func (f *fakeRepo) GetByID(id string) (*route.Route, error) {
	r, ok := f.routes[id]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := *r
	return &cp, nil
}

func (f *fakeRepo) UpdateElevation(id string, geometry [][3]float64, elevGainM, elevLossM float64) error {
	r, ok := f.routes[id]
	if !ok {
		return errors.New("not found")
	}
	r.Geometry = geometry
	r.ElevationGainM = elevGainM
	r.ElevationLossM = elevLossM
	return nil
}

func (f *fakeRepo) List(params route.ListParams) (route.ListResult, error) {
	var routes []*route.Route
	for _, r := range f.routes {
		cp := *r
		routes = append(routes, &cp)
	}
	return route.ListResult{Routes: routes}, nil
}

func TestRouteService_GetRoute(t *testing.T) {
	repo := newFakeRepo()
	svc := app.NewRouteService(repo, nil)
	rt := repo.add("Route A", route.DifficultyHard)

	got, err := svc.GetRoute(rt.ID)
	if err != nil {
		t.Fatalf("GetRoute: %v", err)
	}
	if got.Name != "Route A" {
		t.Errorf("Name: got %q", got.Name)
	}
}

func TestRouteService_ListRoutes(t *testing.T) {
	repo := newFakeRepo()
	svc := app.NewRouteService(repo, nil)
	repo.add("R1", route.DifficultyEasy)
	repo.add("R2", route.DifficultyHard)

	result, err := svc.ListRoutes(route.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("ListRoutes: %v", err)
	}
	if len(result.Routes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(result.Routes))
	}
}

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

func (f *fakeRepo) Create(r *route.Route) error {
	if _, exists := f.routes[r.ID]; exists {
		return errors.New("route already exists")
	}
	// Store a copy
	cp := *r
	f.routes[r.ID] = &cp
	return nil
}

func (f *fakeRepo) GetByID(id string) (*route.Route, error) {
	r, ok := f.routes[id]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := *r
	return &cp, nil
}

func (f *fakeRepo) Update(r *route.Route) error {
	if _, ok := f.routes[r.ID]; !ok {
		return errors.New("not found")
	}
	cp := *r
	f.routes[r.ID] = &cp
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

func (f *fakeRepo) Delete(id string) error {
	delete(f.routes, id)
	return nil
}

func TestRouteService_CreateRoute_DraftStatus(t *testing.T) {
	svc := app.NewRouteService(newFakeRepo())

	rt, err := svc.CreateRoute(
		"My Route", "desc", route.DifficultyEasy, "creator-1",
		[][3]float64{{139.69, 35.68, 0}, {139.70, 35.69, 10}},
		1000, 10, 5,
		nil, nil, []string{"fun"},
	)
	if err != nil {
		t.Fatalf("CreateRoute: %v", err)
	}
	if rt.Status != route.StatusDraft {
		t.Errorf("expected Draft status, got %q", rt.Status)
	}
	if rt.ID == "" {
		t.Error("expected non-empty ID")
	}
	if len(rt.Tags) != 1 || rt.Tags[0] != "fun" {
		t.Errorf("expected tags [fun], got %v", rt.Tags)
	}
}

func TestRouteService_CreateRoute_EmptyName(t *testing.T) {
	svc := app.NewRouteService(newFakeRepo())
	_, err := svc.CreateRoute("", "desc", route.DifficultyEasy, "", nil, 0, 0, 0, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestRouteService_GetRoute(t *testing.T) {
	repo := newFakeRepo()
	svc := app.NewRouteService(repo)

	rt, _ := svc.CreateRoute("Route A", "", route.DifficultyHard, "", nil, 0, 0, 0, nil, nil, nil)

	got, err := svc.GetRoute(rt.ID)
	if err != nil {
		t.Fatalf("GetRoute: %v", err)
	}
	if got.Name != "Route A" {
		t.Errorf("Name: got %q", got.Name)
	}
}

func TestRouteService_UpdateRoute(t *testing.T) {
	svc := app.NewRouteService(newFakeRepo())
	rt, _ := svc.CreateRoute("Old Name", "", route.DifficultyEasy, "", nil, 0, 0, 0, nil, nil, nil)

	updated, err := svc.UpdateRoute(rt.ID, "New Name", "new desc", route.DifficultyHard, []string{"tag1"})
	if err != nil {
		t.Fatalf("UpdateRoute: %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("Name: got %q want %q", updated.Name, "New Name")
	}
	if updated.Difficulty != route.DifficultyHard {
		t.Errorf("Difficulty: got %q", updated.Difficulty)
	}
}

func TestRouteService_ArchiveRoute_PublishedToArchived(t *testing.T) {
	repo := newFakeRepo()
	svc := app.NewRouteService(repo)

	// Create and manually publish via repo to bypass service
	rt, _ := svc.CreateRoute("Route X", "", route.DifficultyEasy, "", nil, 0, 0, 0, nil, nil, nil)

	// Publish directly (domain method)
	stored, _ := repo.GetByID(rt.ID)
	_ = stored.Publish()
	_ = repo.Update(stored)

	archived, err := svc.ArchiveRoute(rt.ID)
	if err != nil {
		t.Fatalf("ArchiveRoute: %v", err)
	}
	if archived.Status != route.StatusArchived {
		t.Errorf("Status: got %q want archived", archived.Status)
	}
}

func TestRouteService_ArchiveRoute_DraftFails(t *testing.T) {
	svc := app.NewRouteService(newFakeRepo())
	rt, _ := svc.CreateRoute("Route Y", "", route.DifficultyEasy, "", nil, 0, 0, 0, nil, nil, nil)

	_, err := svc.ArchiveRoute(rt.ID)
	if err == nil {
		t.Fatal("expected error: cannot archive a draft route")
	}
	if !errors.Is(err, route.ErrInvalidTransition) {
		t.Errorf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestRouteService_ListRoutes(t *testing.T) {
	svc := app.NewRouteService(newFakeRepo())
	svc.CreateRoute("R1", "", route.DifficultyEasy, "", nil, 0, 0, 0, nil, nil, nil)
	svc.CreateRoute("R2", "", route.DifficultyHard, "", nil, 0, 0, 0, nil, nil, nil)

	result, err := svc.ListRoutes(route.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("ListRoutes: %v", err)
	}
	if len(result.Routes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(result.Routes))
	}
}

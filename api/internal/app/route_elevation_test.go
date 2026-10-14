package app_test

import (
	"testing"

	"komorebi/internal/app"
	"komorebi/internal/domain/route"
	"komorebi/internal/infra/valhalla"
)

// stubSampler implements app.ElevationSampler with fixed heights.
type stubSampler struct {
	heights []float64
	profile []valhalla.ElevationPoint
	calls   int
}

func (s *stubSampler) Heights(shape [][2]float64) ([]float64, error) {
	s.calls++
	return s.heights, nil
}

func (s *stubSampler) ElevationProfile(shape [][2]float64) ([]valhalla.ElevationPoint, error) {
	return s.profile, nil
}

func flatRoute(t *testing.T, repo *fakeRepo) *route.Route {
	t.Helper()
	rt, err := route.NewRoute("Palace Loop", "test", route.DifficultyEasy, "creator-1")
	if err != nil {
		t.Fatalf("NewRoute: %v", err)
	}
	rt.SetGeometry([][3]float64{
		{139.75, 35.68, 0}, {139.76, 35.69, 0}, {139.77, 35.70, 0},
	}, 4800, 0, 0)
	if err := repo.Create(rt); err != nil {
		t.Fatalf("Create: %v", err)
	}
	return rt
}

func TestGetRouteBackfillsMissingElevation(t *testing.T) {
	repo := newFakeRepo()
	rt := flatRoute(t, repo)
	sampler := &stubSampler{
		heights: []float64{5, 12, 8},
		profile: []valhalla.ElevationPoint{
			{DistanceM: 0, ElevationM: 5},
			{DistanceM: 100, ElevationM: 12},
			{DistanceM: 200, ElevationM: 8},
		},
	}
	svc := app.NewRouteService(repo, sampler)

	got, err := svc.GetRoute(rt.ID)
	if err != nil {
		t.Fatalf("GetRoute: %v", err)
	}
	if got.Geometry[1][2] != 12 {
		t.Errorf("expected z backfilled to 12, got %v", got.Geometry[1][2])
	}
	if got.ElevationGainM != 7 || got.ElevationLossM != 4 {
		t.Errorf("expected gain 7 / loss 4, got %v / %v", got.ElevationGainM, got.ElevationLossM)
	}

	// Persisted: second read serves stored elevation without sampling again.
	again, err := svc.GetRoute(rt.ID)
	if err != nil {
		t.Fatalf("GetRoute again: %v", err)
	}
	if again.Geometry[2][2] != 8 {
		t.Errorf("expected persisted z 8, got %v", again.Geometry[2][2])
	}
	if sampler.calls != 1 {
		t.Errorf("expected 1 sampler call after persistence, got %d", sampler.calls)
	}
	// Backfill is a read-path side effect: it must persist elevation only,
	// never rewrite the whole aggregate (which would race concurrent edits).
	if repo.updateCalls != 0 {
		t.Errorf("expected backfill to avoid full Update, got %d calls", repo.updateCalls)
	}
}

func TestGetRouteSkipsBackfillWhenElevationPresent(t *testing.T) {
	repo := newFakeRepo()
	rt, err := route.NewRoute("Hilly", "test", route.DifficultyHard, "creator-1")
	if err != nil {
		t.Fatalf("NewRoute: %v", err)
	}
	rt.SetGeometry([][3]float64{{139.75, 35.68, 20}, {139.76, 35.69, 45}}, 2000, 25, 0)
	if err := repo.Create(rt); err != nil {
		t.Fatalf("Create: %v", err)
	}
	sampler := &stubSampler{heights: []float64{1, 1}}
	svc := app.NewRouteService(repo, sampler)

	got, err := svc.GetRoute(rt.ID)
	if err != nil {
		t.Fatalf("GetRoute: %v", err)
	}
	if sampler.calls != 0 {
		t.Errorf("expected no sampler call for route with elevation, got %d", sampler.calls)
	}
	if got.Geometry[0][2] != 20 {
		t.Errorf("stored z overwritten: got %v", got.Geometry[0][2])
	}
}

func TestGetRouteSkipsPersistWhenNoCoverage(t *testing.T) {
	repo := newFakeRepo()
	rt := flatRoute(t, repo)
	sampler := &stubSampler{heights: []float64{0, 0, 0}}
	svc := app.NewRouteService(repo, sampler)

	if _, err := svc.GetRoute(rt.ID); err != nil {
		t.Fatalf("GetRoute: %v", err)
	}
	// All-zero heights mean no DEM coverage: don't persist, retry next read.
	if _, err := svc.GetRoute(rt.ID); err != nil {
		t.Fatalf("GetRoute again: %v", err)
	}
	if sampler.calls != 2 {
		t.Errorf("expected retry on next read (2 calls), got %d", sampler.calls)
	}
}

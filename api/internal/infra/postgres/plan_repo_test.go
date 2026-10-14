// internal/infra/postgres/plan_repo_test.go
package postgres_test

import (
	"testing"
	"time"

	"komorebi/internal/domain/plan"
	"komorebi/internal/infra/postgres"
)

func samplePlan(t *testing.T) *plan.RoutePlan {
	t.Helper()
	p := plan.NewRoutePlan("00000000-0000-0000-0000-000000000001")
	p.DepartureAt = time.Now().UTC().Truncate(time.Second)
	p.SpeedModel = plan.SpeedModelElevation
	p.Preferences = plan.Preferences{ShadeWeight: 0.5, GreeneryWeight: 0.3, WindWeight: 0.2}
	p.AddStop(plan.StopPoint{
		ID: plan.NewStopID(), Lat: 35.6762, Lon: 139.6503,
		Type: plan.StopManual, SortOrder: 0, ResolvedName: "Origin",
	})
	p.AddStop(plan.StopPoint{
		ID: plan.NewStopID(), Lat: 35.6895, Lon: 139.6917,
		Type: plan.StopManual, SortOrder: 1, ResolvedName: "Destination",
	})
	return p
}

func TestPlanRepo_CreateAndGetByID(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewPlanRepo(pool)

	p := samplePlan(t)
	if err := repo.Create(p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(p.ID) })

	got, err := repo.GetByID(p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != p.ID {
		t.Errorf("ID mismatch: got %s, want %s", got.ID, p.ID)
	}
	if len(got.Stops) != 2 {
		t.Errorf("expected 2 stops, got %d", len(got.Stops))
	}
}

func TestPlanRepo_GetByID_NotFound(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewPlanRepo(pool)

	_, err := repo.GetByID("00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
}

func TestPlanRepo_Update_ReplaceStops(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewPlanRepo(pool)

	p := samplePlan(t)
	if err := repo.Create(p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(p.ID) })

	// Add a third stop and update.
	p.AddStop(plan.StopPoint{
		ID: plan.NewStopID(), Lat: 35.700, Lon: 139.700,
		Type: plan.StopManual, SortOrder: 2,
	})
	if err := repo.Update(p); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.GetByID(p.ID)
	if err != nil {
		t.Fatalf("GetByID after Update: %v", err)
	}
	if len(got.Stops) != 3 {
		t.Errorf("expected 3 stops after update, got %d", len(got.Stops))
	}
}

func TestPlanRepo_Update_WithRouteWKT(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewPlanRepo(pool)

	p := samplePlan(t)
	p.RouteWKT = "LINESTRING(139.6503 35.6762, 139.6917 35.6895)"
	if err := repo.Create(p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(p.ID) })

	got, err := repo.GetByID(p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.RouteWKT != p.RouteWKT {
		t.Errorf("RouteWKT mismatch: got %q, want %q", got.RouteWKT, p.RouteWKT)
	}
}

func TestPlanRepo_Update_WithTask(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewPlanRepo(pool)

	p := samplePlan(t)
	p.AddTask(plan.PlanTask{
		ID:          plan.NewTaskID(),
		Description: "Buy coffee at #cafe",
		Hashtag:     "#cafe",
		Status:      plan.TaskUnresolved,
	})
	if err := repo.Create(p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(p.ID) })

	got, err := repo.GetByID(p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(got.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(got.Tasks))
	}
	if got.Tasks[0].Hashtag != "#cafe" {
		t.Errorf("expected hashtag #cafe, got %q", got.Tasks[0].Hashtag)
	}
	if got.Tasks[0].Status != plan.TaskUnresolved {
		t.Errorf("expected status unresolved, got %q", got.Tasks[0].Status)
	}
}

func TestPlanRepo_Delete(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewPlanRepo(pool)

	p := samplePlan(t)
	if err := repo.Create(p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := repo.GetByID(p.ID)
	if err == nil {
		t.Fatal("expected ErrNotFound after delete, got nil")
	}
}

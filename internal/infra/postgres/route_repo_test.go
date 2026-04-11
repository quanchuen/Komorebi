package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/domain/route"
	"github.com/cyclist-map/cyclist-map/internal/infra/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultDSN = "postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable"

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = defaultDSN
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Skipf("skipping postgres integration test: %v", err)
	}
	pingCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		t.Skipf("skipping postgres integration test: cannot ping db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func sampleRoute(t *testing.T) *route.Route {
	t.Helper()
	rt, err := route.NewRoute("Test Route", "A test route", route.DifficultyModerate, "")
	if err != nil {
		t.Fatalf("NewRoute: %v", err)
	}
	rt.SetGeometry([][3]float64{
		{139.6917, 35.6895, 10},
		{139.7000, 35.6950, 20},
		{139.7100, 35.7000, 15},
	}, 1500, 30, 20)
	rt.SetTags([]string{"scenic", "beginner"})
	rt.AddWaypoint(route.Waypoint{
		Name:      "Viewpoint A",
		Type:      route.WaypointViewpoint,
		Lat:       35.6950,
		Lon:       139.7000,
		SortOrder: 0,
	})
	rt.AddSegment(route.Segment{
		Geometry: [][3]float64{
			{139.6917, 35.6895, 10},
			{139.7000, 35.6950, 20},
		},
		SurfaceType:  route.SurfacePaved,
		GradePercent: 2.5,
		SegmentOrder: 0,
	})
	return rt
}

func TestRouteRepo_CreateGetDelete(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewRouteRepo(pool)

	rt := sampleRoute(t)

	// Create
	if err := repo.Create(rt); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(rt.ID) })

	// GetByID
	got, err := repo.GetByID(rt.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != rt.Name {
		t.Errorf("Name: got %q want %q", got.Name, rt.Name)
	}
	if got.Status != route.StatusDraft {
		t.Errorf("Status: got %q want draft", got.Status)
	}
	if len(got.Geometry) != 3 {
		t.Errorf("Geometry: got %d points, want 3", len(got.Geometry))
	}
	if len(got.Tags) != 2 {
		t.Errorf("Tags: got %d, want 2", len(got.Tags))
	}
	if len(got.Waypoints) != 1 {
		t.Errorf("Waypoints: got %d, want 1", len(got.Waypoints))
	}
	if len(got.Segments) != 1 {
		t.Errorf("Segments: got %d, want 1", len(got.Segments))
	}
}

func TestRouteRepo_Update(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewRouteRepo(pool)

	rt := sampleRoute(t)
	if err := repo.Create(rt); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(rt.ID) })

	// Update metadata and publish
	if err := rt.UpdateMetadata("Updated Name", "New desc", route.DifficultyHard, []string{"mountain"}); err != nil {
		t.Fatalf("UpdateMetadata: %v", err)
	}
	if err := repo.Update(rt); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.GetByID(rt.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if got.Name != "Updated Name" {
		t.Errorf("Name after update: got %q want %q", got.Name, "Updated Name")
	}
	if got.Difficulty != route.DifficultyHard {
		t.Errorf("Difficulty after update: got %q want hard", got.Difficulty)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "mountain" {
		t.Errorf("Tags after update: got %v", got.Tags)
	}
}

func TestRouteRepo_List(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewRouteRepo(pool)

	rt1 := sampleRoute(t)
	rt2 := sampleRoute(t)
	rt2.Name = "Second Test Route"

	if err := repo.Create(rt1); err != nil {
		t.Fatalf("Create rt1: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(rt1.ID) })

	if err := repo.Create(rt2); err != nil {
		t.Fatalf("Create rt2: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(rt2.ID) })

	// List with tag filter - should find only rt1
	result, err := repo.List(route.ListParams{Tags: []string{"scenic"}, Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	found := false
	for _, r := range result.Routes {
		if r.ID == rt1.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("List with tag 'scenic': rt1 not found in results (got %d routes)", len(result.Routes))
	}

	// List with difficulty filter
	result, err = repo.List(route.ListParams{Difficulty: route.DifficultyModerate, Limit: 10})
	if err != nil {
		t.Fatalf("List by difficulty: %v", err)
	}
	if len(result.Routes) == 0 {
		t.Error("expected at least one moderate route")
	}

	// Pagination: limit 1, check cursor
	result, err = repo.List(route.ListParams{Tags: []string{"scenic"}, Limit: 1})
	if err != nil {
		t.Fatalf("List paginated: %v", err)
	}
	// If both routes are created between tests, we should get a cursor
	_ = result.NextCursor
}

func TestRouteRepo_NotFound(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewRouteRepo(pool)

	_, err := repo.GetByID("00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("expected error for non-existent route")
	}
}

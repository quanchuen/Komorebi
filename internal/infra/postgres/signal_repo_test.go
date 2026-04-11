// internal/infra/postgres/signal_repo_test.go
package postgres_test

import (
	"context"
	"testing"

	"github.com/cyclist-map/cyclist-map/internal/domain/environment"
	"github.com/cyclist-map/cyclist-map/internal/infra/postgres"
)

func TestSignalRepo_CountAlongRoute_NonExistentRoute(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewSignalRepo(pool)

	counts, err := repo.CountAlongRoute(environment.RouteSignalParams{
		RouteID: "00000000-0000-0000-0000-000000000000",
		BufferM: 30,
	})
	if err != nil {
		t.Fatalf("CountAlongRoute non-existent route: %v", err)
	}
	if counts == nil {
		t.Fatal("expected non-nil slice")
	}
	if len(counts) != 0 {
		t.Errorf("expected 0 segments for non-existent route, got %d", len(counts))
	}
}

func TestSignalRepo_TotalAlongRoute_NonExistentRoute(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewSignalRepo(pool)

	total, err := repo.TotalAlongRoute(environment.RouteSignalParams{
		RouteID: "00000000-0000-0000-0000-000000000000",
		BufferM: 30,
	})
	if err != nil {
		t.Fatalf("TotalAlongRoute non-existent route: %v", err)
	}
	if total != 0 {
		t.Errorf("expected 0 signals for non-existent route, got %d", total)
	}
}

func TestSignalRepo_TotalAlongRoute_DefaultBuffer(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewSignalRepo(pool)

	// Zero BufferM should apply the 30 m default without error.
	_, err := repo.TotalAlongRoute(environment.RouteSignalParams{
		RouteID: "00000000-0000-0000-0000-000000000000",
		BufferM: 0,
	})
	if err != nil {
		t.Fatalf("TotalAlongRoute zero buffer: %v", err)
	}
}

func TestSignalRepo_CountAlongRoute_DefaultBuffer(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewSignalRepo(pool)

	_, err := repo.CountAlongRoute(environment.RouteSignalParams{
		RouteID: "00000000-0000-0000-0000-000000000000",
		BufferM: 0,
	})
	if err != nil {
		t.Fatalf("CountAlongRoute zero buffer: %v", err)
	}
}

func TestSignalRepo_CountAlongRoute_WithData(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewSignalRepo(pool)

	var routeID string
	err := pool.QueryRow(context.Background(),
		`SELECT id FROM routes.route WHERE EXISTS (
			SELECT 1 FROM routes.route_segment rs WHERE rs.route_id = routes.route.id
		) LIMIT 1`,
	).Scan(&routeID)
	if err != nil {
		t.Skip("no routes with segments in DB; skipping data smoke test")
	}

	counts, err := repo.CountAlongRoute(environment.RouteSignalParams{
		RouteID: routeID,
		BufferM: 30,
	})
	if err != nil {
		t.Fatalf("CountAlongRoute: %v", err)
	}
	for _, sc := range counts {
		if sc.Count < 0 {
			t.Errorf("segment %d: signal count must be non-negative, got %d", sc.SegmentOrder, sc.Count)
		}
	}
}

func TestSignalRepo_TotalAlongRoute_WithData(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewSignalRepo(pool)

	var routeID string
	err := pool.QueryRow(context.Background(),
		`SELECT id FROM routes.route LIMIT 1`,
	).Scan(&routeID)
	if err != nil {
		t.Skip("no routes in DB; skipping data smoke test")
	}

	total, err := repo.TotalAlongRoute(environment.RouteSignalParams{
		RouteID: routeID,
		BufferM: 30,
	})
	if err != nil {
		t.Fatalf("TotalAlongRoute: %v", err)
	}
	if total < 0 {
		t.Errorf("total must be non-negative, got %d", total)
	}
}

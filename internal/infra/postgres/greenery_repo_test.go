// internal/infra/postgres/greenery_repo_test.go
package postgres_test

import (
	"context"
	"testing"

	"github.com/cyclist-map/cyclist-map/internal/domain/environment"
	"github.com/cyclist-map/cyclist-map/internal/infra/postgres"
)

func TestGreeneryRepo_ScoreAlongRoute_NonExistentRoute(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewGreeneryRepo(pool)

	res, err := repo.ScoreAlongRoute(environment.RouteGreeneryParams{
		RouteID:   "00000000-0000-0000-0000-000000000000",
		BufferDeg: 0.00009,
	})
	if err != nil {
		t.Fatalf("ScoreAlongRoute non-existent route: %v", err)
	}
	// No route → COALESCE returns 0, COUNT returns 0
	if res.AvgScore != 0.0 {
		t.Errorf("expected avg_score 0.0 for non-existent route, got %f", res.AvgScore)
	}
	if res.EdgeCount != 0 {
		t.Errorf("expected edge_count 0 for non-existent route, got %d", res.EdgeCount)
	}
}

func TestGreeneryRepo_ScoreAlongRoute_DefaultBuffer(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewGreeneryRepo(pool)

	// Zero BufferDeg should apply the 0.00009° default without error.
	_, err := repo.ScoreAlongRoute(environment.RouteGreeneryParams{
		RouteID:   "00000000-0000-0000-0000-000000000000",
		BufferDeg: 0,
	})
	if err != nil {
		t.Fatalf("ScoreAlongRoute with zero buffer: %v", err)
	}
}

func TestGreeneryRepo_ScoreAlongRoute_WithData(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewGreeneryRepo(pool)

	// Pull any route id to exercise the spatial join.
	var routeID string
	err := pool.QueryRow(context.Background(),
		`SELECT id FROM routes.route LIMIT 1`,
	).Scan(&routeID)
	if err != nil {
		t.Skip("no routes in DB; skipping data smoke test")
	}

	res, err := repo.ScoreAlongRoute(environment.RouteGreeneryParams{
		RouteID:   routeID,
		BufferDeg: 0.00090, // 100 m — wider to maximise chance of hitting edges
	})
	if err != nil {
		t.Fatalf("ScoreAlongRoute: %v", err)
	}
	if res.AvgScore < 0 || res.AvgScore > 1.0 {
		t.Errorf("avg_score out of range [0,1]: %f", res.AvgScore)
	}
	if res.EdgeCount < 0 {
		t.Errorf("edge_count must be non-negative, got %d", res.EdgeCount)
	}
}

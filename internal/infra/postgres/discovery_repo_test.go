// internal/infra/postgres/discovery_repo_test.go
package postgres_test

import (
	"context"
	"os"
	"testing"

	"komorebi/internal/domain/discovery"
	"komorebi/internal/infra/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN not set; skipping DB integration tests")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestDiscoveryRepo_Nearby(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewDiscoveryRepo(pool)

	results, err := repo.Nearby(discovery.NearbyParams{
		Lat:      35.6895,
		Lon:      139.6917,
		RadiusKm: 50,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("Nearby: %v", err)
	}
	// Just verify the call succeeds and returns a slice (may be empty if DB has no routes)
	if results == nil {
		t.Fatal("expected non-nil slice")
	}
	for _, r := range results {
		if r.RouteID == "" {
			t.Fatal("RouteID must not be empty")
		}
		if r.DistFromM < 0 {
			t.Fatalf("DistFromM must be non-negative, got %f", r.DistFromM)
		}
	}
}

func TestDiscoveryRepo_Viewport(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewDiscoveryRepo(pool)

	results, err := repo.Viewport(discovery.ViewportParams{
		BBox:  [4]float64{139.5, 35.5, 140.0, 35.9},
		Limit: 50,
	})
	if err != nil {
		t.Fatalf("Viewport: %v", err)
	}
	if results == nil {
		t.Fatal("expected non-nil slice")
	}
}

func TestDiscoveryRepo_Suggested(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewDiscoveryRepo(pool)

	results, err := repo.Suggested(discovery.SuggestedParams{
		Lat:   35.6895,
		Lon:   139.6917,
		Limit: 5,
	})
	if err != nil {
		t.Fatalf("Suggested: %v", err)
	}
	if results == nil {
		t.Fatal("expected non-nil slice")
	}
}

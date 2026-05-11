// internal/infra/postgres/venue_repo_test.go
package postgres_test

import (
	"testing"

	"komorebi/internal/domain/environment"
	"komorebi/internal/infra/postgres"
)

func TestVenueRepo_ListTags(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewVenueRepo(pool)

	tags, err := repo.ListTags()
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	// May be empty if no seed data; just verify it doesn't error
	if tags == nil {
		t.Fatal("expected non-nil slice")
	}
	for _, tag := range tags {
		if tag.Hashtag == "" {
			t.Fatal("Hashtag must not be empty")
		}
	}
}

func TestVenueRepo_AlongRoute_NoRoute(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewVenueRepo(pool)

	// Non-existent route should return empty, not error
	venues, err := repo.AlongRoute(environment.AlongRouteParams{
		RouteID: "00000000-0000-0000-0000-000000000000",
		BufferM: 200,
	})
	if err != nil {
		t.Fatalf("AlongRoute with non-existent route: %v", err)
	}
	if len(venues) != 0 {
		t.Fatalf("expected 0 venues for non-existent route, got %d", len(venues))
	}
}

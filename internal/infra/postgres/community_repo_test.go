package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/domain/community"
	"github.com/cyclist-map/cyclist-map/internal/infra/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

// --- ContributionRepo ---

func TestContributionRepo_CreateAndGet(t *testing.T) {
	pool := testPool(t)
	userRepo := postgres.NewUserRepo(pool)
	contribRepo := postgres.NewContributionRepo(pool)

	u, _ := community.NewUser("Rider A", uniqueEmail(t))
	_ = userRepo.Create(u)
	t.Cleanup(func() { _ = userRepo.Delete(u.ID) })

	c := community.NewContribution(u.ID, [][3]float64{
		{139.6917, 35.6895, 10},
		{139.7000, 35.6950, 20},
	}, map[string]any{"surface": "gravel"})

	if err := contribRepo.Create(c); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = contribRepo.Delete(c.ID) })

	got, err := contribRepo.GetByID(c.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.UserID != u.ID {
		t.Errorf("UserID: want %q, got %q", u.ID, got.UserID)
	}
	if got.Status != community.StatusPending {
		t.Errorf("Status: want pending, got %v", got.Status)
	}
}

func TestContributionRepo_Update(t *testing.T) {
	pool := testPool(t)
	userRepo := postgres.NewUserRepo(pool)
	contribRepo := postgres.NewContributionRepo(pool)

	u, _ := community.NewUser("Rider B", uniqueEmail(t))
	_ = userRepo.Create(u)
	t.Cleanup(func() { _ = userRepo.Delete(u.ID) })

	c := community.NewContribution(u.ID, nil, nil)
	_ = contribRepo.Create(c)
	t.Cleanup(func() { _ = contribRepo.Delete(c.ID) })

	_ = c.Approve("looks good")
	if err := contribRepo.Update(c); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := contribRepo.GetByID(c.ID)
	if got.Status != community.StatusApproved {
		t.Errorf("Status after approve: %v", got.Status)
	}
}

// --- ReviewRepo ---

func TestReviewRepo_CreateAndList(t *testing.T) {
	pool := testPool(t)
	userRepo := postgres.NewUserRepo(pool)
	reviewRepo := postgres.NewReviewRepo(pool)

	// We need an existing route; use the first published route from seed data.
	routeID := seedRouteID(t, pool)

	u, _ := community.NewUser("Reviewer", uniqueEmail(t))
	_ = userRepo.Create(u)
	t.Cleanup(func() { _ = userRepo.Delete(u.ID) })

	rev, err := community.NewReview(u.ID, routeID, 4, "Great ride!")
	if err != nil {
		t.Fatalf("NewReview: %v", err)
	}
	if err := reviewRepo.Create(rev); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = reviewRepo.Delete(rev.ID) })

	list, err := reviewRepo.ListByRoute(routeID)
	if err != nil {
		t.Fatalf("ListByRoute: %v", err)
	}
	found := false
	for _, r := range list {
		if r.ID == rev.ID {
			found = true
			if r.Rating != 4 {
				t.Errorf("Rating: want 4, got %d", r.Rating)
			}
			break
		}
	}
	if !found {
		t.Error("review not found in ListByRoute result")
	}
}

// --- RideLogRepo ---

func TestRideLogRepo_CreateAndListByUser(t *testing.T) {
	pool := testPool(t)
	userRepo := postgres.NewUserRepo(pool)
	rideLogRepo := postgres.NewRideLogRepo(pool)

	routeID := seedRouteID(t, pool)

	u, _ := community.NewUser("Rider C", uniqueEmail(t))
	_ = userRepo.Create(u)
	t.Cleanup(func() { _ = userRepo.Delete(u.ID) })

	rl, err := community.NewRideLog(u.ID, routeID, int(time.Now().Unix()), 3600)
	if err != nil {
		t.Fatalf("NewRideLog: %v", err)
	}
	if err := rideLogRepo.Create(rl); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = rideLogRepo.Delete(rl.ID) })

	list, err := rideLogRepo.ListByUser(u.ID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("expected at least one ride log")
	}
	if list[0].ID != rl.ID {
		t.Errorf("ID: want %q, got %q", rl.ID, list[0].ID)
	}
}

func TestRideLogRepo_WithGPXTrack(t *testing.T) {
	pool := testPool(t)
	userRepo := postgres.NewUserRepo(pool)
	rideLogRepo := postgres.NewRideLogRepo(pool)

	routeID := seedRouteID(t, pool)
	u, _ := community.NewUser("Rider D", uniqueEmail(t))
	_ = userRepo.Create(u)
	t.Cleanup(func() { _ = userRepo.Delete(u.ID) })

	rl, _ := community.NewRideLog(u.ID, routeID, int(time.Now().Unix()), 1800)
	rl.SetGPXTrack([][3]float64{
		{139.6917, 35.6895, 10},
		{139.7000, 35.6950, 20},
	})
	if err := rideLogRepo.Create(rl); err != nil {
		t.Fatalf("Create with GPX: %v", err)
	}
	t.Cleanup(func() { _ = rideLogRepo.Delete(rl.ID) })

	got, err := rideLogRepo.GetByID(rl.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(got.GPXTrack) == 0 {
		t.Error("expected GPX track to be stored and retrieved")
	}
}

// seedRouteID returns the ID of any published route from the seed data.
func seedRouteID(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`SELECT id FROM routes.route WHERE status = 'published' LIMIT 1`).Scan(&id)
	if err != nil {
		t.Skipf("no published route available for FK test: %v", err)
	}
	return id
}

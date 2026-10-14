package app_test

import (
	"testing"
	"time"

	"komorebi/internal/app"
	"komorebi/internal/domain/community"
)

// --- stub repos ---

type stubContribRepo struct{ items []*community.Contribution }

func (s *stubContribRepo) Create(c *community.Contribution) error {
	s.items = append(s.items, c)
	return nil
}
func (s *stubContribRepo) GetByID(id string) (*community.Contribution, error) {
	for _, c := range s.items {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, app.ErrUserNotFound
}
func (s *stubContribRepo) Update(c *community.Contribution) error { return nil }
func (s *stubContribRepo) Delete(id string) error                 { return nil }

type stubReviewRepo struct{ items []*community.Review }

func (s *stubReviewRepo) Create(r *community.Review) error {
	s.items = append(s.items, r)
	return nil
}
func (s *stubReviewRepo) GetByID(id string) (*community.Review, error) { return nil, nil }
func (s *stubReviewRepo) ListByRoute(routeID string) ([]*community.Review, error) {
	var out []*community.Review
	for _, r := range s.items {
		if r.RouteID == routeID {
			out = append(out, r)
		}
	}
	return out, nil
}
func (s *stubReviewRepo) Delete(id string) error { return nil }

type stubRideLogRepo struct{ items []*community.RideLog }

func (s *stubRideLogRepo) Create(rl *community.RideLog) error {
	s.items = append(s.items, rl)
	return nil
}
func (s *stubRideLogRepo) GetByID(id string) (*community.RideLog, error) { return nil, nil }
func (s *stubRideLogRepo) ListByUser(userID string) ([]*community.RideLog, error) {
	var out []*community.RideLog
	for _, rl := range s.items {
		if rl.UserID == userID {
			out = append(out, rl)
		}
	}
	return out, nil
}
func (s *stubRideLogRepo) ListByRoute(routeID string) ([]*community.RideLog, error) {
	return nil, nil
}
func (s *stubRideLogRepo) Delete(id string) error { return nil }

// --- tests ---

func newTestCommunitySvc() *app.CommunityService {
	return app.NewCommunityService(
		&stubContribRepo{},
		&stubReviewRepo{},
		&stubRideLogRepo{},
	)
}

func TestCommunityService_SubmitContribution(t *testing.T) {
	svc := newTestCommunitySvc()
	c, err := svc.SubmitContribution("user-1", [][3]float64{{139.0, 35.0, 0}}, nil)
	if err != nil {
		t.Fatalf("SubmitContribution: %v", err)
	}
	if c.UserID != "user-1" {
		t.Errorf("UserID: want user-1, got %q", c.UserID)
	}
	if c.Status != community.StatusPending {
		t.Errorf("Status: want pending, got %v", c.Status)
	}
}

func TestCommunityService_AddReview_Valid(t *testing.T) {
	svc := newTestCommunitySvc()
	rev, err := svc.AddReview("user-1", "route-1", 5, "Excellent!")
	if err != nil {
		t.Fatalf("AddReview: %v", err)
	}
	if rev.Rating != 5 {
		t.Errorf("Rating: want 5, got %d", rev.Rating)
	}
}

func TestCommunityService_AddReview_InvalidRating(t *testing.T) {
	svc := newTestCommunitySvc()
	_, err := svc.AddReview("user-1", "route-1", 0, "bad")
	if err == nil {
		t.Fatal("expected error for rating 0")
	}
}

func TestCommunityService_ListReviews(t *testing.T) {
	svc := newTestCommunitySvc()
	_, _ = svc.AddReview("user-1", "route-A", 4, "Good")
	_, _ = svc.AddReview("user-2", "route-A", 3, "OK")
	_, _ = svc.AddReview("user-1", "route-B", 5, "Amazing")

	reviews, err := svc.ListReviews("route-A")
	if err != nil {
		t.Fatalf("ListReviews: %v", err)
	}
	if len(reviews) != 2 {
		t.Errorf("expected 2 reviews for route-A, got %d", len(reviews))
	}
}

func TestCommunityService_LogRide(t *testing.T) {
	svc := newTestCommunitySvc()
	rl, err := svc.LogRide("user-1", "route-1", int(time.Now().Unix()), 3600, nil)
	if err != nil {
		t.Fatalf("LogRide: %v", err)
	}
	if rl.UserID != "user-1" {
		t.Errorf("UserID: want user-1, got %q", rl.UserID)
	}
}

func TestCommunityService_ListUserRideLogs(t *testing.T) {
	svc := newTestCommunitySvc()
	now := int(time.Now().Unix())
	_, _ = svc.LogRide("user-1", "route-1", now, 1800, nil)
	_, _ = svc.LogRide("user-1", "route-2", now, 3600, nil)
	_, _ = svc.LogRide("user-2", "route-1", now, 900, nil)

	logs, err := svc.ListUserRideLogs("user-1")
	if err != nil {
		t.Fatalf("ListUserRideLogs: %v", err)
	}
	if len(logs) != 2 {
		t.Errorf("expected 2 logs for user-1, got %d", len(logs))
	}
}

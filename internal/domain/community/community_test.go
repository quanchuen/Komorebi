package community_test

import (
	"testing"

	"github.com/cyclist-map/cyclist-map/internal/domain/community"
)

// User tests

func TestNewUser_Valid(t *testing.T) {
	u, err := community.NewUser("Yuki Tanaka", "yuki@example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if u.ID == "" {
		t.Error("expected non-empty ID")
	}
	if u.DisplayName != "Yuki Tanaka" {
		t.Errorf("expected DisplayName 'Yuki Tanaka', got %q", u.DisplayName)
	}
	if u.Email != "yuki@example.com" {
		t.Errorf("expected Email 'yuki@example.com', got %q", u.Email)
	}
	if u.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestNewUser_EmptyName(t *testing.T) {
	_, err := community.NewUser("", "yuki@example.com")
	if err != community.ErrEmptyDisplayName {
		t.Errorf("expected ErrEmptyDisplayName, got %v", err)
	}
}

// Review tests

func TestNewReview_Valid(t *testing.T) {
	r, err := community.NewReview("user-1", "route-1", 4, "Great ride!")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if r.ID == "" {
		t.Error("expected non-empty ID")
	}
	if r.Rating != 4 {
		t.Errorf("expected rating 4, got %d", r.Rating)
	}
}

func TestNewReview_RatingZero(t *testing.T) {
	_, err := community.NewReview("user-1", "route-1", 0, "Bad")
	if err != community.ErrInvalidRating {
		t.Errorf("expected ErrInvalidRating for rating 0, got %v", err)
	}
}

func TestNewReview_RatingSix(t *testing.T) {
	_, err := community.NewReview("user-1", "route-1", 6, "Too high")
	if err != community.ErrInvalidRating {
		t.Errorf("expected ErrInvalidRating for rating 6, got %v", err)
	}
}

// RideLog tests

func TestNewRideLog(t *testing.T) {
	rl, err := community.NewRideLog("user-1", "route-1", 1744300000, 7200)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if rl.ID == "" {
		t.Error("expected non-empty ID")
	}
	if rl.DurationS != 7200 {
		t.Errorf("expected DurationS 7200, got %d", rl.DurationS)
	}
}

// Contribution tests

func TestNewContribution(t *testing.T) {
	c := community.NewContribution("user-1", [][3]float64{{35.0, 139.0, 100.0}}, map[string]any{"surface": "gravel"})
	if c.ID == "" {
		t.Error("expected non-empty ID")
	}
	if c.Status != community.StatusPending {
		t.Errorf("expected Pending, got %v", c.Status)
	}
}

func TestContribution_Approve(t *testing.T) {
	c := community.NewContribution("user-1", nil, nil)
	if err := c.Approve("looks good"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if c.Status != community.StatusApproved {
		t.Errorf("expected Approved, got %v", c.Status)
	}
	if c.ModeratorNotes != "looks good" {
		t.Errorf("unexpected notes: %q", c.ModeratorNotes)
	}
}

func TestContribution_Reject(t *testing.T) {
	c := community.NewContribution("user-1", nil, nil)
	if err := c.Reject("needs work"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if c.Status != community.StatusRejected {
		t.Errorf("expected Rejected, got %v", c.Status)
	}
}

func TestContribution_ApproveNonPendingError(t *testing.T) {
	c := community.NewContribution("user-1", nil, nil)
	_ = c.Approve("ok")
	if err := c.Approve("again"); err != community.ErrNotPending {
		t.Errorf("expected ErrNotPending, got %v", err)
	}
}

func TestContribution_RejectNonPendingError(t *testing.T) {
	c := community.NewContribution("user-1", nil, nil)
	_ = c.Reject("no")
	if err := c.Reject("again"); err != community.ErrNotPending {
		t.Errorf("expected ErrNotPending, got %v", err)
	}
}

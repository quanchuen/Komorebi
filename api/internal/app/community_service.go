package app

import (
	"komorebi/internal/domain/community"
)

// CommunityService orchestrates community use cases.
type CommunityService struct {
	contributions community.ContributionRepository
	reviews       community.ReviewRepository
	rideLogs      community.RideLogRepository
}

// NewCommunityService creates a CommunityService.
func NewCommunityService(
	contributions community.ContributionRepository,
	reviews community.ReviewRepository,
	rideLogs community.RideLogRepository,
) *CommunityService {
	return &CommunityService{
		contributions: contributions,
		reviews:       reviews,
		rideLogs:      rideLogs,
	}
}

// SubmitContribution creates a new pending Contribution and persists it.
func (s *CommunityService) SubmitContribution(userID string, geometry [][3]float64, metadata map[string]any) (*community.Contribution, error) {
	c := community.NewContribution(userID, geometry, metadata)
	if err := s.contributions.Create(c); err != nil {
		return nil, err
	}
	return c, nil
}

// AddReview creates and persists a new Review.
func (s *CommunityService) AddReview(userID, routeID string, rating int, body string) (*community.Review, error) {
	rev, err := community.NewReview(userID, routeID, rating, body)
	if err != nil {
		return nil, err
	}
	if err := s.reviews.Create(rev); err != nil {
		return nil, err
	}
	return rev, nil
}

// ListReviews returns all reviews for a given route.
func (s *CommunityService) ListReviews(routeID string) ([]*community.Review, error) {
	return s.reviews.ListByRoute(routeID)
}

// LogRide creates and persists a RideLog, attaching a GPX track when provided.
func (s *CommunityService) LogRide(userID, routeID string, riddenAt, durationS int, gpxTrack [][3]float64) (*community.RideLog, error) {
	rl, err := community.NewRideLog(userID, routeID, riddenAt, durationS)
	if err != nil {
		return nil, err
	}
	if len(gpxTrack) > 0 {
		rl.SetGPXTrack(gpxTrack)
	}
	if err := s.rideLogs.Create(rl); err != nil {
		return nil, err
	}
	return rl, nil
}

// ListUserRideLogs returns all ride logs for a given user.
func (s *CommunityService) ListUserRideLogs(userID string) ([]*community.RideLog, error) {
	return s.rideLogs.ListByUser(userID)
}

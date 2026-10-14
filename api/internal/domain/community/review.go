package community

import (
	"errors"
	"time"
)

// ErrInvalidRating is returned when a review rating is not between 1 and 5.
var ErrInvalidRating = errors.New("community: rating must be between 1 and 5")

// Review holds a user's rating and comment for a route.
type Review struct {
	ID        string
	UserID    string
	RouteID   string
	Rating    int
	Body      string
	CreatedAt time.Time
}

// NewReview creates a Review with a validated rating.
func NewReview(userID, routeID string, rating int, body string) (*Review, error) {
	if rating < 1 || rating > 5 {
		return nil, ErrInvalidRating
	}
	return &Review{
		ID:        newID(),
		UserID:    userID,
		RouteID:   routeID,
		Rating:    rating,
		Body:      body,
		CreatedAt: time.Now().UTC(),
	}, nil
}

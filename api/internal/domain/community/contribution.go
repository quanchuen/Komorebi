package community

import (
	"errors"
	"time"
)

// ContributionStatus tracks moderation state.
type ContributionStatus string

const (
	StatusPending  ContributionStatus = "pending"
	StatusApproved ContributionStatus = "approved"
	StatusRejected ContributionStatus = "rejected"
)

// ErrNotPending is returned when trying to approve/reject a non-pending contribution.
var ErrNotPending = errors.New("community: contribution is not in pending state")

// Contribution is a user-submitted route geometry awaiting moderation.
type Contribution struct {
	ID             string
	UserID         string
	RouteGeometry  [][3]float64
	Metadata       map[string]any
	Status         ContributionStatus
	ModeratorNotes string
	SubmittedAt    time.Time
}

// NewContribution creates a Contribution in Pending status.
func NewContribution(userID string, geometry [][3]float64, metadata map[string]any) *Contribution {
	return &Contribution{
		ID:            newID(),
		UserID:        userID,
		RouteGeometry: geometry,
		Metadata:      metadata,
		Status:        StatusPending,
		SubmittedAt:   time.Now().UTC(),
	}
}

// Approve moves the contribution to Approved and records moderator notes.
func (c *Contribution) Approve(notes string) error {
	if c.Status != StatusPending {
		return ErrNotPending
	}
	c.Status = StatusApproved
	c.ModeratorNotes = notes
	return nil
}

// Reject moves the contribution to Rejected and records moderator notes.
func (c *Contribution) Reject(notes string) error {
	if c.Status != StatusPending {
		return ErrNotPending
	}
	c.Status = StatusRejected
	c.ModeratorNotes = notes
	return nil
}

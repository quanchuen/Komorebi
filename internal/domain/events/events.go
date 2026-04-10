package events

// Event is the base interface for all domain events.
type Event interface {
	EventName() string
}

// RouteCreated is emitted when a new route aggregate is created.
type RouteCreated struct{ RouteID string }

func (e RouteCreated) EventName() string { return "route.created" }

// RoutePublished is emitted when a route transitions from Draft to Published.
type RoutePublished struct{ RouteID string }

func (e RoutePublished) EventName() string { return "route.published" }

// RouteArchived is emitted when a route transitions from Published to Archived.
type RouteArchived struct{ RouteID string }

func (e RouteArchived) EventName() string { return "route.archived" }

// ContributionApproved is emitted when a moderator approves a community contribution.
type ContributionApproved struct {
	ContributionID string
	RouteID        string
}

func (e ContributionApproved) EventName() string { return "contribution.approved" }

// RideLogCreated is emitted when a user logs a completed ride.
type RideLogCreated struct {
	RideLogID   string
	UserID      string
	RouteID     string
	HasGPXTrack bool
}

func (e RideLogCreated) EventName() string { return "ridelog.created" }

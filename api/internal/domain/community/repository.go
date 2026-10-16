package community

// UserRepository defines persistence operations for User aggregates.
type UserRepository interface {
	Create(u *User, passwordHash string) error
	GetByID(id string) (*User, error)
}

// ReviewRepository defines persistence for route reviews.
type ReviewRepository interface {
	Create(r *Review) error
	ListByRoute(routeID string) ([]*Review, error)
}

// RideLogRepository defines persistence for ride logs.
type RideLogRepository interface {
	Create(rl *RideLog) error
	ListByUser(userID string) ([]*RideLog, error)
}

// ContributionRepository defines persistence for community contributions.
type ContributionRepository interface {
	Create(c *Contribution) error
}

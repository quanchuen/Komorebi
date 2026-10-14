package community

// UserRepository defines persistence operations for User aggregates.
type UserRepository interface {
	Create(u *User) error
	GetByID(id string) (*User, error)
	Update(u *User) error
	Delete(id string) error
}

// ReviewRepository defines persistence for route reviews.
type ReviewRepository interface {
	Create(r *Review) error
	GetByID(id string) (*Review, error)
	ListByRoute(routeID string) ([]*Review, error)
	Delete(id string) error
}

// RideLogRepository defines persistence for ride logs.
type RideLogRepository interface {
	Create(rl *RideLog) error
	GetByID(id string) (*RideLog, error)
	ListByUser(userID string) ([]*RideLog, error)
	ListByRoute(routeID string) ([]*RideLog, error)
	Delete(id string) error
}

// ContributionRepository defines persistence for community contributions.
type ContributionRepository interface {
	Create(c *Contribution) error
	GetByID(id string) (*Contribution, error)
	Update(c *Contribution) error
	Delete(id string) error
}

package plan

// Repository defines persistence operations for RoutePlan aggregates.
type Repository interface {
	Create(p *RoutePlan) error
	GetByID(id string) (*RoutePlan, error)
	Update(p *RoutePlan) error
	Delete(id string) error
}

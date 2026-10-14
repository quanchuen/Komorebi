package plan

// TaskStatus tracks the resolution state of a plan task.
type TaskStatus string

const (
	TaskUnresolved TaskStatus = "unresolved"
	TaskMatched    TaskStatus = "matched"
	TaskCompleted  TaskStatus = "completed"
)

// PlanTask is an activity or errand associated with a route plan.
type PlanTask struct {
	ID              string
	Description     string
	Hashtag         string
	Status          TaskStatus
	ResolvedVenueID string
}

package plan_test

import (
	"testing"

	"github.com/cyclist-map/cyclist-map/internal/domain/plan"
)

func TestNewRoutePlan(t *testing.T) {
	p := plan.NewRoutePlan("user-42")
	if p.ID == "" {
		t.Error("expected non-empty ID")
	}
	if p.UserID != "user-42" {
		t.Errorf("expected userID 'user-42', got %q", p.UserID)
	}
	if p.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestRoutePlan_AddStop(t *testing.T) {
	p := plan.NewRoutePlan("user-1")
	sp := plan.StopPoint{
		ID:          "stop-1",
		Lat:         35.6762,
		Lon:         139.6503,
		Type:        plan.StopManual,
		SortOrder:   0,
		ResolvedName: "Tokyo Station",
	}
	p.AddStop(sp)
	if len(p.Stops) != 1 {
		t.Errorf("expected 1 stop, got %d", len(p.Stops))
	}
}

func TestRoutePlan_AddTask(t *testing.T) {
	p := plan.NewRoutePlan("user-1")
	task := plan.PlanTask{
		ID:          "task-1",
		Description: "Buy sports drink",
		Hashtag:     "#konbini",
		Status:      plan.TaskUnresolved,
	}
	p.AddTask(task)
	if len(p.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(p.Tasks))
	}
}

func TestRoutePlan_ValidateLessThanTwoStops(t *testing.T) {
	p := plan.NewRoutePlan("user-1")
	p.AddStop(plan.StopPoint{ID: "s1", Type: plan.StopManual})
	if err := p.Validate(); err == nil {
		t.Error("expected error for less than 2 stops, got nil")
	}
}

func TestRoutePlan_ValidateWithTwoStops(t *testing.T) {
	p := plan.NewRoutePlan("user-1")
	p.AddStop(plan.StopPoint{ID: "s1", Type: plan.StopManual})
	p.AddStop(plan.StopPoint{ID: "s2", Type: plan.StopManual})
	if err := p.Validate(); err != nil {
		t.Errorf("expected no error with 2 stops, got %v", err)
	}
}

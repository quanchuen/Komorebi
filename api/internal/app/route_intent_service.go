package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"komorebi/internal/domain/plan"
)

// ErrIntentUnavailable is returned when no intent adapter is configured
// (for example, no LLM API key). Structured routing remains fully usable.
var ErrIntentUnavailable = errors.New("intent: natural-language interpretation is not configured")

// ErrIntentTextInvalid is returned for empty or oversized intent text.
var ErrIntentTextInvalid = errors.New("intent: text must be 1–500 characters")

// IntentAdapter converts free text into a schema-validated RouteIntent.
// Implementations must not resolve places, produce geometry, or call the
// routing engine (ADR 0003).
type IntentAdapter interface {
	Interpret(ctx context.Context, text string) (*plan.RouteIntent, error)
}

// RouteIntentResult pairs the interpreted intent with the deterministic
// application of its supported parts onto routing preferences.
type RouteIntentResult struct {
	Intent      plan.RouteIntent
	Preferences plan.Preferences
	// Applied names the preference fields the intent changed.
	Applied []string
	// Unsupported names schema-valid constraints the routing pipeline cannot
	// enforce yet; they are surfaced instead of silently dropped.
	Unsupported []string
}

// RouteIntentService orchestrates the LLM intent adapter and applies its
// output deterministically. The adapter may be nil, in which case the
// endpoint degrades gracefully (a model outage never removes structured
// route planning).
type RouteIntentService struct {
	adapter IntentAdapter
}

func NewRouteIntentService(adapter IntentAdapter) *RouteIntentService {
	return &RouteIntentService{adapter: adapter}
}

// InterpretIntent interprets text against the rider's current preferences.
// Unexpressed preferences keep their base values.
func (s *RouteIntentService) InterpretIntent(ctx context.Context, text string, base plan.Preferences) (*RouteIntentResult, error) {
	if s.adapter == nil {
		return nil, ErrIntentUnavailable
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || utf8.RuneCountInString(trimmed) > plan.MaxIntentTextLen {
		return nil, ErrIntentTextInvalid
	}

	intent, err := s.adapter.Interpret(ctx, trimmed)
	if err != nil {
		return nil, fmt.Errorf("intent: interpret: %w", err)
	}
	intent.OriginalText = trimmed
	intent.Normalize()

	result := &RouteIntentResult{Intent: *intent, Preferences: base}
	if v := intent.Preferences.Shade; v != nil {
		result.Preferences.ShadeWeight = *v
		result.Applied = append(result.Applied, "shade")
	}
	if v := intent.Preferences.Greenery; v != nil {
		result.Preferences.GreeneryWeight = *v
		result.Applied = append(result.Applied, "greenery")
	}
	if v := intent.Preferences.Wind; v != nil {
		result.Preferences.WindWeight = *v
		result.Applied = append(result.Applied, "wind")
	}
	// Constraints are schema-representable but not enforceable by the current
	// generate/validate pipeline (ADR 0004). Report them as unsupported so the
	// UI never labels a route as satisfying them.
	if intent.Constraints.MaxDetourM != nil {
		result.Unsupported = append(result.Unsupported, "max_detour_m")
	}
	if intent.Constraints.MaxGradePercent != nil {
		result.Unsupported = append(result.Unsupported, "max_grade_percent")
	}
	return result, nil
}

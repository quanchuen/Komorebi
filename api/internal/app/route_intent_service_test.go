package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"komorebi/internal/domain/plan"
)

type stubIntentAdapter struct {
	intent *plan.RouteIntent
	err    error
	gotTxt string
}

func (s *stubIntentAdapter) Interpret(_ context.Context, text string) (*plan.RouteIntent, error) {
	s.gotTxt = text
	if s.err != nil {
		return nil, s.err
	}
	// Return a copy so the service can mutate freely.
	intent := *s.intent
	return &intent, nil
}

func f(v float64) *float64 { return &v }

func TestInterpretIntentUnavailableWithoutAdapter(t *testing.T) {
	svc := NewRouteIntentService(nil)
	_, err := svc.InterpretIntent(context.Background(), "shady ride", plan.Preferences{})
	if !errors.Is(err, ErrIntentUnavailable) {
		t.Fatalf("expected ErrIntentUnavailable, got %v", err)
	}
}

func TestInterpretIntentRejectsInvalidText(t *testing.T) {
	svc := NewRouteIntentService(&stubIntentAdapter{intent: &plan.RouteIntent{}})
	for _, text := range []string{"", "   ", strings.Repeat("x", plan.MaxIntentTextLen+1)} {
		if _, err := svc.InterpretIntent(context.Background(), text, plan.Preferences{}); !errors.Is(err, ErrIntentTextInvalid) {
			t.Fatalf("text %q: expected ErrIntentTextInvalid, got %v", text, err)
		}
	}
}

func TestInterpretIntentLimitCountsCharactersNotBytes(t *testing.T) {
	svc := NewRouteIntentService(&stubIntentAdapter{intent: &plan.RouteIntent{}})
	// 500 three-byte characters is exactly the limit and must be accepted.
	text := strings.Repeat("木", plan.MaxIntentTextLen)
	if _, err := svc.InterpretIntent(context.Background(), text, plan.Preferences{}); err != nil {
		t.Fatalf("expected %d-character text accepted, got %v", plan.MaxIntentTextLen, err)
	}
}

func TestInterpretIntentWrapsAdapterError(t *testing.T) {
	svc := NewRouteIntentService(&stubIntentAdapter{err: errors.New("boom")})
	if _, err := svc.InterpretIntent(context.Background(), "shady ride", plan.Preferences{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestInterpretIntentAppliesExpressedPreferencesOnly(t *testing.T) {
	adapter := &stubIntentAdapter{intent: &plan.RouteIntent{
		Summary:     "Shady, wind-sheltered ride",
		Preferences: plan.RouteIntentPreferences{Shade: f(0.9), Wind: f(0.8)},
	}}
	svc := NewRouteIntentService(adapter)

	base := plan.Preferences{ShadeWeight: 0.5, GreeneryWeight: 0.4, WindWeight: 0.5}
	res, err := svc.InterpretIntent(context.Background(), "  lots of shade, out of the wind  ", base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adapter.gotTxt != "lots of shade, out of the wind" {
		t.Fatalf("adapter got untrimmed text %q", adapter.gotTxt)
	}
	if res.Preferences.ShadeWeight != 0.9 || res.Preferences.WindWeight != 0.8 {
		t.Fatalf("expressed preferences not applied: %+v", res.Preferences)
	}
	if res.Preferences.GreeneryWeight != 0.4 {
		t.Fatalf("unexpressed greenery changed: %+v", res.Preferences)
	}
	if len(res.Applied) != 2 || res.Applied[0] != "shade" || res.Applied[1] != "wind" {
		t.Fatalf("unexpected applied list: %v", res.Applied)
	}
	if res.Intent.OriginalText != "lots of shade, out of the wind" {
		t.Fatalf("original text not recorded: %q", res.Intent.OriginalText)
	}
	if res.Intent.SchemaVersion != plan.RouteIntentSchemaVersion {
		t.Fatalf("schema version not stamped: %q", res.Intent.SchemaVersion)
	}
}

func TestInterpretIntentClampsWeights(t *testing.T) {
	adapter := &stubIntentAdapter{intent: &plan.RouteIntent{
		Preferences: plan.RouteIntentPreferences{Shade: f(1.7), Greenery: f(-0.3)},
	}}
	svc := NewRouteIntentService(adapter)

	res, err := svc.InterpretIntent(context.Background(), "extreme values", plan.Preferences{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Preferences.ShadeWeight != 1 {
		t.Fatalf("shade not clamped to 1: %v", res.Preferences.ShadeWeight)
	}
	if res.Preferences.GreeneryWeight != 0 {
		t.Fatalf("greenery not clamped to 0: %v", res.Preferences.GreeneryWeight)
	}
}

func TestInterpretIntentReportsUnsupportedConstraints(t *testing.T) {
	adapter := &stubIntentAdapter{intent: &plan.RouteIntent{
		Constraints:     plan.RouteIntentConstraints{MaxDetourM: f(2000), MaxGradePercent: f(8)},
		UnresolvedTerms: []string{"stop by a konbini"},
	}}
	svc := NewRouteIntentService(adapter)

	res, err := svc.InterpretIntent(context.Background(), "flat, short detour, konbini stop", plan.Preferences{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Unsupported) != 2 || res.Unsupported[0] != "max_detour_m" || res.Unsupported[1] != "max_grade_percent" {
		t.Fatalf("unexpected unsupported list: %v", res.Unsupported)
	}
	if len(res.Intent.UnresolvedTerms) != 1 || res.Intent.UnresolvedTerms[0] != "stop by a konbini" {
		t.Fatalf("unresolved terms lost: %v", res.Intent.UnresolvedTerms)
	}
	if len(res.Applied) != 0 {
		t.Fatalf("nothing should be applied: %v", res.Applied)
	}
}

func TestInterpretIntentDropsNonPositiveConstraints(t *testing.T) {
	adapter := &stubIntentAdapter{intent: &plan.RouteIntent{
		Constraints: plan.RouteIntentConstraints{MaxDetourM: f(-5), MaxGradePercent: f(0)},
	}}
	svc := NewRouteIntentService(adapter)

	res, err := svc.InterpretIntent(context.Background(), "weird numbers", plan.Preferences{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Intent.Constraints.MaxDetourM != nil || res.Intent.Constraints.MaxGradePercent != nil {
		t.Fatalf("non-positive constraints kept: %+v", res.Intent.Constraints)
	}
	if len(res.Unsupported) != 0 {
		t.Fatalf("dropped constraints must not be reported: %v", res.Unsupported)
	}
}

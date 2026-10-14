package environment

import "testing"

func TestShadeColor(t *testing.T) {
	tests := []struct {
		shade float64
		want  string
	}{
		{0, "#ffd700"},  // full sun → gold
		{1, "#1e3a8a"},  // full shade → deep blue
		{-1, "#ffd700"}, // clamp below 0 → gold
		{2, "#1e3a8a"},  // clamp above 1 → deep blue
	}
	for _, tc := range tests {
		got := ShadeColor(tc.shade)
		if got != tc.want {
			t.Errorf("ShadeColor(%v) = %q, want %q", tc.shade, got, tc.want)
		}
	}
}

func TestWindColor(t *testing.T) {
	// wind_benefit = +1 → pure tailwind → green
	if got := WindColor(1); got != "#22c55e" {
		t.Errorf("WindColor(1) = %q, want #22c55e", got)
	}
	// wind_benefit = -1 → pure headwind → red
	if got := WindColor(-1); got != "#ef4444" {
		t.Errorf("WindColor(-1) = %q, want #ef4444", got)
	}
	// wind_benefit = 0 → calm/crosswind → neutral slate midpoint,
	// matching the diverging scale in web/src/lib/utils/conditionColors.ts
	if got := WindColor(0); got != "#94a3b8" {
		t.Errorf("WindColor(0) = %q, want #94a3b8", got)
	}
}

func TestRainColor(t *testing.T) {
	// dry → cyan (not white: dry segments must stay visible on the pale basemap)
	if got := RainColor(0); got != "#0891b2" {
		t.Errorf("RainColor(0) = %q, want #0891b2", got)
	}
	if got := RainColor(1); got != "#581b87" {
		t.Errorf("RainColor(1) = %q, want #581b87", got)
	}
}

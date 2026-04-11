package environment

import "testing"

func TestShadeColor(t *testing.T) {
	tests := []struct {
		shade float64
		want  string
	}{
		{0, "#eab308"},  // full sun → yellow
		{1, "#1e3a8a"},  // full shade → deep blue
		{-1, "#eab308"}, // clamp below 0 → yellow
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
}

func TestRainColor(t *testing.T) {
	if got := RainColor(0); got != "#f8fafc" {
		t.Errorf("RainColor(0) = %q, want #f8fafc", got)
	}
	if got := RainColor(1); got != "#7c3aed" {
		t.Errorf("RainColor(1) = %q, want #7c3aed", got)
	}
}

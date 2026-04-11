package environment_test

import (
	"math"
	"testing"

	"github.com/cyclist-map/cyclist-map/internal/domain/environment"
)

func TestWindBenefit(t *testing.T) {
	cases := []struct {
		windFrom   float64
		routeBear  float64
		wantApprox float64
	}{
		// Wind from south (180°), riding north (0°) → tailwind
		{180, 0, 1.0},
		// Wind from north (0°), riding north (0°) → headwind
		{0, 0, -1.0},
		// Wind from east (90°), riding north (0°) → crosswind
		{90, 0, 0.0},
		// Wind from SW (225°), riding NE (45°) → tailwind
		{225, 45, 1.0},
	}
	for _, tc := range cases {
		got := environment.WindBenefit(tc.windFrom, tc.routeBear)
		if math.Abs(got-tc.wantApprox) > 0.001 {
			t.Errorf("WindBenefit(%v, %v) = %v, want %v",
				tc.windFrom, tc.routeBear, got, tc.wantApprox)
		}
	}
}

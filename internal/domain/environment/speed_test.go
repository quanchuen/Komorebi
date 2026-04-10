package environment_test

import (
	"math"
	"testing"

	"github.com/cyclist-map/cyclist-map/internal/domain/environment"
)

func approxEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

func TestAdjustedSpeedKmh_Flat(t *testing.T) {
	got := environment.AdjustedSpeedKmh(0)
	if got != 15.0 {
		t.Errorf("flat grade: expected 15.0, got %v", got)
	}
}

func TestAdjustedSpeedKmh_Uphill5Percent(t *testing.T) {
	// base 15 - 5*1.5 = 15 - 7.5 = 7.5
	got := environment.AdjustedSpeedKmh(5)
	if got != 7.5 {
		t.Errorf("5%% uphill: expected 7.5, got %v", got)
	}
}

func TestAdjustedSpeedKmh_SteepUphillClamped(t *testing.T) {
	// base 15 - 15*1.5 = 15 - 22.5 = -7.5 → clamped to 4
	got := environment.AdjustedSpeedKmh(15)
	if got != 4.0 {
		t.Errorf("steep uphill: expected 4.0 (clamped), got %v", got)
	}
}

func TestAdjustedSpeedKmh_Downhill5Percent(t *testing.T) {
	// base 15 + 5*1.0 = 20
	got := environment.AdjustedSpeedKmh(-5)
	if got != 20.0 {
		t.Errorf("-5%% downhill: expected 20.0, got %v", got)
	}
}

func TestAdjustedSpeedKmh_SteepDownhillCapped(t *testing.T) {
	// base 15 + 25*1.0 = 40 → capped at 35
	got := environment.AdjustedSpeedKmh(-25)
	if got != 35.0 {
		t.Errorf("steep downhill: expected 35.0 (capped), got %v", got)
	}
}

func TestSegmentETASeconds_ThreeSignals(t *testing.T) {
	// distanceKm=2, speedKmh=15, signals=3, no green wave
	// time = (2/15)*3600 + 3*30 = 480 + 90 = 570
	got := environment.SegmentETASeconds(2.0, 15.0, 3, nil)
	if !approxEqual(got, 570.0, 0.001) {
		t.Errorf("3 signals: expected 570.0, got %v", got)
	}
}

func TestSegmentETASeconds_GreenWaveOverride(t *testing.T) {
	// distanceKm=2, green wave speed=20, signals ignored
	// time = (2/20)*3600 = 360
	gw := &environment.GreenWaveOverride{TargetSpeedKmh: 20.0}
	got := environment.SegmentETASeconds(2.0, 15.0, 5, gw)
	if !approxEqual(got, 360.0, 0.001) {
		t.Errorf("green wave: expected 360.0, got %v", got)
	}
}

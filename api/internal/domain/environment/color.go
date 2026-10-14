package environment

import "fmt"

// The scales here mirror web/src/lib/utils/conditionColors.ts — keep the two
// in sync so map gradients and server-computed colors agree.

// lerpColor linearly interpolates between two RGB colors.
// t is clamped to [0, 1]; 0 returns c0, 1 returns c1.
func lerpColor(c0, c1 [3]uint8, t float64) string {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	r := uint8(float64(c0[0]) + t*float64(int(c1[0])-int(c0[0])))
	g := uint8(float64(c0[1]) + t*float64(int(c1[1])-int(c0[1])))
	b := uint8(float64(c0[2]) + t*float64(int(c1[2])-int(c0[2])))
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

var (
	colorSun     = [3]uint8{0xff, 0xd7, 0x00} // #ffd700 — full sun (gold)
	colorDeepBlu = [3]uint8{0x1e, 0x3a, 0x8a} // #1e3a8a — full shade
	colorGreen   = [3]uint8{0x22, 0xc5, 0x5e} // #22c55e — tailwind
	colorRed     = [3]uint8{0xef, 0x44, 0x44} // #ef4444 — headwind
	colorNeutral = [3]uint8{0x94, 0xa3, 0xb8} // #94a3b8 — calm / crosswind
	colorDry     = [3]uint8{0x08, 0x91, 0xb2} // #0891b2 — dry (cyan, visible on pale basemap)
	colorPurple  = [3]uint8{0x58, 0x1b, 0x87} // #581b87 — heavy rain
	colorUVLow   = [3]uint8{0x86, 0xef, 0xac} // #86efac — low UV (green)
	colorUVHigh  = [3]uint8{0xdc, 0x26, 0x26} // #dc2626 — extreme UV (red)
)

// ShadeColor returns a hex color for shade_coverage in [0, 1].
// 0 = full sun (gold), 1 = full shade (deep blue).
func ShadeColor(shadeCoverage float64) string {
	return lerpColor(colorSun, colorDeepBlu, shadeCoverage)
}

// WindColor returns a hex color for wind_benefit in [-1, 1].
// Diverging scale through a neutral midpoint:
// -1 = strong headwind (red), 0 = calm/crosswind (slate), +1 = tailwind (green).
func WindColor(windBenefit float64) string {
	t := (windBenefit + 1) / 2 // -1 → 0, 0 → 0.5, +1 → 1
	if t < 0.5 {
		return lerpColor(colorRed, colorNeutral, t*2)
	}
	return lerpColor(colorNeutral, colorGreen, (t-0.5)*2)
}

// RainColor returns a hex color for precip intensity in [0, 1].
// 0 = dry (cyan — deliberately not white so dry segments stay visible on the
// pale basemap), 1 = heavy rain (dark purple).
func RainColor(precipNorm float64) string {
	return lerpColor(colorDry, colorPurple, precipNorm)
}

// UVColor returns a hex color for UV index (0-11+).
// 0-2 = green (low), 11+ = red (extreme).
func UVColor(uvIndex float64) string {
	t := uvIndex / 11.0 // normalise to 0-1
	return lerpColor(colorUVLow, colorUVHigh, t)
}

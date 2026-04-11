package environment

import "fmt"

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
	colorYellow  = [3]uint8{0xea, 0xb3, 0x08} // #eab308 — full sun
	colorDeepBlu = [3]uint8{0x1e, 0x3a, 0x8a} // #1e3a8a — full shade
	colorGreen   = [3]uint8{0x22, 0xc5, 0x5e} // #22c55e — tailwind
	colorRed     = [3]uint8{0xef, 0x44, 0x44} // #ef4444 — headwind
	colorWhite   = [3]uint8{0xf8, 0xfa, 0xfc} // #f8fafc — dry
	colorPurple  = [3]uint8{0x7c, 0x3a, 0xed} // #7c3aed — heavy rain
)

// ShadeColor returns a hex color for shade_coverage in [0, 1].
// 0 = full sun (yellow), 1 = full shade (deep blue).
func ShadeColor(shadeCoverage float64) string {
	return lerpColor(colorYellow, colorDeepBlu, shadeCoverage)
}

// WindColor returns a hex color for wind_benefit in [-1, 1].
// -1 = strong headwind (red), +1 = strong tailwind (green).
// Normalised to [0, 1] before interpolation (green → red).
func WindColor(windBenefit float64) string {
	// -1 → 1 (red), +1 → 0 (green) after normalisation
	t := (1 - windBenefit) / 2 // -1 → 1.0, 0 → 0.5, +1 → 0.0
	return lerpColor(colorGreen, colorRed, t)
}

// RainColor returns a hex color for precip intensity in [0, 1].
// 0 = dry (white), 1 = heavy rain (dark purple).
func RainColor(precipNorm float64) string {
	return lerpColor(colorWhite, colorPurple, precipNorm)
}

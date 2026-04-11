// web/src/lib/utils/conditionColors.ts
import type { RouteConditionSegment } from '$lib/api/types';

export type OverlayType = 'shade' | 'wind' | 'rain';

/**
 * Map a 0–1 condition value to an RGB hex color string.
 *
 * Shade:  0 = full sun (yellow #FFD700) → 1 = full shade (deep blue #1E3A8A)
 * Wind:   -1 = headwind (red #EF4444) → 0 = neutral → 1 = tailwind (green #22C55E)
 * Rain:   0 = dry (white #F8FAFC) → 1 = heavy (dark purple #4C1D95)
 */
export function conditionColor(overlay: OverlayType, value: number): string {
  const clamp = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, v));

  function lerp(a: number, b: number, t: number): number {
    return a + (b - a) * t;
  }

  function toHex(r: number, g: number, b: number): string {
    return (
      '#' +
      [r, g, b]
        .map((c) => Math.round(clamp(c, 0, 255)).toString(16).padStart(2, '0'))
        .join('')
    );
  }

  if (overlay === 'shade') {
    const t = clamp(value, 0, 1);
    // yellow [255,215,0] → deep blue [30,58,138]
    return toHex(lerp(255, 30, t), lerp(215, 58, t), lerp(0, 138, t));
  }

  if (overlay === 'wind') {
    const t = clamp((value + 1) / 2, 0, 1); // normalize -1..1 → 0..1
    if (t < 0.5) {
      // red [239,68,68] → neutral gray [148,163,184]
      const s = t / 0.5;
      return toHex(lerp(239, 148, s), lerp(68, 163, s), lerp(68, 184, s));
    } else {
      // neutral gray [148,163,184] → green [34,197,94]
      const s = (t - 0.5) / 0.5;
      return toHex(lerp(148, 34, s), lerp(163, 197, s), lerp(184, 94, s));
    }
  }

  if (overlay === 'rain') {
    const t = clamp(value, 0, 1);
    // white [248,250,252] → dark purple [76,29,149]
    return toHex(lerp(248, 76, t), lerp(250, 29, t), lerp(252, 149, t));
  }

  return '#94A3B8'; // slate-400 fallback
}

/**
 * Build a MapLibre line-gradient expression from route condition segments.
 * Returns a MapLibre expression array for use as `line-gradient`.
 */
export function buildLineGradient(
  segments: RouteConditionSegment[],
  overlay: OverlayType,
  totalDistanceM: number
): unknown[] {
  if (segments.length === 0) return ['rgb', 148, 163, 184];

  const stops: unknown[] = ['interpolate', ['linear'], ['line-progress']];

  for (const seg of segments) {
    const progress = totalDistanceM > 0 ? (seg.km * 1000) / totalDistanceM : 0;
    const value =
      overlay === 'shade' ? seg.shade : overlay === 'wind' ? seg.wind_benefit : seg.precip;
    const color = conditionColor(overlay, value);
    stops.push(Math.min(1, Math.max(0, progress)), color);
  }

  // Ensure last stop is at 1.0
  const last = stops[stops.length - 1];
  if (typeof last === 'string' && stops[stops.length - 2] !== 1) {
    stops.push(1, last);
  }

  return stops;
}

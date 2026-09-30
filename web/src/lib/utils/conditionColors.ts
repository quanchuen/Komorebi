// web/src/lib/utils/conditionColors.ts
//
// The only place besides Map.svelte where map colours are written as hex.
// Every value mirrors a token in src/app.css (route, shade-*, rain-*, wind-*)
// so chips and legends match the map. One hue per meaning: routes own blue,
// each data layer owns one ramp.
import type { RouteConditionSegment } from '$lib/api/types';

export type OverlayType = 'shade' | 'wind' | 'rain';

/** Routes — curated routes at rest and the selected route. */
export const ROUTE = '#1A73E8';
/** Unselected routes when one is selected; all routes when data layers are on. */
export const ROUTE_MUTED = '#90A4C3';
/** 1.5–2px each side of every route line. */
export const ROUTE_CASING = '#FFFFFF';
/** Hover / pressed. */
export const ROUTE_PRESSED = '#0B57D0';

/** Shade: sun → deep shade (yellow-300, amber-400, amber-600, amber-800). */
export const SHADE_RAMP = ['#FDE047', '#FBBF24', '#D97706', '#92400E'] as const;

/** Rain in mm/h: drizzle < 0.5, light 0.5–2, moderate 2–8, heavy > 8. */
export const RAIN_RAMP = {
  drizzle: '#BAE6FD',
  light: '#38BDF8',
  moderate: '#0891B2',
  heavy: '#155E75'
} as const;
export const RAIN_STEPS_MMH = [0.5, 2, 8] as const;

/** Wind along the rider's heading. Calm and crosswind carry no colour. */
export const WIND = {
  tail: '#0D9488',
  tailLight: '#5EEAD4',
  headLight: '#FCA5A5',
  head: '#DC2626'
} as const;
/** |wind_benefit| below this is calm: the road shows as road (route blue). */
export const WIND_CALM = 0.1;

/** Cycling infrastructure — neutral zinc-600 so it never competes with data. */
export const CYCLING_INFRA = '#52525B';
/** Greenery fill (green-500) — the greenery ramp's middle step. */
export const GREENERY_FILL = '#22C55E';

const clamp = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, v));

function hexToRgb(hex: string): [number, number, number] {
  const n = parseInt(hex.slice(1), 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

function toHex(rgb: [number, number, number]): string {
  return (
    '#' +
    rgb
      .map((c) =>
        Math.round(clamp(c, 0, 255))
          .toString(16)
          .padStart(2, '0')
      )
      .join('')
      .toUpperCase()
  );
}

function lerpHex(a: string, b: string, t: number): string {
  const [ar, ag, ab] = hexToRgb(a);
  const [br, bg, bb] = hexToRgb(b);
  const s = clamp(t, 0, 1);
  return toHex([ar + (br - ar) * s, ag + (bg - ag) * s, ab + (bb - ab) * s]);
}

/** Piecewise-linear interpolation across an evenly spaced ramp. */
function rampColor(ramp: readonly string[], t: number): string {
  const s = clamp(t, 0, 1) * (ramp.length - 1);
  const i = Math.min(Math.floor(s), ramp.length - 2);
  return lerpHex(ramp[i], ramp[i + 1], s - i);
}

/**
 * Map a condition value to a hex colour for route-line gradients.
 *
 * Shade: 0 = full sun (shade-sun) → 1 = full shade (shade-deep).
 * Wind:  -1 = headwind (wind-head) → calm = route blue → 1 = tailwind (wind-tail).
 * Rain:  0 = dry (route blue) → 1 = heavy (rain-heavy), through the rain ramp.
 */
export function conditionColor(overlay: OverlayType, value: number): string {
  if (overlay === 'shade') return rampColor(SHADE_RAMP, value);

  if (overlay === 'wind') {
    const v = clamp(value, -1, 1);
    if (Math.abs(v) < WIND_CALM) return ROUTE;
    if (v < 0) {
      const t = (-v - WIND_CALM) / (1 - WIND_CALM); // 0 at calm edge → 1 at -1
      return lerpHex(WIND.headLight, WIND.head, t);
    }
    const t = (v - WIND_CALM) / (1 - WIND_CALM);
    return lerpHex(WIND.tailLight, WIND.tail, t);
  }

  if (overlay === 'rain') {
    const t = clamp(value, 0, 1);
    if (t <= 0) return ROUTE;
    return rampColor([RAIN_RAMP.drizzle, RAIN_RAMP.light, RAIN_RAMP.moderate, RAIN_RAMP.heavy], t);
  }

  return ROUTE_MUTED;
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
  if (segments.length === 0) return ['to-color', ROUTE];

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

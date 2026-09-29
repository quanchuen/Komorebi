// web/src/lib/utils/conditionColors.ts
//
// The map's colour source of truth. MapLibre paint takes literal colours, so
// the ramps live here as hex; src/app.css mirrors them as semantic tokens for
// chrome (legends, swatches, readouts). Keep the two in step. Rule: one hue per
// meaning — routes own blue, each data layer owns one ramp.
import type { ExpressionSpecification } from 'maplibre-gl';
import type { RouteConditionSegment } from '$lib/api/types';

export type OverlayType = 'shade' | 'wind' | 'rain';

export const ROUTE_COLORS = {
  /** Curated routes at rest; the selected route. */
  route: '#1A73E8',
  /** Unselected routes when one is selected; all routes when data layers are on. */
  muted: '#90A4C3',
  /** 1.5–2px each side of every route line. */
  casing: '#FFFFFF',
  /** Hover / pressed. */
  pressed: '#0B57D0'
} as const;

/** Sun → deep shade (Tailwind yellow-300, amber-400/600/800). */
export const SHADE_RAMP = {
  sun: '#FDE047',
  partial: '#FBBF24',
  mostly: '#D97706',
  deep: '#92400E'
} as const;

/** Rain intensity, mm/h: < 0.5 · 0.5–2 · 2–8 · > 8. */
export const RAIN_RAMP = {
  drizzle: '#BAE6FD',
  light: '#38BDF8',
  moderate: '#0891B2',
  heavy: '#155E75'
} as const;
export const RAIN_BREAKS_MMH = { light: 0.5, moderate: 2, heavy: 8 } as const;

/** Headwind component along the rider's heading. Calm/crosswind has no colour. */
export const WIND_RAMP = {
  tail: '#0D9488',
  tailLight: '#5EEAD4',
  headLight: '#FCA5A5',
  head: '#DC2626'
} as const;

/** Chrome colours drawn inside map markers (canvas, not CSS). */
export const MARKER_COLORS = {
  surface: '#FFFFFF',
  line: '#E4E4E7',
  icon: '#18181B',
  stale: '#A1A1AA'
} as const;

/**
 * Live position fill. OPEN DECISION (spec: route blue vs near-black); this is
 * the pre-rework colour, kept until the decision lands.
 */
export const POSITION_COLOR = '#38BDF8';

/** Fully transparent: "no colour" for calm wind. */
export const NO_COLOR = 'rgba(0, 0, 0, 0)';

/** Segment `precip` is normalised mm/h ÷ 10, capped at 1 (see environment_service.go). */
export const SEGMENT_PRECIP_SCALE_MMH = 10;

function clamp(v: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(hi, v));
}

function hexToRgb(hex: string): [number, number, number] {
  const n = parseInt(hex.slice(1), 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

function toHex(rgb: number[]): string {
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

/** Piecewise-linear interpolation across evenly spaced colour stops. */
function ramp(stops: readonly string[], t: number): string {
  const x = clamp(t, 0, 1) * (stops.length - 1);
  const i = Math.min(stops.length - 2, Math.floor(x));
  const s = x - i;
  const a = hexToRgb(stops[i]);
  const b = hexToRgb(stops[i + 1]);
  return toHex(a.map((c, k) => c + (b[k] - c) * s));
}

/** Discrete rain band for an intensity in mm/h. Dry (≤ 0) has no colour. */
export function rainColorMmh(mmh: number): string {
  if (!(mmh > 0)) return NO_COLOR;
  if (mmh < RAIN_BREAKS_MMH.light) return RAIN_RAMP.drizzle;
  if (mmh < RAIN_BREAKS_MMH.moderate) return RAIN_RAMP.light;
  if (mmh < RAIN_BREAKS_MMH.heavy) return RAIN_RAMP.moderate;
  return RAIN_RAMP.heavy;
}

// |wind_benefit| below this reads as calm / crosswind and stays uncoloured.
const WIND_CALM = 0.15;

/**
 * Map a condition value to a colour.
 *
 * Shade:  0 = full sun → 1 = full shade (yellow → deep amber)
 * Wind:   -1 = headwind (red) → calm (no colour) → 1 = tailwind (teal)
 * Rain:   normalised segment precip (mm/h ÷ 10) → discrete sky/cyan bands
 */
export function conditionColor(overlay: OverlayType, value: number): string {
  if (overlay === 'shade') {
    return ramp([SHADE_RAMP.sun, SHADE_RAMP.partial, SHADE_RAMP.mostly, SHADE_RAMP.deep], value);
  }

  if (overlay === 'wind') {
    const v = clamp(value, -1, 1);
    if (Math.abs(v) < WIND_CALM) return NO_COLOR;
    const t = (Math.abs(v) - WIND_CALM) / (1 - WIND_CALM);
    return v > 0
      ? ramp([WIND_RAMP.tailLight, WIND_RAMP.tail], t)
      : ramp([WIND_RAMP.headLight, WIND_RAMP.head], t);
  }

  if (overlay === 'rain') {
    return rainColorMmh(clamp(value, 0, 1) * SEGMENT_PRECIP_SCALE_MMH);
  }

  return ROUTE_COLORS.muted;
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
  if (segments.length === 0) return ['to-color', ROUTE_COLORS.route];

  const stops: unknown[] = ['interpolate', ['linear'], ['line-progress']];

  let lastProgress = -1;
  for (const seg of segments) {
    const progress = totalDistanceM > 0 ? (seg.km * 1000) / totalDistanceM : 0;
    const p = Math.min(1, Math.max(0, progress));
    // line-progress stops must be strictly ascending.
    if (p <= lastProgress) continue;
    lastProgress = p;
    const value =
      overlay === 'shade' ? seg.shade : overlay === 'wind' ? seg.wind_benefit : seg.precip;
    stops.push(p, conditionColor(overlay, value));
  }

  // Ensure last stop is at 1.0
  const last = stops[stops.length - 1];
  if (typeof last === 'string' && lastProgress < 1) {
    stops.push(1, last);
  }

  return stops;
}

/** MapLibre fill-color for shadow cells by `shade_coverage` (0–1). */
export const SHADOW_FILL_COLOR: ExpressionSpecification = [
  'interpolate',
  ['linear'],
  ['get', 'shade_coverage'],
  0,
  SHADE_RAMP.sun,
  0.33,
  SHADE_RAMP.partial,
  0.66,
  SHADE_RAMP.mostly,
  1,
  SHADE_RAMP.deep
];

/** Faint sun wash where coverage is low, deepening with shade. */
export const SHADOW_FILL_OPACITY: ExpressionSpecification = [
  'interpolate',
  ['linear'],
  ['get', 'shade_coverage'],
  0,
  0.1,
  0.4,
  0.22,
  1,
  0.42
];

/** MapLibre fill-color for rain cells by `precip` (mm/h), discrete bands. */
export const RAIN_FILL_COLOR: ExpressionSpecification = [
  'step',
  ['get', 'precip'],
  RAIN_RAMP.drizzle,
  RAIN_BREAKS_MMH.light,
  RAIN_RAMP.light,
  RAIN_BREAKS_MMH.moderate,
  RAIN_RAMP.moderate,
  RAIN_BREAKS_MMH.heavy,
  RAIN_RAMP.heavy
];

/** Dry cells stay invisible; wetter bands read stronger. */
export const RAIN_FILL_OPACITY: ExpressionSpecification = [
  'interpolate',
  ['linear'],
  ['get', 'precip'],
  0,
  0,
  0.05,
  0.3,
  RAIN_BREAKS_MMH.moderate,
  0.4,
  RAIN_BREAKS_MMH.heavy,
  0.5
];

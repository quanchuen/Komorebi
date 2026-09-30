// web/src/lib/utils/conditions.ts
//
// Route-level readings from per-segment conditions, worded as the route card
// (frame D) and the plan rows (frame G) show them.
import type { RouteConditionSegment } from '$lib/api/types';
import { hhmm } from './format';

export interface ConditionSummary {
  /** 0–1 share of the route in shade. */
  shade: number;
  /** Mean wind benefit −1 (headwind) … 1 (tailwind). */
  wind: number;
  /** Mean wind benefit on the last third of the route. */
  windHome: number;
  /** Mean wind benefit on the first third. */
  windOut: number;
  /** Peak precipitation, mm/h scaled 0–1 as the API returns it. */
  rain: number;
  /** ETA of the first wet segment, or null when dry throughout. */
  rainFrom: Date | null;
  signals: number;
  /** ETA of the last segment (arrival), if the API sent ETAs. */
  arrival: Date | null;
}

const mean = (xs: number[]) => (xs.length ? xs.reduce((s, x) => s + x, 0) / xs.length : 0);

export function summarise(segments: RouteConditionSegment[]): ConditionSummary | null {
  if (segments.length === 0) return null;
  const third = Math.max(1, Math.floor(segments.length / 3));
  const wet = segments.find((s) => s.precip > 0.05);
  const last = segments[segments.length - 1];
  return {
    shade: mean(segments.map((s) => s.shade)),
    wind: mean(segments.map((s) => s.wind_benefit)),
    windHome: mean(segments.slice(-third).map((s) => s.wind_benefit)),
    windOut: mean(segments.slice(0, third).map((s) => s.wind_benefit)),
    rain: Math.max(...segments.map((s) => s.precip)),
    rainFrom: wet?.eta ? new Date(wet.eta) : null,
    signals: segments.reduce((s, c) => s + c.signals, 0),
    arrival: last?.eta ? new Date(last.eta) : null
  };
}

export function windWord(v: number): 'Tailwind' | 'Headwind' | 'Crosswind' {
  if (v > 0.3) return 'Tailwind';
  if (v < -0.3) return 'Headwind';
  return 'Crosswind';
}

/** Card tile: "Tailwind / home stretch", "Headwind / on the way out", "Calm / most of the way". */
export function windTile(s: ConditionSummary): { value: string; note: string } {
  if (s.windHome > 0.3) return { value: 'Tailwind', note: 'home stretch' };
  if (s.windOut > 0.3) return { value: 'Tailwind', note: 'on the way out' };
  if (s.windHome < -0.3) return { value: 'Headwind', note: 'home stretch' };
  if (s.windOut < -0.3) return { value: 'Headwind', note: 'on the way out' };
  if (Math.abs(s.wind) < 0.1) return { value: 'Calm', note: 'most of the way' };
  return { value: windWord(s.wind), note: 'most of the way' };
}

/** Card tile: "Dry / for the whole ride", "Rain / from 14:20". */
export function rainTile(s: ConditionSummary): { value: string; note: string } {
  if (s.rain <= 0.05) return { value: 'Dry', note: 'for the whole ride' };
  const level = s.rain < 0.3 ? 'Light rain' : s.rain < 0.6 ? 'Rain' : 'Heavy rain';
  return { value: level, note: s.rainFrom ? `from ${hhmm(s.rainFrom)}` : 'on the way' };
}

/** Plan row: "64% shaded · light headwind · dry". */
export function conditionsLine(s: ConditionSummary | null): string | null {
  if (!s) return null;
  const wind =
    s.wind > 0.3
      ? 'tailwind'
      : s.wind > 0.1
        ? 'light tailwind'
        : s.wind < -0.3
          ? 'headwind'
          : s.wind < -0.1
            ? 'light headwind'
            : 'calm';
  const rain = s.rain <= 0.05 ? 'dry' : s.rain < 0.3 ? 'light rain' : 'rain';
  return `${Math.round(s.shade * 100)}% shaded · ${wind} · ${rain}`;
}

/** Time chip suffix when a route is selected: "tailwind on the way back". */
export function windChipPhrase(s: ConditionSummary | null): string | null {
  if (!s) return null;
  if (s.windHome > 0.3) return 'tailwind on the way back';
  if (s.windOut > 0.3) return 'tailwind on the way out';
  if (s.windHome < -0.3) return 'headwind on the way back';
  if (s.windOut < -0.3) return 'headwind on the way out';
  return null;
}

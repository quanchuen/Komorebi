// web/src/lib/stores/weather.ts
//
// The next 24 hours at the map centre, for the time chip and the scrubber's
// rain bars. The API serves one point-in-time per call, so eight calls three
// hours apart are made and the hours between take the nearest reading.
import { writable, get } from 'svelte/store';

export interface HourSlot {
  time: Date;
  temperatureC: number | null;
  windSpeedMs: number | null;
  windBearingDeg: number | null;
  precipMmh: number | null;
}

interface PointWeather {
  temperature_c?: number;
  wind_speed_ms?: number;
  wind_bearing_deg?: number;
  precip_intensity_mmh?: number;
}

export const hourly = writable<HourSlot[]>([]);
export const weatherStatus = writable<'idle' | 'loading' | 'ok' | 'error'>('idle');

/** The scrubber's origin: the current hour, floored. */
export function timelineStart(now = new Date()): Date {
  const d = new Date(now);
  d.setMinutes(0, 0, 0);
  return d;
}

function emptySlots(start: Date): HourSlot[] {
  return Array.from({ length: 24 }, (_, i) => ({
    time: new Date(start.getTime() + i * 3_600_000),
    temperatureC: null,
    windSpeedMs: null,
    windBearingDeg: null,
    precipMmh: null
  }));
}

let lastKey = '';
let seq = 0;

/** Refresh the forecast for a map centre; a nearby centre reuses the last run. */
export async function refreshHourly(centre: { lat: number; lon: number }): Promise<void> {
  const lat = centre.lat.toFixed(2);
  const lon = centre.lon.toFixed(2);
  const key = `${lat},${lon}`;
  if (key === lastKey && get(hourly).length > 0) return;
  lastKey = key;
  const run = ++seq;
  const start = timelineStart();
  const slots = emptySlots(start);
  if (get(hourly).length === 0) hourly.set(slots);
  weatherStatus.set('loading');

  const offsets = [0, 3, 6, 9, 12, 15, 18, 21];
  const readings = await Promise.all(
    offsets.map(async (offset) => {
      const at = new Date(start.getTime() + offset * 3_600_000).toISOString();
      try {
        const res = await fetch(`/api/v1/weather/point?lat=${lat}&lon=${lon}&at=${at}`);
        if (!res.ok) return null;
        return { offset, data: (await res.json()) as PointWeather };
      } catch {
        return null;
      }
    })
  );
  if (run !== seq) return;
  const known = readings.filter((r): r is { offset: number; data: PointWeather } => r !== null);
  if (known.length === 0) {
    weatherStatus.set('error');
    hourly.set(slots);
    return;
  }
  hourly.set(
    slots.map((slot, i) => {
      const nearest = known.reduce((best, r) =>
        Math.abs(r.offset - i) < Math.abs(best.offset - i) ? r : best
      );
      const d = nearest.data;
      return {
        ...slot,
        temperatureC: d.temperature_c ?? null,
        windSpeedMs: d.wind_speed_ms ?? null,
        windBearingDeg: d.wind_bearing_deg ?? null,
        precipMmh: d.precip_intensity_mmh ?? null
      };
    })
  );
  weatherStatus.set('ok');
}

/** The slot covering an ISO time, or null outside the 24-hour window. */
export function slotAt(slots: HourSlot[], iso: string): HourSlot | null {
  const t = new Date(iso).getTime();
  let best: HourSlot | null = null;
  for (const s of slots) {
    if (!best || Math.abs(s.time.getTime() - t) < Math.abs(best.time.getTime() - t)) best = s;
  }
  return best && Math.abs(best.time.getTime() - t) <= 3_600_000 ? best : null;
}

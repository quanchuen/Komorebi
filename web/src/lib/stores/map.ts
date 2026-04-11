// web/src/lib/stores/map.ts
import { writable, derived } from 'svelte/store';
import type { Map as MapLibreMap } from 'maplibre-gl';

export type OverlayType = 'shade' | 'wind' | 'rain' | null;

// The MapLibre map instance — set once the map mounts
export const mapInstance = writable<MapLibreMap | null>(null);

// Current map viewport
export const mapBounds = writable<{ minLon: number; minLat: number; maxLon: number; maxLat: number } | null>(null);

// Active condition overlay
export const activeOverlay = writable<OverlayType>(null);

// The route ID currently highlighted on the map
export const highlightedRouteId = writable<string | null>(null);

// Departure time used for all condition queries (ISO 8601)
function defaultDeparture(): string {
  const d = new Date();
  d.setMinutes(0, 0, 0);
  return d.toISOString();
}
export const departureAt = writable<string>(defaultDeparture());

// Derived bbox string for API calls
export const bboxString = derived(mapBounds, ($b) =>
  $b ? `${$b.minLon},${$b.minLat},${$b.maxLon},${$b.maxLat}` : null
);

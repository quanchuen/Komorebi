// web/src/lib/stores/map.ts
import { writable, derived } from 'svelte/store';
import type { Map as MapLibreMap } from 'maplibre-gl';

export type OverlayType = 'shade' | 'wind' | 'rain' | null;

// Optional map layers. Data layers are off by default (progressive
// disclosure); the time-scrubbed environment layers are driven exclusively by
// the timeline-layer switch in the weather timeline (rain cells by default).
export type MapLayer = 'cycling-roads' | 'venues' | 'landuse' | 'shadows' | 'rain-cells';
export const visibleLayers = writable<Set<MapLayer>>(new Set(['rain-cells']));

// Which environment lens the weather timeline scrubs: weather (rain radar +
// rain route coloring), shade (building shadows), or sun (sun-exposure route
// coloring). Exclusive — selecting one swaps map layers and route overlay.
export type TimelineLayer = 'weather' | 'shade' | 'sun';
export const timelineLayer = writable<TimelineLayer>('weather');

// The MapLibre map instance — set once the map mounts
export const mapInstance = writable<MapLibreMap | null>(null);

// Current map viewport
export const mapBounds = writable<{
  minLon: number;
  minLat: number;
  maxLon: number;
  maxLat: number;
} | null>(null);

// Active condition overlay
export const activeOverlay = writable<OverlayType>('rain');

// The route ID currently highlighted on the map
export const highlightedRouteId = writable<string | null>(null);

// Departure time used for all condition queries (ISO 8601)
function defaultDeparture(): string {
  const d = new Date();
  d.setMinutes(0, 0, 0);
  return d.toISOString();
}
export const departureAt = writable<string>(defaultDeparture());

// The shadow-grid slice matching the departure time. The grid is keyed by
// JST hour (fixed UTC+9, no DST) and calendar month; the tile function snaps
// the month to the nearest one with data.
export const shadowSlice = derived(departureAt, ($d) => {
  const jst = new Date(new Date($d).getTime() + 9 * 3600_000);
  return { hourSlot: jst.getUTCHours(), month: jst.getUTCMonth() + 1 };
});

// Route alternatives displayed on the map
export interface RouteDisplayInfo {
  coords: [number, number][];
  selected: boolean;
  profile: string;
  color: string;
  distanceM: number;
}
export const routeDisplays = writable<RouteDisplayInfo[]>([]);

// The selected route's geometry (for the highlight line + condition gradient)
export const selectedRouteGeometry = writable<[number, number][] | null>(null);
export const selectedRouteDistanceM = writable<number>(0);

// Current foreground-navigation fix. It is deliberately not persisted.
export const liveNavigationPosition = writable<GeolocationCoordinates | null>(null);

// Derived bbox string for API calls
export const bboxString = derived(mapBounds, ($b) =>
  $b ? `${$b.minLon},${$b.minLat},${$b.maxLon},${$b.maxLat}` : null
);

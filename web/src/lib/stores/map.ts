// web/src/lib/stores/map.ts
import { writable, derived, get } from 'svelte/store';
import type { Map as MapLibreMap } from 'maplibre-gl';
import { flags } from '$lib/flags';

export type OverlayType = 'shade' | 'wind' | 'rain' | null;

// Map layers (Penpot frame C). Conditions follow the time slider — shade,
// rain, wind, greenery — and are all off at rest (frame A: basemap + curated
// routes only). Curated routes and cycling infrastructure are the always-on
// group; the design still gives each a switch.
export type MapLayer =
  | 'shadows'
  | 'rain-cells'
  | 'wind'
  | 'greenery'
  | 'curated-routes'
  | 'cycling-roads';
export const visibleLayers = writable<Set<MapLayer>>(
  new Set<MapLayer>(flags.curatedRoutes ? ['curated-routes'] : [])
);

export function toggleLayer(layer: MapLayer): void {
  visibleLayers.update((set) => {
    const next = new Set(set);
    if (next.has(layer)) next.delete(layer);
    else next.add(layer);
    return next;
  });
}

// The MapLibre map instance — set once the map mounts
export const mapInstance = writable<MapLibreMap | null>(null);

// Current map viewport
export const mapBounds = writable<{
  minLon: number;
  minLat: number;
  maxLon: number;
  maxLat: number;
} | null>(null);

export const mapCentre = derived(mapBounds, ($b) =>
  $b ? { lat: ($b.minLat + $b.maxLat) / 2, lon: ($b.minLon + $b.maxLon) / 2 } : null
);

// Which condition colours the selected route's line. The explore page derives
// it from the layer switches (wind > rain > shade); the route-detail and
// planner pages set it from their own toggle.
export const activeOverlay = writable<OverlayType>(null);

/** The overlay implied by the layer switches. */
export function overlayForLayers(layers: Set<MapLayer>): OverlayType {
  if (layers.has('wind')) return 'wind';
  if (layers.has('rain-cells')) return 'rain';
  if (layers.has('shadows')) return 'shade';
  return null;
}

// The curated route currently selected (frame D)
export const highlightedRouteId = writable<string | null>(null);
// The routes-list row under the pointer (frame I): its line lifts, the rest mute.
export const hoveredRouteId = writable<string | null>(null);

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

// Route alternatives displayed on the map while planning (frame G). Each
// carries a time pill at its midpoint; tapping a line or pill selects it.
export interface RouteDisplayInfo {
  profile: string;
  label: string;
  coords: [number, number][];
  selected: boolean;
  distanceM: number;
  durationS: number;
}
export const routeDisplays = writable<RouteDisplayInfo[]>([]);
/** Set by the map when a rider taps an alternative's line or time pill. */
export const alternativePick = writable<string | null>(null);

// The selected planner route's geometry (for the highlight line + condition gradient)
export const selectedRouteGeometry = writable<[number, number][] | null>(null);
export const selectedRouteDistanceM = writable<number>(0);

// The rider's position: the Locate button's one-off fix or the foreground
// navigation watch. Deliberately not persisted.
export const liveNavigationPosition = writable<GeolocationCoordinates | null>(null);

// Derived bbox string for API calls
export const bboxString = derived(mapBounds, ($b) =>
  $b ? `${$b.minLon},${$b.minLat},${$b.maxLon},${$b.maxLat}` : null
);

// Opt-in venue pins. Nothing shows until the rider picks a type; each type
// maps onto one or more venue categories in the tiles.
export type VenueType = 'konbini' | 'cafe' | 'water' | 'repair';
export const VENUE_TYPE_CATEGORIES: Record<VenueType, string[]> = {
  konbini: ['konbini'],
  cafe: ['cafe'],
  water: ['water'],
  repair: ['bike-repair', 'bike-shop']
};
export const visibleVenueTypes = writable<Set<VenueType>>(new Set());

// Stops of the route being planned, published by whichever planner UI is on
// the page so the map can draw start / waypoint / end markers for them.
export interface RouteStop {
  lat: number;
  lon: number;
  kind: 'start' | 'via' | 'end';
  /** 1-based number shown on waypoint markers; unused for start / end. */
  index: number;
}
export const routeStops = writable<RouteStop[]>([]);

// --- Map actions for the chrome (zoom stack, Locate) --------------------
// The controls live outside Map.svelte, so they reach MapLibre only through
// these store-level helpers.

export function zoomIn(): void {
  get(mapInstance)?.zoomIn({ duration: 300 });
}

export function zoomOut(): void {
  get(mapInstance)?.zoomOut({ duration: 300 });
}

export const locateStatus = writable<'idle' | 'locating' | 'denied' | 'unavailable'>('idle');

/** One-off position fix on a user action; centres the map on the rider. */
export function locate(): void {
  if (typeof navigator === 'undefined' || !('geolocation' in navigator)) {
    locateStatus.set('unavailable');
    return;
  }
  locateStatus.set('locating');
  navigator.geolocation.getCurrentPosition(
    ({ coords }) => {
      liveNavigationPosition.set(coords);
      locateStatus.set('idle');
      get(mapInstance)?.flyTo({
        center: [coords.longitude, coords.latitude],
        zoom: Math.max(get(mapInstance)?.getZoom() ?? 14, 14),
        duration: 800
      });
    },
    (error) => {
      locateStatus.set(error.code === error.PERMISSION_DENIED ? 'denied' : 'unavailable');
    },
    { enableHighAccuracy: true, maximumAge: 30_000, timeout: 15_000 }
  );
}

/** Padding that keeps a fitted line clear of the chrome: the 400px panel on
    the left (desktop) or the sheet along the bottom (mobile). */
export function chromePadding(mobile: boolean): {
  top: number;
  bottom: number;
  left: number;
  right: number;
} {
  if (mobile) {
    const h = typeof window === 'undefined' ? 800 : window.innerHeight;
    return { top: 120, bottom: Math.round(h * 0.62) + 24, left: 32, right: 32 };
  }
  return { top: 90, bottom: 90, left: 460, right: 90 };
}

/** Fit the map to a line, leaving room for the floating chrome. */
export function fitToLine(
  coords: [number, number][],
  padding: number | { top: number; bottom: number; left: number; right: number } = 80
): void {
  const map = get(mapInstance);
  if (!map || coords.length === 0) return;
  const lons = coords.map((c) => c[0]);
  const lats = coords.map((c) => c[1]);
  map.fitBounds(
    [
      [Math.min(...lons), Math.min(...lats)],
      [Math.max(...lons), Math.max(...lats)]
    ],
    { padding, duration: 800 }
  );
}

# SvelteKit Frontend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the SvelteKit web frontend for cyclist-map in `/web`, covering Discovery, Route Detail, and Route Planner pages with a shared MapLibre GL JS map component, dark theme, typed API client, and responsive layout (desktop panel + mobile bottom sheet).

**Tech Stack:** SvelteKit 2, TypeScript, MapLibre GL JS 4, Tailwind CSS 4, Vite.

**Service endpoints:**
- Go API: `http://localhost:8080`
- Martin tile server: `http://localhost:3000`

---

## Task 1 — SvelteKit Project Init

**Files:**
- `web/` (scaffold via `npm create svelte@latest`)
- `web/package.json`
- `web/svelte.config.js`
- `web/vite.config.ts`
- `web/tailwind.config.ts`
- `web/src/app.css`
- `web/src/app.html`

### Steps

- [ ] 1.1 Scaffold the SvelteKit project inside `/web`.

```bash
cd /Users/lug/src/cyclist-map
npm create svelte@latest web -- --template skeleton --types typescript --no-eslint --no-prettier --no-playwright --no-vitest
cd web
npm install
```

- [ ] 1.2 Install dependencies.

```bash
cd /Users/lug/src/cyclist-map/web
npm install maplibre-gl @types/maplibre-gl
npm install -D tailwindcss @tailwindcss/vite
```

- [ ] 1.3 Wire Tailwind into Vite. Replace `web/vite.config.ts`:

```typescript
// web/vite.config.ts
import { defineConfig } from 'vite';
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [tailwindcss(), sveltekit()],
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/tiles': 'http://localhost:3000'
    }
  }
});
```

- [ ] 1.4 Create `web/src/app.css` with dark theme base and Tailwind import:

```css
/* web/src/app.css */
@import 'tailwindcss';

:root {
  color-scheme: dark;
}

body {
  @apply bg-slate-900 text-slate-100 antialiased;
  margin: 0;
  padding: 0;
  height: 100dvh;
  overflow: hidden;
}

/* MapLibre canvas fills its container */
.maplibregl-map {
  width: 100%;
  height: 100%;
}
```

- [ ] 1.5 Update `web/src/app.html` to import the stylesheet and set dark background:

```html
<!doctype html>
<html lang="en" class="dark">
  <head>
    <meta charset="utf-8" />
    <link rel="icon" href="%sveltekit.assets%/favicon.png" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    %sveltekit.head%
  </head>
  <body data-sveltekit-preload-data="hover" class="bg-slate-900">
    <div style="display: contents">%sveltekit.body%</div>
  </body>
</html>
```

- [ ] 1.6 Verify the dev server starts:

```bash
cd /Users/lug/src/cyclist-map/web && npm run dev
```

Expected: Vite dev server at `http://localhost:5173` with no errors in the terminal.

---

## Task 2 — TypeScript API Client

**Files:**
- `web/src/lib/api/types.ts` (create)
- `web/src/lib/api/client.ts` (create)

### Steps

- [ ] 2.1 Create `web/src/lib/api/types.ts` with typed response shapes matching the Go API:

```typescript
// web/src/lib/api/types.ts

export type Difficulty = 'easy' | 'moderate' | 'hard' | 'expert';
export type SurfaceType = 'paved' | 'gravel' | 'dirt' | 'cobblestone';
export type RouteStatus = 'draft' | 'published' | 'archived';
export type WaypointType = 'viewpoint' | 'rest_stop' | 'water' | 'shrine' | 'konbini' | 'other';

export interface GeoPoint {
  type: 'Point';
  coordinates: [lon: number, lat: number] | [lon: number, lat: number, ele: number];
}

export interface GeoLineString {
  type: 'LineString';
  coordinates: Array<[lon: number, lat: number] | [lon: number, lat: number, ele: number]>;
}

// --- Routes ---

export interface Waypoint {
  id: string;
  routeId: string;
  geometry: GeoPoint;
  name: string;
  type: WaypointType;
  sortOrder: number;
}

export interface RouteSegment {
  id: string;
  routeId: string;
  geometry: GeoLineString;
  surfaceType: SurfaceType;
  gradePercent: number;
  segmentOrder: number;
}

export interface Route {
  id: string;
  name: string;
  description: string;
  geometry: GeoLineString;
  distanceM: number;
  elevationGainM: number;
  elevationLossM: number;
  difficulty: Difficulty;
  status: RouteStatus;
  creatorId: string;
  tags: string[];
  waypoints?: Waypoint[];
  segments?: RouteSegment[];
  createdAt: string;
  updatedAt: string;
}

export interface RouteListResponse {
  routes: Route[];
  nextCursor: string | null;
}

// --- Conditions ---

export interface GreenWaveInfo {
  speedKmh: number;
  lengthKm: number;
}

export interface RouteConditionSegment {
  km: number;
  eta: string;
  shade: number;        // 0.0–1.0
  windBenefit: number;  // -1.0 (headwind) to 1.0 (tailwind)
  precip: number;       // 0.0–1.0
  greenWave: GreenWaveInfo | null;
  signals: number;
}

export interface RouteConditionsResponse {
  routeId: string;
  departureAt: string;
  segments: RouteConditionSegment[];
}

// --- Discovery ---

export interface DiscoverNearbyParams {
  lat: number;
  lon: number;
  radiusKm: number;
}

export interface DiscoverViewportParams {
  bbox: string; // "minLon,minLat,maxLon,maxLat"
}

export interface DiscoverSuggestedParams {
  lat: number;
  lon: number;
  departureAt: string; // ISO 8601
}

// --- Routing ---

export type StopType = 'manual' | 'venue';

export interface ManualStop {
  type: 'manual';
  lat: number;
  lon: number;
}

export interface VenueStop {
  type: 'venue';
  hashtag: string;
}

export type RoutingStop = ManualStop | VenueStop;

export interface RoutingPreferences {
  shade: number;    // 0.0–1.0
  greenery: number; // 0.0–1.0
  wind: number;     // 0.0–1.0
}

export interface DirectionsRequest {
  stops: RoutingStop[];
  departureAt: string;
  speedModel: 'elevation';
  preferences: RoutingPreferences;
}

export interface DirectionsResponse {
  geometry: GeoLineString;
  distanceM: number;
  durationS: number;
  segments: RouteConditionSegment[];
}

// --- Venues ---

export interface VenueTag {
  hashtag: string;
  description: string;
  isBrand: boolean;
}

export interface Venue {
  id: string;
  osmId: number;
  geometry: GeoPoint;
  name: string;
  category: string;
  brand: string | null;
}

// --- Reviews ---

export interface Review {
  id: string;
  userId: string;
  routeId: string;
  rating: number; // 1–5
  body: string;
  createdAt: string;
}

export interface ReviewListResponse {
  reviews: Review[];
  nextCursor: string | null;
}

// --- Plans ---

export type StopPointType = 'manual' | 'venue_resolved' | 'waypoint';
export type PlanTaskStatus = 'unresolved' | 'matched' | 'completed';

export interface StopPoint {
  id: string;
  planId: string;
  geometry: GeoPoint;
  type: StopPointType;
  sortOrder: number;
  venueId: string | null;
  resolvedName: string;
}

export interface PlanTask {
  id: string;
  planId: string;
  description: string;
  hashtag: string | null;
  status: PlanTaskStatus;
  resolvedVenueId: string | null;
}

export interface RoutePlan {
  id: string;
  userId: string;
  departureAt: string;
  speedModel: 'elevation';
  shadeWeight: number;
  greeneryWeight: number;
  windWeight: number;
  stops: StopPoint[];
  tasks: PlanTask[];
  routeGeometry: GeoLineString | null;
  segments: RouteConditionSegment[];
  createdAt: string;
}
```

- [ ] 2.2 Create `web/src/lib/api/client.ts` with a typed fetch wrapper for all endpoints:

```typescript
// web/src/lib/api/client.ts
import type {
  Route,
  RouteListResponse,
  RouteConditionsResponse,
  DirectionsRequest,
  DirectionsResponse,
  ReviewListResponse,
  Review,
  RoutePlan,
  StopPoint,
  PlanTask,
  Venue,
  VenueTag,
  DiscoverNearbyParams,
  DiscoverViewportParams,
  DiscoverSuggestedParams
} from './types';

const BASE = '/api/v1';

async function get<T>(path: string, params?: Record<string, string | number>): Promise<T> {
  const url = new URL(path, 'http://localhost'); // URL for param building only
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      url.searchParams.set(k, String(v));
    }
  }
  const res = await fetch(`${BASE}${url.pathname}${url.search}`);
  if (!res.ok) throw new Error(`GET ${path} failed: ${res.status} ${res.statusText}`);
  return res.json() as Promise<T>;
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  });
  if (!res.ok) throw new Error(`POST ${path} failed: ${res.status} ${res.statusText}`);
  return res.json() as Promise<T>;
}

// --- Routes ---

export const routes = {
  list: (params?: {
    bbox?: string;
    difficulty?: string;
    tags?: string;
    cursor?: string;
  }) => get<RouteListResponse>('/routes', params as Record<string, string>),

  get: (id: string) => get<Route>(`/routes/${id}`),

  conditions: (id: string, departureAt: string, speedModel = 'elevation') =>
    get<RouteConditionsResponse>(`/routes/${id}/conditions`, { departure_at: departureAt, speed_model: speedModel })
};

// --- Discovery ---

export const discovery = {
  nearby: (p: DiscoverNearbyParams) =>
    get<RouteListResponse>('/discover/nearby', { lat: p.lat, lon: p.lon, radius_km: p.radiusKm }),

  viewport: (p: DiscoverViewportParams) =>
    get<RouteListResponse>('/discover/viewport', { bbox: p.bbox }),

  suggested: (p: DiscoverSuggestedParams) =>
    get<RouteListResponse>('/discover/suggested', { lat: p.lat, lon: p.lon, departure_at: p.departureAt })
};

// --- Routing ---

export const routing = {
  directions: (req: DirectionsRequest) =>
    post<DirectionsResponse>('/routing/directions', req),

  conditionsPreview: (bbox: string, departureAt: string) =>
    get<{ features: unknown[] }>('/routing/conditions/preview', { bbox, departure_at: departureAt })
};

// --- Venues ---

export const venues = {
  alongRoute: (routeId: string, type?: string, bufferM = 200) =>
    get<{ venues: Venue[] }>('/venues/along-route', { route_id: routeId, ...(type && { type }), buffer_m: bufferM }),

  tags: () => get<{ tags: VenueTag[] }>('/venues/tags')
};

// --- Reviews ---

export const reviews = {
  list: (routeId: string, cursor?: string) =>
    get<ReviewListResponse>(`/routes/${routeId}/reviews`, cursor ? { cursor } : undefined),

  create: (routeId: string, rating: number, body: string) =>
    post<Review>(`/routes/${routeId}/reviews`, { rating, body })
};

// --- Plans ---

export const plans = {
  createFromRoute: (routeId: string, departureAt: string) =>
    post<RoutePlan>(`/routes/${routeId}/plans`, { departure_at: departureAt }),

  create: (departureAt: string, shadeWeight: number, greeneryWeight: number, windWeight: number) =>
    post<RoutePlan>('/plans', { departure_at: departureAt, shade_weight: shadeWeight, greenery_weight: greeneryWeight, wind_weight: windWeight }),

  get: (id: string) => get<RoutePlan>(`/plans/${id}`),

  addStop: (planId: string, stop: { lat: number; lon: number; type: 'manual' }) =>
    post<StopPoint>(`/plans/${planId}/stops`, stop),

  addTask: (planId: string, description: string, hashtag?: string) =>
    post<PlanTask>(`/plans/${planId}/tasks`, { description, ...(hashtag && { hashtag }) }),

  removeStop: async (planId: string, stopId: string): Promise<void> => {
    const res = await fetch(`${BASE}/plans/${planId}/stops/${stopId}`, { method: 'DELETE' });
    if (!res.ok) throw new Error(`DELETE stop failed: ${res.status}`);
  }
};
```

- [ ] 2.3 Verify TypeScript compiles cleanly:

```bash
cd /Users/lug/src/cyclist-map/web && npx tsc --noEmit
```

Expected: no errors.

---

## Task 3 — Svelte Stores for Map State

**Files:**
- `web/src/lib/stores/map.ts` (create)
- `web/src/lib/stores/discovery.ts` (create)
- `web/src/lib/stores/planner.ts` (create)

### Steps

- [ ] 3.1 Create `web/src/lib/stores/map.ts` — shared map state reactive stores:

```typescript
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
```

- [ ] 3.2 Create `web/src/lib/stores/discovery.ts` — route list and filter state:

```typescript
// web/src/lib/stores/discovery.ts
import { writable } from 'svelte/store';
import type { Route } from '$lib/api/types';
import type { Difficulty } from '$lib/api/types';

export interface DiscoveryFilters {
  difficulty: Difficulty | null;
  shade: boolean;
  greenery: boolean;
  searchQuery: string;
}

export const discoveryFilters = writable<DiscoveryFilters>({
  difficulty: null,
  shade: false,
  greenery: false,
  searchQuery: ''
});

export const discoveryRoutes = writable<Route[]>([]);
export const discoveryLoading = writable<boolean>(false);
export const discoveryError = writable<string | null>(null);
```

- [ ] 3.3 Create `web/src/lib/stores/planner.ts` — route planner state:

```typescript
// web/src/lib/stores/planner.ts
import { writable } from 'svelte/store';
import type { RoutePlan, DirectionsResponse, RoutingPreferences } from '$lib/api/types';

export interface PlannerStop {
  id: string; // local UUID before plan is created
  lat: number;
  lon: number;
  label: string;
}

export const plannerStops = writable<PlannerStop[]>([]);
export const plannerPreferences = writable<RoutingPreferences>({ shade: 0.5, greenery: 0.5, wind: 0.5 });
export const plannerResult = writable<DirectionsResponse | null>(null);
export const plannerPlan = writable<RoutePlan | null>(null);
export const plannerLoading = writable<boolean>(false);
export const plannerError = writable<string | null>(null);
export const plannerTaskInput = writable<string>('');
```

---

## Task 4 — Shared Map Component

**Files:**
- `web/src/lib/components/Map.svelte` (create)
- `web/src/lib/components/MapOverlayToggle.svelte` (create)
- `web/src/lib/utils/conditionColors.ts` (create)

### Steps

- [ ] 4.1 Create `web/src/lib/utils/conditionColors.ts` — color LUT for condition overlays:

```typescript
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
      overlay === 'shade' ? seg.shade : overlay === 'wind' ? seg.windBenefit : seg.precip;
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
```

- [ ] 4.2 Create `web/src/lib/components/Map.svelte` — the shared MapLibre map component:

```svelte
<!-- web/src/lib/components/Map.svelte -->
<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import maplibregl from 'maplibre-gl';
  import 'maplibre-gl/dist/maplibre-gl.css';
  import { mapInstance, mapBounds, activeOverlay, highlightedRouteId } from '$lib/stores/map';
  import { buildLineGradient } from '$lib/utils/conditionColors';
  import type { RouteConditionSegment } from '$lib/api/types';

  // Props
  export let interactive = true;
  export let showControls = true;
  export let initialCenter: [number, number] = [139.6917, 35.6895]; // Tokyo
  export let initialZoom = 12;

  // Optional condition overlay data for a highlighted route
  export let conditionSegments: RouteConditionSegment[] = [];
  export let conditionRouteDistanceM = 0;
  // GeoJSON LineString coordinates for the highlighted route
  export let highlightGeometry: [number, number][] | null = null;

  const dispatch = createEventDispatcher<{
    click: { lng: number; lat: number };
    moveend: { bounds: maplibregl.LngLatBounds };
  }>();

  let container: HTMLDivElement;
  let map: maplibregl.Map;

  const MARTIN_TILES = 'http://localhost:3000';

  onMount(() => {
    map = new maplibregl.Map({
      container,
      style: {
        version: 8,
        glyphs: 'https://demotiles.maplibre.org/font/{fontstack}/{range}.pbf',
        sources: {
          osm: {
            type: 'vector',
            tiles: [`${MARTIN_TILES}/public.planet_osm_line/{z}/{x}/{y}`],
            minzoom: 0,
            maxzoom: 14
          }
        },
        layers: [
          {
            id: 'background',
            type: 'background',
            paint: { 'background-color': '#0F172A' } // slate-950
          },
          {
            id: 'osm-roads',
            type: 'line',
            source: 'osm',
            'source-layer': 'public.planet_osm_line',
            paint: {
              'line-color': '#334155', // slate-700
              'line-width': ['interpolate', ['linear'], ['zoom'], 8, 0.5, 14, 2]
            }
          }
        ]
      },
      center: initialCenter,
      zoom: initialZoom,
      interactive
    });

    if (showControls) {
      map.addControl(new maplibregl.NavigationControl(), 'top-right');
    }

    map.on('load', () => {
      // Source + layer for highlighted route line
      map.addSource('highlight-route', {
        type: 'geojson',
        lineMetrics: true,
        data: { type: 'FeatureCollection', features: [] }
      });

      map.addLayer({
        id: 'highlight-route-line',
        type: 'line',
        source: 'highlight-route',
        layout: { 'line-cap': 'round', 'line-join': 'round' },
        paint: {
          'line-width': 5,
          'line-color': '#38BDF8', // sky-400 default
          'line-opacity': 0.9
        }
      });

      // Source for planner stops
      map.addSource('planner-stops', {
        type: 'geojson',
        data: { type: 'FeatureCollection', features: [] }
      });

      map.addLayer({
        id: 'planner-stops-circle',
        type: 'circle',
        source: 'planner-stops',
        paint: {
          'circle-radius': 8,
          'circle-color': '#38BDF8',
          'circle-stroke-color': '#0F172A',
          'circle-stroke-width': 2
        }
      });

      mapInstance.set(map);
    });

    map.on('moveend', () => {
      const bounds = map.getBounds();
      const b = {
        minLon: bounds.getWest(),
        minLat: bounds.getSouth(),
        maxLon: bounds.getEast(),
        maxLat: bounds.getNorth()
      };
      mapBounds.set(b);
      dispatch('moveend', { bounds });
    });

    map.on('click', (e) => {
      dispatch('click', { lng: e.lngLat.lng, lat: e.lngLat.lat });
    });
  });

  onDestroy(() => {
    if (map) {
      mapInstance.set(null);
      map.remove();
    }
  });

  // Reactively update highlighted route geometry and condition overlay
  $: if (map && map.isStyleLoaded() && highlightGeometry !== null) {
    const source = map.getSource('highlight-route') as maplibregl.GeoJSONSource;
    source.setData({
      type: 'Feature',
      geometry: { type: 'LineString', coordinates: highlightGeometry },
      properties: {}
    });

    if ($activeOverlay && conditionSegments.length > 0) {
      const gradient = buildLineGradient(conditionSegments, $activeOverlay, conditionRouteDistanceM);
      map.setPaintProperty('highlight-route-line', 'line-gradient', gradient);
    } else {
      map.setPaintProperty('highlight-route-line', 'line-color', '#38BDF8');
    }
  }
</script>

<div bind:this={container} class="w-full h-full" />
```

- [ ] 4.3 Create `web/src/lib/components/MapOverlayToggle.svelte` — overlay toggle buttons:

```svelte
<!-- web/src/lib/components/MapOverlayToggle.svelte -->
<script lang="ts">
  import { activeOverlay } from '$lib/stores/map';
  import type { OverlayType } from '$lib/stores/map';

  const overlays: { id: OverlayType; label: string; activeClass: string }[] = [
    { id: 'shade', label: 'Shade', activeClass: 'bg-blue-800 text-blue-100' },
    { id: 'wind', label: 'Wind', activeClass: 'bg-green-800 text-green-100' },
    { id: 'rain', label: 'Rain', activeClass: 'bg-purple-800 text-purple-100' }
  ];

  function toggle(id: OverlayType) {
    activeOverlay.update((cur) => (cur === id ? null : id));
  }
</script>

<div class="flex gap-2">
  {#each overlays as ov}
    <button
      on:click={() => toggle(ov.id)}
      class="px-3 py-1.5 rounded-full text-xs font-semibold border transition-colors
             {$activeOverlay === ov.id
        ? ov.activeClass + ' border-transparent'
        : 'bg-slate-800 text-slate-400 border-slate-700 hover:bg-slate-700'}"
    >
      {ov.label}
    </button>
  {/each}
</div>
```

- [ ] 4.4 Manual smoke test: import `Map.svelte` in a temporary test page and confirm the MapLibre canvas renders over the dark background with no console errors.

---

## Task 5 — Discovery Page (/)

**Files:**
- `web/src/routes/+page.svelte` (create)
- `web/src/routes/+page.server.ts` (create — SSR route list load)
- `web/src/lib/components/RouteCard.svelte` (create)
- `web/src/lib/components/ConditionSparkline.svelte` (create)
- `web/src/lib/components/DepartureTimePicker.svelte` (create)
- `web/src/lib/components/FilterChips.svelte` (create)
- `web/src/lib/components/DiscoveryPanel.svelte` (create)

### Steps

- [ ] 5.1 Create `web/src/lib/components/ConditionSparkline.svelte` — mini bars summarizing shade/wind/rain per route card:

```svelte
<!-- web/src/lib/components/ConditionSparkline.svelte -->
<script lang="ts">
  import type { RouteConditionSegment } from '$lib/api/types';
  import { conditionColor } from '$lib/utils/conditionColors';

  export let segments: RouteConditionSegment[] = [];
  export let overlay: 'shade' | 'wind' | 'rain' = 'shade';

  const BAR_COUNT = 20;

  function sampleSegments(segs: RouteConditionSegment[], n: number): number[] {
    if (segs.length === 0) return Array(n).fill(0.5);
    return Array.from({ length: n }, (_, i) => {
      const idx = Math.floor((i / n) * segs.length);
      const seg = segs[idx];
      return overlay === 'shade' ? seg.shade : overlay === 'wind' ? (seg.windBenefit + 1) / 2 : seg.precip;
    });
  }

  $: values = sampleSegments(segments, BAR_COUNT);
</script>

<div class="flex items-end gap-px h-4" aria-hidden="true">
  {#each values as v}
    <div
      class="flex-1 rounded-sm"
      style="height: {Math.max(20, v * 100)}%; background: {conditionColor(overlay, overlay === 'wind' ? v * 2 - 1 : v)};"
    />
  {/each}
</div>
```

- [ ] 5.2 Create `web/src/lib/components/DepartureTimePicker.svelte`:

```svelte
<!-- web/src/lib/components/DepartureTimePicker.svelte -->
<script lang="ts">
  import { departureAt } from '$lib/stores/map';

  // Convert ISO string to "YYYY-MM-DDTHH:MM" for datetime-local input
  function toInputValue(iso: string): string {
    return iso.slice(0, 16);
  }

  function fromInputValue(v: string): string {
    return new Date(v).toISOString();
  }

  let inputValue = toInputValue($departureAt);

  function handleChange(e: Event) {
    const v = (e.target as HTMLInputElement).value;
    if (v) {
      inputValue = v;
      departureAt.set(fromInputValue(v));
    }
  }
</script>

<div class="flex flex-col gap-1">
  <label class="text-xs text-slate-400 font-medium uppercase tracking-wide" for="departure-time">
    Depart
  </label>
  <input
    id="departure-time"
    type="datetime-local"
    value={inputValue}
    on:change={handleChange}
    class="bg-slate-800 border border-slate-700 text-slate-100 text-sm rounded-lg px-3 py-2
           focus:outline-none focus:ring-2 focus:ring-sky-500 focus:border-transparent
           [color-scheme:dark]"
  />
</div>
```

- [ ] 5.3 Create `web/src/lib/components/FilterChips.svelte`:

```svelte
<!-- web/src/lib/components/FilterChips.svelte -->
<script lang="ts">
  import { discoveryFilters } from '$lib/stores/discovery';
  import type { Difficulty } from '$lib/api/types';

  const difficulties: { value: Difficulty; label: string }[] = [
    { value: 'easy', label: 'Easy' },
    { value: 'moderate', label: 'Moderate' },
    { value: 'hard', label: 'Hard' },
    { value: 'expert', label: 'Expert' }
  ];

  function toggleDifficulty(d: Difficulty) {
    discoveryFilters.update((f) => ({ ...f, difficulty: f.difficulty === d ? null : d }));
  }

  function toggleShade() {
    discoveryFilters.update((f) => ({ ...f, shade: !f.shade }));
  }

  function toggleGreenery() {
    discoveryFilters.update((f) => ({ ...f, greenery: !f.greenery }));
  }
</script>

<div class="flex flex-wrap gap-2">
  {#each difficulties as d}
    <button
      on:click={() => toggleDifficulty(d.value)}
      class="px-3 py-1 rounded-full text-xs font-semibold border transition-colors
             {$discoveryFilters.difficulty === d.value
        ? 'bg-sky-600 text-white border-transparent'
        : 'bg-slate-800 text-slate-400 border-slate-700 hover:bg-slate-700'}"
    >
      {d.label}
    </button>
  {/each}

  <button
    on:click={toggleShade}
    class="px-3 py-1 rounded-full text-xs font-semibold border transition-colors
           {$discoveryFilters.shade
      ? 'bg-blue-800 text-blue-100 border-transparent'
      : 'bg-slate-800 text-slate-400 border-slate-700 hover:bg-slate-700'}"
  >
    Shade
  </button>

  <button
    on:click={toggleGreenery}
    class="px-3 py-1 rounded-full text-xs font-semibold border transition-colors
           {$discoveryFilters.greenery
      ? 'bg-green-800 text-green-100 border-transparent'
      : 'bg-slate-800 text-slate-400 border-slate-700 hover:bg-slate-700'}"
  >
    Greenery
  </button>
</div>
```

- [ ] 5.4 Create `web/src/lib/components/RouteCard.svelte`:

```svelte
<!-- web/src/lib/components/RouteCard.svelte -->
<script lang="ts">
  import type { Route, RouteConditionSegment } from '$lib/api/types';
  import ConditionSparkline from './ConditionSparkline.svelte';
  import { highlightedRouteId } from '$lib/stores/map';

  export let route: Route;
  export let conditions: RouteConditionSegment[] = [];

  const difficultyColor: Record<string, string> = {
    easy: 'text-green-400',
    moderate: 'text-yellow-400',
    hard: 'text-orange-400',
    expert: 'text-red-400'
  };

  function distanceLabel(m: number): string {
    return m >= 1000 ? `${(m / 1000).toFixed(1)} km` : `${m} m`;
  }

  function elevLabel(m: number): string {
    return `+${m} m`;
  }

  $: isHighlighted = $highlightedRouteId === route.id;

  function handleClick() {
    highlightedRouteId.set(isHighlighted ? null : route.id);
  }
</script>

<button
  on:click={handleClick}
  class="w-full text-left rounded-xl p-4 border transition-colors
         {isHighlighted
    ? 'bg-slate-700 border-sky-500'
    : 'bg-slate-800 border-slate-700 hover:bg-slate-750 hover:border-slate-600'}"
>
  <div class="flex items-start justify-between gap-2 mb-2">
    <h3 class="text-sm font-semibold text-slate-100 leading-snug">{route.name}</h3>
    <span class="text-xs font-medium capitalize {difficultyColor[route.difficulty]} shrink-0">
      {route.difficulty}
    </span>
  </div>

  <div class="flex gap-3 text-xs text-slate-400 mb-3">
    <span>{distanceLabel(route.distanceM)}</span>
    <span>{elevLabel(route.elevationGainM)}</span>
    {#if route.tags.length > 0}
      <span class="text-slate-500">{route.tags.slice(0, 2).join(' · ')}</span>
    {/if}
  </div>

  {#if conditions.length > 0}
    <div class="flex gap-3">
      <div class="flex-1">
        <div class="text-[10px] text-slate-500 mb-1">Shade</div>
        <ConditionSparkline {conditions} overlay="shade" />
      </div>
      <div class="flex-1">
        <div class="text-[10px] text-slate-500 mb-1">Wind</div>
        <ConditionSparkline {conditions} overlay="wind" />
      </div>
      <div class="flex-1">
        <div class="text-[10px] text-slate-500 mb-1">Rain</div>
        <ConditionSparkline {conditions} overlay="rain" />
      </div>
    </div>
  {/if}
</button>
```

- [ ] 5.5 Create `web/src/lib/components/DiscoveryPanel.svelte` — the left panel / bottom sheet:

```svelte
<!-- web/src/lib/components/DiscoveryPanel.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { discovery, routes as routesApi } from '$lib/api/client';
  import { discoveryRoutes, discoveryLoading, discoveryFilters } from '$lib/stores/discovery';
  import { departureAt, bboxString, highlightedRouteId, activeOverlay } from '$lib/stores/map';
  import type { Route, RouteConditionSegment } from '$lib/api/types';
  import RouteCard from './RouteCard.svelte';
  import DepartureTimePicker from './DepartureTimePicker.svelte';
  import FilterChips from './FilterChips.svelte';
  import MapOverlayToggle from './MapOverlayToggle.svelte';

  export let initialRoutes: Route[] = [];

  // Condition cache: routeId → segments
  let conditionsCache: Map<string, RouteConditionSegment[]> = new Map();

  // Bottom sheet state (mobile)
  let sheetOpen = false;

  // Seed with SSR routes
  onMount(() => {
    if (initialRoutes.length > 0) {
      discoveryRoutes.set(initialRoutes);
    }
  });

  // Reload routes when bbox or departure changes
  async function loadRoutes(bbox: string | null, departure: string) {
    if (!bbox) return;
    discoveryLoading.set(true);
    try {
      const res = await discovery.viewport({ bbox });
      discoveryRoutes.set(res.routes);
      // Prefetch conditions for visible routes
      await Promise.allSettled(
        res.routes.map(async (r) => {
          if (!conditionsCache.has(r.id)) {
            const c = await routesApi.conditions(r.id, departure);
            conditionsCache.set(r.id, c.segments);
            conditionsCache = conditionsCache; // trigger reactivity
          }
        })
      );
    } catch {
      // silently ignore in this version
    } finally {
      discoveryLoading.set(false);
    }
  }

  $: loadRoutes($bboxString, $departureAt);

  $: filteredRoutes = $discoveryRoutes.filter((r) => {
    const f = $discoveryFilters;
    if (f.difficulty && r.difficulty !== f.difficulty) return false;
    if (f.searchQuery && !r.name.toLowerCase().includes(f.searchQuery.toLowerCase())) return false;
    return true;
  });

  // When a route is highlighted, fly map to it
  import { mapInstance } from '$lib/stores/map';
  $: if ($highlightedRouteId && $mapInstance) {
    const route = $discoveryRoutes.find((r) => r.id === $highlightedRouteId);
    if (route && route.geometry.coordinates.length > 0) {
      const coords = route.geometry.coordinates;
      const lons = coords.map((c) => c[0]);
      const lats = coords.map((c) => c[1]);
      $mapInstance.fitBounds(
        [
          [Math.min(...lons), Math.min(...lats)],
          [Math.max(...lons), Math.max(...lats)]
        ],
        { padding: 80, duration: 800 }
      );
    }
  }
</script>

<!-- Desktop: left panel -->
<aside class="hidden md:flex flex-col w-96 h-full bg-slate-900 border-r border-slate-800 z-10">
  <div class="p-4 border-b border-slate-800 space-y-3">
    <h1 class="text-lg font-bold text-slate-100">Cyclist Map</h1>
    <DepartureTimePicker />
    <FilterChips />
    <div class="flex items-center justify-between">
      <span class="text-xs text-slate-500">Overlay</span>
      <MapOverlayToggle />
    </div>
    <div class="relative">
      <input
        type="text"
        placeholder="Search routes…"
        bind:value={$discoveryFilters.searchQuery}
        class="w-full bg-slate-800 border border-slate-700 text-slate-100 text-sm rounded-lg
               pl-3 pr-3 py-2 focus:outline-none focus:ring-2 focus:ring-sky-500"
      />
    </div>
  </div>

  <div class="flex-1 overflow-y-auto p-3 space-y-2">
    {#if $discoveryLoading}
      <div class="text-slate-500 text-sm text-center py-8">Loading routes…</div>
    {:else if filteredRoutes.length === 0}
      <div class="text-slate-500 text-sm text-center py-8">No routes found</div>
    {:else}
      {#each filteredRoutes as route (route.id)}
        <RouteCard {route} conditions={conditionsCache.get(route.id) ?? []} />
      {/each}
    {/if}
  </div>
</aside>

<!-- Mobile: bottom sheet -->
<div class="md:hidden fixed bottom-0 inset-x-0 z-20">
  <div
    class="bg-slate-900 border-t border-slate-800 rounded-t-2xl transition-all duration-300"
    style="height: {sheetOpen ? '70vh' : '6rem'};"
  >
    <!-- Sheet handle -->
    <button
      on:click={() => (sheetOpen = !sheetOpen)}
      class="w-full flex flex-col items-center pt-3 pb-2 gap-1"
      aria-label="Toggle route list"
    >
      <div class="w-10 h-1 rounded-full bg-slate-600" />
      <span class="text-xs text-slate-400">{sheetOpen ? 'Hide' : `${filteredRoutes.length} routes`}</span>
    </button>

    {#if sheetOpen}
      <div class="px-4 pb-2 space-y-2">
        <DepartureTimePicker />
        <FilterChips />
        <MapOverlayToggle />
      </div>
      <div class="overflow-y-auto px-3 space-y-2" style="height: calc(70vh - 8rem);">
        {#each filteredRoutes as route (route.id)}
          <RouteCard {route} conditions={conditionsCache.get(route.id) ?? []} />
        {/each}
      </div>
    {/if}
  </div>
</div>
```

- [ ] 5.6 Create `web/src/routes/+page.server.ts` for SSR initial load:

```typescript
// web/src/routes/+page.server.ts
import type { PageServerLoad } from './$types';
import type { Route } from '$lib/api/types';

export const load: PageServerLoad = async ({ fetch }) => {
  // Load suggested routes for Tokyo center as a default
  const DEFAULT_LAT = 35.6895;
  const DEFAULT_LON = 139.6917;
  const now = new Date();
  now.setMinutes(0, 0, 0);
  const departureAt = now.toISOString();

  try {
    const res = await fetch(
      `/api/v1/discover/suggested?lat=${DEFAULT_LAT}&lon=${DEFAULT_LON}&departure_at=${encodeURIComponent(departureAt)}`
    );
    if (res.ok) {
      const data = await res.json();
      return { routes: (data.routes ?? []) as Route[], departureAt };
    }
  } catch {
    // API not running in SSR context; return empty — client hydrates
  }

  return { routes: [] as Route[], departureAt };
};
```

- [ ] 5.7 Create `web/src/routes/+page.svelte` — the Discovery page:

```svelte
<!-- web/src/routes/+page.svelte -->
<script lang="ts">
  import type { PageData } from './$types';
  import Map from '$lib/components/Map.svelte';
  import DiscoveryPanel from '$lib/components/DiscoveryPanel.svelte';
  import { highlightedRouteId, activeOverlay, departureAt } from '$lib/stores/map';
  import { discoveryRoutes } from '$lib/stores/discovery';
  import { routes as routesApi } from '$lib/api/client';
  import type { RouteConditionSegment } from '$lib/api/types';

  export let data: PageData;

  // Condition segments for the currently highlighted route
  let highlightedConditions: RouteConditionSegment[] = [];
  let highlightedDistanceM = 0;
  let highlightedGeometry: [number, number][] | null = null;

  $: if ($highlightedRouteId) {
    const route = $discoveryRoutes.find((r) => r.id === $highlightedRouteId);
    if (route) {
      highlightedGeometry = route.geometry.coordinates.map(
        (c) => [c[0], c[1]] as [number, number]
      );
      highlightedDistanceM = route.distanceM;

      // Fetch conditions for this route
      routesApi
        .conditions($highlightedRouteId, $departureAt)
        .then((c) => {
          highlightedConditions = c.segments;
        })
        .catch(() => {
          highlightedConditions = [];
        });
    }
  } else {
    highlightedGeometry = null;
    highlightedConditions = [];
  }
</script>

<svelte:head>
  <title>Cyclist Map — Discover Routes</title>
  <meta name="description" content="Discover cycling routes with shade, wind, and rain forecasts." />
</svelte:head>

<div class="flex h-dvh w-screen overflow-hidden bg-slate-900">
  <DiscoveryPanel initialRoutes={data.routes} />

  <div class="flex-1 relative">
    <Map
      {highlightGeometry}
      conditionSegments={highlightedConditions}
      conditionRouteDistanceM={highlightedDistanceM}
    />
  </div>
</div>
```

- [ ] 5.8 Verify in browser: navigate to `/` — map renders, route list appears in left panel (desktop) or bottom sheet (mobile), departure time picker works, filter chips toggle, clicking a route card highlights it on the map.

---

## Task 6 — Route Detail Page (/routes/[id])

**Files:**
- `web/src/routes/routes/[id]/+page.svelte` (create)
- `web/src/routes/routes/[id]/+page.server.ts` (create)
- `web/src/lib/components/ElevationProfile.svelte` (create)
- `web/src/lib/components/ConditionPanel.svelte` (create)
- `web/src/lib/components/ReviewList.svelte` (create)

### Steps

- [ ] 6.1 Create `web/src/lib/components/ElevationProfile.svelte` — SVG elevation chart:

```svelte
<!-- web/src/lib/components/ElevationProfile.svelte -->
<script lang="ts">
  export let coordinates: Array<[number, number, number?]> = [];
  export let widthPx = 400;
  export let heightPx = 80;

  interface ElevPoint { x: number; y: number; elevM: number; }

  $: points = (() => {
    const withEle = coordinates.filter((c) => c[2] !== undefined) as Array<[number, number, number]>;
    if (withEle.length < 2) return [] as ElevPoint[];
    const elevs = withEle.map((c) => c[2]);
    const minE = Math.min(...elevs);
    const maxE = Math.max(...elevs);
    const range = maxE - minE || 1;
    return withEle.map((c, i) => ({
      x: (i / (withEle.length - 1)) * widthPx,
      y: heightPx - ((c[2] - minE) / range) * (heightPx - 8),
      elevM: c[2]
    }));
  })();

  $: pathD = points.length > 1
    ? points.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(' ')
    : '';

  $: areaD = pathD.length > 0
    ? `${pathD} L ${widthPx} ${heightPx} L 0 ${heightPx} Z`
    : '';
</script>

{#if points.length > 1}
  <svg
    viewBox="0 0 {widthPx} {heightPx}"
    width="100%"
    height={heightPx}
    class="overflow-visible"
    aria-label="Elevation profile"
    role="img"
  >
    <defs>
      <linearGradient id="elev-grad" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0%" stop-color="#38BDF8" stop-opacity="0.5" />
        <stop offset="100%" stop-color="#38BDF8" stop-opacity="0.05" />
      </linearGradient>
    </defs>
    <path d={areaD} fill="url(#elev-grad)" />
    <path d={pathD} fill="none" stroke="#38BDF8" stroke-width="1.5" stroke-linejoin="round" />
  </svg>
{:else}
  <div class="text-slate-500 text-xs">No elevation data</div>
{/if}
```

- [ ] 6.2 Create `web/src/lib/components/ConditionPanel.svelte` — shade/wind/rain breakdown panel:

```svelte
<!-- web/src/lib/components/ConditionPanel.svelte -->
<script lang="ts">
  import type { RouteConditionSegment } from '$lib/api/types';
  import ConditionSparkline from './ConditionSparkline.svelte';

  export let segments: RouteConditionSegment[] = [];

  $: avgShade = segments.length > 0
    ? segments.reduce((s, r) => s + r.shade, 0) / segments.length
    : null;
  $: avgWind = segments.length > 0
    ? segments.reduce((s, r) => s + r.windBenefit, 0) / segments.length
    : null;
  $: avgPrecip = segments.length > 0
    ? segments.reduce((s, r) => s + r.precip, 0) / segments.length
    : null;

  function shadeLabel(v: number | null): string {
    if (v === null) return '—';
    if (v > 0.7) return 'Well shaded';
    if (v > 0.4) return 'Partial shade';
    return 'Exposed';
  }

  function windLabel(v: number | null): string {
    if (v === null) return '—';
    if (v > 0.3) return 'Tailwind';
    if (v < -0.3) return 'Headwind';
    return 'Cross / calm';
  }

  function precipLabel(v: number | null): string {
    if (v === null) return '—';
    if (v < 0.1) return 'Dry';
    if (v < 0.4) return 'Light rain';
    return 'Rain expected';
  }
</script>

<div class="space-y-4">
  {#each [
    { label: 'Shade', overlay: 'shade' as const, summary: shadeLabel(avgShade) },
    { label: 'Wind', overlay: 'wind' as const, summary: windLabel(avgWind) },
    { label: 'Rain', overlay: 'rain' as const, summary: precipLabel(avgPrecip) }
  ] as item}
    <div>
      <div class="flex items-center justify-between mb-1">
        <span class="text-sm font-medium text-slate-300">{item.label}</span>
        <span class="text-xs text-slate-400">{item.summary}</span>
      </div>
      <ConditionSparkline {segments} overlay={item.overlay} />
    </div>
  {/each}
</div>
```

- [ ] 6.3 Create `web/src/lib/components/ReviewList.svelte`:

```svelte
<!-- web/src/lib/components/ReviewList.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { reviews as reviewsApi } from '$lib/api/client';
  import type { Review } from '$lib/api/types';

  export let routeId: string;

  let items: Review[] = [];
  let loading = true;

  onMount(async () => {
    try {
      const res = await reviewsApi.list(routeId);
      items = res.reviews;
    } catch {
      // no reviews or API unavailable
    } finally {
      loading = false;
    }
  });

  function stars(n: number): string {
    return '★'.repeat(n) + '☆'.repeat(5 - n);
  }
</script>

<div class="space-y-3">
  <h3 class="text-sm font-semibold text-slate-300">Reviews</h3>

  {#if loading}
    <div class="text-slate-500 text-sm">Loading reviews…</div>
  {:else if items.length === 0}
    <div class="text-slate-500 text-sm">No reviews yet.</div>
  {:else}
    {#each items as review (review.id)}
      <div class="bg-slate-800 rounded-lg p-3 space-y-1">
        <div class="flex items-center gap-2">
          <span class="text-yellow-400 text-xs tracking-wide">{stars(review.rating)}</span>
          <span class="text-xs text-slate-500">{new Date(review.createdAt).toLocaleDateString()}</span>
        </div>
        <p class="text-sm text-slate-300 leading-snug">{review.body}</p>
      </div>
    {/each}
  {/if}
</div>
```

- [ ] 6.4 Create `web/src/routes/routes/[id]/+page.server.ts`:

```typescript
// web/src/routes/routes/[id]/+page.server.ts
import type { PageServerLoad } from './$types';
import { error } from '@sveltejs/kit';
import type { Route, RouteConditionsResponse } from '$lib/api/types';

export const load: PageServerLoad = async ({ params, fetch, url }) => {
  const now = new Date();
  now.setMinutes(0, 0, 0);
  const departureAt = url.searchParams.get('departure_at') ?? now.toISOString();

  const [routeRes, condRes] = await Promise.allSettled([
    fetch(`/api/v1/routes/${params.id}`),
    fetch(`/api/v1/routes/${params.id}/conditions?departure_at=${encodeURIComponent(departureAt)}&speed_model=elevation`)
  ]);

  if (routeRes.status === 'rejected' || (routeRes.status === 'fulfilled' && !routeRes.value.ok)) {
    error(404, 'Route not found');
  }

  const route = await (routeRes as PromiseFulfilledResult<Response>).value.json() as Route;

  let conditions: RouteConditionsResponse | null = null;
  if (condRes.status === 'fulfilled' && condRes.value.ok) {
    conditions = await condRes.value.json() as RouteConditionsResponse;
  }

  return { route, conditions, departureAt };
};
```

- [ ] 6.5 Create `web/src/routes/routes/[id]/+page.svelte`:

```svelte
<!-- web/src/routes/routes/[id]/+page.svelte -->
<script lang="ts">
  import type { PageData } from './$types';
  import Map from '$lib/components/Map.svelte';
  import ElevationProfile from '$lib/components/ElevationProfile.svelte';
  import ConditionPanel from '$lib/components/ConditionPanel.svelte';
  import ReviewList from '$lib/components/ReviewList.svelte';
  import MapOverlayToggle from '$lib/components/MapOverlayToggle.svelte';
  import { activeOverlay, departureAt } from '$lib/stores/map';
  import { plans } from '$lib/api/client';

  export let data: PageData;

  $: route = data.route;
  $: conditions = data.conditions?.segments ?? [];
  $: geometry = route.geometry.coordinates.map((c) => [c[0], c[1]] as [number, number]);

  const difficultyColor: Record<string, string> = {
    easy: 'text-green-400',
    moderate: 'text-yellow-400',
    hard: 'text-orange-400',
    expert: 'text-red-400'
  };

  let planLoading = false;
  async function planThisRide() {
    planLoading = true;
    try {
      const plan = await plans.createFromRoute(route.id, $departureAt);
      window.location.href = `/planner?plan=${plan.id}`;
    } catch {
      planLoading = false;
    }
  }
</script>

<svelte:head>
  <title>{route.name} — Cyclist Map</title>
  <meta name="description" content="{route.description || route.name} — {(route.distanceM / 1000).toFixed(1)} km cycling route." />
</svelte:head>

<div class="flex h-dvh w-screen overflow-hidden bg-slate-900">
  <!-- Detail panel -->
  <aside class="w-full md:w-96 h-full bg-slate-900 border-r border-slate-800 overflow-y-auto flex-shrink-0 z-10">
    <div class="p-5 space-y-5">
      <!-- Back -->
      <a href="/" class="inline-flex items-center gap-1 text-sm text-slate-400 hover:text-slate-100">
        ← Back
      </a>

      <!-- Title + meta -->
      <div class="space-y-1">
        <h1 class="text-xl font-bold text-slate-100">{route.name}</h1>
        <div class="flex gap-3 text-sm text-slate-400">
          <span>{(route.distanceM / 1000).toFixed(1)} km</span>
          <span>+{route.elevationGainM} m</span>
          <span class="capitalize {difficultyColor[route.difficulty]}">{route.difficulty}</span>
        </div>
        {#if route.tags.length > 0}
          <div class="flex flex-wrap gap-1 mt-1">
            {#each route.tags as tag}
              <span class="text-xs bg-slate-800 text-slate-400 rounded px-2 py-0.5">{tag}</span>
            {/each}
          </div>
        {/if}
      </div>

      {#if route.description}
        <p class="text-sm text-slate-400 leading-relaxed">{route.description}</p>
      {/if}

      <!-- Overlay toggle -->
      <div>
        <div class="text-xs text-slate-500 mb-2 uppercase tracking-wide">Condition overlay</div>
        <MapOverlayToggle />
      </div>

      <!-- Condition panel -->
      {#if conditions.length > 0}
        <div class="bg-slate-800 rounded-xl p-4">
          <h2 class="text-sm font-semibold text-slate-300 mb-3">Conditions at departure</h2>
          <ConditionPanel segments={conditions} />
        </div>
      {/if}

      <!-- Elevation profile -->
      <div class="bg-slate-800 rounded-xl p-4">
        <h2 class="text-sm font-semibold text-slate-300 mb-3">Elevation</h2>
        <ElevationProfile coordinates={route.geometry.coordinates as any} />
      </div>

      <!-- Waypoints -->
      {#if route.waypoints && route.waypoints.length > 0}
        <div>
          <h2 class="text-sm font-semibold text-slate-300 mb-2">Stops</h2>
          <ul class="space-y-1">
            {#each route.waypoints.sort((a, b) => a.sortOrder - b.sortOrder) as wp}
              <li class="flex items-center gap-2 text-sm text-slate-400">
                <span class="w-2 h-2 rounded-full bg-sky-400 shrink-0" />
                {wp.name}
                <span class="text-xs text-slate-600 capitalize">({wp.type})</span>
              </li>
            {/each}
          </ul>
        </div>
      {/if}

      <!-- Reviews -->
      <ReviewList routeId={route.id} />

      <!-- Plan CTA -->
      <button
        on:click={planThisRide}
        disabled={planLoading}
        class="w-full bg-sky-600 hover:bg-sky-500 disabled:opacity-50 text-white font-semibold
               rounded-xl py-3 text-sm transition-colors"
      >
        {planLoading ? 'Creating plan…' : 'Plan this ride'}
      </button>
    </div>
  </aside>

  <!-- Map -->
  <div class="hidden md:block flex-1 relative">
    <Map
      highlightGeometry={geometry}
      conditionSegments={conditions}
      conditionRouteDistanceM={route.distanceM}
    />
  </div>
</div>
```

- [ ] 6.6 Verify in browser: navigate to `/routes/<valid-id>` — route detail panel with elevation profile, condition bars, reviews, and "Plan this ride" button visible on desktop. Map shows the route with overlay capability.

---

## Task 7 — Route Planner Page (/planner)

**Files:**
- `web/src/routes/planner/+page.svelte` (create — CSR only)
- `web/src/lib/components/PlannerPanel.svelte` (create)
- `web/src/lib/components/PreferenceSliders.svelte` (create)
- `web/src/lib/components/PlannerStopList.svelte` (create)
- `web/src/lib/components/TaskInput.svelte` (create)

### Steps

- [ ] 7.1 Create `web/src/lib/components/PreferenceSliders.svelte`:

```svelte
<!-- web/src/lib/components/PreferenceSliders.svelte -->
<script lang="ts">
  import { plannerPreferences } from '$lib/stores/planner';

  const sliders: { key: keyof typeof $plannerPreferences; label: string; color: string }[] = [
    { key: 'shade', label: 'Shade', color: 'accent-blue-500' },
    { key: 'greenery', label: 'Greenery', color: 'accent-green-500' },
    { key: 'wind', label: 'Wind avoid', color: 'accent-red-400' }
  ];
</script>

<div class="space-y-3">
  {#each sliders as s}
    <div class="space-y-1">
      <div class="flex justify-between text-xs text-slate-400">
        <span>{s.label}</span>
        <span>{Math.round($plannerPreferences[s.key] * 100)}%</span>
      </div>
      <input
        type="range"
        min="0"
        max="1"
        step="0.05"
        bind:value={$plannerPreferences[s.key]}
        class="w-full h-1.5 bg-slate-700 rounded-full appearance-none cursor-pointer {s.color}"
      />
    </div>
  {/each}
</div>
```

- [ ] 7.2 Create `web/src/lib/components/PlannerStopList.svelte`:

```svelte
<!-- web/src/lib/components/PlannerStopList.svelte -->
<script lang="ts">
  import { plannerStops, plannerPlan } from '$lib/stores/planner';
  import { plans } from '$lib/api/client';

  async function removeStop(id: string) {
    plannerStops.update((stops) => stops.filter((s) => s.id !== id));
    const plan = $plannerPlan;
    if (plan) {
      const planStop = plan.stops.find(
        (s) =>
          $plannerStops.find((ps) => ps.id === id) === undefined &&
          s.resolvedName === id
      );
      if (planStop) {
        await plans.removeStop(plan.id, planStop.id).catch(() => {});
      }
    }
  }
</script>

<div class="space-y-1">
  {#each $plannerStops as stop, i (stop.id)}
    <div class="flex items-center gap-2 bg-slate-800 rounded-lg px-3 py-2">
      <div class="flex flex-col items-center gap-1 shrink-0">
        <div class="w-3 h-3 rounded-full {i === 0 ? 'bg-green-400' : i === $plannerStops.length - 1 ? 'bg-red-400' : 'bg-sky-400'}" />
        {#if i < $plannerStops.length - 1}
          <div class="w-0.5 h-3 bg-slate-700" />
        {/if}
      </div>
      <span class="flex-1 text-sm text-slate-300 truncate">{stop.label}</span>
      {#if $plannerStops.length > 2}
        <button
          on:click={() => removeStop(stop.id)}
          class="text-slate-600 hover:text-red-400 text-xs transition-colors"
          aria-label="Remove stop"
        >
          ✕
        </button>
      {/if}
    </div>
  {/each}
</div>
```

- [ ] 7.3 Create `web/src/lib/components/TaskInput.svelte` — #hashtag task entry with venue resolution:

```svelte
<!-- web/src/lib/components/TaskInput.svelte -->
<script lang="ts">
  import { plannerTaskInput, plannerPlan } from '$lib/stores/planner';
  import { plans } from '$lib/api/client';

  let adding = false;

  async function addTask() {
    const desc = $plannerTaskInput.trim();
    if (!desc || !$plannerPlan) return;

    const hashMatch = desc.match(/#(\S+)/);
    const hashtag = hashMatch ? `#${hashMatch[1]}` : undefined;

    adding = true;
    try {
      await plans.addTask($plannerPlan.id, desc, hashtag);
      plannerTaskInput.set('');
    } finally {
      adding = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') addTask();
  }
</script>

<div class="flex gap-2">
  <input
    type="text"
    placeholder="Add task: coffee at #cafe"
    bind:value={$plannerTaskInput}
    on:keydown={handleKeydown}
    class="flex-1 bg-slate-800 border border-slate-700 text-slate-100 text-sm rounded-lg
           px-3 py-2 focus:outline-none focus:ring-2 focus:ring-sky-500 placeholder-slate-600"
  />
  <button
    on:click={addTask}
    disabled={adding || !$plannerPlan}
    class="bg-sky-600 hover:bg-sky-500 disabled:opacity-40 text-white text-sm font-semibold
           rounded-lg px-3 py-2 transition-colors"
  >
    Add
  </button>
</div>
```

- [ ] 7.4 Create `web/src/lib/components/PlannerPanel.svelte` — full planner sidebar:

```svelte
<!-- web/src/lib/components/PlannerPanel.svelte -->
<script lang="ts">
  import { onDestroy } from 'svelte';
  import { plannerStops, plannerPreferences, plannerResult, plannerPlan, plannerLoading, plannerError } from '$lib/stores/planner';
  import { departureAt, mapInstance } from '$lib/stores/map';
  import { routing, plans } from '$lib/api/client';
  import DepartureTimePicker from './DepartureTimePicker.svelte';
  import PreferenceSliders from './PreferenceSliders.svelte';
  import PlannerStopList from './PlannerStopList.svelte';
  import TaskInput from './TaskInput.svelte';
  import type { DirectionsRequest } from '$lib/api/types';

  // Trigger routing whenever stops (≥2) or preferences change
  let routeDebounceTimer: ReturnType<typeof setTimeout> | null = null;

  function scheduleRoute() {
    if ($plannerStops.length < 2) return;
    if (routeDebounceTimer) clearTimeout(routeDebounceTimer);
    routeDebounceTimer = setTimeout(computeRoute, 600);
  }

  async function computeRoute() {
    plannerLoading.set(true);
    plannerError.set(null);
    try {
      const req: DirectionsRequest = {
        stops: $plannerStops.map((s) => ({ type: 'manual', lat: s.lat, lon: s.lon })),
        departureAt: $departureAt,
        speedModel: 'elevation',
        preferences: $plannerPreferences
      };
      const res = await routing.directions(req);
      plannerResult.set(res);
    } catch (e) {
      plannerError.set(e instanceof Error ? e.message : 'Routing failed');
    } finally {
      plannerLoading.set(false);
    }
  }

  const unsubStops = plannerStops.subscribe(() => scheduleRoute());
  const unsubPrefs = plannerPreferences.subscribe(() => scheduleRoute());
  const unsubDep = departureAt.subscribe(() => scheduleRoute());

  onDestroy(() => {
    unsubStops();
    unsubPrefs();
    unsubDep();
    if (routeDebounceTimer) clearTimeout(routeDebounceTimer);
  });

  $: distanceLabel = $plannerResult
    ? `${($plannerResult.distanceM / 1000).toFixed(1)} km`
    : null;
  $: durationLabel = $plannerResult
    ? `${Math.round($plannerResult.durationS / 60)} min`
    : null;
</script>

<aside class="flex flex-col w-full md:w-96 h-full bg-slate-900 border-r border-slate-800 z-10">
  <div class="p-4 border-b border-slate-800 space-y-4">
    <div class="flex items-center gap-2">
      <a href="/" class="text-slate-400 hover:text-slate-100 text-sm">← Discover</a>
      <h1 class="text-base font-bold text-slate-100 ml-auto">Route Planner</h1>
    </div>

    <DepartureTimePicker />

    {#if $plannerStops.length >= 2}
      <PlannerStopList />
    {:else}
      <div class="text-sm text-slate-500 text-center py-3 border border-dashed border-slate-700 rounded-xl">
        Click the map to add two or more stops
      </div>
    {/if}

    <div>
      <div class="text-xs text-slate-500 mb-2 uppercase tracking-wide">Preferences</div>
      <PreferenceSliders />
    </div>

    {#if $plannerPlan}
      <div>
        <div class="text-xs text-slate-500 mb-2 uppercase tracking-wide">Tasks</div>
        <TaskInput />
      </div>
    {/if}
  </div>

  {#if $plannerLoading}
    <div class="p-4 text-sm text-slate-400 text-center">Computing route…</div>
  {:else if $plannerError}
    <div class="p-4 text-sm text-red-400">{$plannerError}</div>
  {:else if $plannerResult}
    <div class="p-4 border-b border-slate-800">
      <div class="flex gap-4 text-sm">
        <div>
          <div class="text-slate-500 text-xs">Distance</div>
          <div class="text-slate-100 font-semibold">{distanceLabel}</div>
        </div>
        <div>
          <div class="text-slate-500 text-xs">Time</div>
          <div class="text-slate-100 font-semibold">{durationLabel}</div>
        </div>
      </div>
    </div>

    <div class="p-4">
      <button
        on:click={async () => {
          if (!$plannerPlan) {
            const p = await plans.create($departureAt, $plannerPreferences.shade, $plannerPreferences.greenery, $plannerPreferences.wind);
            plannerPlan.set(p);
          }
        }}
        class="w-full bg-sky-600 hover:bg-sky-500 text-white text-sm font-semibold rounded-xl py-2.5 transition-colors"
      >
        Save plan
      </button>
    </div>
  {/if}
</aside>
```

- [ ] 7.5 Create `web/src/routes/planner/+page.svelte` (CSR — no server load, `export const ssr = false`):

```svelte
<!-- web/src/routes/planner/+page.svelte -->
<script lang="ts">
  export const ssr = false;

  import { onMount } from 'svelte';
  import Map from '$lib/components/Map.svelte';
  import PlannerPanel from '$lib/components/PlannerPanel.svelte';
  import MapOverlayToggle from '$lib/components/MapOverlayToggle.svelte';
  import { plannerStops, plannerResult } from '$lib/stores/planner';
  import { activeOverlay } from '$lib/stores/map';
  import type { RouteConditionSegment } from '$lib/api/types';

  let stopCounter = 0;

  function handleMapClick(e: CustomEvent<{ lng: number; lat: number }>) {
    const { lng, lat } = e.detail;
    plannerStops.update((stops) => [
      ...stops,
      {
        id: `stop-${++stopCounter}`,
        lat,
        lon: lng,
        label: `Stop ${stopCounter} (${lat.toFixed(4)}, ${lng.toFixed(4)})`
      }
    ]);
  }

  $: planGeometry = $plannerResult?.geometry.coordinates.map(
    (c) => [c[0], c[1]] as [number, number]
  ) ?? null;

  $: planConditions: RouteConditionSegment[] = $plannerResult?.segments ?? [];
  $: planDistanceM = $plannerResult?.distanceM ?? 0;
</script>

<svelte:head>
  <title>Route Planner — Cyclist Map</title>
</svelte:head>

<div class="flex h-dvh w-screen overflow-hidden bg-slate-900">
  <PlannerPanel />

  <div class="flex-1 relative">
    <Map
      on:click={handleMapClick}
      highlightGeometry={planGeometry}
      conditionSegments={planConditions}
      conditionRouteDistanceM={planDistanceM}
    />

    <!-- Overlay toggle (floating, top-left of map) -->
    <div class="absolute top-4 left-4 z-10">
      <MapOverlayToggle />
    </div>

    {#if $plannerResult}
      <div class="absolute bottom-6 left-1/2 -translate-x-1/2 z-10
                  bg-slate-900/90 border border-slate-700 rounded-full px-4 py-2
                  text-xs text-slate-300 backdrop-blur-sm">
        Click map to add more stops
      </div>
    {:else if $plannerStops.length === 0}
      <div class="absolute bottom-6 left-1/2 -translate-x-1/2 z-10
                  bg-slate-900/90 border border-slate-700 rounded-full px-4 py-2
                  text-xs text-slate-300 backdrop-blur-sm">
        Click map to set origin
      </div>
    {:else if $plannerStops.length === 1}
      <div class="absolute bottom-6 left-1/2 -translate-x-1/2 z-10
                  bg-slate-900/90 border border-slate-700 rounded-full px-4 py-2
                  text-xs text-slate-300 backdrop-blur-sm">
        Click map to set destination
      </div>
    {/if}
  </div>
</div>
```

- [ ] 7.6 Verify in browser: navigate to `/planner` — map is full-screen with planner panel on left, clicking map adds stop markers, two stops trigger routing, preference sliders adjust weights and re-trigger routing. Plan geometry renders with condition overlay support.

---

## Task 8 — Navigation + Root Layout

**Files:**
- `web/src/routes/+layout.svelte` (create)

### Steps

- [ ] 8.1 Create `web/src/routes/+layout.svelte` — minimal root layout that imports the CSS and provides mobile-safe viewport:

```svelte
<!-- web/src/routes/+layout.svelte -->
<script lang="ts">
  import '../app.css';
</script>

<slot />
```

- [ ] 8.2 Verify all pages still load correctly after adding the layout.

---

## Task 9 — SvelteKit Config: SSR + CSR Settings

**Files:**
- `web/src/routes/planner/+page.ts` (create — disable SSR for planner)
- `web/svelte.config.js` (update — adapter-node or adapter-auto)

### Steps

- [ ] 9.1 Create `web/src/routes/planner/+page.ts` to disable SSR for the planner:

```typescript
// web/src/routes/planner/+page.ts
export const ssr = false;
export const prerender = false;
```

Note: this complements the `export const ssr = false` inside `+page.svelte` — the `+page.ts` file is the canonical SvelteKit way to set this at the route level.

- [ ] 9.2 Update `web/svelte.config.js` for Node adapter (matches docker-compose deployment):

```javascript
// web/svelte.config.js
import adapter from '@sveltejs/adapter-node';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  preprocess: vitePreprocess(),
  kit: {
    adapter: adapter({ out: 'build' }),
    alias: {
      $lib: 'src/lib'
    }
  }
};

export default config;
```

- [ ] 9.3 Install the Node adapter:

```bash
cd /Users/lug/src/cyclist-map/web && npm install -D @sveltejs/adapter-node
```

- [ ] 9.4 Run `npm run build` and confirm the build succeeds:

```bash
cd /Users/lug/src/cyclist-map/web && npm run build
```

Expected: no TypeScript or Svelte compiler errors; `web/build/` directory created.

---

## Task 10 — Docker Integration

**Files:**
- `web/Dockerfile` (create)
- `docker-compose.yml` (update — add `web` service)

### Steps

- [ ] 10.1 Create `web/Dockerfile`:

```dockerfile
# web/Dockerfile
FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:22-alpine AS runtime
WORKDIR /app
COPY --from=build /app/build ./build
COPY --from=build /app/package.json ./
ENV NODE_ENV=production
ENV PORT=3001
EXPOSE 3001
CMD ["node", "build"]
```

- [ ] 10.2 Add the `web` service to the root `docker-compose.yml`. Read the existing file first, then add below the `api` service:

```yaml
  web:
    build:
      context: ./web
      dockerfile: Dockerfile
    ports:
      - "3001:3001"
    environment:
      - NODE_ENV=production
      - ORIGIN=http://localhost:3001
    depends_on:
      - api
    restart: unless-stopped
```

Note: the Vite dev proxy (`/api` → `localhost:8080`) is only active during development. In production the SvelteKit Node server needs a reverse proxy (nginx or Caddy) to route `/api` to the Go API. For local docker-compose dev, `vite dev` with the proxy config in `vite.config.ts` handles this.

- [ ] 10.3 Verify docker build succeeds:

```bash
cd /Users/lug/src/cyclist-map && docker build -f web/Dockerfile web/
```

Expected: image builds without errors.

---

## Self-Review Checklist

Before marking this plan complete, verify each item:

- [ ] `npm run dev` starts without errors and loads `localhost:5173`
- [ ] Discovery page (`/`) shows map + route list panel on desktop; bottom sheet on mobile
- [ ] Departure time picker updates `departureAt` store; route conditions refresh
- [ ] Filter chips narrow the route list reactively
- [ ] Overlay toggle buttons switch between shade/wind/rain on highlighted route
- [ ] Clicking a route card flies the map to the route and renders the line
- [ ] Route Detail page (`/routes/[id]`) SSR renders with `<title>` and `<meta>` tags visible in page source
- [ ] Elevation profile renders for routes with Z coordinates
- [ ] Condition panel shows shade/wind/rain sparklines with correct color gradients
- [ ] "Plan this ride" button creates a plan and navigates to `/planner?plan=<id>`
- [ ] Planner page (`/planner`) is CSR-only (no SSR flash)
- [ ] Clicking map adds stops; two stops trigger routing call; route line appears
- [ ] Preference sliders debounce and re-trigger routing
- [ ] `npm run build` succeeds with zero TypeScript errors
- [ ] Docker image builds successfully
- [ ] All condition color gradients visually match the spec: shade (yellow→blue), wind (red→green), rain (white→purple)

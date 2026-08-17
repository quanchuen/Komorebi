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

// Wire shape returned by the Go routes API. Unlike routing alternatives, saved
// routes use plain coordinate arrays and snake_case fields.
export interface ApiRoute {
  id: string;
  name: string;
  description: string;
  geometry: Array<[number, number] | [number, number, number]>;
  distance_m: number;
  elevation_gain_m: number;
  elevation_loss_m: number;
  difficulty: Difficulty;
  status: RouteStatus;
  creator_id: string;
  tags: string[];
  waypoints?: Array<{
    name: string;
    type: WaypointType;
    lat: number;
    lon: number;
    sort_order: number;
  }>;
  segments?: Array<{
    geometry: Array<[number, number] | [number, number, number]>;
    surface_type: SurfaceType;
    grade_percent: number;
    segment_order: number;
  }>;
  created_at: string;
  updated_at: string;
}

export function apiRouteToRoute(route: ApiRoute): Route {
  return {
    id: route.id,
    name: route.name,
    description: route.description,
    geometry: { type: 'LineString', coordinates: route.geometry ?? [] },
    distanceM: route.distance_m,
    elevationGainM: route.elevation_gain_m,
    elevationLossM: route.elevation_loss_m,
    difficulty: route.difficulty,
    status: route.status,
    creatorId: route.creator_id,
    tags: route.tags ?? [],
    waypoints: (route.waypoints ?? []).map((waypoint, index) => ({
      id: `${route.id}-waypoint-${index}`,
      routeId: route.id,
      geometry: { type: 'Point', coordinates: [waypoint.lon, waypoint.lat] },
      name: waypoint.name,
      type: waypoint.type,
      sortOrder: waypoint.sort_order
    })),
    segments: (route.segments ?? []).map((segment, index) => ({
      id: `${route.id}-segment-${index}`,
      routeId: route.id,
      geometry: { type: 'LineString', coordinates: segment.geometry ?? [] },
      surfaceType: segment.surface_type,
      gradePercent: segment.grade_percent,
      segmentOrder: segment.segment_order
    })),
    createdAt: route.created_at,
    updatedAt: route.updated_at
  };
}

// --- Conditions ---

export interface GreenWaveInfo {
  speedKmh: number;
  lengthKm: number;
}

export interface ConditionColors {
  shade: string; // hex
  wind: string;
  rain: string;
}

export interface RouteConditionSegment {
  km: number;
  eta: string;
  shade: number; // 0.0–1.0
  wind_benefit: number; // -1.0 (headwind) to 1.0 (tailwind)
  precip: number; // 0.0–1.0
  green_wave: GreenWaveInfo | null;
  signals: number;
  colors: ConditionColors;
}

export interface RouteConditionsResponse {
  route_id: string;
  segments: RouteConditionSegment[];
}

// --- Discovery ---

// The discovery API returns a different shape than the full Route object
export interface DiscoveryRoute {
  route_id: string;
  name: string;
  description: string;
  distance_m: number;
  elevation_gain_m: number;
  elevation_loss_m: number;
  difficulty: Difficulty;
  status: RouteStatus;
  tags: string[];
  dist_from_m: number;
}

export interface DiscoveryListResponse {
  routes: DiscoveryRoute[];
}

// Map a discovery result into a Route-compatible shape for the UI
export function discoveryRouteToRoute(dr: DiscoveryRoute): Route {
  return {
    id: dr.route_id,
    name: dr.name,
    description: dr.description,
    geometry: { type: 'LineString', coordinates: [] }, // no geometry in discovery results
    distanceM: dr.distance_m,
    elevationGainM: dr.elevation_gain_m,
    elevationLossM: dr.elevation_loss_m,
    difficulty: dr.difficulty,
    status: dr.status,
    creatorId: '',
    tags: dr.tags ?? [],
    waypoints: [],
    segments: [],
    createdAt: '',
    updatedAt: ''
  };
}

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
  shade: number; // 0.0–1.0
  greenery: number; // 0.0–1.0
  wind: number; // 0.0–1.0
}

export interface DirectionsRequest {
  stops: RoutingStop[];
  departure_at: string;
  speed_model: 'elevation';
  preferences: RoutingPreferences;
}

export interface RouteAlternative {
  profile: string; // "suggested" | "fast" | "avoid_main_roads"
  label: string; // "Suggested" | "Fast" | "Avoid main roads"
  total_distance_km: number;
  total_duration_s: number;
  elevation_gain_m: number;
  elevation_loss_m: number;
  elevation_profile: Array<{ distance_m: number; elevation_m: number }>;
  legs: { distance_km: number; duration_s: number; eta_at: string }[];
  geometry: GeoLineString;
}

export interface DirectionsResponse {
  alternatives: RouteAlternative[];
}

// Natural-language route intent (ADR 0003). The LLM only interprets text into
// this schema; the client displays it and applies preferences deterministically.
export interface RouteIntent {
  schema_version: string;
  summary: string;
  original_text: string;
  preferences: {
    shade: number | null;
    greenery: number | null;
    wind: number | null;
  };
  constraints: {
    max_detour_m: number | null;
    max_grade_percent: number | null;
  };
  unresolved_terms: string[];
  model: string;
  prompt_version: string;
}

export interface RouteIntentResponse {
  intent: RouteIntent;
  preferences: RoutingPreferences;
  applied: string[];
  unsupported: string[];
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

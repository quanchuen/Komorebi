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

export interface ConditionColors {
  shade: string;  // hex
  wind: string;
  rain: string;
}

export interface RouteConditionSegment {
  km: number;
  eta: string;
  shade: number;           // 0.0–1.0
  wind_benefit: number;    // -1.0 (headwind) to 1.0 (tailwind)
  precip: number;          // 0.0–1.0
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
  shade: number;    // 0.0–1.0
  greenery: number; // 0.0–1.0
  wind: number;     // 0.0–1.0
}

export interface DirectionsRequest {
  stops: RoutingStop[];
  departure_at: string;
  speed_model: 'elevation';
  preferences: RoutingPreferences;
}

export interface RouteAlternative {
  profile: string;           // "suggested" | "fast" | "avoid_main_roads"
  label: string;             // "Suggested" | "Fast" | "Avoid main roads"
  total_distance_km: number;
  total_duration_s: number;
  legs: { distance_km: number; duration_s: number; eta_at: string }[];
  geometry: GeoLineString;
}

export interface DirectionsResponse {
  alternatives: RouteAlternative[];
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

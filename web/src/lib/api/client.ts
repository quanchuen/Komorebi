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
  DiscoverSuggestedParams,
  DiscoveryListResponse
} from './types';
import { discoveryRouteToRoute } from './types';

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

async function getDiscoveryRoutes(path: string, params: Record<string, string | number>): Promise<RouteListResponse> {
  const raw = await get<DiscoveryListResponse>(path, params);
  return { routes: (raw.routes ?? []).map(discoveryRouteToRoute), nextCursor: null };
}

export const discovery = {
  nearby: (p: DiscoverNearbyParams) =>
    getDiscoveryRoutes('/discover/nearby', { lat: p.lat, lon: p.lon, radius_km: p.radiusKm }),

  viewport: (p: DiscoverViewportParams) =>
    getDiscoveryRoutes('/discover/viewport', { bbox: p.bbox }),

  suggested: (p: DiscoverSuggestedParams) =>
    getDiscoveryRoutes('/discover/suggested', { lat: p.lat, lon: p.lon, departure_at: p.departureAt })
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

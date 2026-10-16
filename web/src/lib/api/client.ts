// web/src/lib/api/client.ts
import type {
  ApiRoute,
  RouteListResponse,
  RouteConditionsResponse,
  DirectionsRequest,
  DirectionsResponse,
  GeoLineString,
  RouteIntentResponse,
  RoutingPreferences,
  ReviewListResponse,
  DiscoverViewportParams,
  DiscoverSuggestedParams,
  DiscoveryListResponse,
  WeatherGridResponse
} from './types';
import { apiRouteToRoute, discoveryRouteToRoute } from './types';

const BASE = '/api/v1';

/**
 * A non-2xx answer from the API. `status` lets callers tell a request the
 * server rejected (4xx) from an outage (5xx); `detail` is the server's own
 * `error` message when the body carried one. The message keeps the
 * `METHOD path failed: status text` form that existing callers match on.
 */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
    readonly detail: string | null
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

async function fail(method: string, path: string, res: Response): Promise<never> {
  let detail: string | null = null;
  try {
    const body: unknown = await res.json();
    if (
      body &&
      typeof body === 'object' &&
      typeof (body as { error?: unknown }).error === 'string'
    ) {
      detail = (body as { error: string }).error;
    }
  } catch {
    // No JSON body (a proxy error page, an empty response).
  }
  throw new ApiError(
    res.status,
    `${method} ${path} failed: ${res.status} ${res.statusText}`,
    detail
  );
}

async function get<T>(path: string, params?: Record<string, string | number>): Promise<T> {
  const url = new URL(path, 'http://localhost'); // URL for param building only
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      url.searchParams.set(k, String(v));
    }
  }
  const res = await fetch(`${BASE}${url.pathname}${url.search}`);
  if (!res.ok) await fail('GET', path, res);
  return res.json() as Promise<T>;
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  });
  if (!res.ok) await fail('POST', path, res);
  return res.json() as Promise<T>;
}

// --- Routes ---

export const routes = {
  get: (id: string) => get<ApiRoute>(`/routes/${id}`).then(apiRouteToRoute),

  conditions: (id: string, departureAt: string, speedModel = 'elevation') =>
    get<RouteConditionsResponse>(`/routes/${id}/conditions`, {
      departure_at: departureAt,
      speed_model: speedModel
    })
};

// --- Discovery ---

async function getDiscoveryRoutes(
  path: string,
  params: Record<string, string | number>
): Promise<RouteListResponse> {
  const raw = await get<DiscoveryListResponse>(path, params);
  return { routes: (raw.routes ?? []).map(discoveryRouteToRoute), nextCursor: null };
}

export const discovery = {
  viewport: (p: DiscoverViewportParams) =>
    getDiscoveryRoutes('/discover/viewport', { bbox: p.bbox }),

  suggested: (p: DiscoverSuggestedParams) =>
    getDiscoveryRoutes('/discover/suggested', {
      lat: p.lat,
      lon: p.lon,
      departure_at: p.departureAt
    })
};

// --- Routing ---

export const routing = {
  directions: (req: DirectionsRequest) => post<DirectionsResponse>('/routing/directions', req),

  interpretIntent: (text: string, preferences: RoutingPreferences) =>
    post<RouteIntentResponse>('/routing/intent', { text, preferences }),

  conditions: (
    geometry: GeoLineString,
    elevationProfile: Array<{ distance_m: number; elevation_m: number }>,
    departureAt: string
  ) =>
    post<RouteConditionsResponse>('/routing/conditions', {
      geometry,
      elevation_profile: elevationProfile,
      departure_at: departureAt,
      speed_model: 'elevation'
    })
};

// --- Weather ---

export const weather = {
  grid: (bbox: string, at: string) => get<WeatherGridResponse>('/weather/grid', { bbox, at })
};

// --- Reviews ---

export const reviews = {
  list: (routeId: string, cursor?: string) =>
    get<ReviewListResponse>(`/routes/${routeId}/reviews`, cursor ? { cursor } : undefined)
};

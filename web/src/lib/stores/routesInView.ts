// web/src/lib/stores/routesInView.ts
//
// Curated routes in the viewport (frame A's peek bar and frame I's list),
// plus the per-route detail and conditions caches the route card (frame D)
// reads. The list follows the map; conditions follow the departure time.
import { writable, get } from 'svelte/store';
import { discovery, routes as routesApi } from '$lib/api/client';
import type { Route, RouteConditionSegment } from '$lib/api/types';

export const routesInView = writable<Route[]>([]);

// The featured set (routes.route.featured), listed under Plan / Describe in
// the search panel (decided 2026-10-01). Never pinned in the viewport list
// and not shown at rest; loaded once per departure time.
export const featuredRoutes = writable<Route[]>([]);
let featuredFor = '';

export async function refreshFeatured(
  centre: { lat: number; lon: number },
  departureAt: string
): Promise<void> {
  if (featuredFor === departureAt && get(featuredRoutes).length > 0) return;
  featuredFor = departureAt;
  try {
    const res = await discovery.suggested({ ...centre, departureAt });
    if (featuredFor === departureAt) featuredRoutes.set(res.routes);
  } catch {
    // The section simply stays empty; the viewport list is unaffected.
  }
}
export const routesInViewLoading = writable<boolean>(false);
export const routesInViewError = writable<string | null>(null);

export const routeDetails = writable<Map<string, Route>>(new Map());
export const routeConditions = writable<Map<string, RouteConditionSegment[]>>(new Map());
/** The departure the conditions cache was computed for. */
let conditionsFor = '';

let viewportSeq = 0;

export async function refreshRoutesInView(bbox: string): Promise<void> {
  const seq = ++viewportSeq;
  if (get(routesInView).length === 0) routesInViewLoading.set(true);
  routesInViewError.set(null);
  try {
    const res = await discovery.viewport({ bbox });
    if (seq !== viewportSeq) return;
    routesInView.set(res.routes);
  } catch (e) {
    if (seq !== viewportSeq) return;
    const msg = e instanceof Error ? e.message : String(e);
    routesInViewError.set(msg.includes('Failed to fetch') ? "Can't reach Komorebi" : msg);
  } finally {
    if (seq === viewportSeq) routesInViewLoading.set(false);
  }
}

const detailInFlight = new Map<string, Promise<Route | null>>();

/** Full route (geometry, waypoints) — fetched once per id. */
export function ensureDetail(id: string): Promise<Route | null> {
  const cached = get(routeDetails).get(id);
  if (cached) return Promise.resolve(cached);
  const pending = detailInFlight.get(id);
  if (pending) return pending;
  const p = routesApi
    .get(id)
    .then((route) => {
      routeDetails.update((m) => new Map(m).set(id, route));
      return route;
    })
    .catch(() => null)
    .finally(() => detailInFlight.delete(id));
  detailInFlight.set(id, p);
  return p;
}

const conditionsInFlight = new Map<string, Promise<RouteConditionSegment[]>>();

/** Conditions at `departureAt`; the cache resets when the departure changes. */
export function ensureConditions(
  id: string,
  departureAt: string
): Promise<RouteConditionSegment[]> {
  if (conditionsFor !== departureAt) {
    conditionsFor = departureAt;
    routeConditions.set(new Map());
    conditionsInFlight.clear();
  }
  const cached = get(routeConditions).get(id);
  if (cached) return Promise.resolve(cached);
  const pending = conditionsInFlight.get(id);
  if (pending) return pending;
  const p = routesApi
    .conditions(id, departureAt)
    .then((c) => {
      const segs = c.segments ?? [];
      if (conditionsFor === departureAt) routeConditions.update((m) => new Map(m).set(id, segs));
      return segs;
    })
    .catch(() => [] as RouteConditionSegment[])
    .finally(() => conditionsInFlight.delete(id));
  conditionsInFlight.set(id, p);
  return p;
}

// web/tests/helpers/api.ts
//
// Stubs for the Komorebi API so UI tests never depend on a running backend
// or on live weather. Shapes follow web/src/lib/api/types.ts.
import type { Page } from '@playwright/test';

const PALACE_LOOP: [number, number, number][] = [
  [139.7528, 35.6852, 20],
  [139.7592, 35.6873, 22],
  [139.7621, 35.6908, 25],
  [139.7568, 35.6941, 24],
  [139.7503, 35.6925, 21],
  [139.7478, 35.6888, 19],
  [139.7528, 35.6852, 20]
];

const OUT_AND_BACK: [number, number, number][] = [
  [139.7, 35.66, 10],
  [139.72, 35.665, 14],
  [139.74, 35.672, 18],
  [139.76, 35.68, 30]
];

export const ROUTES = [
  {
    id: 'route-palace',
    name: 'Imperial Palace Loop',
    description: 'The classic 5 km loop around the palace.',
    geometry: PALACE_LOOP,
    distance_m: 4800,
    elevation_gain_m: 113,
    elevation_loss_m: 115,
    difficulty: 'easy',
    status: 'published',
    creator_id: 'seed',
    tags: ['beginner-friendly', 'iconic'],
    waypoints: [
      { name: 'Sakuradamon', type: 'viewpoint', lat: 35.6873, lon: 139.7592, sort_order: 0 },
      { name: 'Otemon', type: 'rest_stop', lat: 35.6941, lon: 139.7568, sort_order: 1 }
    ],
    created_at: '2026-04-10T00:00:00Z',
    updated_at: '2026-04-10T00:00:00Z'
  },
  {
    id: 'route-river',
    name: 'Arakawa Riverside',
    description: 'A flat riverside ride.',
    geometry: OUT_AND_BACK,
    distance_m: 20400,
    elevation_gain_m: 334,
    elevation_loss_m: 337,
    difficulty: 'expert',
    status: 'published',
    creator_id: 'seed',
    tags: ['flat'],
    waypoints: [],
    created_at: '2026-04-10T00:00:00Z',
    updated_at: '2026-04-10T00:00:00Z'
  }
];

function discoveryRoute(r: (typeof ROUTES)[number]) {
  return {
    ...r,
    shade: null,
    conditions: null
  };
}

/** Stub every /api/v1 call the home page makes. */
export async function stubApi(page: Page) {
  await page.route('**/api/v1/**', async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname.replace(/^.*\/api\/v1/, '');
    const json = (body: unknown) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });

    if (path.startsWith('/discover/')) return json({ routes: ROUTES.map(discoveryRoute) });
    const m = path.match(/^\/routes\/([^/]+)\/conditions$/);
    if (m)
      return json({
        route_id: m[1],
        departure_at: url.searchParams.get('departure_at'),
        segments: []
      });
    const r = path.match(/^\/routes\/([^/]+)$/);
    if (r) {
      const found = ROUTES.find((x) => x.id === r[1]);
      return found ? json(found) : route.fulfill({ status: 404, body: '{}' });
    }
    if (path.startsWith('/weather/')) return json({ hours: [], cells: [] });
    if (path.startsWith('/venues')) return json({ tags: [] });
    return route.fulfill({ status: 404, contentType: 'application/json', body: '{}' });
  });
  // Tiles and the basemap are not under test; fail them fast and quietly.
  await page.route(/basemaps\.cartocdn\.com|localhost:3000|\/tiles\//, (route) => route.abort());
}

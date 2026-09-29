// Shared fixture for UI and layout tests (rules: tests/AGENTS.md).
//
// - Animations and transitions are disabled globally.
// - The network is sealed: API calls get deterministic stubs, and every
//   third-party request (basemap tiles, glyphs, Nominatim, Martin) is
//   aborted so runs never depend on the internet.
import { test as base, expect, type Page, type Route } from '@playwright/test';

export const DISCOVERY_ROUTE = {
  route_id: '00000000-0000-4000-8000-000000000001',
  name: 'Sumida riverside loop',
  description: 'Along the Sumida from Asakusa-bashi',
  distance_m: 24100,
  elevation_gain_m: 118,
  elevation_loss_m: 118,
  difficulty: 'moderate',
  status: 'published',
  tags: ['riverside'],
  dist_from_m: 4600
};

export const DIRECTIONS = {
  alternatives: [
    {
      profile: 'suggested',
      label: 'Suggested',
      total_distance_km: 3.2,
      total_duration_s: 900,
      elevation_gain_m: 12,
      elevation_loss_m: 10,
      elevation_profile: [],
      legs: [],
      geometry: {
        type: 'LineString',
        coordinates: [
          [139.7967, 35.7148],
          [139.7901, 35.7071],
          [139.7745, 35.6987]
        ]
      }
    }
  ]
};

async function stubApi(route: Route) {
  const url = new URL(route.request().url());
  const path = url.pathname.replace(/^\/api\/v1/, '');
  const json = (body: unknown) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });

  if (path.startsWith('/discover/')) return json({ routes: [DISCOVERY_ROUTE] });
  if (path === '/routing/directions') return json(DIRECTIONS);
  if (path === '/weather/grid') return json({ valid_at: new Date().toISOString(), cells: [] });
  // Everything else (conditions, weather point, route detail) is "offline".
  return route.fulfill({ status: 503, body: 'stubbed offline' });
}

export async function sealNetwork(page: Page) {
  await page.route('**/*', (route) => {
    const url = new URL(route.request().url());
    const local = url.hostname === '127.0.0.1' || url.hostname === 'localhost';
    if (!local) return route.abort('blockedbyclient');
    if (url.pathname.startsWith('/api/')) return stubApi(route);
    if (url.pathname.startsWith('/tiles/') || url.port === '3000') {
      return route.abort('blockedbyclient');
    }
    return route.continue();
  });
}

export const test = base.extend({
  page: async ({ page }, use) => {
    await sealNetwork(page);
    await page.addInitScript(() => {
      const style = document.createElement('style');
      style.textContent =
        '*, *::before, *::after { animation: none !important; transition: none !important; }';
      document.addEventListener('DOMContentLoaded', () => document.head.appendChild(style));
    });
    await use(page);
  }
});

export { expect };

// web/src/routes/+page.server.ts
import type { PageServerLoad } from './$types';
import type { Route } from '$lib/api/types';
import { discoveryRouteToRoute } from '$lib/api/types';
import { env } from '$env/dynamic/private';

export const load: PageServerLoad = async ({ fetch }) => {
  const DEFAULT_LAT = 35.6895;
  const DEFAULT_LON = 139.6917;
  const now = new Date();
  now.setMinutes(0, 0, 0);
  const departureAt = now.toISOString();

  try {
    const apiURL = env.API_URL ?? 'http://127.0.0.1:8080';
    const res = await fetch(
      `${apiURL}/api/v1/discover/suggested?lat=${DEFAULT_LAT}&lon=${DEFAULT_LON}&departure_at=${encodeURIComponent(departureAt)}`
    );
    if (res.ok) {
      const data = await res.json();
      const routes: Route[] = (data.routes ?? []).map(discoveryRouteToRoute);
      return { routes, departureAt };
    }
  } catch {
    // API not running in SSR context; return empty — client hydrates
  }

  return { routes: [] as Route[], departureAt };
};

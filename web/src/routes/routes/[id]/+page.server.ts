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

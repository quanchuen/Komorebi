// web/src/lib/stores/discovery.ts
//
// Curated routes handed over by server-side rendering; the explore page
// seeds the routes-in-view list from them until the first viewport query.
import { writable } from 'svelte/store';
import type { Route } from '$lib/api/types';

export const discoveryRoutes = writable<Route[]>([]);

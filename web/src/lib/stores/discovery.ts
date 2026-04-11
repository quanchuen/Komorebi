// web/src/lib/stores/discovery.ts
import { writable } from 'svelte/store';
import type { Route } from '$lib/api/types';
import type { Difficulty } from '$lib/api/types';

export interface DiscoveryFilters {
  difficulty: Difficulty | null;
  shade: boolean;
  greenery: boolean;
  searchQuery: string;
}

export const discoveryFilters = writable<DiscoveryFilters>({
  difficulty: null,
  shade: false,
  greenery: false,
  searchQuery: ''
});

export const discoveryRoutes = writable<Route[]>([]);
export const discoveryLoading = writable<boolean>(false);
export const discoveryError = writable<string | null>(null);

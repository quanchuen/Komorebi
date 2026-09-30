// web/src/lib/stores/recents.ts
//
// RECENT rows of the search panel (frame B) and the route card's Save
// (frame D). Browser-only, per ADR 0006; tolerant of disabled or malformed
// storage. Only place names and route ids are kept — never a live fix.
import { browser } from '$app/environment';
import { writable, type Writable } from 'svelte/store';

export interface RecentPlace {
  kind: 'place';
  name: string;
  detail: string;
  lat: number;
  lon: number;
  at: number;
}

export interface SavedRoute {
  kind: 'route';
  id: string;
  name: string;
  distanceM: number;
  at: number;
}

/** A planned trip, kept as its shareable stop URL (frame G's Save). */
export interface SavedPlan {
  kind: 'plan';
  url: string;
  name: string;
  distanceM: number;
  at: number;
}

function persistedList<T>(key: string, validate: (v: unknown) => v is T): Writable<T[]> {
  let initial: T[] = [];
  if (browser) {
    try {
      const raw = JSON.parse(localStorage.getItem(key) ?? '[]');
      if (Array.isArray(raw)) initial = raw.filter(validate);
    } catch {
      initial = [];
    }
  }
  const store = writable<T[]>(initial);
  if (browser) {
    store.subscribe((value) => {
      try {
        localStorage.setItem(key, JSON.stringify(value));
      } catch {
        // Storage disabled or full: the list still works this session.
      }
    });
  }
  return store;
}

const isPlace = (v: unknown): v is RecentPlace =>
  typeof v === 'object' &&
  v !== null &&
  (v as RecentPlace).kind === 'place' &&
  typeof (v as RecentPlace).name === 'string' &&
  Number.isFinite((v as RecentPlace).lat) &&
  Number.isFinite((v as RecentPlace).lon);

const isRoute = (v: unknown): v is SavedRoute =>
  typeof v === 'object' &&
  v !== null &&
  (v as SavedRoute).kind === 'route' &&
  typeof (v as SavedRoute).id === 'string' &&
  typeof (v as SavedRoute).name === 'string';

const isPlan = (v: unknown): v is SavedPlan =>
  typeof v === 'object' &&
  v !== null &&
  (v as SavedPlan).kind === 'plan' &&
  typeof (v as SavedPlan).url === 'string' &&
  typeof (v as SavedPlan).name === 'string';

export const recentPlaces = persistedList<RecentPlace>('komorebi:recent-places:v1', isPlace);
export const savedRoutes = persistedList<SavedRoute>('komorebi:saved-routes:v1', isRoute);
export const savedPlans = persistedList<SavedPlan>('komorebi:saved-plans:v1', isPlan);

export function toggleSavedPlan(plan: { url: string; name: string; distanceM: number }): void {
  savedPlans.update((list) =>
    list.some((p) => p.url === plan.url)
      ? list.filter((p) => p.url !== plan.url)
      : [{ kind: 'plan' as const, at: Date.now(), ...plan }, ...list]
  );
}

const MAX_RECENT = 6;

export function rememberPlace(place: Omit<RecentPlace, 'kind' | 'at'>): void {
  recentPlaces.update((list) => {
    const rest = list.filter((p) => p.name !== place.name);
    return [{ kind: 'place' as const, at: Date.now(), ...place }, ...rest].slice(0, MAX_RECENT);
  });
}

export function toggleSavedRoute(route: { id: string; name: string; distanceM: number }): void {
  savedRoutes.update((list) =>
    list.some((r) => r.id === route.id)
      ? list.filter((r) => r.id !== route.id)
      : [{ kind: 'route' as const, at: Date.now(), ...route }, ...list]
  );
}

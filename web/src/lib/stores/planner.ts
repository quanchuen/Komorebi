// web/src/lib/stores/planner.ts
import { browser } from '$app/environment';
import { writable, type Writable } from 'svelte/store';
import type { RoutingPreferences } from '$lib/api/types';

const RIDER_PREFERENCES_KEY = 'komorebi:rider-preferences:v1';

const defaultRoutingPreferences: RoutingPreferences = {
  shade: 0.5,
  greenery: 0.5,
  wind: 0.5
};

function clampWeight(value: unknown, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value)
    ? Math.min(1, Math.max(0, value))
    : fallback;
}

function loadRoutingPreferences(): RoutingPreferences {
  if (!browser) return defaultRoutingPreferences;

  try {
    const saved = JSON.parse(
      localStorage.getItem(RIDER_PREFERENCES_KEY) ?? '{}'
    ) as Partial<RoutingPreferences>;
    return {
      shade: clampWeight(saved.shade, defaultRoutingPreferences.shade),
      greenery: clampWeight(saved.greenery, defaultRoutingPreferences.greenery),
      wind: clampWeight(saved.wind, defaultRoutingPreferences.wind)
    };
  } catch {
    return defaultRoutingPreferences;
  }
}

function persistedRoutingPreferences(): Writable<RoutingPreferences> {
  const store = writable<RoutingPreferences>(loadRoutingPreferences());

  if (browser) {
    store.subscribe((value) => {
      try {
        localStorage.setItem(RIDER_PREFERENCES_KEY, JSON.stringify(value));
      } catch {
        // Storage may be disabled, unavailable, or full. Preferences remain
        // usable for the current session through the in-memory store.
      }
    });
  }

  return store;
}

export const plannerPreferences = persistedRoutingPreferences();

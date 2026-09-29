// web/src/lib/stores/ui.ts
//
// Collapsed/expanded state for the floating map panels. Each panel persists
// its own flag so the layout a rider folds into place survives reloads; the
// stores degrade to in-memory when storage is disabled, full, or malformed.
import { browser } from '$app/environment';
import { writable, type Writable } from 'svelte/store';

// The Data sources sheet (opened from the attribution pill, the first-ride
// disclaimer, and later Menu → About). Session-only.
export const dataSourcesOpen = writable<boolean>(false);

/** A boolean flag persisted under a versioned localStorage key. */
export function persistedFlag(key: string, fallback = false): Writable<boolean> {
  let initial = fallback;
  if (browser) {
    try {
      const raw = localStorage.getItem(key);
      if (raw === '1') initial = true;
      else if (raw === '0') initial = false;
    } catch {
      // Storage unavailable; keep the default.
    }
  }

  const store = writable<boolean>(initial);

  if (browser) {
    store.subscribe((value) => {
      try {
        localStorage.setItem(key, value ? '1' : '0');
      } catch {
        // Storage may be disabled or full; the toggle still works this session.
      }
    });
  }

  return store;
}

export const navCardCollapsed = persistedFlag('komorebi:nav-card-collapsed:v1');
export const resultsPanelCollapsed = persistedFlag('komorebi:results-panel-collapsed:v1');
export const weatherTimelineCollapsed = persistedFlag('komorebi:weather-timeline-collapsed:v1');

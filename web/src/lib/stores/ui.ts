// web/src/lib/stores/ui.ts
//
// Collapsed/expanded state for the floating map panels. Each panel persists
// its own flag so the layout a rider folds into place survives reloads; the
// stores degrade to in-memory when storage is disabled, full, or malformed.
import { browser } from '$app/environment';
import { writable, type Writable } from 'svelte/store';

function persistedCollapsed(key: string, fallback = false): Writable<boolean> {
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

export const navCardCollapsed = persistedCollapsed('komorebi:nav-card-collapsed:v1');
export const resultsPanelCollapsed = persistedCollapsed('komorebi:results-panel-collapsed:v1');
export const weatherTimelineCollapsed = persistedCollapsed(
  'komorebi:weather-timeline-collapsed:v1'
);

// The Data sources sheet (frame E). Opened from the attribution pill and from
// the first-ride disclaimer; not persisted.
export const dataSourcesOpen = writable<boolean>(false);

// First-ride disclaimer (frame F). `seen` persists so it shows once; `request`
// is set by whoever wants it shown — with an onAccept to continue into the
// ride once the rider taps "I understand", or without one when simply
// re-reading it.
export const firstRideDisclaimerSeen = persistedCollapsed('komorebi:first-ride-disclaimer-seen:v1');
export const firstRideDisclaimerRequest = writable<{ onAccept?: () => void } | null>(null);

/** Show the disclaimer before `ride` runs, or run it straight away once seen. */
export function requestRide(ride: () => void): void {
  let seen = false;
  firstRideDisclaimerSeen.subscribe((v) => (seen = v))();
  if (seen) ride();
  else firstRideDisclaimerRequest.set({ onAccept: ride });
}

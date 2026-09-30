// web/src/lib/stores/ui.ts
//
// Chrome state for the explore page (Penpot frames A–D, G, H, I). One
// floating panel at a time on desktop, one sheet at a time on mobile; the
// map never hosts more than one panel's worth of chrome.
import { browser } from '$app/environment';
import { writable, type Writable } from 'svelte/store';

function persistedFlag(key: string, fallback = false): Writable<boolean> {
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

/**
 * The left panel (desktop) or bottom sheet (mobile):
 *  none     — frame A, the resting map;
 *  search   — frame B, under the search pill;
 *  plan     — frame G, plan a route;
 *  describe — frame H, describe a ride;
 *  routes   — frame I, the routes-in-view list.
 * The route card (frame D) shows whenever a curated route is highlighted.
 */
export type ExplorePanel = 'none' | 'search' | 'plan' | 'describe' | 'routes';
export const explorePanel = writable<ExplorePanel>('none');

// Layers popover (desktop) / sheet (mobile), frame C.
export const layersOpen = writable<boolean>(false);
// The time chip expanded into the scrubber.
export const scrubberOpen = writable<boolean>(false);
// Menu under the search pill's ☰ (About, Data sources) until frame M ships.
export const menuOpen = writable<boolean>(false);

// One-line feedback ("Link copied"), cleared by whoever set it.
export const toast = writable<string | null>(null);
let toastTimer: ReturnType<typeof setTimeout> | undefined;
export function showToast(message: string, ms = 2400): void {
  toast.set(message);
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => toast.set(null), ms);
}

// The Data sources sheet (frame E). Opened from the attribution pill and from
// the first-ride disclaimer; not persisted.
export const dataSourcesOpen = writable<boolean>(false);

// First-ride disclaimer (frame F). `seen` persists so it shows once; `request`
// is set by whoever wants it shown — with an onAccept to continue into the
// ride once the rider taps "I understand", or without one when simply
// re-reading it.
export const firstRideDisclaimerSeen = persistedFlag('komorebi:first-ride-disclaimer-seen:v1');
export const firstRideDisclaimerRequest = writable<{ onAccept?: () => void } | null>(null);

/** Show the disclaimer before `ride` runs, or run it straight away once seen. */
export function requestRide(ride: () => void): void {
  let seen = false;
  firstRideDisclaimerSeen.subscribe((v) => (seen = v))();
  if (seen) ride();
  else firstRideDisclaimerRequest.set({ onAccept: ride });
}

// Breakpoint the layout switches at: below it the panels become sheets
// (Penpot mobile frames, 390 wide); at and above it the desktop frames apply.
export const MOBILE_MAX_PX = 767;
export const isMobile = writable<boolean>(false);
if (browser) {
  const mq = window.matchMedia(`(max-width: ${MOBILE_MAX_PX}px)`);
  isMobile.set(mq.matches);
  mq.addEventListener('change', (e) => isMobile.set(e.matches));
}

// Whether POST /routing/intent is configured on this server (503 otherwise);
// the search panel greys out "Describe a ride" once it learns it is not.
export const describeAvailable = writable<boolean>(true);

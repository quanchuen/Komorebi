// web/src/lib/stores/disclaimer.ts
//
// First-ride disclaimer (spec frame F): shown once, when the rider first
// starts a ride; always reachable again from Menu → About.
import { browser } from '$app/environment';
import { get, writable } from 'svelte/store';
import { persistedFlag } from './ui';

export type DisclaimerLocale = 'en' | 'ja';

// Bump the version when the copy changes materially (e.g. after legal
// review) so every rider sees the new text once.
export const rideDisclaimerAccepted = persistedFlag('komorebi:ride-disclaimer-accepted:v1');

export const rideDisclaimerOpen = writable<boolean>(false);

function deviceLocale(): DisclaimerLocale {
  if (!browser) return 'en';
  const langs = navigator.languages?.length ? navigator.languages : [navigator.language];
  return langs.some((l) => l?.toLowerCase().startsWith('ja')) ? 'ja' : 'en';
}

/** Follows the device locale; the dialog's EN / 日本語 switch overrides it. */
export const disclaimerLocale = writable<DisclaimerLocale>(deviceLocale());

let pending: (() => void) | null = null;

/**
 * Run `start` once the rider has accepted the disclaimer. The first time, the
 * dialog opens and `start` runs only after "I understand"; dismissing the
 * dialog cancels the ride.
 */
export function requestRide(start: () => void) {
  if (get(rideDisclaimerAccepted)) {
    start();
    return;
  }
  pending = start;
  rideDisclaimerOpen.set(true);
}

/** Open the disclaimer for reading only (Menu → About). */
export function showRideDisclaimer() {
  pending = null;
  rideDisclaimerOpen.set(true);
}

export function acceptRideDisclaimer() {
  rideDisclaimerAccepted.set(true);
  rideDisclaimerOpen.set(false);
  const start = pending;
  pending = null;
  start?.();
}

export function dismissRideDisclaimer() {
  pending = null;
  rideDisclaimerOpen.set(false);
}

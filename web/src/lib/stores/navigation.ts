import { browser } from '$app/environment';
import { writable } from 'svelte/store';

export type ForegroundNavigationStatus = 'idle' | 'requesting' | 'active' | 'paused' | 'error';

export interface ForegroundNavigationState {
  status: ForegroundNavigationStatus;
  position: GeolocationCoordinates | null;
  error: string | null;
  offRoute: boolean;
  distanceFromRouteM: number | null;
  remainingDistanceM: number | null;
  wakeLockActive: boolean;
}

const initialState: ForegroundNavigationState = {
  status: 'idle',
  position: null,
  error: null,
  offRoute: false,
  distanceFromRouteM: null,
  remainingDistanceM: null,
  wakeLockActive: false
};

export const foregroundNavigation = writable<ForegroundNavigationState>(initialState);

let watchId: number | null = null;
let wakeLock: WakeLockSentinel | null = null;
let activeRoute: [number, number][] = [];

function distanceM(a: [number, number], b: [number, number]): number {
  const earthRadiusM = 6_371_000;
  const lat1 = (a[1] * Math.PI) / 180;
  const lat2 = (b[1] * Math.PI) / 180;
  const dLat = ((b[1] - a[1]) * Math.PI) / 180;
  const dLon = ((b[0] - a[0]) * Math.PI) / 180;
  const h = Math.sin(dLat / 2) ** 2 + Math.cos(lat1) * Math.cos(lat2) * Math.sin(dLon / 2) ** 2;
  return 2 * earthRadiusM * Math.asin(Math.sqrt(h));
}

function routeProgress(position: [number, number]): {
  distanceFromRouteM: number | null;
  remainingDistanceM: number | null;
} {
  if (activeRoute.length === 0) {
    return { distanceFromRouteM: null, remainingDistanceM: null };
  }

  let nearestIndex = 0;
  let nearestDistance = Number.POSITIVE_INFINITY;
  for (let index = 0; index < activeRoute.length; index += 1) {
    const candidateDistance = distanceM(position, activeRoute[index]);
    if (candidateDistance < nearestDistance) {
      nearestDistance = candidateDistance;
      nearestIndex = index;
    }
  }

  let remainingDistanceM = distanceM(position, activeRoute[nearestIndex]);
  for (let index = nearestIndex; index < activeRoute.length - 1; index += 1) {
    remainingDistanceM += distanceM(activeRoute[index], activeRoute[index + 1]);
  }

  return { distanceFromRouteM: nearestDistance, remainingDistanceM };
}

async function requestWakeLock(): Promise<void> {
  if (!browser || document.visibilityState !== 'visible' || wakeLock) return;
  try {
    wakeLock = (await navigator.wakeLock?.request('screen')) ?? null;
    if (wakeLock) {
      foregroundNavigation.update((state) => ({ ...state, wakeLockActive: true }));
      wakeLock.addEventListener('release', () => {
        wakeLock = null;
        foregroundNavigation.update((state) => ({ ...state, wakeLockActive: false }));
      });
    }
  } catch {
    foregroundNavigation.update((state) => ({ ...state, wakeLockActive: false }));
  }
}

async function releaseWakeLock(): Promise<void> {
  const current = wakeLock;
  wakeLock = null;
  if (current && !current.released) await current.release().catch(() => {});
  foregroundNavigation.update((state) => ({ ...state, wakeLockActive: false }));
}

function handleVisibilityChange(): void {
  if (watchId === null) return;
  if (document.visibilityState === 'visible') {
    foregroundNavigation.update((state) => ({ ...state, status: 'active' }));
    void requestWakeLock();
  } else {
    foregroundNavigation.update((state) => ({ ...state, status: 'paused' }));
    void releaseWakeLock();
  }
}

export function setNavigationRoute(route: [number, number][]): void {
  activeRoute = route;
}

export function startForegroundNavigation(): void {
  if (!browser || watchId !== null) return;
  if (!('geolocation' in navigator)) {
    foregroundNavigation.set({
      ...initialState,
      status: 'error',
      error: 'Location is unavailable'
    });
    return;
  }

  foregroundNavigation.set({ ...initialState, status: 'requesting' });
  document.addEventListener('visibilitychange', handleVisibilityChange);
  void requestWakeLock();

  watchId = navigator.geolocation.watchPosition(
    ({ coords }) => {
      const progress = routeProgress([coords.longitude, coords.latitude]);
      foregroundNavigation.set({
        status: document.visibilityState === 'visible' ? 'active' : 'paused',
        position: coords,
        error: null,
        offRoute: progress.distanceFromRouteM !== null && progress.distanceFromRouteM > 60,
        distanceFromRouteM: progress.distanceFromRouteM,
        remainingDistanceM: progress.remainingDistanceM,
        wakeLockActive: wakeLock !== null
      });
    },
    (error) => {
      foregroundNavigation.update((state) => ({
        ...state,
        status: 'error',
        error:
          error.code === error.PERMISSION_DENIED
            ? 'Location permission was denied'
            : 'Current location is unavailable'
      }));
    },
    { enableHighAccuracy: true, maximumAge: 2_000, timeout: 15_000 }
  );
}

export function stopForegroundNavigation(): void {
  if (browser && watchId !== null) navigator.geolocation.clearWatch(watchId);
  watchId = null;
  activeRoute = [];
  if (browser) document.removeEventListener('visibilitychange', handleVisibilityChange);
  void releaseWakeLock();
  foregroundNavigation.set(initialState);
}

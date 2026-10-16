// web/src/lib/utils/mapMarkers.ts
//
// DOM builders for the map markers (Penpot page "Markers"). Every marker is a
// plain element styled by the :global(.km-marker…) rules in Map.svelte with
// the chrome / route tokens from app.css, so no colour is written here.
import type { VenueType } from '$lib/stores/map';
import { ICONS, iconSvg, type IconName } from '$lib/utils/icons';

export type MarkerKind = 'start' | 'end' | 'loop' | 'waypoint';

/** Start and end closer than this are one loop (one combined marker). */
export const LOOP_M = 100;
export type MarkerSize = 20 | 24 | 28;

/** Sizes by zoom: z ≤ 13 → 20px · z14–15 → 24px · z16+ → 28px. */
export function markerSizeForZoom(zoom: number): MarkerSize {
  if (zoom < 14) return 20;
  if (zoom < 16) return 24;
  return 28;
}

export const VENUE_ICON: Record<VenueType, IconName> = {
  konbini: 'konbini',
  cafe: 'cafe',
  water: 'water',
  repair: 'repair'
};

export const VENUE_LABEL: Record<VenueType, string> = {
  konbini: 'Konbini',
  cafe: 'Café',
  water: 'Water',
  repair: 'Repair'
};

/** Start, end, combined loop (start + small end badge) or numbered waypoint. */
export function buildRouteMarker(
  kind: MarkerKind,
  size: MarkerSize,
  opts: { number?: number } = {}
): HTMLElement {
  const el = document.createElement('div');
  el.className = 'km-marker';
  el.dataset.kind = kind;
  el.dataset.size = String(size);
  el.setAttribute('role', 'img');
  const label =
    kind === 'start'
      ? 'Start'
      : kind === 'end'
        ? 'End'
        : kind === 'loop'
          ? 'Start and end'
          : `Stop ${opts.number ?? ''}`.trim();
  el.setAttribute('aria-label', label);

  const disc = document.createElement('div');
  disc.className = 'km-marker__disc';
  if (kind === 'waypoint') {
    disc.textContent = String(opts.number ?? '');
  } else {
    disc.innerHTML = iconSvg(kind === 'end' ? 'flag' : 'bicycle', 2.2);
  }
  el.appendChild(disc);

  if (kind === 'loop') {
    const badge = document.createElement('div');
    badge.className = 'km-marker__badge';
    badge.innerHTML = iconSvg('flag', 2.6);
    el.appendChild(badge);
  }
  return el;
}

/** The selected venue: near-black fill, white icon and a label chip. */
export function buildSelectedVenueMarker(type: VenueType, name: string): HTMLElement {
  const el = document.createElement('div');
  el.className = 'km-venue';
  el.setAttribute('role', 'img');
  el.setAttribute('aria-label', `${name || VENUE_LABEL[type]} (selected)`);

  const disc = document.createElement('div');
  disc.className = 'km-venue__disc';
  disc.innerHTML = iconSvg(VENUE_ICON[type], 1.8);
  el.appendChild(disc);

  const chip = document.createElement('div');
  chip.className = 'km-venue__chip';
  chip.textContent = name || VENUE_LABEL[type];
  el.appendChild(chip);
  return el;
}

/**
 * Rasterise a venue pin (28px white circle, 1px line ring, line icon) for a
 * MapLibre symbol layer. Colours are passed in by Map.svelte, the one place
 * allowed to hold map hex.
 */
export async function renderVenuePinImage(
  type: VenueType,
  colors: { fill: string; ring: string; icon: string },
  pixelRatio = 2
): Promise<ImageData> {
  const px = 28 * pixelRatio;
  const svg =
    `<svg xmlns="http://www.w3.org/2000/svg" width="${px}" height="${px}" viewBox="0 0 28 28">` +
    `<circle cx="14" cy="14" r="13.5" fill="${colors.fill}" stroke="${colors.ring}" stroke-width="1"/>` +
    `<g transform="translate(6 6) scale(0.6667)" stroke="${colors.icon}" fill="none" stroke-width="1.8" ` +
    `stroke-linecap="round" stroke-linejoin="round">${ICONS[VENUE_ICON[type]]}</g></svg>`;
  const url = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
  const img = new Image(px, px);
  await new Promise<void>((resolve, reject) => {
    img.onload = () => resolve();
    img.onerror = () => reject(new Error(`venue pin ${type} failed to render`));
    img.src = url;
  });
  const canvas = document.createElement('canvas');
  canvas.width = px;
  canvas.height = px;
  const ctx = canvas.getContext('2d');
  if (!ctx) throw new Error('canvas 2d context unavailable');
  ctx.drawImage(img, 0, 0, px, px);
  return ctx.getImageData(0, 0, px, px);
}

/** Metres between two [lon, lat] points (equirectangular; fine at city scale). */
export function metresBetween(a: [number, number], b: [number, number]): number {
  const rad = Math.PI / 180;
  const x = (b[0] - a[0]) * rad * Math.cos(((a[1] + b[1]) / 2) * rad);
  const y = (b[1] - a[1]) * rad;
  return Math.sqrt(x * x + y * y) * 6371000;
}

/** Shortest distance in metres from a point to a polyline. */
export function metresToPolyline(p: [number, number], line: [number, number][]): number {
  const rad = Math.PI / 180;
  const cosLat = Math.cos(p[1] * rad);
  const toXY = (c: [number, number]): [number, number] => [
    (c[0] - p[0]) * rad * cosLat * 6371000,
    (c[1] - p[1]) * rad * 6371000
  ];
  let best = Infinity;
  let prev = toXY(line[0]);
  for (let i = 1; i < line.length; i++) {
    const cur = toXY(line[i]);
    const dx = cur[0] - prev[0];
    const dy = cur[1] - prev[1];
    const len2 = dx * dx + dy * dy;
    const t = len2 === 0 ? 0 : Math.max(0, Math.min(1, -(prev[0] * dx + prev[1] * dy) / len2));
    const qx = prev[0] + t * dx;
    const qy = prev[1] + t * dy;
    best = Math.min(best, Math.sqrt(qx * qx + qy * qy));
    prev = cur;
  }
  return best;
}

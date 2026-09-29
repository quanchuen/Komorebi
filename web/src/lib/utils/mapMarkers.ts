// web/src/lib/utils/mapMarkers.ts
//
// Map marker images (Penpot page "Markers"), drawn on a canvas so MapLibre can
// receive them synchronously from `styleimagemissing`. Icon paths are Lucide
// (ISC licence) line icons on a 24×24 grid. Colours come from conditionColors.
import type { ExpressionSpecification } from 'maplibre-gl';
import { MARKER_COLORS, POSITION_COLOR, ROUTE_COLORS } from './conditionColors';

const PIXEL_RATIO = 2;

/** Endpoint marker diameter at z16+; smaller zooms scale it via icon-size. */
export const ENDPOINT_SIZE = 28;
/** icon-size for endpoints: z ≤ 13: 20px · z14–15: 24px · z16+: 28px. */
export const ENDPOINT_ICON_SIZE: ExpressionSpecification = [
  'step',
  ['zoom'],
  20 / ENDPOINT_SIZE,
  14,
  24 / ENDPOINT_SIZE,
  16,
  1
];

export type VenueIcon = 'konbini' | 'cafe' | 'water' | 'repair';

const ICONS: Record<'bike' | 'flag' | VenueIcon, string[]> = {
  bike: [
    'M5.5 14a3.5 3.5 0 1 0 0 7 3.5 3.5 0 1 0 0-7z',
    'M18.5 14a3.5 3.5 0 1 0 0 7 3.5 3.5 0 1 0 0-7z',
    'M15 4a1 1 0 1 0 0 2 1 1 0 1 0 0-2z',
    'M12 17.5V14l-3-3 4-3 2 3h2'
  ],
  flag: ['M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1z', 'M4 22v-7'],
  konbini: [
    'M6 2 3 6v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V6l-3-4Z',
    'M3 6h18',
    'M16 10a4 4 0 0 1-8 0'
  ],
  cafe: [
    'M17 8h1a4 4 0 1 1 0 8h-1',
    'M3 8h14v9a4 4 0 0 1-4 4H7a4 4 0 0 1-4-4Z',
    'M6 2v2',
    'M10 2v2',
    'M14 2v2'
  ],
  water: [
    'M12 22a7 7 0 0 0 7-7c0-2-1-3.9-3-5.5s-3.5-4-4-6.5c-.5 2.5-2 4.9-4 6.5C6 11.1 5 13 5 15a7 7 0 0 0 7 7z'
  ],
  repair: [
    'M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z'
  ]
};

interface Canvas {
  ctx: CanvasRenderingContext2D;
  /** Logical (CSS px) size; the backing store is PIXEL_RATIO times larger. */
  size: number;
  c: number;
}

function makeCanvas(size: number): Canvas | null {
  const el = document.createElement('canvas');
  el.width = size * PIXEL_RATIO;
  el.height = size * PIXEL_RATIO;
  const ctx = el.getContext('2d');
  if (!ctx) return null;
  ctx.scale(PIXEL_RATIO, PIXEL_RATIO);
  return { ctx, size, c: size / 2 };
}

function finish({ ctx, size }: Canvas) {
  const px = size * PIXEL_RATIO;
  return { image: ctx.getImageData(0, 0, px, px), options: { pixelRatio: PIXEL_RATIO } };
}

/** Kumo `lg` shadow, softened to fit the marker padding. */
function withShadow(ctx: CanvasRenderingContext2D, draw: () => void) {
  ctx.save();
  ctx.shadowColor = 'rgba(0, 0, 0, 0.18)';
  ctx.shadowBlur = 6;
  ctx.shadowOffsetY = 2;
  draw();
  ctx.restore();
}

function disc(ctx: CanvasRenderingContext2D, x: number, y: number, r: number, fill: string) {
  ctx.beginPath();
  ctx.arc(x, y, r, 0, Math.PI * 2);
  ctx.fillStyle = fill;
  ctx.fill();
}

function ring(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  r: number,
  width: number,
  color: string
) {
  ctx.beginPath();
  ctx.arc(x, y, r - width / 2, 0, Math.PI * 2);
  ctx.lineWidth = width;
  ctx.strokeStyle = color;
  ctx.stroke();
}

function icon(
  ctx: CanvasRenderingContext2D,
  name: keyof typeof ICONS,
  x: number,
  y: number,
  size: number,
  color: string,
  strokePx = 1.5
) {
  const scale = size / 24;
  ctx.save();
  ctx.translate(x - size / 2, y - size / 2);
  ctx.scale(scale, scale);
  ctx.lineWidth = strokePx / scale;
  ctx.lineCap = 'round';
  ctx.lineJoin = 'round';
  ctx.strokeStyle = color;
  for (const d of ICONS[name]) ctx.stroke(new Path2D(d));
  ctx.restore();
}

function drawStart(cv: Canvas) {
  const { ctx, c } = cv;
  const r = ENDPOINT_SIZE / 2;
  withShadow(ctx, () => disc(ctx, c, c, r, MARKER_COLORS.surface));
  ring(ctx, c, c, r, 2.5, ROUTE_COLORS.route);
  icon(ctx, 'bike', c, c, 16, ROUTE_COLORS.route, 1.75);
}

function drawEnd(cv: Canvas) {
  const { ctx, c } = cv;
  const r = ENDPOINT_SIZE / 2;
  withShadow(ctx, () => disc(ctx, c, c, r, MARKER_COLORS.surface));
  disc(ctx, c, c, r - 2, ROUTE_COLORS.route);
  icon(ctx, 'flag', c, c, 14, MARKER_COLORS.surface, 1.75);
}

function drawLoop(cv: Canvas) {
  drawStart(cv);
  // Small end badge offset to the start marker's top-right.
  const { ctx, c } = cv;
  const bx = c + 11;
  const by = c - 11;
  withShadow(ctx, () => disc(ctx, bx, by, 7, MARKER_COLORS.surface));
  disc(ctx, bx, by, 5.5, ROUTE_COLORS.route);
  icon(ctx, 'flag', bx, by, 7, MARKER_COLORS.surface, 1.25);
}

function drawWaypoint(cv: Canvas, n: number) {
  const { ctx, c } = cv;
  const r = 11;
  withShadow(ctx, () => disc(ctx, c, c, r, MARKER_COLORS.surface));
  ring(ctx, c, c, r, 2, ROUTE_COLORS.route);
  ctx.fillStyle = ROUTE_COLORS.route;
  ctx.font = `600 ${n > 9 ? 10 : 11}px 'Inter Variable', ui-sans-serif, system-ui, sans-serif`;
  ctx.textAlign = 'center';
  ctx.textBaseline = 'middle';
  ctx.fillText(String(n), c, c + 0.5);
}

function drawVenue(cv: Canvas, type: VenueIcon, selected: boolean) {
  const { ctx, c } = cv;
  const r = 14;
  withShadow(ctx, () => disc(ctx, c, c, r, selected ? MARKER_COLORS.icon : MARKER_COLORS.surface));
  if (!selected) ring(ctx, c, c, r, 1, MARKER_COLORS.line);
  icon(ctx, type, c, c, 15, selected ? MARKER_COLORS.surface : MARKER_COLORS.icon);
}

/** Heading cone: a soft wedge pointing up (north); rotated by the layer. */
function drawHeading(cv: Canvas) {
  const { ctx, c } = cv;
  const grad = ctx.createRadialGradient(c, c, 0, c, c, c);
  grad.addColorStop(0, POSITION_COLOR);
  grad.addColorStop(1, 'rgba(255, 255, 255, 0)');
  ctx.globalAlpha = 0.45;
  ctx.beginPath();
  ctx.moveTo(c, c);
  ctx.arc(c, c, c, -Math.PI / 2 - Math.PI / 6, -Math.PI / 2 + Math.PI / 6);
  ctx.closePath();
  ctx.fillStyle = grad;
  ctx.fill();
}

const VENUE_ICONS: VenueIcon[] = ['konbini', 'cafe', 'water', 'repair'];

/**
 * Draw the image a MapLibre layer asked for, or null when the id is not ours.
 * Ids: marker-start | marker-end | marker-loop | marker-waypoint-<n> |
 * venue-<type>[-selected] | position-heading.
 */
export function markerImage(id: string) {
  if (typeof document === 'undefined') return null;
  const pad = 10;
  const endpointCanvas = () => makeCanvas(ENDPOINT_SIZE + pad * 2);

  if (id === 'marker-start' || id === 'marker-end' || id === 'marker-loop') {
    const cv = endpointCanvas();
    if (!cv) return null;
    if (id === 'marker-start') drawStart(cv);
    else if (id === 'marker-end') drawEnd(cv);
    else drawLoop(cv);
    return finish(cv);
  }

  const wp = /^marker-waypoint-(\d+)$/.exec(id);
  if (wp) {
    const cv = makeCanvas(22 + 12);
    if (!cv) return null;
    drawWaypoint(cv, Number(wp[1]));
    return finish(cv);
  }

  const venue = /^venue-([a-z]+)(-selected)?$/.exec(id);
  if (venue && VENUE_ICONS.includes(venue[1] as VenueIcon)) {
    const cv = makeCanvas(28 + 16);
    if (!cv) return null;
    drawVenue(cv, venue[1] as VenueIcon, Boolean(venue[2]));
    return finish(cv);
  }

  if (id === 'position-heading') {
    const cv = makeCanvas(72);
    if (!cv) return null;
    drawHeading(cv);
    return finish(cv);
  }

  return null;
}

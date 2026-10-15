// web/src/lib/geocode.ts
//
// Place lookup shared by the search panel and the plan panel.
//
// Nominatim (OpenStreetMap) answers most queries, but OSM carries no Japanese
// block data (番地 / 号): "Shikahama 7-15-14" only reaches the chōme centroid,
// 150 m or more from the actual block, and the Japanese form "鹿浜7-15-14"
// finds nothing at all. Japan's GSI address search does know the blocks, but
// only in Japanese script. So:
//
//   1. Nominatim is queried as before (with namedetails so the hit carries
//      its Japanese name).
//   2. When the query ends in a block pattern (7-15-14, 7丁目15-14,
//      7-chome 15-14) and the best Nominatim hit is coarser than a building,
//      GSI is asked for the hit's Japanese name plus the leftover numbers.
//      A query already in Japanese script asks GSI directly, in parallel.
//   3. A GSI block hit is listed first; the Nominatim hits follow.
//
// Both services are reached through same-origin proxies (/nominatim/*,
// /gsi/*) so the browser never talks to them directly.

export interface PlaceHit {
  name: string;
  detail: string;
  lat: number;
  lon: number;
}

/** One Nominatim search result (the fields this module reads). */
export interface NominatimHit {
  display_name: string;
  lat: string;
  lon: string;
  addresstype?: string;
  namedetails?: Record<string, string>;
}

/** One GSI address-search feature. */
export interface GsiFeature {
  geometry: { coordinates: [number, number] };
  properties: { title: string };
}

/** The parts of a query that end in a Japanese block address. */
export interface BlockQuery {
  /** The place name before the numbers, e.g. "Shikahama" or "鹿浜". */
  base: string;
  /** chōme, block and (optionally) building number, e.g. [7, 15, 14]. */
  numbers: number[];
  /** Whether `base` is written in Japanese script. */
  japanese: boolean;
}

const JAPANESE_SCRIPT = /[぀-ヿ㐀-䶿一-鿿]/;
const KANJI_DIGITS: Record<string, number> = {
  〇: 0,
  一: 1,
  二: 2,
  三: 3,
  四: 4,
  五: 5,
  六: 6,
  七: 7,
  八: 8,
  九: 9
};

/** Convert a kanji numeral such as 七 or 十二 to a number (1–99). */
export function kanjiToNumber(kanji: string): number | null {
  if (/^\d+$/.test(kanji)) return Number(kanji);
  let total = 0;
  let current = 0;
  for (const ch of kanji) {
    if (ch === '十') {
      total += (current || 1) * 10;
      current = 0;
    } else if (ch in KANJI_DIGITS) {
      current = KANJI_DIGITS[ch];
    } else {
      return null;
    }
  }
  return total + current;
}

/**
 * Normalise width, dashes and the 番/号 suffixes so one regex can read
 * "鹿浜７丁目１５番１４号", "Shikahama 7-chome 15-14" and "鹿浜7-15-14" alike.
 */
function normalise(query: string): string {
  return query
    .normalize('NFKC')
    .replace(/[‐‑‒–—−ー]/g, '-')
    .replace(/\s*番地?\s*/g, '-')
    .replace(/\s*号\s*/g, '')
    .replace(/\s+/g, ' ')
    .trim();
}

const BLOCK_PATTERN =
  /^(.*?)[\s,]*(?:([0-9一二三四五六七八九十]+)\s*-?\s*(?:ch[oō]me|丁目)[\s,-]*)?(\d+(?:\s*-\s*\d+)+)\s*$/iu;

/**
 * Recognise a query that ends in a block address. The numbers are the
 * chōme-block(-building) sequence; an explicit "N丁目" / "N-chome" is folded
 * into them, so "Shikahama 7-chome 15-14" and "Shikahama 7-15-14" parse the
 * same way. Returns null for anything else (a plain name, a single number).
 */
export function parseBlockQuery(query: string): BlockQuery | null {
  const m = normalise(query).match(BLOCK_PATTERN);
  if (!m) return null;
  const [, rawBase, chome, tail] = m;
  const base = rawBase.replace(/[\s,-]+$/, '').trim();
  if (!base) return null;
  const numbers = tail.split('-').map((n) => Number(n.trim()));
  if (chome !== undefined) {
    const c = kanjiToNumber(chome);
    if (c === null) return null;
    numbers.unshift(c);
  }
  if (numbers.length < 2 || numbers.length > 3) return null;
  return { base, numbers, japanese: JAPANESE_SCRIPT.test(base) };
}

/** Address types Nominatim returns for a chōme or a suburb, not a building. */
const COARSE_ADDRESS_TYPES = new Set(['neighbourhood', 'suburb', 'quarter']);

export function isCoarse(hit: NominatimHit): boolean {
  return hit.addresstype === undefined || COARSE_ADDRESS_TYPES.has(hit.addresstype);
}

/** The Japanese name of a Nominatim hit, without a trailing N丁目. */
export function japaneseBase(hit: NominatimHit): string | null {
  const details = hit.namedetails ?? {};
  const name = details['name:ja'] ?? details['name'] ?? null;
  if (!name || !JAPANESE_SCRIPT.test(name)) return null;
  return stripChome(name);
}

function stripChome(name: string): string {
  return name.replace(/[0-9一二三四五六七八九十]+丁目$/u, '').trim();
}

/** The GSI query for a block: the Japanese base plus the numbers, "鹿浜7-15-14". */
export function gsiQuery(base: string, numbers: number[]): string {
  return `${stripChome(base)}${numbers.join('-')}`;
}

export function nominatimToPlace(hit: NominatimHit): PlaceHit {
  const parts = hit.display_name.split(',').map((p) => p.trim());
  return {
    name: parts[0],
    detail: parts.slice(1, 3).join(' · '),
    lat: Number(hit.lat),
    lon: Number(hit.lon)
  };
}

/**
 * The place for a GSI block hit, labelled with the block the rider typed
 * ("鹿浜7-15-14") and, when a Nominatim hit for the same area exists, that
 * hit's English surroundings ("Shikahama · Adachi"); otherwise GSI's own
 * full title.
 */
export function gsiToPlace(
  feature: GsiFeature,
  base: string,
  numbers: number[],
  context?: NominatimHit
): PlaceHit {
  const [lon, lat] = feature.geometry.coordinates;
  const title = feature.properties.title.normalize('NFKC');
  let detail = title;
  if (context) {
    const parts = context.display_name.split(',').map((p) => p.trim());
    // Skip the hit's own name when it is a chōme ("Shikahama 7-chome"): the
    // block label already says which chōme it is.
    const from = isCoarse(context) && /\d+-chome$|丁目$/u.test(parts[0]) ? 1 : 0;
    detail = parts.slice(from, from + 2).join(' · ');
  }
  return { name: gsiQuery(base, numbers), detail, lat, lon };
}

/** GSI first, then the Nominatim hits that are not the same point. */
export function mergeResults(gsi: PlaceHit | null, nominatim: PlaceHit[]): PlaceHit[] {
  if (!gsi) return nominatim;
  return [gsi, ...nominatim.filter((h) => !samePoint(h, gsi))];
}

function samePoint(a: PlaceHit, b: PlaceHit): boolean {
  return Math.abs(a.lat - b.lat) < 1e-5 && Math.abs(a.lon - b.lon) < 1e-5;
}

type Fetch = typeof fetch;

async function fetchNominatim(query: string, fetchFn: Fetch): Promise<NominatimHit[]> {
  try {
    const res = await fetchFn(
      `/nominatim/search?format=json&q=${encodeURIComponent(query)}&limit=5&countrycodes=jp&accept-language=en&namedetails=1`
    );
    return res.ok ? await res.json() : [];
  } catch {
    return [];
  }
}

async function fetchGsi(query: string, fetchFn: Fetch): Promise<GsiFeature | null> {
  try {
    const res = await fetchFn(`/gsi/address-search/AddressSearch?q=${encodeURIComponent(query)}`);
    if (!res.ok) return null;
    const features: GsiFeature[] = await res.json();
    const first = features[0];
    return first?.geometry?.coordinates?.length === 2 && first.properties?.title ? first : null;
  } catch {
    return null;
  }
}

/**
 * Look a query up and report results as they arrive: `emit` is called with
 * the Nominatim hits first and again, with the block hit on top, once GSI
 * answers. The returned promise settles when every lookup is done. Callers
 * guard `emit` with their own sequence counter so a stale lookup cannot
 * overwrite a newer one.
 */
export async function searchPlaces(
  query: string,
  emit: (hits: PlaceHit[]) => void,
  fetchFn: Fetch = fetch
): Promise<void> {
  const q = query.trim();
  const block = parseBlockQuery(q);
  let nominatim: PlaceHit[] = [];
  let gsi: PlaceHit | null = null;
  const publish = () => emit(mergeResults(gsi, nominatim));

  // A Japanese block address asks GSI straight away, alongside Nominatim.
  const direct =
    block?.japanese &&
    fetchGsi(gsiQuery(block.base, block.numbers), fetchFn).then((feature) => {
      if (!feature) return;
      gsi = gsiToPlace(feature, block.base, block.numbers);
      publish();
    });

  const hits = await fetchNominatim(q, fetchFn);
  nominatim = hits.map(nominatimToPlace);
  publish();

  if (direct) {
    await direct;
    return;
  }

  // Romaji: the best Nominatim hit carries the Japanese name GSI needs.
  const best = hits[0];
  if (!block || !best || !isCoarse(best)) return;
  const base = japaneseBase(best);
  if (!base) return;
  const feature = await fetchGsi(gsiQuery(base, block.numbers), fetchFn);
  if (!feature) return;
  gsi = gsiToPlace(feature, base, block.numbers, best);
  publish();
}

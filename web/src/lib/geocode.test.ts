// web/src/lib/geocode.test.ts
//
// Unit tests for the place-lookup parsing and merging (node --test, no
// browser). Run with `npm run test:unit`.
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import {
  gsiQuery,
  gsiToPlace,
  isCoarse,
  japaneseBase,
  kanjiToNumber,
  mergeResults,
  nominatimToPlace,
  parseBlockQuery,
  searchPlaces,
  type GsiFeature,
  type NominatimHit,
  type PlaceHit
} from './geocode.ts';

const SHIKAHAMA_CHOME: NominatimHit = {
  display_name: 'Shikahama 7-chome, Shikahama, Adachi, Tokyo, 123-0861, Japan',
  lat: '35.7853137',
  lon: '139.7638584',
  addresstype: 'neighbourhood',
  namedetails: { name: '鹿浜七丁目', 'name:en': 'Shikahama 7-chome', 'name:ja': '鹿浜七丁目' }
};

const SHIKAHAMA_BLOCK: GsiFeature = {
  geometry: { coordinates: [139.762985, 35.7841] },
  properties: { title: '東京都足立区鹿浜七丁目１５番１４号' }
};

describe('parseBlockQuery', () => {
  it('reads a romaji chōme-block-building tail', () => {
    assert.deepEqual(parseBlockQuery('Shikahama 7-15-14'), {
      base: 'Shikahama',
      numbers: [7, 15, 14],
      japanese: false
    });
  });

  it('folds an explicit -chome into the numbers', () => {
    assert.deepEqual(parseBlockQuery('Shikahama 7-chome 15-14'), {
      base: 'Shikahama',
      numbers: [7, 15, 14],
      japanese: false
    });
    assert.deepEqual(parseBlockQuery('Shikahama 7 chome, 15-14')?.numbers, [7, 15, 14]);
  });

  it('reads Japanese script, full-width digits, 丁目 and 番/号', () => {
    assert.deepEqual(parseBlockQuery('鹿浜7-15-14'), {
      base: '鹿浜',
      numbers: [7, 15, 14],
      japanese: true
    });
    assert.deepEqual(parseBlockQuery('鹿浜7丁目15-14')?.numbers, [7, 15, 14]);
    assert.deepEqual(parseBlockQuery('鹿浜七丁目15番14号')?.numbers, [7, 15, 14]);
    assert.deepEqual(parseBlockQuery('鹿浜７−１５−１４')?.numbers, [7, 15, 14]);
    assert.equal(parseBlockQuery('鹿浜７−１５−１４')?.japanese, true);
  });

  it('accepts a two-number block without a building number', () => {
    assert.deepEqual(parseBlockQuery('Shikahama 7-15')?.numbers, [7, 15]);
  });

  it('rejects plain names, single numbers and over-long tails', () => {
    assert.equal(parseBlockQuery('Kiyosumi Garden'), null);
    assert.equal(parseBlockQuery('Shikahama 7'), null);
    assert.equal(parseBlockQuery('7-15-14'), null);
    assert.equal(parseBlockQuery('Shikahama 1-2-3-4'), null);
  });
});

describe('kanjiToNumber', () => {
  it('converts kanji numerals up to 99', () => {
    assert.equal(kanjiToNumber('七'), 7);
    assert.equal(kanjiToNumber('十'), 10);
    assert.equal(kanjiToNumber('十二'), 12);
    assert.equal(kanjiToNumber('二十三'), 23);
    assert.equal(kanjiToNumber('7'), 7);
    assert.equal(kanjiToNumber('x'), null);
  });
});

describe('Nominatim hit helpers', () => {
  it('treats a neighbourhood, suburb or quarter as coarser than a building', () => {
    assert.equal(isCoarse(SHIKAHAMA_CHOME), true);
    assert.equal(isCoarse({ ...SHIKAHAMA_CHOME, addresstype: 'suburb' }), true);
    assert.equal(isCoarse({ ...SHIKAHAMA_CHOME, addresstype: 'building' }), false);
    assert.equal(isCoarse({ ...SHIKAHAMA_CHOME, addresstype: 'amenity' }), false);
  });

  it('takes the Japanese name and drops the trailing 丁目', () => {
    assert.equal(japaneseBase(SHIKAHAMA_CHOME), '鹿浜');
    assert.equal(japaneseBase({ ...SHIKAHAMA_CHOME, namedetails: { name: '鹿浜' } }), '鹿浜');
    assert.equal(japaneseBase({ ...SHIKAHAMA_CHOME, namedetails: { name: 'Shikahama' } }), null);
    assert.equal(japaneseBase({ ...SHIKAHAMA_CHOME, namedetails: undefined }), null);
  });

  it('builds the GSI query from the base and the numbers', () => {
    assert.equal(gsiQuery('鹿浜', [7, 15, 14]), '鹿浜7-15-14');
    assert.equal(gsiQuery('鹿浜七丁目', [7, 15, 14]), '鹿浜7-15-14');
  });

  it('splits a display name into name and detail', () => {
    assert.deepEqual(nominatimToPlace(SHIKAHAMA_CHOME), {
      name: 'Shikahama 7-chome',
      detail: 'Shikahama · Adachi',
      lat: 35.7853137,
      lon: 139.7638584
    });
  });
});

describe('gsiToPlace', () => {
  it('labels the block and borrows the Nominatim surroundings', () => {
    assert.deepEqual(gsiToPlace(SHIKAHAMA_BLOCK, '鹿浜', [7, 15, 14], SHIKAHAMA_CHOME), {
      name: '鹿浜7-15-14',
      detail: 'Shikahama · Adachi',
      lat: 35.7841,
      lon: 139.762985
    });
  });

  it('falls back to the half-width GSI title without a Nominatim hit', () => {
    const place = gsiToPlace(SHIKAHAMA_BLOCK, '鹿浜', [7, 15, 14]);
    assert.equal(place.name, '鹿浜7-15-14');
    assert.equal(place.detail, '東京都足立区鹿浜七丁目15番14号');
  });
});

describe('mergeResults', () => {
  const block: PlaceHit = { name: '鹿浜7-15-14', detail: '', lat: 35.7841, lon: 139.762985 };
  const chome: PlaceHit = { name: 'Shikahama 7-chome', detail: '', lat: 35.7853, lon: 139.7639 };

  it('puts the block hit first and keeps the chōme hit', () => {
    assert.deepEqual(mergeResults(block, [chome]), [block, chome]);
  });

  it('passes Nominatim through when GSI had nothing', () => {
    assert.deepEqual(mergeResults(null, [chome]), [chome]);
  });

  it('drops a Nominatim hit at the same point as the block', () => {
    assert.deepEqual(mergeResults(block, [{ ...chome, lat: 35.7841, lon: 139.762985 }]), [block]);
  });
});

/** A fetch stub serving canned Nominatim and GSI answers and logging the URLs. */
function stubFetch(answers: { nominatim?: NominatimHit[]; gsi?: GsiFeature[] }) {
  const calls: string[] = [];
  const fetchFn = (async (input: string | URL | Request) => {
    const url = String(input);
    calls.push(decodeURIComponent(url));
    const body = url.startsWith('/nominatim/') ? (answers.nominatim ?? []) : (answers.gsi ?? []);
    return new Response(JSON.stringify(body), {
      status: 200,
      headers: { 'content-type': 'application/json' }
    });
  }) as typeof fetch;
  return { fetchFn, calls };
}

describe('searchPlaces', () => {
  it('romaji block: Nominatim first, then the GSI block hit on top', async () => {
    const { fetchFn, calls } = stubFetch({ nominatim: [SHIKAHAMA_CHOME], gsi: [SHIKAHAMA_BLOCK] });
    const emitted: PlaceHit[][] = [];
    await searchPlaces('Shikahama 7-15-14', (hits) => emitted.push(hits), fetchFn);

    assert.equal(emitted.length, 2);
    assert.deepEqual(
      emitted[0].map((h) => h.name),
      ['Shikahama 7-chome']
    );
    assert.deepEqual(
      emitted[1].map((h) => h.name),
      ['鹿浜7-15-14', 'Shikahama 7-chome']
    );
    assert.ok(calls[1].includes('AddressSearch?q=鹿浜7-15-14'), calls[1]);
  });

  it('Japanese block: GSI is asked directly, alongside Nominatim', async () => {
    const { fetchFn, calls } = stubFetch({ nominatim: [], gsi: [SHIKAHAMA_BLOCK] });
    const emitted: PlaceHit[][] = [];
    await searchPlaces('鹿浜7-15-14', (hits) => emitted.push(hits), fetchFn);

    assert.ok(calls[0].startsWith('/gsi/address-search/AddressSearch?q=鹿浜7-15-14'), calls[0]);
    assert.ok(calls[1].startsWith('/nominatim/search?'), calls[1]);
    const last = emitted.at(-1)!;
    assert.equal(last.length, 1);
    assert.equal(last[0].name, '鹿浜7-15-14');
    assert.ok(Math.abs(last[0].lat - 35.7841) < 0.0002);
    assert.ok(Math.abs(last[0].lon - 139.763) < 0.0002);
  });

  it('a plain name never asks GSI', async () => {
    const { fetchFn, calls } = stubFetch({ nominatim: [SHIKAHAMA_CHOME] });
    const emitted: PlaceHit[][] = [];
    await searchPlaces('Shikahama', (hits) => emitted.push(hits), fetchFn);
    assert.equal(calls.length, 1);
    assert.equal(emitted.length, 1);
  });

  it('a building-level Nominatim hit is left alone', async () => {
    const building = { ...SHIKAHAMA_CHOME, addresstype: 'building' };
    const { fetchFn, calls } = stubFetch({ nominatim: [building], gsi: [SHIKAHAMA_BLOCK] });
    await searchPlaces('Shikahama 7-15-14', () => {}, fetchFn);
    assert.equal(calls.length, 1);
  });

  it('keeps the Nominatim hits when GSI finds nothing', async () => {
    const { fetchFn } = stubFetch({ nominatim: [SHIKAHAMA_CHOME], gsi: [] });
    const emitted: PlaceHit[][] = [];
    await searchPlaces('Shikahama 7-15-14', (hits) => emitted.push(hits), fetchFn);
    assert.deepEqual(
      emitted.at(-1)?.map((h) => h.name),
      ['Shikahama 7-chome']
    );
  });
});

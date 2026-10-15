// web/tests/place-search.spec.ts
//
// Place search resolves a Japanese block address to the block, not the chōme
// centroid: the GSI block hit is listed first, the Nominatim chōme hit stays
// below it, and picking the block lands on its coordinates. Both geocoders
// are stubbed (see stubGeocoders); the real answers these stubs mirror were
// recorded for 鹿浜7-15-14 / "Shikahama 7-15-14" on 2026-10-15.
import { test, expect, gotoHydrated } from './helpers/fixtures';
import { expectLayoutInvariants } from './helpers/layout';
import { stubApi } from './helpers/api';
import type { Page } from '@playwright/test';

/** Nominatim's only answer for "Shikahama 7-15-14": the chōme centroid. */
const SHIKAHAMA_CHOME = {
  display_name: 'Shikahama 7-chome, Shikahama, Adachi, Tokyo, 123-0861, Japan',
  lat: '35.7853137',
  lon: '139.7638584',
  addresstype: 'neighbourhood',
  namedetails: { name: '鹿浜七丁目', 'name:en': 'Shikahama 7-chome', 'name:ja': '鹿浜七丁目' }
};

/** GSI's answer for 鹿浜7-15-14: the block itself. */
const SHIKAHAMA_BLOCK = {
  geometry: { coordinates: [139.762985, 35.7841], type: 'Point' },
  properties: { addressCode: '', title: '東京都足立区鹿浜七丁目１５番１４号' }
};

const BLOCK_LAT = 35.7841;
const BLOCK_LON = 139.763;
/** 20 m at Tokyo's latitude, in degrees. */
const TOLERANCE_DEG = 20 / 111_000;

/**
 * Stub both geocoders the way the live services answered: Nominatim knows
 * the romaji chōme but nothing in Japanese script; GSI knows the block only
 * when asked in Japanese.
 */
async function stubGeocoders(page: Page) {
  await page.route('**/nominatim/search**', (route) => {
    const q = new URL(route.request().url()).searchParams.get('q') ?? '';
    const body = /^shikahama/i.test(q) ? [SHIKAHAMA_CHOME] : [];
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(body)
    });
  });
  await page.route('**/gsi/address-search/AddressSearch**', (route) => {
    const q = new URL(route.request().url()).searchParams.get('q') ?? '';
    const body = q === '鹿浜7-15-14' ? [SHIKAHAMA_BLOCK] : [];
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(body)
    });
  });
}

async function lastRememberedPlace(page: Page) {
  return page.evaluate(() => {
    const list = JSON.parse(localStorage.getItem('komorebi:recent-places:v1') ?? '[]');
    return list[0] as { name: string; lat: number; lon: number } | undefined;
  });
}

test.beforeEach(async ({ page }) => {
  await stubApi(page);
  await stubGeocoders(page);
});

for (const query of ['Shikahama 7-15-14', '鹿浜7-15-14']) {
  test(`search panel: "${query}" lists the block first and lands on it`, async ({ page }) => {
    await gotoHydrated(page, '/');
    const box = page.getByRole('searchbox');
    await box.click();
    const gsi = page.waitForRequest(/\/gsi\/address-search\/AddressSearch\?q=/);
    await box.fill(query);
    expect(decodeURIComponent((await gsi).url())).toContain('AddressSearch?q=鹿浜7-15-14');

    const panel = page.getByRole('region', { name: 'Search' });
    const block = panel.getByRole('button', { name: /^鹿浜7-15-14/ });
    await expect(block).toBeVisible();
    await expectLayoutInvariants(page, block, 'block hit');

    const places = panel.getByRole('button', { name: /^(鹿浜7-15-14|Shikahama 7-chome)/ });
    await expect(places.first()).toHaveAccessibleName(/^鹿浜7-15-14/);
    if (query.startsWith('Shikahama')) {
      // Romaji: the chōme hit Nominatim found stays listed below the block.
      await expect(places).toHaveCount(2);
      await expect(places.nth(1)).toHaveAccessibleName(/^Shikahama 7-chome/);
      await expect(block).toHaveAccessibleName(/Shikahama · Adachi/);
    } else {
      // Japanese script: Nominatim has nothing, GSI's own title is the detail.
      await expect(places).toHaveCount(1);
      await expect(block).toHaveAccessibleName(/東京都足立区鹿浜七丁目15番14号/);
    }

    await block.click();
    await expect(box).toHaveValue('鹿浜7-15-14');
    const picked = await lastRememberedPlace(page);
    expect(picked?.name).toBe('鹿浜7-15-14');
    expect(Math.abs((picked?.lat ?? 0) - BLOCK_LAT)).toBeLessThan(TOLERANCE_DEG);
    expect(Math.abs((picked?.lon ?? 0) - BLOCK_LON)).toBeLessThan(TOLERANCE_DEG);
  });
}

test('plan panel: a block address fills the stop with the block', async ({ page }) => {
  await gotoHydrated(page, '/');
  await page.getByRole('searchbox').click();
  await page.getByRole('button', { name: /Plan a route/ }).click();

  const to = page.getByRole('textbox', { name: 'To' });
  await to.click();
  await to.fill('Shikahama 7-15-14');

  const list = page.getByRole('listbox', { name: 'Place suggestions' });
  const options = list.getByRole('option');
  await expect(options.first()).toHaveAccessibleName(/^鹿浜7-15-14/);
  await expect(options).toHaveCount(2);
  await expect(options.nth(1)).toHaveAccessibleName(/^Shikahama 7-chome/);
  await expectLayoutInvariants(page, options.first(), 'block suggestion');

  await options.first().click();
  await expect(to).toHaveValue('鹿浜7-15-14');
  const picked = await lastRememberedPlace(page);
  expect(picked?.name).toBe('鹿浜7-15-14');
  expect(Math.abs((picked?.lat ?? 0) - BLOCK_LAT)).toBeLessThan(TOLERANCE_DEG);
  expect(Math.abs((picked?.lon ?? 0) - BLOCK_LON)).toBeLessThan(TOLERANCE_DEG);
});

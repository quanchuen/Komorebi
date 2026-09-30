// web/tests/disclaimer.spec.ts
//
// First-ride disclaimer (frame F): shown once before the first "Ride this",
// EN/JA, and re-readable from the Data sources sheet.
import { test, expect, gotoHydrated } from './helpers/fixtures';
import { expectLayoutInvariants } from './helpers/layout';
import { stubApi } from './helpers/api';

const EN_TITLE = 'Before you ride';
const JA_TITLE = 'ご利用前に';

// A routed trip via the shareable stop URL, so "Ride this" is available.
const ROUTED = '/?from=35.6852,139.7528,Sakuradamon&to=35.6941,139.7568,Otemon';

test.beforeEach(async ({ page }) => {
  await stubApi(page);
});

test('Ride this shows the disclaimer once, then starts the ride on I understand', async ({
  page,
  context
}) => {
  await context.grantPermissions(['geolocation']);
  await context.setGeolocation({ latitude: 35.6852, longitude: 139.7528 });

  const routed = page.waitForResponse(/\/routing\/directions/);
  await gotoHydrated(page, ROUTED);
  await routed;

  const ride = page.getByRole('button', { name: 'Ride this' });
  await expectLayoutInvariants(page, ride, 'Ride this');
  await ride.click();

  const dialog = page.getByRole('dialog', { name: EN_TITLE });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole('listitem')).toHaveCount(5);
  // Nothing starts until the rider accepts.
  await expect(page.getByRole('button', { name: 'Stop', exact: true })).toHaveCount(0);

  const accept = dialog.getByRole('button', { name: 'I understand' });
  await expectLayoutInvariants(page, accept, 'I understand');
  await accept.click();
  await expect(dialog).toBeHidden();
  // The ride started: the panel now offers to stop guidance.
  await expect(page.getByRole('button', { name: 'Stop', exact: true })).toBeVisible();

  // Second time round it is remembered and not shown again.
  const remembered = await page.evaluate(() =>
    localStorage.getItem('komorebi:first-ride-disclaimer-seen:v1')
  );
  expect(remembered).toBe('1');
});

test('the disclaimer switches to Japanese and is reachable from Data sources', async ({ page }) => {
  await gotoHydrated(page, '/');
  await page.getByRole('button', { name: /OpenStreetMap contributors|OSM/ }).click();
  const sources = page.getByRole('dialog', { name: 'Data sources' });
  const before = sources.getByRole('button', { name: EN_TITLE });
  await expectLayoutInvariants(page, before, 'Before you ride link');
  await before.click();

  const dialog = page.getByRole('dialog', { name: EN_TITLE });
  await expect(dialog).toBeVisible();
  await expect(dialog).toHaveAttribute('lang', 'en');

  const ja = dialog.getByRole('button', { name: '日本語' });
  await expectLayoutInvariants(page, ja, '日本語 switch');
  await ja.click();
  const jaDialog = page.getByRole('dialog', { name: JA_TITLE });
  await expect(jaDialog).toBeVisible();
  await expect(jaDialog).toHaveAttribute('lang', 'ja');
  await expect(jaDialog.getByRole('button', { name: '理解しました' })).toBeVisible();
  // Japanese text renders in Noto Sans JP, not Inter's fallback.
  const family = await jaDialog
    .getByRole('heading', { name: JA_TITLE })
    .evaluate((el) => getComputedStyle(el).fontFamily);
  expect(family).toMatch(/Noto Sans JP/);

  // Escape dismisses without accepting, so it stays unseen.
  await page.keyboard.press('Escape');
  await expect(jaDialog).toBeHidden();
  expect(
    await page.evaluate(() => localStorage.getItem('komorebi:first-ride-disclaimer-seen:v1'))
  ).not.toBe('1');
});

// web/tests/attribution.spec.ts
//
// The attribution pill credits what is on screen and opens the Data sources
// sheet (frame E).
import { test, expect, gotoHydrated } from './helpers/fixtures';
import { expectLayoutInvariants } from './helpers/layout';
import { stubApi } from './helpers/api';

test.beforeEach(async ({ page }) => {
  await stubApi(page);
  await gotoHydrated(page, '/');
});

test('pill credits OSM and CARTO, grows with the shade layer, and meets layout invariants', async ({
  page
}) => {
  const pill = page.getByRole('button', { name: /OpenStreetMap contributors/ });
  await expect(pill).toBeVisible();
  await expect(pill).toContainText('CARTO');
  await expect(pill).not.toContainText('PLATEAU');
  await expectLayoutInvariants(page, pill, 'attribution pill');

  // The timeline's Shade lens turns the building-shadow layer on.
  await page.getByRole('button', { name: 'Shade', exact: true }).click();
  await expect(pill).toContainText('PLATEAU (MLIT)');
});

test('Data sources sheet opens from the pill, lists every source, and closes on Escape', async ({
  page
}) => {
  const pill = page.getByRole('button', { name: /OpenStreetMap contributors/ });
  await pill.click();

  const dialog = page.getByRole('dialog', { name: 'Data sources' });
  await expect(dialog).toBeVisible();

  for (const name of [
    'OpenStreetMap contributors',
    'CARTO',
    'Project PLATEAU · MLIT',
    'Open-Meteo',
    'Nominatim',
    'Valhalla'
  ]) {
    await expect(dialog.getByRole('link', { name }).first()).toBeVisible();
  }
  await expect(
    dialog.getByRole('link', { name: 'Fix a map error on OpenStreetMap' })
  ).toBeVisible();
  await expect(dialog).toContainText(
    "Derived scores (shade, greenery) keep their sources' licences."
  );

  const close = dialog.getByRole('button', { name: 'Close' });
  await expectLayoutInvariants(page, close, 'sheet close button');
  await expect(
    dialog.getByRole('link', { name: 'Fix a map error on OpenStreetMap' })
  ).toBeVisible();

  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();
  await expect(pill).toHaveAttribute('aria-expanded', 'false');
});

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
  const pill = page.getByRole('button', { name: /OpenStreetMap contributors|OSM/ });
  await expect(pill).toBeVisible();
  await expect(pill).toContainText('CARTO');
  await expect(pill).not.toContainText('PLATEAU');
  await expectLayoutInvariants(page, pill, 'attribution pill');

  // The Shade switch in Map layers (frame C) turns the building-shadow layer on.
  await page.getByRole('button', { name: 'Map layers' }).click();
  const shade = page.getByRole('switch', { name: /^Shade/ });
  await expectLayoutInvariants(page, shade, 'Shade switch');
  await shade.click();
  await expect(shade).toHaveAttribute('aria-checked', 'true');
  await expect(pill).toContainText('PLATEAU');
});

test('Data sources sheet opens from the pill, lists every source, and closes on Escape', async ({
  page
}) => {
  const pill = page.getByRole('button', { name: /OpenStreetMap contributors|OSM/ });
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

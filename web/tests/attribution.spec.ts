// Attribution pill + Data sources sheet (spec § Attribution, frame E).
import { test, expect } from './fixtures';
import { expectLayoutInvariants } from './layout';

test.describe('attribution', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
  });

  test('pill credits the sources on screen and meets layout invariants', async ({ page }) => {
    const pill = page.getByRole('button', { name: /^Data sources:/ });
    await expect(pill).toBeVisible();
    // Basemap always; the weather strip and the default rain layer show
    // Open-Meteo data. No shade layer yet, so no PLATEAU.
    await expect(pill).toHaveAccessibleName(
      'Data sources: © OpenStreetMap contributors · CARTO · Open-Meteo'
    );
    await expectLayoutInvariants(pill);
  });

  test('pill grows with the shade layer', async ({ page }) => {
    const pill = page.getByRole('button', { name: /^Data sources:/ });
    const lenses = page.getByRole('group', { name: 'Timeline layer' });
    await lenses.getByRole('button', { name: 'Shade' }).click();
    await expect(pill).toHaveAccessibleName(/PLATEAU \(MLIT\)/);
    await lenses.getByRole('button', { name: 'Weather' }).click();
    await expect(pill).not.toHaveAccessibleName(/PLATEAU/);
  });

  test('MapLibre default attribution control is gone', async ({ page }) => {
    await expect(page.getByRole('button', { name: /^Data sources:/ })).toBeVisible();
    await expect(page.getByRole('link', { name: 'CARTO', exact: true })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Toggle attribution' })).toHaveCount(0);
  });

  test('sheet lists every source and closes back to the pill', async ({ page }) => {
    const pill = page.getByRole('button', { name: /^Data sources:/ });
    await pill.click();

    const sheet = page.getByRole('dialog', { name: 'Data sources' });
    await expect(sheet).toBeVisible();

    const rows = sheet.getByRole('row');
    await expect(rows).toHaveCount(7); // header + six sources
    for (const [name, licence] of [
      ['OpenStreetMap contributors', 'ODbL 1.0'],
      ['CARTO', 'CARTO basemap terms'],
      ['Project PLATEAU · MLIT', 'CC BY 4.0'],
      ['Open-Meteo', 'CC BY 4.0'],
      ['Nominatim', 'ODbL (OSM data)'],
      ['Valhalla', 'MIT']
    ]) {
      const row = sheet.getByRole('row', { name: new RegExp(`^${name.replace(/[()]/g, '\\$&')}`) });
      await expect(row).toContainText(licence);
    }
    await expect(sheet).toContainText('keep their licences');

    for (const control of [
      sheet.getByRole('button', { name: 'Close data sources' }),
      sheet.getByRole('link', { name: 'OpenStreetMap contributors' }),
      sheet.getByRole('link', { name: 'Valhalla' }),
      sheet.getByRole('link', { name: 'Fix a map error on OpenStreetMap' }),
      sheet.getByRole('button', { name: 'Done' })
    ]) {
      await expectLayoutInvariants(control);
    }

    await sheet.getByRole('button', { name: 'Done' }).click();
    await expect(sheet).toBeHidden();
    await expect(pill).toBeFocused();
  });

  test('Escape closes the sheet', async ({ page }) => {
    await page.getByRole('button', { name: /^Data sources:/ }).click();
    const sheet = page.getByRole('dialog', { name: 'Data sources' });
    await expect(sheet).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(sheet).toBeHidden();
  });
});

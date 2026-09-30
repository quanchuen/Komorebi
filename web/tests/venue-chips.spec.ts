// web/tests/venue-chips.spec.ts
//
// Opt-in venue pins: the "Show on map" chips in the search panel (frame B)
// are the only way venues appear.
import { test, expect, gotoHydrated } from './helpers/fixtures';
import { expectLayoutInvariants } from './helpers/layout';
import { stubApi } from './helpers/api';

test.beforeEach(async ({ page }) => {
  await stubApi(page);
  await gotoHydrated(page, '/');
  await page.getByRole('searchbox').click();
});

test('venue chips are off by default, toggle, and meet the layout invariants', async ({ page }) => {
  const group = page.getByRole('group', { name: 'Show on map' });
  await expect(group).toBeVisible();

  for (const name of ['Konbini', 'Café', 'Water', 'Repair']) {
    const chip = group.getByRole('button', { name });
    await expect(chip).toHaveAttribute('aria-pressed', 'false');
    await expectLayoutInvariants(page, chip, `chip ${name}`);
  }

  const konbini = group.getByRole('button', { name: 'Konbini' });
  await konbini.click();
  await expect(konbini).toHaveAttribute('aria-pressed', 'true');
  await konbini.click();
  await expect(konbini).toHaveAttribute('aria-pressed', 'false');
});

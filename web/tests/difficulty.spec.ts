// web/tests/difficulty.spec.ts
//
// Difficulty is a neutral badge with 1–4 bars and a label; it never carries
// colour (spec "Difficulty — no colour").
import { test, expect, gotoHydrated } from './helpers/fixtures';
import { expectLayoutInvariants } from './helpers/layout';
import { stubApi } from './helpers/api';

test('route cards show a neutral 1–4 bar difficulty badge', async ({ page }) => {
  await stubApi(page);
  // Cards appear once the suggested (featured) query returns.
  const viewportLoaded = page.waitForResponse(/\/discover\/suggested/);
  await gotoHydrated(page, '/');
  await viewportLoaded;

  const easy = page.getByRole('button', { name: /Imperial Palace Loop/ });
  await expect(easy).toBeVisible();
  const easyBadge = easy.getByRole('img', { name: 'Difficulty: Easy, 1 of 4' });
  await expect(easyBadge).toBeVisible();
  await expect(easyBadge).toHaveText('Easy');

  const expert = page.getByRole('button', { name: /Arakawa Riverside/ });
  await expect(expert.getByRole('img', { name: 'Difficulty: Expert, 4 of 4' })).toBeVisible();

  // The badge is the same neutral grey whatever the level: no hue creeps in.
  const colours = await Promise.all(
    [easyBadge, expert.getByRole('img', { name: /Difficulty/ })].map((b) =>
      b.evaluate((el) => {
        const s = getComputedStyle(el);
        return `${s.backgroundColor}|${s.color}|${s.borderColor}`;
      })
    )
  );
  expect(colours[0]).toBe(colours[1]);

  await expectLayoutInvariants(page, easy, 'Imperial Palace Loop card');
});

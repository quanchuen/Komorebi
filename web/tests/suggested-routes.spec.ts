// web/tests/suggested-routes.spec.ts
//
// Suggested routes are the featured set, not a viewport query: selecting one
// zooms the map, and the other suggestions must stay in the list.
import { test, expect, gotoHydrated } from './helpers/fixtures';
import { stubApi } from './helpers/api';

test('the suggested list keeps every featured route while one is selected', async ({ page }) => {
  await stubApi(page);
  const loaded = page.waitForResponse(/\/discover\/suggested/);
  await gotoHydrated(page, '/');
  await loaded;

  const palace = page.getByRole('button', { name: /Imperial Palace Loop/ });
  const river = page.getByRole('button', { name: /Arakawa Riverside/ });
  await expect(palace).toBeVisible();
  await expect(river).toBeVisible();

  // Selecting zooms the map to the route; the list must not follow the map.
  const detail = page.waitForResponse(/\/routes\/route-palace$/);
  await palace.click();
  await detail;
  await expect(page.getByRole('img', { name: 'Start' })).toBeVisible();
  await expect(river).toBeVisible();
  await expect(palace).toBeVisible();

  // Escape clears the selection and the same list is still there.
  await page.keyboard.press('Escape');
  await expect(page.getByRole('img', { name: 'Start' })).toHaveCount(0);
  await expect(river).toBeVisible();
  await expect(palace).toBeVisible();
});

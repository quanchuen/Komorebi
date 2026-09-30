// web/tests/routes-list.spec.ts
//
// Frames A, I and D: the peek bar counts the curated routes in view, opens
// the list, and a row opens the route card. Escape walks back out.
import { test, expect, gotoHydrated } from './helpers/fixtures';
import { expectLayoutInvariants } from './helpers/layout';
import { stubApi } from './helpers/api';

test('peek → list → route card, and back with Escape', async ({ page }, info) => {
  await stubApi(page);
  const loaded = page.waitForResponse(/\/discover\/viewport/);
  await gotoHydrated(page, '/');
  await loaded;

  const peek = page.getByRole('button', { name: /2 curated routes in view/ });
  await expectLayoutInvariants(page, peek, 'routes peek');
  await peek.click();

  const list = page
    .getByRole('dialog', { name: 'Routes in view' })
    .or(page.getByRole('heading', { name: 'Routes in view' }));
  await expect(list.first()).toBeVisible();
  const palace = page.getByRole('button', { name: /Imperial Palace Loop/ });
  const river = page.getByRole('button', { name: /Arakawa Riverside/ });
  await expect(river).toBeVisible();
  await expectLayoutInvariants(page, palace, 'Imperial Palace Loop row');
  for (const name of ['Nearest', 'Shadiest now', 'Shortest', 'Loops only']) {
    await expectLayoutInvariants(page, page.getByRole('button', { name, exact: true }), name);
  }

  // Selecting a row opens the route card (frame D) and frames the route. The
  // list already fetched the geometry for its rows, so no request is awaited.
  await palace.click();
  await expect(page.getByRole('heading', { name: 'Imperial Palace Loop' })).toBeVisible();
  await expect(page.getByRole('img', { name: 'Start' })).toBeVisible();
  // The mobile sheet has no Close (Penpot D mobile): a map tap or Escape puts it away.
  const mobile = info.project.name.startsWith('mobile');
  for (const name of mobile
    ? ['Ride this', 'Save route', 'More']
    : ['Ride this', 'Save route', 'Customize', 'Close']) {
    await expectLayoutInvariants(
      page,
      page.getByRole('button', { name, exact: true }).first(),
      name
    );
  }

  // Escape clears the selection; the peek bar is back with the same count.
  await page.keyboard.press('Escape');
  await expect(page.getByRole('heading', { name: 'Imperial Palace Loop' })).toHaveCount(0);
  await expect(page.getByRole('img', { name: 'Start' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: /2 curated routes in view/ })).toBeVisible();
});

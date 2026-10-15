// web/tests/explore-layout.spec.ts
//
// The explore chrome (Penpot frames A, C, G, H): every control on the
// resting map meets the layout invariants, the time chip expands into the
// scrubber, Plan a route shows alternatives from a stop URL, and Describe a
// ride turns the model's answer into preference chips.
import { test, expect, gotoHydrated } from './helpers/fixtures';
import { expectLayoutInvariants } from './helpers/layout';
import { stubApi } from './helpers/api';

test.beforeEach(async ({ page }) => {
  await stubApi(page);
});

test('frame A: the resting map controls meet the layout invariants', async ({ page }, info) => {
  const loaded = page.waitForResponse(/\/discover\/viewport/);
  await gotoHydrated(page, '/');
  await loaded;
  const mobile = info.project.name.startsWith('mobile');

  await expectLayoutInvariants(page, page.getByRole('searchbox'), 'search field');
  await expectLayoutInvariants(page, page.getByRole('button', { name: 'Menu' }), 'menu');
  await expectLayoutInvariants(page, page.getByRole('button', { name: 'Map layers' }), 'layers');
  await expectLayoutInvariants(
    page,
    page.getByRole('button', { name: 'Show my location' }),
    'locate'
  );
  if (!mobile) {
    await expectLayoutInvariants(page, page.getByRole('button', { name: 'Zoom in' }), 'zoom in');
    await expectLayoutInvariants(page, page.getByRole('button', { name: 'Zoom out' }), 'zoom out');
  }
  await expectLayoutInvariants(
    page,
    page.getByRole('button', { name: /curated routes in view/ }),
    'peek'
  );
  const chip = page.getByRole('button', { name: /^Departure time: Now/ });
  await expectLayoutInvariants(page, chip, 'time chip');
  await expectLayoutInvariants(
    page,
    page.getByRole('button', { name: /OpenStreetMap|OSM/ }),
    'attribution'
  );

  // The menu carries About and Data sources until frame M ships.
  await page.getByRole('button', { name: 'Menu' }).click();
  await expectLayoutInvariants(
    page,
    page.getByRole('menuitem', { name: /Before you ride/ }),
    'about'
  );
  await page.keyboard.press('Escape');
  await expect(page.getByRole('menu')).toHaveCount(0);
});

test('frame B: featured routes sit under Plan / Describe and open the route card', async ({
  page
}) => {
  const featured = page.waitForResponse(/\/discover\/suggested/);
  await gotoHydrated(page, '/');
  await featured;
  // Nothing featured shows at rest: no route card, no list.
  await expect(page.getByRole('heading', { name: 'Imperial Palace Loop' })).toHaveCount(0);

  await page.getByRole('searchbox').click();
  const panel = page.getByRole('region', { name: 'Search' });
  await expect(panel.getByText('Featured routes')).toBeVisible();
  const row = panel.getByRole('button', { name: /Imperial Palace Loop/ });
  await expectLayoutInvariants(page, row, 'featured row');
  await expect(row.getByRole('img', { name: 'Difficulty: Easy, 1 of 4' })).toBeVisible();
  await row.click();
  await expect(page.getByRole('heading', { name: 'Imperial Palace Loop' })).toBeVisible();
});

test('frame C: the time chip opens the scrubber; Now and the slider move the departure', async ({
  page
}) => {
  await gotoHydrated(page, '/');
  await page.getByRole('button', { name: /^Departure time:/ }).click();
  const scrubber = page.getByRole('region', { name: 'Departure time' });
  await expect(scrubber).toBeVisible();
  const slider = scrubber.getByRole('slider', { name: 'Departure time' });
  await expectLayoutInvariants(page, slider, 'time slider');
  await expectLayoutInvariants(page, scrubber.getByRole('button', { name: 'Now' }), 'Now');

  // Desktop: the scrubber expands in place of the centred time chip, so its
  // centre is the map's centre (no panel is open here). Mobile docks it
  // edge to edge above the sheet instead.
  if (test.info().project.name === 'desktop-chromium') {
    const box = await scrubber.boundingBox();
    const viewport = page.viewportSize();
    expect(box).not.toBeNull();
    expect(viewport).not.toBeNull();
    expect(Math.abs(box!.x + box!.width / 2 - viewport!.width / 2)).toBeLessThanOrEqual(1);
  }

  // End moves to the far end of the 24-hour window; Now brings it back to
  // within the first hour (values are minutes from the current hour).
  await slider.focus();
  await page.keyboard.press('End');
  expect(Number(await slider.inputValue())).toBeGreaterThan(20 * 60);
  await scrubber.getByRole('button', { name: 'Now' }).click();
  expect(Number(await slider.inputValue())).toBeLessThanOrEqual(60);
  await page.keyboard.press('Escape');
  await expect(scrubber).toHaveCount(0);
});

test('frame G: a stop URL opens Plan a route with three alternatives', async ({ page }) => {
  const routed = page.waitForResponse(/\/routing\/directions/);
  await gotoHydrated(page, '/?from=35.6852,139.7528,Sakuradamon&to=35.6941,139.7568,Otemon');
  await routed;

  await expect(page.getByRole('heading', { name: 'Plan a route' })).toBeVisible();
  await expect(page.getByRole('textbox', { name: 'From' })).toHaveValue('Sakuradamon');
  await expect(page.getByRole('textbox', { name: 'To' })).toHaveValue('Otemon');
  const suggested = page.getByRole('radio', { name: /Suggested/ });
  await expect(suggested).toHaveAttribute('aria-checked', 'true');
  for (const [role, name] of [
    ['textbox', 'From'],
    ['button', 'Add stop'],
    ['button', 'Swap start and destination'],
    ['button', /^Leave/],
    ['button', 'Preferences'],
    ['radio', /Suggested/],
    ['button', 'Ride this'],
    ['button', 'Save'],
    ['button', 'Share this plan'],
    ['button', 'Back']
  ] as const) {
    await expectLayoutInvariants(page, page.getByRole(role, { name }).first(), String(name));
  }
  // The map draws a time pill per alternative (it may sit under the sheet on mobile).
  await expect(page.getByRole('button', { name: /Suggested: \d+ min/ })).toBeAttached();

  // Back returns to the search panel; the plan leaves the map.
  await page.getByRole('button', { name: 'Back' }).click();
  await expect(page.getByRole('button', { name: /Plan a route/ })).toBeVisible();
  await expect(page.getByRole('button', { name: /Suggested: \d+ min/ })).toHaveCount(0);
});

test('frame H: Describe a ride shows preference chips and hands over to Plan', async ({ page }) => {
  await gotoHydrated(page, '/');
  await page.getByRole('searchbox').click();
  await page.getByRole('button', { name: /Describe a ride/ }).click();

  const box = page.getByRole('textbox', { name: 'Describe your ride' });
  await expectLayoutInvariants(page, box, 'ride description');
  await box.fill('shady loop, not too many hills');
  const interpreted = page.waitForResponse(/\/routing\/intent/);
  await page.getByRole('button', { name: 'Interpret' }).click();
  await interpreted;

  await expect(page.getByText('A shady loop with a climb limit.')).toBeVisible();
  await expect(page.getByText('More shade')).toBeVisible();
  await expect(page.getByText(/climb limit isn't available yet/)).toBeVisible();
  await expectLayoutInvariants(
    page,
    page.getByRole('button', { name: 'Remove More shade' }),
    'remove chip'
  );
  const plan = page.getByRole('button', { name: 'Plan this route' });
  await expectLayoutInvariants(page, plan, 'Plan this route');
  await plan.click();
  await expect(page.getByRole('heading', { name: 'Plan a route' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Prefer shade' })).toBeVisible();
});

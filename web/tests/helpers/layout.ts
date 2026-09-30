// web/tests/helpers/layout.ts
//
// The non-negotiable layout invariants from tests/AGENTS.md, checked for one
// interactive element after scrolling it into view:
//   1. its bounding box is fully inside the visual viewport;
//   2. elementFromPoint at its centre returns it or a descendant;
//   3. its hit box is at least 44×44 CSS px;
//   4. sequential focus traversal from the document start reaches it;
//   5. no ancestor clips it (scrollWidth <= clientWidth) unless that ancestor
//      is an intentional scroll container marked data-scroll="true".
import { expect, type Locator, type Page } from '@playwright/test';

export const MIN_HIT_PX = 44;

interface Probe {
  box: { x: number; y: number; width: number; height: number };
  viewport: { width: number; height: number };
  centreHit: boolean;
  clippers: string[];
}

async function probe(locator: Locator): Promise<Probe> {
  return locator.evaluate((el) => {
    const r = el.getBoundingClientRect();
    const hit = document.elementFromPoint(r.x + r.width / 2, r.y + r.height / 2);
    const clippers: string[] = [];
    let node = el.parentElement;
    while (node && node !== document.body) {
      if (node.scrollWidth > node.clientWidth && node.dataset.scroll !== 'true') {
        clippers.push(`${node.tagName.toLowerCase()}${node.id ? '#' + node.id : ''}`);
      }
      node = node.parentElement;
    }
    return {
      box: { x: r.x, y: r.y, width: r.width, height: r.height },
      viewport: { width: window.innerWidth, height: window.innerHeight },
      centreHit: hit === el || el.contains(hit),
      clippers
    };
  });
}

/** Tab from the document start until `locator` owns focus, or fail. */
export async function expectFocusReachable(page: Page, locator: Locator, maxTabs = 150) {
  await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
  const handle = await locator.elementHandle();
  expect(handle, 'element must exist to be focus-reachable').not.toBeNull();
  for (let i = 0; i < maxTabs; i++) {
    await page.keyboard.press('Tab');
    const focused = await page.evaluate(
      (el) => document.activeElement === el,
      handle as NonNullable<typeof handle>
    );
    if (focused) return;
  }
  throw new Error(`element not reached by ${maxTabs} Tab presses from the document start`);
}

/** Check every layout invariant for one interactive element. */
export async function expectLayoutInvariants(page: Page, locator: Locator, label?: string) {
  const name =
    label ??
    (await locator.evaluate(
      (el) => el.getAttribute('aria-label') ?? el.textContent?.trim() ?? el.tagName
    ));
  await locator.scrollIntoViewIfNeeded();
  await expect(locator, `${name}: visible`).toBeVisible();
  const p = await probe(locator);

  expect(p.box.x, `${name}: left edge inside viewport`).toBeGreaterThanOrEqual(0);
  expect(p.box.y, `${name}: top edge inside viewport`).toBeGreaterThanOrEqual(0);
  expect(p.box.x + p.box.width, `${name}: right edge inside viewport`).toBeLessThanOrEqual(
    p.viewport.width
  );
  expect(p.box.y + p.box.height, `${name}: bottom edge inside viewport`).toBeLessThanOrEqual(
    p.viewport.height
  );
  expect(p.centreHit, `${name}: elementFromPoint at centre hits it`).toBe(true);
  expect(p.box.width, `${name}: hit box width >= ${MIN_HIT_PX}`).toBeGreaterThanOrEqual(MIN_HIT_PX);
  expect(p.box.height, `${name}: hit box height >= ${MIN_HIT_PX}`).toBeGreaterThanOrEqual(
    MIN_HIT_PX
  );
  expect(p.clippers, `${name}: no clipping ancestor`).toEqual([]);
  await expectFocusReachable(page, locator);
}

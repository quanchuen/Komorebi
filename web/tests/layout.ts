// Layout invariants for interactive elements (tests/AGENTS.md § Layout
// invariants). Each check is proven to fail on a synthetic defect in
// layout-invariants.spec.ts.
import { expect, type Locator } from '@playwright/test';

export const MIN_HIT_PX = 44;
// Sequential focus traversal gives up after this many Tab presses.
const MAX_TABS = 150;

/** 1. The bounding box lies fully inside the visual viewport. */
export async function expectInViewport(el: Locator) {
  const box = await el.boundingBox();
  expect(box, 'element has a bounding box').not.toBeNull();
  const vv = await el.page().evaluate(() => {
    const v = window.visualViewport;
    return v
      ? { x: v.offsetLeft, y: v.offsetTop, w: v.width, h: v.height }
      : { x: 0, y: 0, w: window.innerWidth, h: window.innerHeight };
  });
  const b = box!;
  expect(b.x, 'left edge inside viewport').toBeGreaterThanOrEqual(vv.x);
  expect(b.y, 'top edge inside viewport').toBeGreaterThanOrEqual(vv.y);
  expect(b.x + b.width, 'right edge inside viewport').toBeLessThanOrEqual(vv.x + vv.w);
  expect(b.y + b.height, 'bottom edge inside viewport').toBeLessThanOrEqual(vv.y + vv.h);
}

/** 2. elementFromPoint at the centre returns the element or a descendant. */
export async function expectHitAtCenter(el: Locator) {
  const hit = await el.evaluate((node) => {
    const r = node.getBoundingClientRect();
    const top = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
    return {
      ok: top !== null && (top === node || node.contains(top)),
      got: top ? top.outerHTML.slice(0, 120) : 'null'
    };
  });
  expect(hit.ok, `centre hit reaches the element (got ${hit.got})`).toBe(true);
}

/** 3. The hit box is at least 44×44 CSS px. */
export async function expectMinHitBox(el: Locator) {
  const box = await el.boundingBox();
  expect(box, 'element has a bounding box').not.toBeNull();
  expect(box!.width, 'hit box width').toBeGreaterThanOrEqual(MIN_HIT_PX);
  expect(box!.height, 'hit box height').toBeGreaterThanOrEqual(MIN_HIT_PX);
}

/** 4. Reachable by sequential focus traversal from the document start. */
export async function expectFocusReachable(el: Locator) {
  const page = el.page();
  const handle = await el.elementHandle();
  expect(handle, 'element is attached').not.toBeNull();
  await page.evaluate(() => {
    (document.activeElement as HTMLElement | null)?.blur?.();
    window.getSelection()?.removeAllRanges();
  });
  let reached = false;
  for (let i = 0; i < MAX_TABS && !reached; i++) {
    await page.keyboard.press('Tab');
    reached = await handle!.evaluate((node) => document.activeElement === node);
  }
  expect(reached, `reachable within ${MAX_TABS} Tab presses`).toBe(true);
}

/** 5. No ancestor clips it horizontally, unless marked data-scroll="true". */
export async function expectNoClippingAncestor(el: Locator) {
  const clipped = await el.evaluate((node) => {
    const offenders: string[] = [];
    for (let a = node.parentElement; a; a = a.parentElement) {
      if (a.dataset.scroll === 'true') continue;
      if (a.scrollWidth > a.clientWidth) {
        offenders.push(`${a.tagName.toLowerCase()} ${a.scrollWidth}>${a.clientWidth}`);
      }
    }
    return offenders;
  });
  expect(clipped, 'ancestors that clip horizontally').toEqual([]);
}

/** Every layout invariant, after scrolling the element into view. */
export async function expectLayoutInvariants(el: Locator) {
  await expect(el).toBeVisible();
  await el.scrollIntoViewIfNeeded();
  await expectInViewport(el);
  await expectHitAtCenter(el);
  await expectMinHitBox(el);
  await expectNoClippingAncestor(el);
  await expectFocusReachable(el);
}

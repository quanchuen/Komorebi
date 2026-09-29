// Proves each layout-invariant helper fails on a synthetic defect and passes
// on a sound control (tests/AGENTS.md: every assertion helper must be proven
// to fail). Pages are inline HTML; nothing here touches the app.
import { test, expect } from '@playwright/test';
import {
  expectFocusReachable,
  expectHitAtCenter,
  expectInViewport,
  expectMinHitBox,
  expectNoClippingAncestor
} from './layout';

const SOUND = '<button style="width:48px;height:48px">OK</button>';

async function mustFail(check: () => Promise<void>) {
  let failed = false;
  try {
    await check();
  } catch {
    failed = true;
  }
  expect(failed, 'helper rejects the synthetic defect').toBe(true);
}

test.describe('layout invariant helpers', () => {
  test('in-viewport: rejects a control pushed past the right edge', async ({ page }) => {
    await page.setContent(SOUND);
    await expectInViewport(page.getByRole('button', { name: 'OK' }));

    await page.setContent(
      '<button style="position:fixed;right:-20px;top:10px;width:48px;height:48px">Off</button>'
    );
    await mustFail(() => expectInViewport(page.getByRole('button', { name: 'Off' })));
  });

  test('hit at centre: rejects a control covered by an overlay', async ({ page }) => {
    await page.setContent(SOUND);
    await expectHitAtCenter(page.getByRole('button', { name: 'OK' }));

    await page.setContent(
      '<button style="width:48px;height:48px">Covered</button>' +
        '<div style="position:fixed;inset:0"></div>'
    );
    await mustFail(() => expectHitAtCenter(page.getByRole('button', { name: 'Covered' })));
  });

  test('hit box: rejects a 32px control', async ({ page }) => {
    await page.setContent(SOUND);
    await expectMinHitBox(page.getByRole('button', { name: 'OK' }));

    await page.setContent('<button style="width:32px;height:32px">Small</button>');
    await mustFail(() => expectMinHitBox(page.getByRole('button', { name: 'Small' })));
  });

  test('focus: rejects a control removed from the tab order', async ({ page }) => {
    await page.setContent(SOUND);
    await expectFocusReachable(page.getByRole('button', { name: 'OK' }));

    await page.setContent('<button tabindex="-1" style="width:48px;height:48px">Skip</button>');
    await mustFail(() => expectFocusReachable(page.getByRole('button', { name: 'Skip' })));
  });

  test('clipping: rejects an overflowing ancestor unless it is a scroll container', async ({
    page
  }) => {
    const row = (attr: string) =>
      `<div ${attr} style="width:100px;overflow:hidden;white-space:nowrap">` +
      '<button style="width:48px;height:48px">In</button>' +
      '<span style="display:inline-block;width:300px"></span></div>';

    await page.setContent(row('data-scroll="true"'));
    await expectNoClippingAncestor(page.getByRole('button', { name: 'In' }));

    await page.setContent(row(''));
    await mustFail(() => expectNoClippingAncestor(page.getByRole('button', { name: 'In' })));
  });
});

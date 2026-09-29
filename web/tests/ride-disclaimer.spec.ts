// First-ride disclaimer (spec frame F): shown once, before the first ride.
import { test, expect } from './fixtures';
import { expectLayoutInvariants } from './layout';

// A shareable path URL routes on load; directions are stubbed in fixtures.
const ROUTED = '/?from=35.71480,139.79670,Asakusa&to=35.69870,139.77450,Akihabara';

test.use({ permissions: ['geolocation'], geolocation: { latitude: 35.7148, longitude: 139.7967 } });

test.describe('first-ride disclaimer', () => {
  test('gates the first ride, then never again', async ({ page }) => {
    await page.goto(ROUTED);
    const start = page.getByRole('button', { name: 'Start foreground navigation' });
    await expect(start).toBeVisible();
    await start.click();

    const dialog = page.getByRole('dialog', { name: 'Before you ride' });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole('listitem')).toHaveCount(5);
    await expect(dialog).toContainText('It can’t see the road you’re on.');

    for (const control of [
      dialog.getByRole('button', { name: 'EN' }),
      dialog.getByRole('button', { name: '日本語' }),
      dialog.getByRole('button', { name: 'I understand' }),
      dialog.getByRole('button', { name: 'Data sources' })
    ]) {
      await expectLayoutInvariants(control);
    }

    await dialog.getByRole('button', { name: 'I understand' }).click();
    await expect(dialog).toBeHidden();
    const stop = page.getByRole('button', { name: 'Stop' }).first();
    await expect(stop).toBeVisible();

    // Accepted once: later rides, including after a reload, start directly.
    await stop.click();
    await page.reload();
    await page.getByRole('button', { name: 'Start foreground navigation' }).click();
    await expect(page.getByRole('button', { name: 'Stop' }).first()).toBeVisible();
    await expect(page.getByRole('dialog', { name: 'Before you ride' })).toBeHidden();
  });

  test('dismissing cancels the ride and asks again next time', async ({ page }) => {
    await page.goto(ROUTED);
    const start = page.getByRole('button', { name: 'Start foreground navigation' });
    await start.click();
    const dialog = page.getByRole('dialog', { name: 'Before you ride' });
    await expect(dialog).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expect(start).toBeVisible();

    await start.click();
    await expect(dialog).toBeVisible();
  });

  test('opens the Data sources sheet above itself', async ({ page }) => {
    await page.goto(ROUTED);
    await page.getByRole('button', { name: 'Start foreground navigation' }).click();
    const dialog = page.getByRole('dialog', { name: 'Before you ride' });
    await dialog.getByRole('button', { name: 'Data sources' }).click();
    const sheet = page.getByRole('dialog', { name: 'Data sources' });
    await expect(sheet).toBeVisible();
    await expectLayoutInvariants(sheet.getByRole('button', { name: 'Done' }));
    await sheet.getByRole('button', { name: 'Done' }).click();
    await expect(dialog).toBeVisible();
  });
});

test.describe('first-ride disclaimer in Japanese', () => {
  test.use({ locale: 'ja-JP' });

  test('follows the device locale and switches language', async ({ page }) => {
    await page.goto(ROUTED);
    await page.getByRole('button', { name: 'Start foreground navigation' }).click();

    const ja = page.getByRole('dialog', { name: 'ご利用前に' });
    await expect(ja).toBeVisible();
    await expect(ja).toHaveAttribute('lang', 'ja');
    await expect(ja.getByRole('button', { name: '日本語' })).toHaveAttribute(
      'aria-pressed',
      'true'
    );
    await expect(ja.getByRole('button', { name: '理解しました' })).toBeVisible();
    await expectLayoutInvariants(ja.getByRole('button', { name: '理解しました' }));

    await ja.getByRole('button', { name: 'EN' }).click();
    const en = page.getByRole('dialog', { name: 'Before you ride' });
    await expect(en).toHaveAttribute('lang', 'en');
    await expect(en.getByRole('button', { name: 'I understand' })).toBeVisible();
  });
});

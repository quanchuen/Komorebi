// web/tests/helpers/fixtures.ts
//
// Shared test fixture: animations and transitions are disabled globally so
// layout assertions never race a transition, and network calls to the
// Komorebi API are stubbed per test (the map tiles and basemap are left to
// fail quietly; the UI under test must not depend on them).
import { test as base, expect } from '@playwright/test';

export const test = base.extend({
  page: async ({ page }, use) => {
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await page.addInitScript(() => {
      const style = document.createElement('style');
      style.textContent =
        '*, *::before, *::after { animation: none !important; transition: none !important; }';
      document.addEventListener('DOMContentLoaded', () => document.head.appendChild(style));
    });
    await use(page);
  }
});

export { expect };

/**
 * Navigate and wait until the client has hydrated (the root layout sets
 * `data-hydrated` on <html> in onMount). Server-rendered controls exist
 * before that but cannot act, so tests must not click them earlier.
 */
export async function gotoHydrated(page: import('@playwright/test').Page, path: string) {
  await page.goto(path);
  await page.locator('html[data-hydrated="true"]').waitFor({ state: 'attached' });
}

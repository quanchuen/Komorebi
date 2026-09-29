// Difficulty badges carry no colour: bars plus a label (spec § Difficulty).
import { test, expect } from './fixtures';

test('route card shows a neutral difficulty badge', async ({ page }) => {
  // Viewport discovery starts once the map has loaded; wait for that request
  // rather than a timer.
  const discovered = page.waitForResponse((r) => r.url().includes('/api/v1/discover/viewport'));
  await page.goto('/');
  await discovered;

  const card = page.getByRole('button', { name: /Sumida riverside loop/ });
  await expect(card).toBeVisible();
  const badge = card.getByTestId('difficulty-badge');
  await expect(badge).toHaveText(/Moderate/);
  await expect(badge).toContainText('level 2 of 4');
});

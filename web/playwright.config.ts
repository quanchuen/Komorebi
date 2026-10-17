// web/playwright.config.ts
//
// Browser UI and layout tests. See tests/AGENTS.md for the mandatory rules:
// role/name locators, no sleeps, layout invariants on every interactive element.
import { defineConfig, devices } from '@playwright/test';

const PORT = 4173;

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  // Every test boots a WebGL map; more workers than this starve the GPU and
  // turn map-load timing into flakes.
  workers: 2,
  forbidOnly: !!process.env.CI,
  retries: 0,
  reporter: process.env.CI ? 'github' : 'list',
  timeout: 45_000,
  expect: { timeout: 10_000 },
  use: {
    baseURL: `http://localhost:${PORT}`,
    trace: 'retain-on-failure',
    // Animations are disabled globally in tests/helpers/fixtures.ts.
    reducedMotion: 'reduce'
  },
  projects: [
    {
      name: 'desktop-chromium',
      use: { ...devices['Desktop Chrome'], viewport: { width: 1440, height: 900 } }
    },
    { name: 'mobile-chromium', use: { ...devices['Pixel 7'] } }
  ],
  // Tests run against a production build served by `vite preview`: the dev
  // server reloads the page when it discovers a new dependency, which lands a
  // click on a not-yet-hydrated button. API_URL points nowhere so SSR renders
  // no routes; every API call the browser makes is stubbed in tests/helpers/api.ts.
  webServer: {
    command: `npm run build && npm run preview -- --port ${PORT} --strictPort`,
    url: `http://localhost:${PORT}/`,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    // Curated routes are feature-flagged off by default; the tests cover them on.
    env: { API_URL: 'http://127.0.0.1:9', PUBLIC_FLAG_CURATED_ROUTES: '1' }
  }
});

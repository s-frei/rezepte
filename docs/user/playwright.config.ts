import { defineConfig, devices } from "@playwright/test";

// Screenshots are snapshot tests against `rezepte --demo` on the worktree's
// demo port (`mise run //:demo`). `mise run //docs/user:screenshots` rewrites
// public/screenshots/<name>-<project>.png, `screenshots:check` reports drift.
// Not executed in CI: font rendering differs between macOS and Linux.
export default defineConfig({
  testDir: "screenshots",
  outputDir: "test-results",
  // One worker: every spec shares the demo database and the settings spec
  // creates a user; determinism beats speed for 17 screenshots.
  fullyParallel: false,
  workers: 1,
  // One retry: every run so far lost exactly one test to a 30s timeout, each
  // time a different one, with "screencast.showOverlays: browser has been
  // closed" - Playwright's own recording going away mid-shot, not the page.
  retries: 1,
  reporter: "list",
  snapshotPathTemplate:
    "{testDir}/../public/screenshots/{arg}-{projectName}{ext}",
  expect: {
    toHaveScreenshot: {
      maxDiffPixelRatio: 0.01,
      animations: "disabled",
      caret: "hide",
      scale: "css",
    },
  },
  use: {
    baseURL:
      process.env.SCREENSHOT_BASE_URL ??
      `http://localhost:${process.env.RZP_DEMO_PORT ?? 8070}`,
    // Paraglide has no PARAGLIDE_LOCALE cookie to read before login, so the
    // login screen falls back to this browser locale (its `preferredLanguage`
    // strategy) rather than to REZEPTE_LOCALE; every other screen instead
    // reflects the signed-in account's own locale, which the demo seeds in
    // English (see docs/memory/content/features/languages.mdx).
    locale: "en-US",
    timezoneId: "Europe/Berlin",
    contextOptions: { reducedMotion: "reduce" },
  },
  projects: [
    {
      name: "desktop-light",
      use: {
        ...devices["Desktop Chrome"],
        viewport: { width: 1440, height: 900 },
        colorScheme: "light",
      },
    },
    {
      name: "desktop-dark",
      use: {
        ...devices["Desktop Chrome"],
        viewport: { width: 1440, height: 900 },
        colorScheme: "dark",
      },
    },
    // Pixel 7's default viewport (412px) is documented in "Widths that matter"
    // (docs/memory/content/howtos/verify-ui.mdx) as too forgiving; override to 360px.
    {
      name: "mobile-light",
      use: {
        ...devices["Pixel 7"],
        viewport: { width: 360, height: 780 },
        colorScheme: "light",
      },
    },
    {
      name: "mobile-dark",
      use: {
        ...devices["Pixel 7"],
        viewport: { width: 360, height: 780 },
        colorScheme: "dark",
      },
    },
  ],
});

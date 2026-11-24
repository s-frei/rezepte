import { defineConfig, devices } from "@playwright/test";

// Screenshots are snapshot tests against `rezepte --demo` on the worktree's
// demo port (`mise run //:demo`). `mise run //docs/user:screenshots` rewrites
// public/screenshots/<name>-<project>.png, `screenshots:check` reports drift.
// Not executed in CI: font rendering differs between macOS and Linux.
export default defineConfig({
  testDir: "screenshots",
  outputDir: "test-results",
  // Four workers on one demo instance: the specs only read, except
  // settings.spec.ts, which adds, edits and removes users and tokens. Its
  // four projects run one after another (below), beside the others, so no two
  // of its tests ever see each other's changes.
  fullyParallel: false,
  workers: 4,
  // One retry: every run so far lost exactly one test to a 30s timeout, each
  // time a different one, with "screencast.showOverlays: browser has been
  // closed" - Playwright's own recording going away mid-shot, not the page.
  retries: 1,
  reporter: "list",
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
      `http://localhost:${process.env.RZP_SCREENSHOTS_PORT ?? 8080}`,
    // Paraglide has no PARAGLIDE_LOCALE cookie to read before login, so the
    // login screen falls back to this browser locale (its `preferredLanguage`
    // strategy) rather than to REZEPTE_LOCALE; every other screen instead
    // reflects the signed-in account's own locale, which the demo seeds in
    // English (see docs/memory/content/features/languages.mdx).
    locale: "en-US",
    timezoneId: "Europe/Berlin",
    contextOptions: { reducedMotion: "reduce" },
  },
  // Pixel 7's default viewport (412px) is documented in "Widths that matter"
  // (docs/memory/content/howtos/verify-ui.mdx) as too forgiving; override to 360px.
  // `look` (desktop-light ... mobile-dark) ends every screenshot's file name;
  // read it with look() from screenshots/helpers.ts, not the project name.
  projects: (
    [
      ["desktop", devices["Desktop Chrome"], { width: 1440, height: 900 }],
      ["mobile", devices["Pixel 7"], { width: 360, height: 780 }],
    ] as const
  )
    .flatMap(([form, device, viewport]) =>
      (["light", "dark"] as const).map((colorScheme) => ({
        look: `${form}-${colorScheme}`,
        use: { ...device, viewport, colorScheme },
      })),
    )
    .flatMap(({ look, use }, i, looks) => {
      const shared = {
        use,
        metadata: { look },
        snapshotPathTemplate: `{testDir}/../public/screenshots/{arg}-${look}{ext}`,
      };
      return [
        { ...shared, name: look, testIgnore: /settings\.spec\.ts/ },
        {
          ...shared,
          name: `settings ${look}`,
          testMatch: /settings\.spec\.ts/,
          // Each waits for the one before, so one settings project runs at a time.
          dependencies: i > 0 ? [`settings ${looks[i - 1].look}`] : [],
        },
      ];
    }),
});

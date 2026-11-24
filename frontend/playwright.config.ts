import { defineConfig, devices } from '@playwright/test';

// An iPhone at one device pixel per CSS pixel. Linux WebKit paints on the
// CPU, and at the iPhone's 3x every repaint blurs nine times the pixels of
// each card's shadow: on CI's four shared cores a frame took a second, and
// every click waited two frames for its target to hold still. No test here
// measures device pixels (the share image picks its own scale).
const iPhone = { ...devices['iPhone 15'], deviceScaleFactor: 1 };

// E2E tests run against the production binary (mise run e2e), which serves
// the SPA and the API on one origin. Override with E2E_BASE_URL for a dev server.
export default defineConfig({
	testDir: 'e2e',
	fullyParallel: true,
	retries: process.env.CI ? 1 : 0,
	// The default, half the cores, leaves two of a hosted runner's four idle.
	workers: process.env.CI ? '100%' : undefined,
	// On CI the JSON report feeds scripts/flaky-summary.ts, which lists the
	// tests the retry saved in the job summary.
	reporter: process.env.CI
		? [['github'], ['json', { outputFile: 'playwright-results.json' }]]
		: 'list',
	use: {
		baseURL: process.env.E2E_BASE_URL ?? `http://localhost:${process.env.RZP_BACKEND_PORT ?? 8060}`,
		trace: 'retain-on-failure',
		screenshot: 'only-on-failure'
	},
	projects: [
		{ name: 'desktop', use: { ...devices['Desktop Chrome'] } },
		// Only the files with a phone path of their own: the rest would run the
		// same steps again at 412px. cook-timers and source size their own
		// viewport; the others never ask isMobile. public-shares and
		// link-preview flip instance-wide switches, so only one project runs them.
		{
			name: 'mobile',
			use: { ...devices['Pixel 7'] },
			testIgnore:
				/(cook-timers|notifications|tokens|source|locale|required-fields|transfer|sign-in-settings|smoke|titles|icons|lightbox-window|top-bar|public-shares|link-preview)\.test\.ts/
		},
		// Safari draws the recipe card differently enough (fonts, pictures,
		// canvas size) to deserve its own run, and focuses no button on click,
		// which the step-time popover has to survive; only those tests pay for it.
		{
			name: 'webkit',
			use: iPhone,
			testMatch: /(share-image|step-times)\.test\.ts/
		},
		// Safari 26.4 lays the overview out with native `grid-lanes`, the path
		// the Chromium projects (the JS fallback) never exercise, and scrolls
		// on its own terms while a focused card is held and let go of.
		{
			name: 'webkit-masonry',
			use: iPhone,
			testMatch: /recipes\.test\.ts/,
			grep: /text card|cards (never|re-flow)|card per row|search field stays|holds still|let go of with Done/
		},
		// The desktop card sizes on the same native path, and the narrow
		// row's focus scroll, which WebKit's own scroll to a focus cut short.
		{
			name: 'webkit-desktop-masonry',
			use: { ...devices['Desktop Safari'] },
			testMatch: /recipes\.test\.ts/,
			grep: /card size|overview columns|clear of the arrows/
		}
	]
});

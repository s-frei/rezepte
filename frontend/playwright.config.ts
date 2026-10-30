import { defineConfig, devices } from '@playwright/test';

// E2E tests run against the production binary (mise run e2e), which serves
// the SPA and the API on one origin. Override with E2E_BASE_URL for a dev server.
export default defineConfig({
	testDir: 'e2e',
	fullyParallel: true,
	retries: process.env.CI ? 1 : 0,
	// The default, half the cores, leaves two of a hosted runner's four idle.
	workers: process.env.CI ? '100%' : undefined,
	reporter: process.env.CI ? 'github' : 'list',
	use: {
		baseURL: process.env.E2E_BASE_URL ?? `http://localhost:${process.env.RZP_BACKEND_PORT ?? 8060}`,
		trace: 'retain-on-failure',
		screenshot: 'only-on-failure'
	},
	projects: [
		{ name: 'desktop', use: { ...devices['Desktop Chrome'] } },
		// Only the files with a phone path of their own: the rest would run the
		// same steps again at 412px. cook-timers and source size their own
		// viewport; the others never ask isMobile.
		{
			name: 'mobile',
			use: { ...devices['Pixel 7'] },
			testIgnore:
				/(cook-timers|notifications|tokens|source|locale|required-fields|transfer|sign-in-settings|smoke|titles|icons)\.test\.ts/
		},
		// Safari draws the recipe card differently enough (fonts, pictures,
		// canvas size) to deserve its own run, and focuses no button on click,
		// which the step-time popover has to survive; only those tests pay for it.
		{
			name: 'webkit',
			use: { ...devices['iPhone 15'] },
			testMatch: /(share-image|step-times)\.test\.ts/
		},
		// Safari 26.4 lays the overview out with native `grid-lanes`, the path
		// the Chromium projects (the JS fallback) never exercise.
		{
			name: 'webkit-masonry',
			use: { ...devices['iPhone 15'] },
			testMatch: /recipes\.test\.ts/,
			grep: /text card|cards (never|re-flow)|card per row/
		},
		// The desktop card sizes on the same native path.
		{
			name: 'webkit-desktop-masonry',
			use: { ...devices['Desktop Safari'] },
			testMatch: /recipes\.test\.ts/,
			grep: /card size|overview columns/
		}
	]
});

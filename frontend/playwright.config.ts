import { defineConfig, devices } from '@playwright/test';

// E2E tests run against the production binary (mise run e2e), which serves
// the SPA and the API on one origin. Override with E2E_BASE_URL for a dev server.
export default defineConfig({
	testDir: 'e2e',
	fullyParallel: true,
	retries: process.env.CI ? 1 : 0,
	reporter: process.env.CI ? 'github' : 'list',
	use: {
		baseURL: process.env.E2E_BASE_URL ?? `http://localhost:${process.env.RZP_BACKEND_PORT ?? 8060}`,
		trace: 'retain-on-failure',
		screenshot: 'only-on-failure'
	},
	projects: [
		{ name: 'desktop', use: { ...devices['Desktop Chrome'] } },
		{ name: 'mobile', use: { ...devices['Pixel 7'] } }
	]
});

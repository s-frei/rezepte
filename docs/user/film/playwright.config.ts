import { defineConfig } from '@playwright/test';

import base from '../playwright.config';

// The product film (`mise run //docs/user:film`): its own config, so the
// screenshot projects neither run it nor wait for it. Same demo instance,
// locale and time zone as the screenshots, but motion is allowed - a film of
// an app with its transitions off looks broken.
export default defineConfig({
	testDir: '.',
	outputDir: '../test-results/film',
	workers: 1,
	retries: 0,
	timeout: 180_000,
	reporter: 'list',
	use: {
		...base.use,
		contextOptions: { reducedMotion: 'no-preference' },
		viewport: { width: 1200, height: 750 }
	},
	projects: [{ name: 'light', metadata: { look: 'desktop-light' }, use: { colorScheme: 'light' } }]
});

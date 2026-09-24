import { test } from '@playwright/test';
import { prepare, shot } from './helpers';

test('login', async ({ page }, testInfo) => {
	await prepare(page, testInfo);
	await page.goto('/login');
	await shot(page, 'login');
});

// A fresh instance has no recipes, but the demo seeds some, so the three list
// calls the overview makes are answered empty. Only the overview's own reads
// are stubbed; the session and everything else still come from the instance.
test('first-login', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.route(/\/api\/v1\/recipes(\?|$)/, (route) =>
		route.fulfill({ json: { items: [], page: 1, limit: 24, total: 0 } })
	);
	await page.route(/\/api\/v1\/(tags|authors)(\?|$)/, (route) => route.fulfill({ json: { items: [] } }));
	await page.goto('/');
	await page.getByText('No recipes yet').waitFor();
	await shot(page, 'first-login');
});

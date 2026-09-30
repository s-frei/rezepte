import { expect, test } from '@playwright/test';
import { prepare, shot } from './helpers';

test('login', async ({ page }, testInfo) => {
	await prepare(page, testInfo);
	await page.goto('/login');
	await shot(page, 'login');
});

// The demo's invited account has an open link, but only its hash is stored,
// so this issues one itself through the API, as the signed-in admin the page
// is - then drops the session, since the person a link reaches is not
// signed in (a signed-in browser gets a notice above the form).
test('welcome', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	const origin = new URL(page.url()).origin;
	const res = await page.request.post('/api/v1/users', {
		headers: { Origin: origin },
		data: { username: `welcome-${testInfo.project.name}`, role: 'user' }
	});
	const created = (await res.json()) as { setupLink: { path: string } };
	await page.context().clearCookies({ name: 'rezepte_session' });
	await page.goto(created.setupLink.path);
	await expect(page.getByRole('heading', { name: /^Welcome,/ })).toBeVisible();
	await shot(page, 'welcome');
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

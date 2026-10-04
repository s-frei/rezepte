import { expect, test } from '@playwright/test';
import { configureMail, prepare, shot, turnMailOff } from './helpers';

test('login', async ({ page }, testInfo) => {
	await prepare(page, testInfo);
	await page.goto('/login');
	await shot(page, 'login');
});

// The demo's invited account has an open link, but only its hash is stored,
// so this issues one itself through the API, as the signed-in admin the page
// is - then drops the session, since the person a link reaches is not
// signed in (a signed-in browser gets a notice above the form). The account
// goes again afterwards: the run shares one instance, and the people
// screenshots would otherwise list it.
test('welcome', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	const origin = new URL(page.url()).origin;
	const res = await page.request.post('/api/v1/users', {
		headers: { Origin: origin },
		data: { username: `welcome-${testInfo.project.name}`, role: 'user' }
	});
	const created = (await res.json()) as { id: string; setupLink: { path: string } };
	const session = (await page.context().cookies()).filter((c) => c.name === 'rezepte_session');
	await page.context().clearCookies({ name: 'rezepte_session' });
	await page.goto(created.setupLink.path);
	await expect(page.getByRole('heading', { name: /^Welcome,/ })).toBeVisible();
	await shot(page, 'welcome');
	await page.context().addCookies(session);
	await page.request.delete(`/api/v1/users/${created.id}`, { headers: { Origin: origin } });
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

// The link shows only while mail is configured; the screenshot instance has
// none, so it is set up for this shot and the session dropped, since the
// login page is what a signed-out person sees.
test('login-forgot-link', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await configureMail(page);
	const session = (await page.context().cookies()).filter((c) => c.name === 'rezepte_session');
	try {
		await page.context().clearCookies({ name: 'rezepte_session' });
		await page.goto('/login');
		await expect(page.getByRole('link', { name: 'Forgot password?' })).toBeVisible();
		await shot(page, 'login-forgot-link');
	} finally {
		await page.context().addCookies(session);
		await turnMailOff(page);
	}
});

// Stubbed as on, since mail is instance-wide; the name comes from the login
// page the way a person carries it over.
test('forgot-password', async ({ page }, testInfo) => {
	await prepare(page, testInfo);
	await page.route('**/api/v1/auth/password', (route) => route.fulfill({ json: { available: true } }));
	await page.goto('/login');
	await page.getByLabel('Username').fill('mila');
	await page.getByRole('link', { name: 'Forgot password?' }).click();
	await expect(page.getByLabel('Username or email')).toHaveValue('mila');
	await shot(page, 'forgot-password');
});

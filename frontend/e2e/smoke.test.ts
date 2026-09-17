import { expect, test } from '@playwright/test';
import { login } from './helpers';

test('serves the app shell', async ({ page }) => {
	await login(page);
	await expect(page.getByRole('heading', { name: 'Rezepte' })).toBeVisible();
});

test('health endpoint answers', async ({ request }) => {
	const res = await request.get('/healthz');
	expect(res.ok()).toBeTruthy();
	expect(await res.text()).toContain('ok');
});

test('serves the app shell for deep links', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/recipes/some-deep-link');
	await expect(page.getByRole('heading', { name: 'Rezepte' })).toBeVisible();
});

test('immutable assets are cached', async ({ request }) => {
	const html = await (await request.get('/')).text();
	const match = html.match(/\/_app\/immutable\/[^"']+\.js/);
	expect(match).not.toBeNull();
	const res = await request.get(match![0]);
	expect(res.headers()['cache-control']).toBe('public, max-age=31536000, immutable');
});

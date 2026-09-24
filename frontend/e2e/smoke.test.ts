import { expect, test } from '@playwright/test';
import { login } from './helpers';

test('serves the app shell', async ({ page }) => {
	await login(page);
	// The word "Rezepte" in the top bar is the logo, not a heading, and the
	// bar itself is desktop-only - the overview's own headline is what both
	// projects can see.
	await expect(page.getByRole('heading', { name: 'What are we cooking today?' })).toBeVisible();
});

test('health endpoint answers with a status and a version', async ({ request }) => {
	const res = await request.get('/healthz');
	expect(res.ok()).toBeTruthy();
	// The version differs per build, so assert only that one is reported - the
	// point is that /healthz carries it at all.
	const body = (await res.json()) as { status?: string; version?: string };
	expect(body.status).toBe('ok');
	expect(body.version).toBeTruthy();
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

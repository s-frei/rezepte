import { expect, test } from '@playwright/test';

test('serves the app shell', async ({ page }) => {
	await page.goto('/');
	await expect(page.getByRole('heading', { name: 'Rezepte' })).toBeVisible();
});

test('health endpoint answers', async ({ request }) => {
	const res = await request.get('/healthz');
	expect(res.ok()).toBeTruthy();
	expect(await res.text()).toContain('ok');
});

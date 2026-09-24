import { expect, test, type Page } from '@playwright/test';
import { login } from './helpers';

// The lockups are inlined and colored by the --color-logo-* tokens, so they
// follow the app's own theme switch, not only the system scheme.
const ink = { light: 'rgb(124, 61, 10)', dark: 'rgb(240, 195, 145)' };

async function inkOf(page: Page, scope: string): Promise<string> {
	return page
		.locator(`${scope} .ink`)
		.first()
		.evaluate((el) => getComputedStyle(el).fill);
}

test('the login page shows the stacked lockup, named Rezepte', async ({ page }) => {
	await page.context().clearCookies();
	await page.goto('/login');
	const logo = page.getByRole('img', { name: 'Rezepte' });
	await expect(logo).toBeVisible();
	await expect(logo.locator('.wordmark')).toHaveCount(1);
	expect(await inkOf(page, '[role="img"][aria-label="Rezepte"]')).toBe(ink.light);
});

test('the lockup takes the dark ink in the dark theme', async ({ page }) => {
	await page.context().clearCookies();
	await page.addInitScript(() => window.localStorage.setItem('rezepte-theme', 'dark'));
	await page.goto('/login');
	await expect(page.getByRole('img', { name: 'Rezepte' })).toBeVisible();
	expect(await inkOf(page, '[role="img"][aria-label="Rezepte"]')).toBe(ink.dark);
});

test('the top bar links home with the compact lockup', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'the top bar is desktop-only');
	await login(page);
	const home = page.getByRole('banner').getByRole('link', { name: 'Rezepte', exact: true });
	await expect(home).toBeVisible();
	await expect(home.locator('.wordmark')).toHaveCount(1);
	await expect(home.locator('.sprout')).toHaveCount(1);
});

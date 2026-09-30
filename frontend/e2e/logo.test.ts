import { expect, test, type Locator } from '@playwright/test';
import { login } from './helpers';

// A broken image still has a box, so each check also asks whether it decoded.
async function loaded(img: Locator): Promise<boolean> {
	return img.evaluate(
		(el) => (el as HTMLImageElement).complete && (el as HTMLImageElement).naturalWidth > 0
	);
}

test('the login page shows the stacked lockup, named Rezepte', async ({ page }) => {
	await page.context().clearCookies();
	await page.goto('/login');
	const logo = page.getByRole('img', { name: 'Rezepte' });
	await expect(logo).toBeVisible();
	await expect(logo).toHaveAttribute('src', /rezepte-lockup-stacked/);
	expect(await loaded(logo)).toBe(true);
});

test('the top bar links home with the horizontal lockup', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'the top bar is desktop-only');
	await login(page);
	const home = page.getByRole('banner').getByRole('link', { name: 'Rezepte', exact: true });
	await expect(home).toBeVisible();
	const logo = home.locator('img');
	await expect(logo).toHaveAttribute('src', /rezepte-lockup-horizontal/);
	// The link names the logo, so the picture itself stays silent.
	await expect(logo).toHaveAttribute('alt', '');
	expect(await loaded(logo)).toBe(true);
});

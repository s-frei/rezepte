import { expect, test } from '@playwright/test';
import { createRecipe, loadFixture, login, openNewRecipe, uniqueToken } from './helpers';

// Every route names itself in the browser tab. The titles are asserted after
// client-side navigation, not only after a full load: a route that renders
// no <title> of its own would keep whatever the previous route set.

test('the login page names itself', async ({ page }) => {
	await page.context().clearCookies();
	await page.goto('/login');
	await expect(page).toHaveTitle('Sign in · Rezepte');
});

test('each route sets its own tab title after client-side navigation', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await expect(page).toHaveTitle('Rezepte');

	const title = `Title ${uniqueToken()}`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title });

	// goto loads the document once; the clicks and goBack after it stay inside
	// the SPA, which is where a stale title survives.
	await page.goto('/');
	await openNewRecipe(page);
	await expect(page).toHaveTitle('New recipe · Rezepte');
	await page.goBack();
	await expect(page).toHaveURL('/');
	await expect(page).toHaveTitle('Rezepte');

	await page.goto(`/recipes/${recipe.slug}`);
	await expect(page).toHaveTitle(`${title} · Rezepte`);
	await page
		.getByRole('link', { name: /^(Start cook|Cook) mode$/ })
		.filter({ visible: true })
		.first()
		.click();
	await expect(page).toHaveTitle(`${title} · Cook mode`);
	await page.goBack();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	await expect(page).toHaveTitle(`${title} · Rezepte`);
});

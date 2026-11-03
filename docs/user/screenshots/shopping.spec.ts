import { expect, test, type Page } from '@playwright/test';
import { prepare, shot } from './helpers';

// Shopping lists as the demo seeds them for its admin: "Weekly shop" with
// two samples at 4 servings, typed-in items and three rows checked off, and
// "Saturday market" with typed-in items only. "Weekly shop" is shared with
// Mila, Mila's "Sunday dinner" with the demo person, and Jonas's "Barbecue"
// waits as an invitation. Keep the test names - they are
// the contract with <Screenshot name=…/>.

async function openWeeklyShop(page: Page) {
	await page.goto('/lists');
	await page.getByRole('link', { name: /Weekly shop/ }).click();
	await expect(page.getByRole('heading', { level: 1, name: 'Weekly shop' })).toBeVisible();
}

test('lists-overview', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/lists');
	await expect(page.getByRole('heading', { level: 3, name: 'Saturday market' })).toBeVisible();
	await shot(page, 'lists-overview');
});

test('list', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await openWeeklyShop(page);
	await expect(page.getByRole('heading', { name: /^Done/ })).toBeVisible();
	await shot(page, 'list');
});

test('list-by-recipe', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await openWeeklyShop(page);
	await page.getByRole('radio', { name: 'Recipe' }).click();
	await expect(page.getByRole('heading', { level: 2, name: 'Without a recipe' })).toBeVisible();
	await shot(page, 'list-by-recipe');
});

test('add-to-list', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/recipes/shepherd-s-pie');
	// Salt is at home: checked off on the recipe, it starts deselected.
	const salt = page.getByRole('checkbox', { name: 'salt', exact: true });
	await expect(salt).toHaveCount(2);
	await salt.first().click();
	await salt.last().click();
	await page.getByRole('button', { name: 'Add to shopping list' }).click();
	const dialog = page.getByRole('dialog');
	await expect(dialog.getByText('Ingredients · 12 of 14')).toBeVisible();
	await shot(page, 'add-to-list');
});

test('lists-invitation', async ({ page }, testInfo) => {
  await prepare(page, testInfo, { login: true });
  await page.goto('/lists');
  await expect(page.getByText(/invites you to/)).toBeVisible();
  await shot(page, 'lists-invitation');
});

test('list-share', async ({ page }, testInfo) => {
  await prepare(page, testInfo, { login: true });
  await openWeeklyShop(page);
  await page.getByRole('button', { name: 'Share', exact: true }).first().click();
  await expect(page.getByRole('dialog').getByText('Has access')).toBeVisible();
  await shot(page, 'list-share');
});

test('list-shop', async ({ page }, testInfo) => {
  await prepare(page, testInfo, { login: true });
  await openWeeklyShop(page);
  await page.getByRole('link', { name: 'Shop', exact: true }).first().click();
  await expect(page.getByRole('main', { name: 'Shopping mode' })).toBeVisible();
  await shot(page, 'list-shop');
});

test('list-shop-entry', async ({ page }, testInfo) => {
  await prepare(page, testInfo, { login: true });
  await openWeeklyShop(page);
  await expect(page.getByRole('link', { name: 'Shop', exact: true }).first()).toBeVisible();
  await shot(page, 'list-shop-entry');
});

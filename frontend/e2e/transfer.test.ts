import { expect, test } from '@playwright/test';
import {
	createRecipe,
	createUser,
	loadFixture,
	login,
	signOut,
	tinyPng,
	uniqueToken,
	uploadImage
} from './helpers';

test('an exported recipe comes back with its photos after an import', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	const title = `Transfer ${uniqueToken()}`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title });
	await uploadImage(page, recipe.id, tinyPng([180, 83, 9]));
	await uploadImage(page, recipe.id, tinyPng([127, 176, 105]));

	await page.goto('/settings/transfer');
	await page.getByRole('searchbox').fill(title);
	await page.getByRole('checkbox', { name: title }).check();
	const downloading = page.waitForEvent('download');
	await page.getByRole('button', { name: 'Export .zip' }).click();
	const file = await (await downloading).path();

	const origin = new URL(page.url()).origin;
	const deleted = await page.request.delete(`/api/v1/recipes/${recipe.id}`, {
		headers: { Origin: origin }
	});
	expect(deleted.ok()).toBeTruthy();
	await page.reload();

	await page.getByLabel('Choose a file').setInputFiles(file);
	await expect(page.getByRole('checkbox', { name: title })).toBeChecked();
	await page.getByRole('button', { name: 'Import 1 recipe' }).click();
	await expect(page.getByText('✓ Imported', { exact: true })).toBeVisible();

	const res = await page.request.get(`/api/v1/recipes?q=${encodeURIComponent(title)}`);
	const { items } = await res.json();
	expect(items).toHaveLength(1);
	expect(items[0].imageCount).toBe(2);
});

test('the import waits for the recipe list before it takes a file', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	// Hold the list back: until it arrives, a file would tick every duplicate.
	let release!: () => void;
	const held = new Promise<void>((resolve) => (release = resolve));
	await page.route('**/api/v1/recipes?*', async (route) => {
		await held;
		await route.continue();
	});
	await page.goto('/settings/transfer');
	await expect(page.getByRole('heading', { name: 'Import' })).toBeVisible();
	await expect(page.getByLabel('Choose a file')).toHaveCount(0);
	release();
	await expect(page.getByLabel('Choose a file')).toHaveCount(1);
});

test('a member sees neither the page nor the import button', async ({ page }, testInfo) => {
	await login(page);
	await expect(page).toHaveURL('/');
	const username = `member${uniqueToken()}`;
	await createUser(page, { username, role: 'user' });
	await signOut(page, testInfo);
	await login(page, username);
	await expect(page).toHaveURL('/');
	await expect(page.getByRole('link', { name: 'Import', exact: true })).toHaveCount(0);
	await page.goto('/settings/transfer');
	await expect(page).toHaveURL('/settings');
});

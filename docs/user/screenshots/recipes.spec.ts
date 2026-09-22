import { expect, test } from '@playwright/test';
import { prepare, shot } from './helpers';

const DETAIL = '/recipes/shepherd-s-pie';

test('recipe-detail', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto(DETAIL);
	await expect(page.getByRole('heading', { level: 1, name: "Shepherd's Pie" })).toBeVisible();
	await shot(page, 'recipe-detail');
});

test('lightbox', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto(DETAIL);
	// The demo gives the first nine samples one image each, so the label is
	// "Open photo 1 of 1"; the regex survives a second image.
	await page.getByRole('button', { name: /^Open photo 1 of/ }).click();
	await expect(page.getByRole('dialog')).toBeVisible();
	await shot(page, 'lightbox');
});

test('editor', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/recipes/new');
	// Role locators, like frontend/e2e: every editor list item repeats the
	// label of the field inside it, which leaves getByLabel ambiguous.
	await page.getByRole('textbox', { name: 'Title', exact: true }).fill('Onion Tart');
	await shot(page, 'editor');
});

test('editor-ingredients', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto(`${DETAIL}/edit`);
	await page.getByRole('heading', { name: 'Ingredients' }).scrollIntoViewIfNeeded();
	await shot(page, 'editor-ingredients');
});

test('editor-images', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto(`${DETAIL}/edit`);
	await page.getByRole('heading', { name: 'Photos' }).scrollIntoViewIfNeeded();
	await shot(page, 'editor-images');
});

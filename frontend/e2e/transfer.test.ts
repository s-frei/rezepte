import { readFileSync } from 'node:fs';
import { expect, test, type Page } from '@playwright/test';
import { unzipSync } from 'fflate';
import type { RecipeInput } from '../src/lib/api/recipes';
import type { PdfRequest } from '../src/lib/recipe/pdf-document';
import {
	createRecipe,
	createUser,
	loadFixture,
	login,
	openImport,
	signOut,
	tinyPng,
	uniqueToken,
	uploadImage
} from './helpers';

test('an exported recipe comes back with its photos after an import', async ({ page }) => {
	await login(page);
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

	const deleted = await page.request.delete(`/api/v1/recipes/${recipe.id}`);
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

test('a member sees neither the page nor the zip import link', async ({ page }, testInfo) => {
	await login(page);
	const username = `member${uniqueToken()}`;
	await createUser(page, { username, role: 'user' });
	await signOut(page, testInfo);
	await login(page, username);
	// The import dialog is for everyone; its zip-file row is for admins.
	await openImport(page);
	await expect(page.getByRole('link', { name: 'Import a zip file' })).toHaveCount(0);
	await page.keyboard.press('Escape');
	await page.goto('/settings/transfer');
	await expect(page).toHaveURL('/settings');
});

test('a recipe that moves to the next page while the list loads is listed once', async ({
	page
}) => {
	await login(page);
	const token = uniqueToken();
	await createRecipe(page, { ...loadFixture(0), title: `Moved ${token}` });
	await createRecipe(page, { ...loadFixture(1), title: `Stays ${token}` });
	// The list pages through the most recently updated first, so a recipe
	// somebody saves meanwhile pushes the last row of one page onto the next.
	// Play that: page 1 holds both, page 2 starts with the first again.
	const listed = await (await page.request.get(`/api/v1/recipes?q=${token}`)).json();
	await page.route('**/api/v1/recipes?*', async (route) => {
		const n = Number(new URL(route.request().url()).searchParams.get('page') ?? 1);
		const items = n === 1 ? listed.items : n === 2 ? [listed.items[1]] : [];
		await route.fulfill({ json: { ...listed, items, total: 3 } });
	});
	await page.goto('/settings/transfer');
	await expect(page.getByRole('checkbox', { name: `Moved ${token}` })).toHaveCount(1);
	await expect(page.getByLabel('Choose a file')).toHaveCount(1);
});

/** A recipe for the PDF export: 4 servings with 400 g of flour, so scaling shows in the body. */
function pdfRecipe(title: string): RecipeInput {
	return {
		title,
		description: '',
		servings: 4,
		prepMinutes: null,
		cookMinutes: null,
		sourceUrl: null,
		sourceName: null,
		tags: [],
		ingredientGroups: [
			{ name: null, ingredients: [{ quantity: 400, unit: 'g', name: 'Flour', note: null }] }
		],
		steps: [{ text: 'Mix the flour.', references: [], times: [] }]
	};
}

/**
 * Three recipes ticked through the search, created out of title order, so
 * "Title A–Z" and "Last changed" put them in different orders.
 */
async function tickThreeForPdf(page: Page): Promise<string[]> {
	const token = uniqueToken();
	const titles = ['Banana', 'Cherry', 'Apple'].map((t) => `${t} ${token}`);
	for (const title of titles) await createRecipe(page, pdfRecipe(title));
	await page.goto('/settings/transfer');
	await page.getByRole('searchbox').fill(token);
	await expect(page.getByRole('checkbox', { name: titles[0] })).toBeVisible();
	await page.getByRole('button', { name: 'Tick all shown' }).click();
	return titles;
}

/** The instance's settings with `pdf.available` answered, not set: the service decides it at startup. */
async function pdfAvailable(page: Page, available: boolean) {
	await page.route('**/api/v1/settings', async (route) => {
		const response = await route.fetch();
		const json = await response.json();
		await route.fulfill({
			response,
			json: { ...json, pdf: { ...json.pdf, available } }
		});
	});
}

test('As PDF sends the ticked recipes in the list order, scaled to the chosen servings', async ({
	page
}) => {
	await pdfAvailable(page, true);
	const bodies: PdfRequest[] = [];
	await page.route('**/api/v1/pdf', async (route) => {
		bodies.push(route.request().postDataJSON());
		await route.fulfill({
			body: '%PDF-1.7\n',
			contentType: 'application/pdf',
			headers: { 'Content-Disposition': 'attachment; filename="rezepte-2026-11-25.pdf"' }
		});
	});
	await login(page);
	const [banana, cherry, apple] = await tickThreeForPdf(page);

	await page.getByRole('button', { name: 'As PDF' }).click();
	const dialog = page.getByRole('dialog');
	await expect(dialog.getByText('3 recipes')).toBeVisible();
	await dialog.getByRole('radio', { name: 'Adjust' }).click();
	await expect(
		dialog.getByRole('group', { name: 'Servings' }).getByText('2', { exact: true })
	).toBeVisible();
	const downloading = page.waitForEvent('download');
	await dialog.getByRole('button', { name: 'Create PDF' }).click();
	await expect(dialog).toBeHidden();
	expect((await downloading).suggestedFilename()).toBe('rezepte-2026-11-25.pdf');

	const book = bodies[0];
	expect(book.options).toMatchObject({ bundle: 'book', paper: 'a4' });
	expect(book.recipes.map((r) => r.servings.value)).toEqual([2, 2, 2]);
	expect(book.recipes.map((r) => r.title)).toEqual([apple, banana, cherry]);
	expect(book.recipes[0].ingredientGroups[0].ingredients[0].quantity?.display).toBe('200');
	expect(book.book?.title).toBe('Recipes');
	expect(book.book?.date).toMatch(/^\d{4}-\d{2}-\d{2}$/);

	// The other sort, as one PDF per recipe at their own servings.
	await page.getByRole('button', { name: 'Sort by' }).click();
	await page.getByRole('option', { name: 'Last changed' }).click();
	await page.getByRole('button', { name: 'As PDF' }).click();
	await dialog.getByRole('radio', { name: 'One per recipe (.zip)' }).click();
	await dialog.getByRole('radio', { name: 'As in recipe' }).click();
	await dialog.getByRole('button', { name: 'Create PDF' }).click();
	await expect.poll(() => bodies.length).toBe(2);
	expect(bodies[1].options).toMatchObject({ bundle: 'zip' });
	expect(bodies[1].recipes.map((r) => r.servings.value)).toEqual([4, 4, 4]);
	expect(bodies[1].book).toBeNull();
	expect(bodies[1].recipes.map((r) => r.title)).toEqual([apple, cherry, banana]);
	expect(bodies[1].recipes[0].ingredientGroups[0].ingredients[0].quantity?.display).toBe('400');
});

test('without PDF rendering the Export card has no As PDF', async ({ page }) => {
	await pdfAvailable(page, false);
	await login(page);
	await tickThreeForPdf(page);
	await expect(page.getByRole('button', { name: 'Export .zip' })).toBeEnabled();
	await expect(page.getByRole('button', { name: 'As PDF' })).toHaveCount(0);
});

// Against the real renderer: the files the service sends back.
test('As PDF downloads one cookbook PDF', async ({ page }) => {
	await login(page);
	await tickThreeForPdf(page);
	await page.getByRole('button', { name: 'As PDF' }).click();
	const downloading = page.waitForEvent('download');
	await page.getByRole('dialog').getByRole('button', { name: 'Create PDF' }).click();
	const download = await downloading;
	expect(download.suggestedFilename()).toMatch(/^rezepte-\d{4}-\d{2}-\d{2}\.pdf$/);
	expect(
		readFileSync(await download.path())
			.subarray(0, 4)
			.toString()
	).toBe('%PDF');
});

test('As PDF downloads a zip with one PDF per recipe', async ({ page }) => {
	await login(page);
	const titles = await tickThreeForPdf(page);
	await page.getByRole('button', { name: 'As PDF' }).click();
	const dialog = page.getByRole('dialog');
	await dialog.getByRole('radio', { name: 'One per recipe (.zip)' }).click();
	const downloading = page.waitForEvent('download');
	await dialog.getByRole('button', { name: 'Create PDF' }).click();
	const entries = unzipSync(readFileSync(await (await downloading).path()));
	expect(Object.keys(entries).sort()).toEqual(titles.map((t) => `${t}.pdf`).sort());
});

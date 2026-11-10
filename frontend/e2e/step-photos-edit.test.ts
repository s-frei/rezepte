import { expect, test, type Page } from '@playwright/test';
import type { Recipe, RecipeInput } from '#lib/api/recipes.js';
import {
	createRecipe,
	expectSpotlight,
	login,
	selectInStep,
	tinyPng,
	uniqueToken,
	uploadImage
} from './helpers';

// Linking a step's words to a photo in the editor and saving it: an edited
// recipe (desktop, and the mobile project through the sheet), a new one whose
// photos are only queued, and the cases where the photo is gone or still
// uploading when the save goes out.

test.beforeEach(async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
});

const SCRAPE = 'Scrape the dough from the board into the boiling water.';
const REST = 'Rest the dough for 10 minutes.';

function input(first = SCRAPE): RecipeInput {
	return {
		title: `Photo link ${uniqueToken()}`,
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
		steps: [
			{ text: first, references: [], times: [] },
			{ text: REST, references: [], times: [{ phrase: '10 minutes', seconds: 600 }] }
		],
		editPolicy: 'open'
	};
}

/** A stored recipe with `count` photos, open in the editor. */
async function editRecipeWithPhotos(page: Page, count = 3, first = SCRAPE) {
	const recipe = await createRecipe(page, input(first));
	const ids: string[] = [];
	for (let i = 0; i < count; i++) {
		ids.push((await uploadImage(page, recipe.id, tinyPng([60 * i, 120, 200 - 40 * i]))).id);
	}
	await page.goto(`/recipes/${recipe.slug}/edit`);
	await expect(step(page, 1)).toBeVisible();
	return { recipe, ids };
}

const step = (page: Page, number: number) =>
	page.getByRole('textbox', { name: `Step ${number}`, exact: true });

/** Selects `words` (or puts the caret in them) in step `number`, then picks photo `photo` of `count`. */
async function linkPhoto(
	page: Page,
	number: number,
	words: string,
	what: 'word' | 'caret',
	photo: number,
	count: number
) {
	// Focused first, so no click lands in the step after the selection is set.
	await step(page, number).focus();
	await selectInStep(step(page, number), words, what);
	await page.getByRole('button', { name: 'Link photo', exact: true }).click();
	await page.getByRole('button', { name: `${photo} / ${count}`, exact: true }).click();
	await page.getByRole('button', { name: `Link photo ${photo}`, exact: true }).click();
	await expect(step(page, number).locator('.photo-link')).toHaveCount(1);
}

async function save(page: Page) {
	await page.getByRole('button', { name: 'Save', exact: true }).click();
	// The detail page, not the editor (`/edit`) nor the new recipe's (`/new`).
	await expect(page).toHaveURL(/\/recipes\/(?!new$)[^/]+$/);
}

async function stored(page: Page, id: string): Promise<Recipe> {
	const response = await page.request.get(`/api/v1/recipes/${id}`);
	expect(response.ok()).toBeTruthy();
	return (await response.json()) as Recipe;
}

test('links the word at the caret to a photo of an edited recipe', async ({ page }) => {
	const { recipe, ids } = await editRecipeWithPhotos(page);
	await linkPhoto(page, 1, 'Scrape', 'caret', 3, 3);
	await save(page);
	expect((await stored(page, recipe.id)).steps[0].photos).toEqual([
		{ phrase: 'Scrape', imageId: ids[2] }
	]);
});

test('the phone picker marks its phrase and lights its step below the bar', async ({
	page,
	isMobile
}) => {
	test.skip(!isMobile, 'the sheet and its spotlight are the phone path');
	const long =
		'Scrape the dough from the board into the boiling water in thin strips, a little at a time, stir once so the strips do not stick, and lift them out once they float.';
	await editRecipeWithPhotos(page, 3, long);
	const field = step(page, 1);
	await field.focus();
	await selectInStep(field, 'they float', 'word');
	// Low on the screen, so the reveal lifts the step far, under the editor's bar.
	await field.evaluate((element) =>
		window.scrollBy(0, element.getBoundingClientRect().bottom - window.innerHeight + 30)
	);
	await page.getByRole('button', { name: 'Link photo', exact: true }).click();
	await expect(page.getByRole('dialog')).toBeVisible();

	// Marked in the step, not selected: no handles over the sheet.
	await expect(field.locator('[data-photo-picking]')).toHaveText('they float');
	expect(await page.evaluate(() => String(window.getSelection()))).toBe('');
	await expectSpotlight(field);

	// Closed without a pick: the mark goes, nothing was linked.
	await page.keyboard.press('Escape');
	await expect(page.getByRole('dialog')).toHaveCount(0);
	await expect(field.locator('[data-photo-picking]')).toHaveCount(0);
	await expect(field.locator('.photo-link')).toHaveCount(0);
});

test('the phone picker keeps its button above the bottom edge for a long phrase', async ({
	page,
	isMobile
}) => {
	test.skip(!isMobile, 'the sheet is the phone path');
	// Short enough that header, photo, strip and button outgrow 80dvh: the
	// photo shrinks and the button stays in the footer with its padding.
	await page.setViewportSize({ width: 360, height: 560 });
	const long =
		'Scrape the dough from the board into the boiling water in thin strips, a little at a time';
	await editRecipeWithPhotos(page, 3, `${long}, and lift them out once they float.`);
	const field = step(page, 1);
	await field.focus();
	await selectInStep(field, long, 'word');
	await page.getByRole('button', { name: 'Link photo', exact: true }).click();
	const sheet = page.getByRole('dialog');
	const button = sheet.getByRole('button', { name: 'Link photo 1', exact: true });
	await expect(button).toBeVisible();

	// The fly-in settled: the sheet rests on the bottom edge.
	const bottom = async () => {
		const box = (await sheet.boundingBox())!;
		return box.y + box.height;
	};
	await expect.poll(bottom).toBeCloseTo(560, 0);
	const box = (await sheet.boundingBox())!;
	const action = (await button.boundingBox())!;
	expect(action.y).toBeGreaterThanOrEqual(box.y);
	expect(action.y + action.height).toBeLessThanOrEqual(box.y + box.height - 24);
	// The header is clamped, the strip whole above the button.
	const strip = (await sheet.getByRole('button', { name: '3 / 3', exact: true }).boundingBox())!;
	expect(strip.y + strip.height).toBeLessThanOrEqual(action.y);
});

test('links several selected words as one phrase', async ({ page, isMobile }) => {
	test.skip(isMobile, 'the editor flow is covered on the phone by the test above');
	const { recipe, ids } = await editRecipeWithPhotos(page);
	await linkPhoto(page, 1, 'the boiling water', 'word', 2, 3);
	await save(page);
	expect((await stored(page, recipe.id)).steps[0].photos).toEqual([
		{ phrase: 'the boiling water', imageId: ids[1] }
	]);
});

test('a new recipe links a photo that is only queued', async ({ page, isMobile }) => {
	test.skip(isMobile, 'the save flow does not depend on the layout');
	const title = `Queued link ${uniqueToken()}`;
	await page.goto('/recipes/new');
	await page.getByRole('textbox', { name: 'Title', exact: true }).fill(title);
	await page.getByRole('textbox', { name: 'Ingredient', exact: true }).fill('Flour');
	await page.getByLabel('Choose photos').setInputFiles([
		{ name: 'one.png', mimeType: 'image/png', buffer: tinyPng([200, 120, 40]) },
		{ name: 'two.png', mimeType: 'image/png', buffer: tinyPng([40, 120, 200]) }
	]);
	await step(page, 1).click();
	await page.keyboard.type(SCRAPE);
	await linkPhoto(page, 1, 'Scrape', 'caret', 2, 2);
	const created = page.waitForResponse(
		(response) =>
			response.request().method() === 'POST' && response.url().endsWith('/api/v1/recipes')
	);
	await save(page);

	const recipe = await stored(page, ((await (await created).json()) as Recipe).id);
	expect(recipe.steps[0].photos).toEqual([{ phrase: 'Scrape', imageId: recipe.images[1].id }]);
	await page.locator('[data-step-photo]').click();
	await expect(page.getByRole('dialog').getByText('Photo 2 of 2')).toBeVisible();
});

test('a new recipe whose links fail to save opens its editor, created once', async ({
	page,
	isMobile
}) => {
	test.skip(isMobile, 'the save flow does not depend on the layout');
	const title = `Links failed ${uniqueToken()}`;
	await page.route('**/api/v1/recipes/*', (route) =>
		route.request().method() === 'PUT'
			? route.fulfill({ status: 500, json: { title: 'Internal Server Error', status: 500 } })
			: route.continue()
	);
	let creates = 0;
	page.on('request', (request) => {
		if (request.method() === 'POST' && request.url().endsWith('/api/v1/recipes')) creates++;
	});
	await page.goto('/recipes/new');
	await page.getByRole('textbox', { name: 'Title', exact: true }).fill(title);
	await page.getByRole('textbox', { name: 'Ingredient', exact: true }).fill('Flour');
	await page
		.getByLabel('Choose photos')
		.setInputFiles([{ name: 'one.png', mimeType: 'image/png', buffer: tinyPng() }]);
	await step(page, 1).click();
	await page.keyboard.type(SCRAPE);
	await linkPhoto(page, 1, 'Scrape', 'caret', 1, 1);
	const created = page.waitForResponse(
		(response) =>
			response.request().method() === 'POST' && response.url().endsWith('/api/v1/recipes')
	);
	await page.getByRole('button', { name: 'Save', exact: true }).click();

	await expect(
		page.getByText('Saved, but the photo links in the steps could not be set.', { exact: false })
	).toBeVisible();
	const recipe = await stored(page, ((await (await created).json()) as Recipe).id);
	// Its editor, where the author links again and saves.
	await expect(page).toHaveURL(`/recipes/${recipe.slug}/edit`);
	await expect(step(page, 1)).toHaveText(SCRAPE);
	expect(creates).toBe(1);
	expect(recipe.images).toHaveLength(1);
	expect(recipe.steps[0].photos ?? []).toEqual([]);
});

test('a new recipe whose linked photo fails to upload opens its editor', async ({
	page,
	isMobile
}) => {
	test.skip(isMobile, 'the save flow does not depend on the layout');
	await page.route('**/api/v1/recipes/*/images', (route) =>
		route.request().method() === 'POST' ? route.abort('failed') : route.continue()
	);
	await page.goto('/recipes/new');
	await page
		.getByRole('textbox', { name: 'Title', exact: true })
		.fill(`Upload failed ${uniqueToken()}`);
	await page.getByRole('textbox', { name: 'Ingredient', exact: true }).fill('Flour');
	await page
		.getByLabel('Choose photos')
		.setInputFiles([{ name: 'one.png', mimeType: 'image/png', buffer: tinyPng() }]);
	await step(page, 1).click();
	await page.keyboard.type(SCRAPE);
	await linkPhoto(page, 1, 'Scrape', 'caret', 1, 1);
	await page.getByRole('button', { name: 'Save', exact: true }).click();

	await expect(
		page.getByText('Saved, but the photo links in the steps could not be set.', { exact: false })
	).toBeVisible();
	await expect(page).toHaveURL(/\/recipes\/[^/]+\/edit$/);
});

test('a removed photo whose delete fails says its link went', async ({ page, isMobile }) => {
	test.skip(isMobile, 'the save flow does not depend on the layout');
	const { recipe, ids } = await editRecipeWithPhotos(page);
	await linkPhoto(page, 1, 'Scrape', 'caret', 3, 3);
	await page.route('**/api/v1/recipes/*/images/*', (route) =>
		route.request().method() === 'DELETE'
			? route.fulfill({ status: 500, json: { title: 'Internal Server Error', status: 500 } })
			: route.continue()
	);
	await page.getByRole('button', { name: 'Photo 3 options' }).click();
	await page.getByRole('menuitem', { name: 'Remove' }).click();
	await save(page);

	await expect(page.getByText('its link in the steps was removed', { exact: false })).toBeVisible();
	const after = await stored(page, recipe.id);
	expect(after.images.map((image) => image.id)).toEqual(ids);
	expect(after.steps[0].photos ?? []).toEqual([]);
});

test('a removal undone keeps the link, one not undone saves without it', async ({
	page,
	isMobile
}) => {
	test.skip(isMobile, 'the save flow does not depend on the layout');
	const { recipe, ids } = await editRecipeWithPhotos(page);
	await linkPhoto(page, 1, 'Scrape', 'caret', 3, 3);

	await page.getByRole('button', { name: 'Photo 3 options' }).click();
	await page.getByRole('menuitem', { name: 'Remove' }).click();
	await expect(step(page, 1).locator('.photo-link')).toHaveCount(0);
	await page.getByRole('button', { name: 'Undo' }).click();
	await expect(step(page, 1).locator('.photo-link')).toHaveCount(1);
	await save(page);
	expect((await stored(page, recipe.id)).steps[0].photos).toEqual([
		{ phrase: 'Scrape', imageId: ids[2] }
	]);

	await page.goto(`/recipes/${recipe.slug}/edit`);
	await expect(step(page, 1).locator('.photo-link')).toHaveCount(1);
	await page.getByRole('button', { name: 'Photo 3 options' }).click();
	await page.getByRole('menuitem', { name: 'Remove' }).click();
	await expect(step(page, 1).locator('.photo-link')).toHaveCount(0);
	const put = page.waitForResponse(
		(response) => response.request().method() === 'PUT' && response.url().endsWith(recipe.id)
	);
	await save(page);
	expect((await put).status()).toBe(200);
	const after = await stored(page, recipe.id);
	expect(after.images.map((image) => image.id)).toEqual(ids.slice(0, 2));
	expect(after.steps[0].photos ?? []).toEqual([]);
});

test('a failed save leaves a removal waiting for its undo undoable', async ({ page, isMobile }) => {
	test.skip(isMobile, 'the save flow does not depend on the layout');
	const { recipe, ids } = await editRecipeWithPhotos(page);
	await page.route(`**/api/v1/recipes/${recipe.id}`, (route) =>
		route.request().method() === 'PUT'
			? route.fulfill({ status: 500, json: { title: 'Internal Server Error', status: 500 } })
			: route.continue()
	);
	let deletes = 0;
	page.on('request', (request) => {
		if (request.method() === 'DELETE') deletes++;
	});
	await page.getByRole('button', { name: 'Photo 3 options' }).click();
	await page.getByRole('menuitem', { name: 'Remove' }).click();
	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page.getByText('could not be saved', { exact: false }).first()).toBeVisible();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}/edit`);

	// The error stacks over the removal's toast; a hover spreads the stack.
	await page.locator('[data-sonner-toast]').first().hover();
	await page.getByRole('button', { name: 'Undo' }).click();
	await expect(page.getByRole('button', { name: 'Photo 3 options' })).toBeVisible();
	expect(deletes).toBe(0);
	expect((await stored(page, recipe.id)).images.map((image) => image.id)).toEqual(ids);
});

test('a linked photo whose upload fails during the save stops it', async ({ page, isMobile }) => {
	test.skip(isMobile, 'the save flow does not depend on the layout');
	const { recipe } = await editRecipeWithPhotos(page, 2);
	await page.route('**/api/v1/recipes/*/images', async (route) => {
		if (route.request().method() !== 'POST') return route.continue();
		await new Promise((done) => setTimeout(done, 1500));
		await route.abort('failed');
	});
	let puts = 0;
	page.on('request', (request) => {
		if (request.method() === 'PUT' && request.url().endsWith(recipe.id)) puts++;
	});
	await page
		.getByLabel('Choose photos')
		.setInputFiles([{ name: 'late.png', mimeType: 'image/png', buffer: tinyPng([10, 200, 90]) }]);
	await linkPhoto(page, 1, 'Scrape', 'caret', 3, 3);
	await page.getByRole('button', { name: 'Save', exact: true }).click();

	await expect(
		page.getByText('The linked photo could not be uploaded. Retry it under Photos or remove it.')
	).toBeVisible();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}/edit`);
	expect(puts).toBe(0);
});

test('a save waits for an upload still running and links its new id', async ({
	page,
	isMobile
}) => {
	test.skip(isMobile, 'the save flow does not depend on the layout');
	const { recipe } = await editRecipeWithPhotos(page, 2);
	await page.route('**/api/v1/recipes/*/images', async (route) => {
		if (route.request().method() === 'POST') await new Promise((done) => setTimeout(done, 2000));
		await route.continue();
	});
	await page
		.getByLabel('Choose photos')
		.setInputFiles([{ name: 'late.png', mimeType: 'image/png', buffer: tinyPng([10, 200, 90]) }]);
	await linkPhoto(page, 1, 'Scrape', 'caret', 3, 3);
	await save(page);

	const after = await stored(page, recipe.id);
	expect(after.images).toHaveLength(3);
	expect(after.steps[0].photos).toEqual([{ phrase: 'Scrape', imageId: after.images[2].id }]);
});

test('refuses a phrase overlapping a time', async ({ page, isMobile }) => {
	test.skip(isMobile, 'the check does not depend on the layout');
	await editRecipeWithPhotos(page, 1);
	// The caret in "minutes": the word around it is part of the stored time.
	await selectInStep(step(page, 2), 'minutes', 'caret');
	await page.getByRole('button', { name: 'Link photo', exact: true }).click();
	await expect(page.getByText('This phrase overlaps a time').first()).toBeVisible();
	await expect(page.getByRole('button', { name: 'Link photo 1', exact: true })).toHaveCount(0);
	await expect(step(page, 2).locator('.photo-link')).toHaveCount(0);
});

test('a time inside a photo phrase is neither proposed nor markable', async ({
	page,
	isMobile
}) => {
	test.skip(isMobile, 'the check does not depend on the layout');
	const { recipe, ids } = await editRecipeWithPhotos(
		page,
		1,
		'Scrape the dough into the water and boil 5 minutes.'
	);
	await expect(step(page, 1).locator('.time-proposal')).toHaveCount(1);
	// The phrase is the proposed time itself: a selection across the proposal's edge
	// spans two text nodes, which `selectInStep` does not reach.
	await linkPhoto(page, 1, '5 minutes', 'word', 1, 1);
	await expect(step(page, 1).locator('.time-proposal')).toHaveCount(0);

	await step(page, 1).focus();
	await selectInStep(step(page, 1), '5 minutes', 'word');
	await page.getByRole('button', { name: 'Mark as time' }).click();
	await expect(
		page.getByRole('status').filter({ hasText: 'This part of the step is already marked' })
	).toHaveCount(1);
	await save(page);
	const saved = (await stored(page, recipe.id)).steps[0];
	expect(saved.photos).toEqual([{ phrase: '5 minutes', imageId: ids[0] }]);
	expect(saved.times ?? []).toEqual([]);
});

import { expect, test } from '@playwright/test';
import {
	createRecipe,
	loadFixture,
	login,
	search,
	setCover,
	tinyPng,
	uniqueToken,
	uploadImage
} from './helpers';

// Written for `mise run e2e`, but NOT executed in Phase 4: Chromium cannot
// start on the development machine. Keep the locators role-based so the
// suite is ready to run unchanged once a browser is available.

test.beforeEach(async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
});

test('shows the cover on the card and the detail page', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `With photo ${token}` });
	await uploadImage(page, recipe.id, tinyPng([200, 120, 40]));
	// The cover is the second picture, so neither surface may fall back to
	// "the first image in the list".
	const cover = await uploadImage(page, recipe.id, tinyPng([40, 120, 200]));
	await setCover(page, recipe.id, cover.id);

	await page.goto('/');
	await search(page, token);
	const card = page.getByRole('article', { name: `With photo ${token}` });
	await expect(card.locator('img')).toHaveAttribute(
		'src',
		`/images/${recipe.id}/${cover.id}/thumb.jpg`
	);

	await page.goto(`/recipes/${recipe.slug}`);
	// The detail page renders both galleries and hides one per viewport, and
	// `display: none` keeps it out of the accessibility tree - so the cover's
	// alt text resolves to exactly one image in either project.
	await expect(
		page.getByRole('img', { name: `Cover photo of With photo ${token}` })
	).toHaveAttribute('src', `/images/${recipe.id}/${cover.id}/detail.jpg`);
});

test('opens the lightbox and walks through the images', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, { ...loadFixture(1), title: `Gallery ${token}` });
	await uploadImage(page, recipe.id, tinyPng([200, 120, 40]));
	await uploadImage(page, recipe.id, tinyPng([40, 120, 200]));

	await page.goto(`/recipes/${recipe.slug}`);
	// Only the gallery of the current viewport is displayed, so the name is
	// unique: mobile labels every slide, desktop only the cover, and both
	// open on the cover - here the first upload, which the server promoted.
	await page.getByRole('button', { name: 'Open photo 1 of 2' }).click();

	const dialog = page.getByRole('dialog');
	await expect(dialog).toBeVisible();
	await expect(dialog.getByText('1 / 2')).toBeVisible();

	await page.keyboard.press('ArrowRight');
	await expect(dialog.getByText('2 / 2')).toBeVisible();
	await page.keyboard.press('ArrowLeft');
	await expect(dialog.getByText('1 / 2')).toBeVisible();

	await page.keyboard.press('Escape');
	await expect(dialog).toBeHidden();
});

test('swipes between the lightbox images', async ({ page, isMobile }) => {
	test.skip(!isMobile, 'pointer swipe is exercised on the mobile project only');
	const token = uniqueToken();
	const recipe = await createRecipe(page, { ...loadFixture(1), title: `Swipe ${token}` });
	await uploadImage(page, recipe.id, tinyPng([200, 120, 40]));
	await uploadImage(page, recipe.id, tinyPng([40, 120, 200]));

	await page.goto(`/recipes/${recipe.slug}`);
	await page.getByRole('button', { name: 'Open photo 1 of 2' }).click();
	const dialog = page.getByRole('dialog');
	await expect(dialog.getByText('1 / 2')).toBeVisible();

	const area = dialog.getByRole('group').first();
	const box = await area.boundingBox();
	if (!box) {
		throw new Error('swipe area has no bounding box');
	}
	const y = box.y + box.height / 2;
	const drag = async (from: number, to: number) => {
		await page.mouse.move(box.x + from, y);
		await page.mouse.down();
		await page.mouse.move(box.x + to, y, { steps: 4 });
		await page.mouse.up();
	};

	// Exactly the 50px threshold, leftwards: the next image.
	await drag(250, 200);
	await expect(dialog.getByText('2 / 2')).toBeVisible();
	// 49px rightwards is below the threshold and must not page.
	await drag(200, 249);
	await expect(dialog.getByText('2 / 2')).toBeVisible();
	// 50px rightwards: back to the previous one.
	await drag(200, 250);
	await expect(dialog.getByText('1 / 2')).toBeVisible();
});

test('uploads, re-covers and removes images in the editor', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, { ...loadFixture(2), title: `Editor photos ${token}` });

	await page.goto(`/recipes/${recipe.slug}/edit`);
	await page.getByLabel('Choose photos').setInputFiles([
		{ name: 'one.png', mimeType: 'image/png', buffer: tinyPng([200, 120, 40]) },
		{ name: 'two.png', mimeType: 'image/png', buffer: tinyPng([40, 120, 200]) }
	]);

	// svelte-dnd-action puts `role="listitem"` on every tile; the label is the
	// section's own "Photo {number}", which the ingredient and step lists
	// ("Ingredient 1", "Step 1") cannot collide with.
	const tiles = page.getByRole('listitem', { name: /^Photo \d$/ });
	await expect(tiles).toHaveCount(2);
	await expect(page.getByText('Uploading…')).toHaveCount(0);
	// The first upload became the cover. `exact` matters: without it the badge
	// text also matches the "Make cover" button of every other tile.
	await expect(tiles.nth(0).getByText('Cover', { exact: true })).toBeVisible();

	await tiles.nth(1).hover();
	await tiles.nth(1).getByRole('button', { name: 'Make cover' }).click();
	await expect(tiles.nth(1).getByText('Cover', { exact: true })).toBeVisible();
	await expect(tiles.nth(0).getByText('Cover', { exact: true })).toHaveCount(0);

	await tiles.nth(0).hover();
	await tiles.nth(0).getByRole('button', { name: 'Remove photo' }).click();
	await expect(tiles).toHaveCount(1);
	await expect(tiles.nth(0).getByText('Cover', { exact: true })).toBeVisible();

	// Images are saved immediately, so the detail page already shows them.
	await page.goto(`/recipes/${recipe.slug}`);
	await expect(
		page.getByRole('img', { name: `Cover photo of Editor photos ${token}` })
	).toBeVisible();
});

test('queues images for a new recipe and uploads them on save', async ({ page }) => {
	const token = uniqueToken();
	const title = `New with photo ${token}`;

	await page.goto('/recipes/new');
	await page.getByRole('textbox', { name: 'Title', exact: true }).fill(title);
	await page.getByRole('textbox', { name: 'Ingredient', exact: true }).fill('Flour');
	await page
		.getByLabel('Choose photos')
		.setInputFiles([{ name: 'one.png', mimeType: 'image/png', buffer: tinyPng() }]);
	await expect(page.getByText('Photos upload when you save')).toBeVisible();
	await expect(page.getByRole('listitem', { name: 'Photo 1', exact: true })).toBeVisible();

	await page.getByRole('button', { name: 'Save', exact: true }).click();

	await expect(page).toHaveURL(/\/recipes\/new-with-photo-/);
	await expect(page.getByRole('img', { name: `Cover photo of ${title}` })).toBeVisible();
});

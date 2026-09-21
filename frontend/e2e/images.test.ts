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
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Mit Bild ${token}` });
	await uploadImage(page, recipe.id, tinyPng([200, 120, 40]));
	// The cover is the second picture, so neither surface may fall back to
	// "the first image in the list".
	const cover = await uploadImage(page, recipe.id, tinyPng([40, 120, 200]));
	await setCover(page, recipe.id, cover.id);

	await page.goto('/');
	await search(page, token);
	const card = page.getByRole('article', { name: `Mit Bild ${token}` });
	await expect(card.locator('img')).toHaveAttribute(
		'src',
		`/images/${recipe.id}/${cover.id}/thumb.jpg`
	);

	await page.goto(`/recipes/${recipe.slug}`);
	// The detail page renders both galleries and hides one per viewport, and
	// `display: none` keeps it out of the accessibility tree - so the cover's
	// alt text resolves to exactly one image in either project.
	await expect(page.getByRole('img', { name: `Titelbild von Mit Bild ${token}` })).toHaveAttribute(
		'src',
		`/images/${recipe.id}/${cover.id}/detail.jpg`
	);
});

test('opens the lightbox and walks through the images', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, { ...loadFixture(1), title: `Galerie ${token}` });
	await uploadImage(page, recipe.id, tinyPng([200, 120, 40]));
	await uploadImage(page, recipe.id, tinyPng([40, 120, 200]));

	await page.goto(`/recipes/${recipe.slug}`);
	// Only the gallery of the current viewport is displayed, so the name is
	// unique: mobile labels every slide, desktop only the cover, and both
	// open on the cover - here the first upload, which the server promoted.
	await page.getByRole('button', { name: 'Bild 1 von 2 öffnen' }).click();

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

test('uploads, re-covers and removes images in the editor', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, { ...loadFixture(2), title: `Editorbilder ${token}` });

	await page.goto(`/recipes/${recipe.slug}/edit`);
	await page.getByLabel('Bilder auswählen').setInputFiles([
		{ name: 'eins.png', mimeType: 'image/png', buffer: tinyPng([200, 120, 40]) },
		{ name: 'zwei.png', mimeType: 'image/png', buffer: tinyPng([40, 120, 200]) }
	]);

	// svelte-dnd-action puts `role="listitem"` on every tile; the label is the
	// section's own "Bild {number}", which the ingredient and step lists
	// ("Zutat 1", "Schritt 1") cannot collide with.
	const tiles = page.getByRole('listitem', { name: /^Bild \d$/ });
	await expect(tiles).toHaveCount(2);
	await expect(page.getByText('Lädt …')).toHaveCount(0);
	// The first upload became the cover. `exact` matters: without it the badge
	// text also matches the "Als Cover" button of every other tile.
	await expect(tiles.nth(0).getByText('Cover', { exact: true })).toBeVisible();

	await tiles.nth(1).hover();
	await tiles.nth(1).getByRole('button', { name: 'Als Cover' }).click();
	await expect(tiles.nth(1).getByText('Cover', { exact: true })).toBeVisible();
	await expect(tiles.nth(0).getByText('Cover', { exact: true })).toHaveCount(0);

	await tiles.nth(0).hover();
	await tiles.nth(0).getByRole('button', { name: 'Bild entfernen' }).click();
	await expect(tiles).toHaveCount(1);
	await expect(tiles.nth(0).getByText('Cover', { exact: true })).toBeVisible();

	// Images are saved immediately, so the detail page already shows them.
	await page.goto(`/recipes/${recipe.slug}`);
	await expect(
		page.getByRole('img', { name: `Titelbild von Editorbilder ${token}` })
	).toBeVisible();
});

test('queues images for a new recipe and uploads them on save', async ({ page }) => {
	const token = uniqueToken();
	const title = `Neu mit Bild ${token}`;

	await page.goto('/recipes/new');
	await page.getByRole('textbox', { name: 'Titel', exact: true }).fill(title);
	await page.getByRole('textbox', { name: 'Zutat', exact: true }).fill('Mehl');
	await page
		.getByLabel('Bilder auswählen')
		.setInputFiles([{ name: 'eins.png', mimeType: 'image/png', buffer: tinyPng() }]);
	await expect(page.getByText('Bilder werden beim Speichern hochgeladen')).toBeVisible();
	await expect(page.getByRole('listitem', { name: 'Bild 1', exact: true })).toBeVisible();

	await page.getByRole('button', { name: 'Speichern', exact: true }).click();

	await expect(page).toHaveURL(/\/recipes\/neu-mit-bild-/);
	await expect(page.getByRole('img', { name: `Titelbild von ${title}` })).toBeVisible();
});

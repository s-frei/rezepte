import { expect, test, type Page } from '@playwright/test';
import { createRecipe, loadFixture, login, tinyPng, uniqueToken, uploadImage } from './helpers';
import type { Recipe } from '../src/lib/api/recipes';

// The row slides for half a second; a click mid-glide would land on the photo
// still moving in. Reduced motion places it at once.
test.use({ reducedMotion: 'reduce' });

test.beforeEach(async ({ page }) => {
	await login(page);
});

const tiles = (page: Page) => page.locator('#editor-section-images li[data-sortable-key]');
const stage = (page: Page) => page.getByRole('application');
/** The live region that announces where the point is. */
const position = (page: Page) => page.locator('[data-focus-position]');
const saveButton = (page: Page) =>
	page.getByRole('dialog').getByRole('button', { name: 'Save', exact: true });
const isFocusSave = (url: string, method: string) =>
	method === 'PATCH' && /\/api\/v1\/recipes\/[^/]+\/images\/[^/]+$/.test(url);

/** A portrait photo: every crop of it cuts above and below. */
const portrait = (tint: [number, number, number] = [200, 120, 40]) => tinyPng(tint, 96, 128);

/** The vertical part of an `<img>`'s `object-position`, in percent. */
async function objectY(page: Page, index: number): Promise<number> {
	const position = await tiles(page)
		.nth(index)
		.locator('img')
		.evaluate((img) => getComputedStyle(img).objectPosition);
	return parseFloat(position.split(' ')[1]);
}

async function recipeBySlug(page: Page, slug: string): Promise<Recipe> {
	const response = await page.request.get(`/api/v1/recipes/by-slug/${slug}`);
	expect(response.ok()).toBeTruthy();
	return (await response.json()) as Recipe;
}

async function openFocus(page: Page, number: number) {
	await page.getByRole('button', { name: `Photo ${number} options` }).click();
	await page.getByRole('menuitem', { name: 'Photo focus' }).click();
	await expect(stage(page)).toBeVisible();
}

/** Clicks the stage at a fraction of its height, in the middle across. */
async function clickStage(page: Page, y: number) {
	const box = (await stage(page).boundingBox())!;
	await page.mouse.click(box.x + box.width / 2, box.y + box.height * y);
}

async function editorWith(page: Page, count: number) {
	const recipe = await createRecipe(page, {
		...loadFixture(2),
		title: `Focus ${uniqueToken()}`
	});
	const ids: string[] = [];
	for (let i = 0; i < count; i++) {
		ids.push((await uploadImage(page, recipe.id, portrait([60 * i, 120, 200]))).id);
	}
	await page.goto(`/recipes/${recipe.slug}/edit`);
	await expect(tiles(page)).toHaveCount(count);
	return { recipe, ids };
}

test('the item sits between View and Make cover', async ({ page }) => {
	await editorWith(page, 1);
	await page.getByRole('button', { name: 'Photo 1 options' }).click();
	await expect(page.getByRole('menuitem')).toHaveText([
		'View',
		'Photo focus',
		'Make cover',
		'Move earlier',
		'Move later',
		'Remove'
	]);
});

test('sets a photo’s focus from the editor', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name !== 'desktop', 'the click on the stage is the same on a phone');
	const { recipe } = await editorWith(page, 1);
	expect(await objectY(page, 0)).toBe(50);

	await openFocus(page, 1);
	await clickStage(page, 0.85);
	const saved = page.waitForResponse((r) => isFocusSave(r.url(), r.request().method()));
	await saveButton(page).click();
	expect((await saved).ok()).toBeTruthy();
	await expect(stage(page)).toBeHidden();
	// The square thumb follows the point at once.
	await expect.poll(() => objectY(page, 0)).toBeGreaterThan(50);

	await page.reload();
	await expect(tiles(page)).toHaveCount(1);
	await expect.poll(() => objectY(page, 0)).toBeGreaterThan(50);
	const [image] = (await recipeBySlug(page, recipe.slug)).images;
	expect(image.focus.y).toBeGreaterThan(0.5);
	expect(image.focus.x).toBe(0.5);
});

test('an unchanged point sends nothing', async ({ page }) => {
	await editorWith(page, 1);
	let sent = 0;
	page.on('request', (r) => {
		if (isFocusSave(r.url(), r.method())) sent++;
	});
	await openFocus(page, 1);
	await saveButton(page).click();
	await expect(stage(page)).toBeHidden();
	expect(sent).toBe(0);
});

test('a refused focus keeps the dialog open', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name !== 'desktop', 'the error path is the same on a phone');
	await editorWith(page, 1);
	await page.route(/\/api\/v1\/recipes\/[^/]+\/images\/[^/]+$/, (route) =>
		route.request().method() === 'PATCH' ? route.fulfill({ status: 500 }) : route.fallback()
	);
	await openFocus(page, 1);
	await clickStage(page, 0.85);
	await saveButton(page).click();
	await expect(page.getByText('The photo focus could not be saved.')).toBeVisible();
	await expect(stage(page)).toBeVisible();
	await expect(position(page)).not.toHaveText(/50% from the top/);
});

test('keeps the focus of a photo on a new recipe', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name !== 'desktop', 'the click on the stage is the same on a phone');
	const title = `Queued focus ${uniqueToken()}`;
	await page.goto('/recipes/new');
	await page.getByRole('textbox', { name: 'Title', exact: true }).fill(title);
	await page.getByRole('textbox', { name: 'Ingredient', exact: true }).fill('Flour');
	await page
		.getByLabel('Choose photos')
		.setInputFiles([{ name: 'tall.png', mimeType: 'image/png', buffer: portrait() }]);
	await expect(tiles(page)).toHaveCount(1);
	await openFocus(page, 1);
	await clickStage(page, 0.85);
	await saveButton(page).click();
	await expect(stage(page)).toBeHidden();
	await expect.poll(() => objectY(page, 0)).toBeGreaterThan(50);

	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page).toHaveURL(/\/recipes\/queued-focus-/);
	const slug = new URL(page.url()).pathname.split('/')[2];
	await expect
		.poll(async () => (await recipeBySlug(page, slug)).images[0]?.focus.y ?? 0)
		.toBeGreaterThan(0.5);
});

test('a point set while the photo uploads is sent once it lands', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name !== 'desktop', 'the click on the stage is the same on a phone');
	const { recipe } = await editorWith(page, 0);
	let release = () => {};
	const held = new Promise<void>((resolve) => (release = resolve));
	await page.route(`**/api/v1/recipes/${recipe.id}/images`, async (route) => {
		await held;
		await route.continue();
	});
	await page
		.getByLabel('Choose photos')
		.setInputFiles([{ name: 'tall.png', mimeType: 'image/png', buffer: portrait() }]);
	await expect(tiles(page).locator('img[src^="blob:"]')).toHaveCount(1);
	await openFocus(page, 1);
	await clickStage(page, 0.85);
	await saveButton(page).click();
	await expect(stage(page)).toBeHidden();

	const saved = page.waitForResponse((r) => isFocusSave(r.url(), r.request().method()));
	release();
	expect((await saved).ok()).toBeTruthy();
	await expect(tiles(page).locator('img[src^="/images/"]')).toHaveCount(1);
	await expect.poll(() => objectY(page, 0)).toBeGreaterThan(50);
	expect((await recipeBySlug(page, recipe.slug)).images[0].focus.y).toBeGreaterThan(0.5);
});

test('walks through every photo and saves the changed ones at once', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name !== 'desktop', 'the dots are the same on a phone');
	const { recipe, ids } = await editorWith(page, 3);
	const sent: string[] = [];
	page.on('request', (r) => {
		if (isFocusSave(r.url(), r.method())) sent.push(r.url().split('/').pop()!);
	});
	await openFocus(page, 1);
	const strip = page.getByRole('dialog').getByRole('group', { name: 'Photo thumbnails' });
	await expect(strip.getByRole('button')).toHaveCount(3);
	await expect(strip.getByRole('button', { name: 'Show photo 1' })).toHaveAttribute(
		'aria-current',
		'true'
	);

	await clickStage(page, 0.85);
	await page.getByRole('button', { name: 'Next photo' }).click();
	await expect(strip.getByRole('button', { name: 'Show photo 2' })).toHaveAttribute(
		'aria-current',
		'true'
	);
	// Each photo shows its own point: the second one is still centered.
	await expect(position(page)).toHaveText('50% from the left, 50% from the top');
	await strip.getByRole('button', { name: 'Show photo 3' }).click();
	await clickStage(page, 0.15);
	await expect(strip.getByRole('button', { name: 'Photo 1, focus changed' })).toBeVisible();
	await expect(strip.getByRole('button', { name: 'Photo 3, focus changed' })).toBeVisible();

	await page.getByRole('dialog').getByRole('button', { name: 'Save 2 photos' }).click();
	await expect(stage(page)).toBeHidden();
	expect(sent.sort()).toEqual([ids[0], ids[2]].sort());
	const { images } = await recipeBySlug(page, recipe.slug);
	expect(images[0].focus.y).toBeGreaterThan(0.5);
	expect(images[1].focus).toEqual({ x: 0.5, y: 0.5 });
	expect(images[2].focus.y).toBeLessThan(0.5);
});

test('opens on the cover from the header and walks on with Enter', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name !== 'desktop', 'the facts sit in the foot on a desktop');
	await editorWith(page, 2);
	await page.getByRole('button', { name: 'Photo focus', exact: true }).click();
	await expect(stage(page)).toBeFocused();
	const dialog = page.getByRole('dialog');
	const count = dialog.locator('[data-focus-count]');
	const facts = dialog.locator('[data-focus-facts]');
	await expect(count).toHaveText('1 of 2');
	await expect(facts).toContainText('Portrait 3:4');
	await expect(facts).toContainText('Cover');
	await expect(facts).toContainText('96 × 128 px');
	await expect(facts).toContainText(/MB · added \w+ \d+, \d{4}/);

	await page.keyboard.press('Enter');
	await expect(count).toHaveText('2 of 2');
	await expect(facts).not.toContainText('Cover');
	// At the end the arrow stays where it is and keeps the focus: clicking on
	// never loses its place.
	const next = dialog.getByRole('button', { name: 'Next photo' });
	await next.focus();
	await page.keyboard.press('Enter');
	await expect(next).toBeFocused();
	await expect(count).toHaveText('2 of 2');
	await dialog.getByRole('button', { name: 'Previous photo' }).click();
	await expect(count).toHaveText('1 of 2');
});

test('a sideways drag on a portrait turns the row and keeps its point', async ({
	page
}, testInfo) => {
	test.skip(testInfo.project.name !== 'desktop', 'the mouse stands in for the finger');
	await editorWith(page, 2);
	await openFocus(page, 1);
	const box = (await stage(page).boundingBox())!;
	const [x, y] = [box.x + box.width / 2, box.y + box.height * 0.4];
	await page.mouse.move(x, y);
	await page.mouse.down();
	await page.mouse.move(x - 220, y + 6, { steps: 10 });
	await page.mouse.up();
	await expect(page.getByRole('dialog').locator('[data-focus-count]')).toHaveText('2 of 2');
	// The press landed on the first photo, but a swipe sets no point there.
	await expect(
		page.getByRole('dialog').getByRole('button', { name: 'Show photo 1' })
	).toBeVisible();
	await expect(saveButton(page)).toBeVisible();
});

test('focus stays with its photo after reorder', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name !== 'desktop', 'the click on the stage is the same on a phone');
	const { recipe, ids } = await editorWith(page, 2);
	await openFocus(page, 2);
	await clickStage(page, 0.85);
	const saved = page.waitForResponse((r) => isFocusSave(r.url(), r.request().method()));
	await saveButton(page).click();
	await saved;
	await expect(stage(page)).toBeHidden();

	await page.getByRole('button', { name: 'Photo 2 options' }).click();
	await page.getByRole('menuitem', { name: 'Make cover' }).click();
	await expect.poll(() => objectY(page, 0)).toBeGreaterThan(50);
	expect(await objectY(page, 1)).toBe(50);

	await page.reload();
	await expect(tiles(page)).toHaveCount(2);
	await expect.poll(() => objectY(page, 0)).toBeGreaterThan(50);
	const { images } = await recipeBySlug(page, recipe.slug);
	expect(images.map((image) => image.id)).toEqual([ids[1], ids[0]]);
	expect(images[0].focus.y).toBeGreaterThan(0.5);
	expect(images[1].focus).toEqual({ x: 0.5, y: 0.5 });
});

test('moves the focus with the arrow keys and centers it again', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name !== 'desktop', 'a keyboard is the desktop’s');
	await editorWith(page, 1);
	let sent = 0;
	page.on('request', (r) => {
		if (isFocusSave(r.url(), r.method())) sent++;
	});
	// Opened by keyboard, as the menu's focus return must not take the
	// focus back behind the dialog. A Bits menu focuses its first item twice
	// (a tick, then a frame), so the pick waits for both.
	await page.getByRole('button', { name: 'Photo 1 options' }).focus();
	await page.keyboard.press('Enter');
	await expect
		.poll(() => page.evaluate(() => document.activeElement?.getAttribute('role')))
		.toBe('menuitem');
	await page.evaluate(() => new Promise((done) => requestAnimationFrame(done)));
	await page.getByRole('menuitem', { name: 'Photo focus' }).focus();
	await page.keyboard.press('Enter');
	// The photo has the focus on opening, with its ring: the arrows work at once.
	await expect(stage(page)).toBeFocused();
	await expect(stage(page)).toHaveCSS('outline-style', 'solid');
	for (let i = 0; i < 3; i++) await page.keyboard.press('ArrowDown');
	await page.keyboard.press('Shift+ArrowDown');
	await expect(position(page)).toHaveText('50% from the left, 66% from the top');

	await page.keyboard.press('Tab');
	await expect(page.getByRole('button', { name: 'Center' })).toBeFocused();
	await page.keyboard.press('Enter');
	await expect(stage(page)).toBeFocused();
	await expect(position(page)).toHaveText('50% from the left, 50% from the top');

	await page.keyboard.press('ArrowDown');
	await page.keyboard.press('Escape');
	await expect(stage(page)).toBeHidden();
	expect(sent).toBe(0);
});

for (const width of [360, 320]) {
	test(`the focus sheet fits a ${width}px phone`, async ({ page, isMobile }) => {
		test.skip(!isMobile, 'the sheet is the phone layout');
		await page.setViewportSize({ width, height: width === 320 ? 568 : 740 });
		// Three photos: the dots under the photo and the info line must fit too.
		await editorWith(page, 3);
		await openFocus(page, 1);
		const sheet = page.getByRole('dialog');
		await expect(sheet).toBeVisible();
		// Opened from a tap, the sheet holds the focus, and no ring shows on its handle.
		await expect
			.poll(() => page.evaluate(() => document.activeElement?.matches(':focus-visible') ?? false))
			.toBe(false);
		await expect(saveButton(page)).toBeInViewport({
			ratio: 1
		});
		const overflow = await page.evaluate(
			() => document.documentElement.scrollWidth - document.documentElement.clientWidth
		);
		expect(overflow).toBeLessThanOrEqual(0);
	});
}

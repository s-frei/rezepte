import { expect, test, type Locator, type Page, type Request } from '@playwright/test';
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
	// A swipe selects nothing: a press inside a selection drags the selection
	// instead (Chromium on Linux and Windows at once), and the pointercancel
	// that starts that drag drops the swipe.
	expect(await page.evaluate(() => getSelection()?.type)).not.toBe('Range');
	// 49px rightwards is below the threshold and must not page.
	await drag(200, 249);
	await expect(dialog.getByText('2 / 2')).toBeVisible();
	// 50px rightwards: back to the previous one.
	await drag(200, 250);
	await expect(dialog.getByText('1 / 2')).toBeVisible();
});

test('keeps the first lightbox thumbnail reachable when the strip overflows', async ({
	page,
	isMobile
}) => {
	test.skip(!isMobile, 'only a phone is narrow enough for the strip to overflow');
	const token = uniqueToken();
	const recipe = await createRecipe(page, { ...loadFixture(1), title: `Strip ${token}` });
	for (let i = 0; i < 10; i++) {
		await uploadImage(page, recipe.id, tinyPng([20 * i, 120, 40]));
	}

	await page.goto(`/recipes/${recipe.slug}`);
	await page.getByRole('button', { name: 'Open photo 1 of 10' }).click();
	const strip = page.getByRole('dialog').getByRole('group', { name: 'Photo thumbnails' });
	// A centered row that overflows spills past its left edge, where no
	// scrolling reaches; the first thumbnail must start inside the strip.
	const stripBox = await strip.boundingBox();
	const firstBox = await strip.getByRole('button', { name: 'Photo 1', exact: true }).boundingBox();
	expect(firstBox!.x).toBeGreaterThanOrEqual(stripBox!.x);
});

// The editor's photo grid. A tile's identity is the image id in its
// thumbnail's address; its `aria-label` only says where it stands.
const tiles = (page: Page) => page.locator('#editor-section-images li[data-sortable-key]');

function order(page: Page): Promise<string[]> {
	return tiles(page)
		.locator('img')
		.evaluateAll((imgs) => imgs.map((img) => img.getAttribute('src')!.split('/')[3]));
}

const isOrderSave = (request: Request) =>
	request.method() === 'PUT' && request.url().endsWith('/images/order');

async function editorWithPhotos(page: Page, count: number) {
	const token = uniqueToken();
	const recipe = await createRecipe(page, { ...loadFixture(2), title: `Photo grid ${token}` });
	const ids: string[] = [];
	for (let i = 0; i < count; i++) {
		ids.push((await uploadImage(page, recipe.id, tinyPng([60 * i, 120, 200 - 40 * i]))).id);
	}
	await page.goto(`/recipes/${recipe.slug}/edit`);
	await expect(tiles(page)).toHaveCount(count);
	await tiles(page).first().scrollIntoViewIfNeeded();
	return { recipe, ids, title: `Photo grid ${token}` };
}

async function center(locator: Locator) {
	const box = (await locator.boundingBox())!;
	return { x: box.x + box.width / 2, y: box.y + box.height / 2 };
}

/**
 * Drags `from` onto `to` the way the project's user would: a mouse moves
 * after pressing, a finger holds 350ms first (the sortable lifts at 250ms).
 * Touch goes through CDP, since Playwright's touchscreen only taps.
 */
async function drag(page: Page, isMobile: boolean, from: Locator, to: Locator) {
	const a = await center(from);
	const b = await center(to);
	const steps = 12;
	const at = (i: number) => ({
		x: a.x + ((b.x - a.x) * i) / steps,
		y: a.y + ((b.y - a.y) * i) / steps
	});
	if (!isMobile) {
		await page.mouse.move(a.x, a.y);
		await page.mouse.down();
		for (let i = 1; i <= steps; i++) await page.mouse.move(at(i).x, at(i).y);
		await page.mouse.up();
		return;
	}
	const cdp = await page.context().newCDPSession(page);
	await cdp.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [a] });
	await page.waitForTimeout(350);
	for (let i = 1; i <= steps; i++) {
		await cdp.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: [at(i)] });
	}
	await cdp.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
	await cdp.detach();
}

test('reorders photos by dragging and keeps the order after reload', async ({ page, isMobile }) => {
	const { recipe, ids, title } = await editorWithPhotos(page, 4);
	const [one, two, three, four] = ids;

	const saved = page.waitForResponse((response) => isOrderSave(response.request()));
	await drag(page, isMobile, tiles(page).nth(3), tiles(page).nth(0));
	expect((await saved).ok()).toBeTruthy();

	await expect(tiles(page)).toHaveCount(4);
	expect(await order(page)).toEqual([four, one, two, three]);
	await expect(tiles(page).first().getByText('Cover', { exact: true })).toBeVisible();

	await page.reload();
	await expect(tiles(page)).toHaveCount(4);
	expect(await order(page)).toEqual([four, one, two, three]);

	await page.goto(`/recipes/${recipe.slug}`);
	await expect(page.getByRole('img', { name: `Cover photo of ${title}` })).toHaveAttribute(
		'src',
		new RegExp(`/${four}/`)
	);
});

test('makes a photo the cover from its menu', async ({ page }) => {
	const { ids } = await editorWithPhotos(page, 3);
	const [one, two, three] = ids;

	await page.getByRole('button', { name: 'Photo 3 options' }).click();
	const request = page.waitForRequest(isOrderSave);
	await page.getByRole('menuitem', { name: 'Make cover' }).click();
	expect((await request).postDataJSON()).toEqual({ imageIds: [three, one, two] });
	expect(await order(page)).toEqual([three, one, two]);
	await expect(tiles(page).first().getByText('Cover', { exact: true })).toBeVisible();

	// Removing the cover hands the badge to the next photo.
	await page.getByRole('button', { name: 'Photo 1 options' }).click();
	await page.getByRole('menuitem', { name: 'Remove' }).click();
	await expect(tiles(page)).toHaveCount(2);
	expect(await order(page)).toEqual([one, two]);
	await expect(tiles(page).first().getByText('Cover', { exact: true })).toBeVisible();
});

test('moves a photo earlier and later from its menu', async ({ page }) => {
	const { ids } = await editorWithPhotos(page, 3);
	const [one, two, three] = ids;

	// Keyboard only: focus the menu button, open it, arrow to the item.
	async function choose(trigger: string, item: string) {
		await page.getByRole('button', { name: trigger }).focus();
		await page.keyboard.press('Enter');
		const target = page.getByRole('menuitem', { name: item });
		await expect(page.getByRole('menu')).toBeVisible();
		for (let i = 0; i < 6 && !(await target.evaluate((el) => el === document.activeElement)); i++) {
			await page.keyboard.press('ArrowDown');
		}
		await expect(target).toBeFocused();
		const saved = page.waitForResponse((response) => isOrderSave(response.request()));
		await page.keyboard.press('Enter');
		expect((await saved).ok()).toBeTruthy();
	}

	await choose('Photo 2 options', 'Move earlier');
	expect(await order(page)).toEqual([two, one, three]);
	await expect(page.getByText('Photo moved to position 1')).toBeAttached();

	await choose('Photo 2 options', 'Move later');
	expect(await order(page)).toEqual([two, three, one]);
	await expect(page.getByText('Photo moved to position 3')).toBeAttached();
});

test('a vertical swipe over the photos scrolls instead of reordering', async ({
	page,
	isMobile
}) => {
	test.skip(!isMobile, 'a touch swipe needs the phone');
	const { ids } = await editorWithPhotos(page, 6);
	let saves = 0;
	page.on('request', (request) => {
		if (isOrderSave(request)) saves++;
	});

	const before = await page.evaluate(() => window.scrollY);
	const start = await center(tiles(page).nth(4));
	const cdp = await page.context().newCDPSession(page);
	await cdp.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [start] });
	for (let i = 1; i <= 10; i++) {
		await cdp.send('Input.dispatchTouchEvent', {
			type: 'touchMove',
			touchPoints: [{ x: start.x, y: start.y - 25 * i }]
		});
	}
	await cdp.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
	await cdp.detach();
	await page.waitForTimeout(500);

	expect(await page.evaluate(() => window.scrollY)).toBeGreaterThan(before);
	expect(await order(page)).toEqual(ids);
	expect(saves).toBe(0);
});

test('keeps a photo that finished uploading during a reorder', async ({ page, isMobile }) => {
	const { ids } = await editorWithPhotos(page, 3);
	const [one, two, three] = ids;
	// The order save lingers, so the upload lands on the server first unless
	// the section waits for it.
	await page.route('**/images/order', async (route) => {
		await new Promise((resolve) => setTimeout(resolve, 500));
		await route.continue();
	});

	const saved = page.waitForResponse((response) => isOrderSave(response.request()));
	await drag(page, isMobile, tiles(page).nth(2), tiles(page).nth(1));
	await page
		.getByLabel('Choose photos')
		.setInputFiles([{ name: 'late.png', mimeType: 'image/png', buffer: tinyPng([10, 200, 90]) }]);
	expect((await saved).ok()).toBeTruthy();

	await expect(tiles(page)).toHaveCount(4);
	await expect(tiles(page).locator('img[src^="blob:"]')).toHaveCount(0);
	const shown = await order(page);
	expect(shown.slice(0, 3)).toEqual([one, three, two]);

	await page.reload();
	await expect(tiles(page)).toHaveCount(4);
	expect(await order(page)).toEqual(shown);
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

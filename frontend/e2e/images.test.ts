import { expect, test, type Locator, type Page, type Request } from '@playwright/test';
import {
	createRecipe,
	loadFixture,
	login,
	longPress,
	search,
	setCover,
	tinyPng,
	uniqueToken,
	uploadImage
} from './helpers';
import type { Recipe } from '../src/lib/api/recipes';

test.beforeEach(async ({ page }) => {
	await login(page);
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

test('swipes between the lightbox images', async ({ page, isMobile, browserName }) => {
	test.skip(!isMobile, 'swipe is exercised on the mobile projects only');
	// Real touch through CDP: a mouse pointer ignores `touch-action`.
	test.skip(browserName !== 'chromium', 'touch is dispatched through CDP, Chromium only');
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
	const cdp = await page.context().newCDPSession(page);
	const drag = async (from: number, to: number) => {
		await cdp.send('Input.dispatchTouchEvent', {
			type: 'touchStart',
			touchPoints: [{ x: box.x + from, y }]
		});
		for (let i = 1; i <= 4; i++) {
			await cdp.send('Input.dispatchTouchEvent', {
				type: 'touchMove',
				touchPoints: [{ x: box.x + from + ((to - from) * i) / 4, y }]
			});
		}
		await cdp.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
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
	await page.locator('#editor-section-images').scrollIntoViewIfNeeded();
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
	// Measured while the tiles still slide (after a removal), the press lands on a neighbor.
	await page.waitForFunction(() =>
		document.getAnimations().every((animation) => animation.playState !== 'running')
	);
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

// A phone lowered the page's scroll at its end when the lifted cover hung
// past the right edge for even one layout (Chrome widens its layout viewport).
// A window just short of the editor's height keeps the grid on screen there.
test('dragging the cover at the end of the page keeps the page still', async ({
	page,
	isMobile
}) => {
	const fixture = loadFixture(2);
	const recipe = await createRecipe(page, {
		...fixture,
		title: `Page end ${uniqueToken()}`
	});
	const ids: string[] = [];
	for (const tint of [
		[200, 90, 90],
		[90, 200, 90],
		[90, 90, 200]
	] as const)
		ids.push((await uploadImage(page, recipe.id, tinyPng([...tint]))).id);
	await page.goto(`/recipes/${recipe.slug}/edit`);
	await expect(tiles(page)).toHaveCount(3);
	const height = await page.evaluate(() => document.documentElement.scrollHeight);
	await page.setViewportSize({ width: page.viewportSize()!.width, height: height - 120 });
	await page.evaluate(() => scrollTo(0, document.documentElement.scrollHeight));
	await page.waitForFunction(
		() => scrollY > 0 && scrollY + innerHeight >= document.documentElement.scrollHeight - 1
	);
	await expect(tiles(page).nth(1)).toBeInViewport();
	await page.evaluate(() => {
		const rec = { scrolls: [] as number[], on: true };
		(window as unknown as { rec: typeof rec }).rec = rec;
		const frame = () => {
			rec.scrolls.push(scrollY);
			if (rec.on) requestAnimationFrame(frame);
		};
		requestAnimationFrame(frame);
	});

	const saved = page.waitForResponse((response) => isOrderSave(response.request()));
	await drag(page, isMobile, tiles(page).nth(0), tiles(page).nth(1));
	await saved;

	const scrolls = await page.evaluate(() => {
		const rec = (window as unknown as { rec: { on: boolean; scrolls: number[] } }).rec;
		rec.on = false;
		return rec.scrolls;
	});
	expect(Math.max(...scrolls) - Math.min(...scrolls)).toBe(0);
	expect(await order(page)).toEqual([ids[1], ids[0], ids[2]]);
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

test('moves a photo by Alt+Arrow and stays put at either end', async ({ page }) => {
	const { recipe, ids } = await editorWithPhotos(page, 3);
	const [one, two, three] = ids;
	let saves = 0;
	page.on('request', (request) => {
		if (isOrderSave(request)) saves++;
	});
	// Alt+Left is the browser's Back: the window sees whether it was kept from it.
	await page.evaluate(() =>
		addEventListener('keydown', (e) =>
			document.body.setAttribute('data-kept', `${e.defaultPrevented}`)
		)
	);

	const saved = page.waitForResponse((response) => isOrderSave(response.request()));
	await page.getByRole('button', { name: 'Show photo 2' }).focus();
	await page.keyboard.press('Alt+ArrowRight');
	expect((await saved).ok()).toBeTruthy();
	expect(await order(page)).toEqual([one, three, two]);
	await expect(page.getByText('Photo moved to position 3', { exact: true })).toBeAttached();
	await expect(page.getByRole('button', { name: 'Show photo 3' })).toBeFocused();

	// The cover is already first: nothing moves, nothing is saved, the page stays.
	await page.getByRole('button', { name: 'Show photo 1' }).focus();
	await page.keyboard.press('Alt+ArrowLeft');
	await expect(page.locator('body')).toHaveAttribute('data-kept', 'true');
	await page.waitForTimeout(300);
	expect(await order(page)).toEqual([one, three, two]);
	expect(saves).toBe(1);
	await expect(page).toHaveURL(`/recipes/${recipe.slug}/edit`);
});

test('shows what the server holds when another member changed the photos', async ({
	page,
	isMobile
}) => {
	const { recipe, ids } = await editorWithPhotos(page, 3);
	const [one, two, three] = ids;
	// Someone else adds a photo meanwhile: the order sent misses it.
	const added = (await uploadImage(page, recipe.id, tinyPng([10, 200, 90]))).id;

	const refused = page.waitForResponse((response) => isOrderSave(response.request()));
	await drag(page, isMobile, tiles(page).nth(2), tiles(page).nth(0));
	expect((await refused).status()).toBe(422);

	await expect(page.getByText('The new order could not be saved.')).toBeVisible();
	await expect(tiles(page)).toHaveCount(4);
	await expect.poll(() => order(page)).toEqual([one, two, three, added]);
});

test('leaving the editor waits for the photos still going up', async ({ page }) => {
	const { recipe } = await editorWithPhotos(page, 1);
	// Each upload waits for its own release; the second starts only after the first.
	const releases: (() => void)[] = [];
	await page.route('**/api/v1/recipes/*/images', async (route) => {
		if (route.request().method() === 'POST') {
			await new Promise<void>((resolve) => releases.push(resolve));
		}
		return route.continue();
	});

	await page.getByLabel('Choose photos').setInputFiles([
		{ name: 'one.png', mimeType: 'image/png', buffer: tinyPng([10, 200, 90]) },
		{ name: 'two.png', mimeType: 'image/png', buffer: tinyPng([200, 10, 90]) }
	]);
	const cancel = page.getByRole('button', { name: 'Cancel', exact: true });
	await cancel.click();
	await expect(cancel).toBeDisabled();
	await expect.poll(() => releases.length).toBe(1);
	releases[0]();

	// The second photo is still on its way: the editor stays.
	await expect.poll(() => releases.length).toBe(2);
	await page.waitForTimeout(300);
	await expect(page).toHaveURL(`/recipes/${recipe.slug}/edit`);

	releases[1]();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	const stored = (await (await page.request.get(`/api/v1/recipes/${recipe.id}`)).json()) as Recipe;
	expect(stored.images).toHaveLength(3);
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

	// The swipe scrolled the page; one settle for a reorder that must not come.
	await expect.poll(() => page.evaluate(() => window.scrollY)).toBeGreaterThan(before);
	await page.waitForTimeout(300);
	expect(await order(page)).toEqual(ids);
	expect(saves).toBe(0);
});

// A held photo scrolls the page near the window's edge, so it reaches a
// place below the screen: the finger or mouse stays still while it scrolls.
test('a photo held at the window’s bottom edge scrolls the page and lands further down', async ({
	page,
	isMobile
}) => {
	await page.setViewportSize({ width: page.viewportSize()!.width, height: 300 });
	const { ids } = await editorWithPhotos(page, 20);
	await tiles(page)
		.nth(1)
		.evaluate((el) => el.scrollIntoView({ block: 'start' }));
	await page.evaluate(() => scrollBy(0, -80));
	expect(
		await tiles(page)
			.last()
			.evaluate((el) => el.getBoundingClientRect().top - innerHeight)
	).toBeGreaterThan(100);
	const before = await page.evaluate(() => scrollY);
	const a = await center(tiles(page).nth(1));
	const edge = { x: a.x, y: 300 - 12 };

	const saved = page.waitForResponse((response) => isOrderSave(response.request()));
	const cdp = isMobile ? await page.context().newCDPSession(page) : null;
	const to = async (point: { x: number; y: number }) =>
		cdp
			? cdp.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: [point] })
			: page.mouse.move(point.x, point.y);
	if (cdp) {
		await cdp.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [a] });
		await page.waitForTimeout(350);
	} else {
		await page.mouse.move(a.x, a.y);
		await page.mouse.down();
	}
	for (let i = 1; i <= 8; i++) await to({ x: a.x, y: a.y + ((edge.y - a.y) * i) / 8 });
	await page.waitForFunction(
		() =>
			scrollY + innerHeight >= document.documentElement.scrollHeight - 1 ||
			document.querySelector('#editor-section-images li:last-of-type')!.getBoundingClientRect()
				.bottom < innerHeight
	);
	await page.waitForTimeout(150);
	// Let go over the slot's middle, not its top edge: in the 300px window the
	// slot's top sits within the 64px at the top that scroll the page up, so a
	// release a few frames late - a busy runner's - had already scrolled it
	// back a place.
	await to(await center(page.locator('#editor-section-images li[data-lifted]')));
	if (cdp) {
		await cdp.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
		await cdp.detach();
	} else await page.mouse.up();
	await saved;

	expect(await page.evaluate(() => scrollY)).toBeGreaterThan(before + 100);
	expect((await order(page)).indexOf(ids[1])).toBeGreaterThan(12);
});

test('Alt+Arrow while dragging a photo stays away from the browser', async ({ page, isMobile }) => {
	test.skip(isMobile, 'a key press during a drag needs a keyboard');
	const { ids } = await editorWithPhotos(page, 3);
	await page.evaluate(() => {
		const w = window as unknown as { keys: boolean[] };
		w.keys = [];
		addEventListener('keydown', (e) => {
			if (e.key.startsWith('Arrow')) w.keys.push(e.defaultPrevented);
		});
	});
	const a = await center(tiles(page).nth(1));
	await page.mouse.move(a.x, a.y);
	await page.mouse.down();
	await page.mouse.move(a.x + 10, a.y + 10);
	await expect(page.locator('#editor-section-images li[data-lifted]')).toHaveCount(1);

	await page.keyboard.press('Alt+ArrowLeft');
	await page.mouse.move(a.x, a.y);
	await page.mouse.up();

	expect(await page.evaluate(() => (window as unknown as { keys: boolean[] }).keys)).toEqual([
		true
	]);
	expect(await order(page)).toEqual(ids);
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

test('shows upload progress and retries a failed upload by tapping it', async ({
	page,
	isMobile
}) => {
	await editorWithPhotos(page, 0);
	let posts = 0;
	await page.route('**/api/v1/recipes/*/images', async (route) => {
		if (route.request().method() !== 'POST') return route.continue();
		posts++;
		if (posts === 1) {
			return route.fulfill({ status: 500, json: { title: 'Internal Server Error', status: 500 } });
		}
		// Slow enough to see the progress ring.
		await new Promise((resolve) => setTimeout(resolve, 500));
		return route.continue();
	});

	await page
		.getByLabel('Choose photos')
		.setInputFiles([{ name: 'one.png', mimeType: 'image/png', buffer: tinyPng() }]);
	const retry = page.getByRole('button', { name: 'Upload photo 1 again' });
	await expect(retry).toBeVisible();
	const hint = page.getByText(
		`Photo 1 could not be uploaded. ${isMobile ? 'Tap' : 'Click'} it to try again.`
	);
	await expect(hint).toBeVisible();

	await retry.click();
	await expect(page.getByRole('progressbar', { name: 'Uploading…' })).toBeVisible();
	await expect(tiles(page).locator('img[src^="/images/"]')).toHaveCount(1);
	await expect(hint).toBeHidden();
	await expect(tiles(page)).toHaveCount(1);
	expect(posts).toBe(2);
});

test('undoes a removal', async ({ page }) => {
	const { ids } = await editorWithPhotos(page, 3);
	let deletes = 0;
	page.on('request', (request) => {
		if (request.method() === 'DELETE') deletes++;
	});

	await page.getByRole('button', { name: 'Photo 2 options' }).click();
	await page.getByRole('menuitem', { name: 'Remove' }).click();
	await expect(tiles(page)).toHaveCount(2);
	await expect(page.getByText('Photo removed')).toBeVisible();
	await page.getByRole('button', { name: 'Undo' }).click();

	await expect(tiles(page)).toHaveCount(3);
	expect(await order(page)).toEqual(ids);
	await page.waitForTimeout(5000); // past the toast's own lifetime
	expect(deletes).toBe(0);
});

test('removes for good when the toast closes, also when reordered meanwhile', async ({
	page,
	isMobile
}) => {
	const { ids } = await editorWithPhotos(page, 3);
	const [one, two, three] = ids;

	await page.getByRole('button', { name: 'Photo 2 options' }).click();
	await page.getByRole('menuitem', { name: 'Remove' }).click();
	await expect(tiles(page)).toHaveCount(2);

	const saved = page.waitForRequest(isOrderSave);
	await drag(page, isMobile, tiles(page).nth(1), tiles(page).nth(0));
	// The server still holds the removed photo and wants every photo named.
	expect((await saved).postDataJSON()).toEqual({ imageIds: [three, one, two] });

	const deleted = await page.waitForResponse(
		(response) => response.request().method() === 'DELETE',
		{ timeout: 10_000 }
	);
	expect(deleted.url()).toContain(two);
	expect(deleted.ok()).toBeTruthy();

	await page.reload();
	await expect(tiles(page)).toHaveCount(2);
	expect(await order(page)).toEqual([three, one]);
});

test('names a removed photo in an order saved before its delete', async ({ page, isMobile }) => {
	const { ids } = await editorWithPhotos(page, 3);
	const [one, two, three] = ids;
	// An upload holds the queue, so the order save and then the delete
	// line up behind it.
	let release!: () => void;
	const held = new Promise<void>((resolve) => (release = resolve));
	await page.route('**/api/v1/recipes/*/images', async (route) => {
		if (route.request().method() === 'POST') await held;
		return route.continue();
	});
	let deletes = 0;
	page.on('request', (request) => {
		if (request.method() === 'DELETE') deletes++;
	});

	await page
		.getByLabel('Choose photos')
		.setInputFiles([{ name: 'late.png', mimeType: 'image/png', buffer: tinyPng([10, 200, 90]) }]);
	await page.getByRole('button', { name: 'Photo 2 options' }).click();
	await page.getByRole('menuitem', { name: 'Remove' }).click();
	await drag(page, isMobile, tiles(page).nth(1), tiles(page).nth(0));
	await expect(page.getByText('Photo removed')).toBeHidden({ timeout: 10_000 });

	const saved = page.waitForRequest(isOrderSave);
	const deleted = page.waitForResponse((response) => response.request().method() === 'DELETE');
	release();
	const body = (await saved).postDataJSON() as { imageIds: string[] };
	expect(body.imageIds.slice(0, 2)).toEqual([three, one]);
	expect(body.imageIds).toHaveLength(4);
	expect(body.imageIds.at(-1)).toBe(two);
	expect((await deleted).ok()).toBeTruthy();

	await expect(tiles(page).locator('img[src^="/images/"]')).toHaveCount(3);
	await page.waitForTimeout(500);
	expect(deletes).toBe(1);
	await expect(page.getByText('The new order could not be saved.')).toHaveCount(0);
	await page.reload();
	await expect(tiles(page)).toHaveCount(3);
	expect((await order(page)).slice(0, 2)).toEqual([three, one]);
});

test('sends a pending removal when leaving the editor', async ({ page }) => {
	const { recipe } = await editorWithPhotos(page, 3);
	const stored = async () =>
		((await (await page.request.get(`/api/v1/recipes/${recipe.id}`)).json()) as Recipe).images
			.length;

	// Leaving inside the app.
	await page.getByRole('button', { name: 'Photo 2 options' }).click();
	await page.getByRole('menuitem', { name: 'Remove' }).click();
	await page.getByRole('button', { name: 'Cancel', exact: true }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	await expect.poll(stored).toBe(2);

	// Leaving the page altogether.
	await page.goto(`/recipes/${recipe.slug}/edit`);
	await expect(tiles(page)).toHaveCount(2);
	await page.getByRole('button', { name: 'Photo 2 options' }).click();
	await page.getByRole('menuitem', { name: 'Remove' }).click();
	await page.goto('/');
	await expect.poll(stored).toBe(1);

	await page.goto(`/recipes/${recipe.slug}/edit`);
	await expect(tiles(page)).toHaveCount(1);
});

test('pastes photos and refuses what exceeds the limit', async ({ page }) => {
	await editorWithPhotos(page, 19);
	const png = tinyPng([30, 160, 90]).toString('base64');
	await page.locator('#editor-section-images').evaluate((section, data) => {
		const bytes = Uint8Array.from(atob(data), (c) => c.charCodeAt(0));
		const clipboard = new DataTransfer();
		clipboard.items.add(new File([bytes], 'a.png', { type: 'image/png' }));
		clipboard.items.add(new File([bytes], 'b.png', { type: 'image/png' }));
		section.dispatchEvent(
			new ClipboardEvent('paste', { clipboardData: clipboard, bubbles: true, cancelable: true })
		);
	}, png);

	await expect(tiles(page)).toHaveCount(20);
	await expect(page.getByText('No more than 20 photos.')).toBeVisible();
	await expect(page.getByLabel('Choose photos')).toHaveCount(0);
	await expect(tiles(page).locator('img[src^="/images/"]')).toHaveCount(20);
});

test('pastes photos while the page itself has the focus', async ({ page }) => {
	await editorWithPhotos(page, 1);
	const png = tinyPng([30, 160, 90]).toString('base64');
	await page.evaluate((data) => {
		const bytes = Uint8Array.from(atob(data), (c) => c.charCodeAt(0));
		const clipboard = new DataTransfer();
		clipboard.items.add(new File([bytes], 'a.png', { type: 'image/png' }));
		document.body.dispatchEvent(
			new ClipboardEvent('paste', { clipboardData: clipboard, bubbles: true, cancelable: true })
		);
	}, png);
	await expect(tiles(page)).toHaveCount(2);
	await expect(tiles(page).locator('img[src^="/images/"]')).toHaveCount(2);
});

test('makes a queued photo the cover of a new recipe', async ({ page }) => {
	const title = `Queued cover ${uniqueToken()}`;
	await page.goto('/recipes/new');
	await page.getByRole('textbox', { name: 'Title', exact: true }).fill(title);
	await page.getByRole('textbox', { name: 'Ingredient', exact: true }).fill('Flour');
	// Told apart by width: the detail rendition keeps a small photo's size.
	await page.getByLabel('Choose photos').setInputFiles([
		{ name: 'one.png', mimeType: 'image/png', buffer: tinyPng([200, 120, 40], 64, 64) },
		{ name: 'two.png', mimeType: 'image/png', buffer: tinyPng([40, 120, 200], 96, 64) }
	]);
	await expect(tiles(page)).toHaveCount(2);
	await page.getByRole('button', { name: 'Photo 2 options' }).click();
	await page.getByRole('menuitem', { name: 'Make cover' }).click();

	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page).toHaveURL(/\/recipes\/queued-cover-/);
	const cover = page.getByRole('img', { name: `Cover photo of ${title}` });
	await expect(cover).toBeVisible();
	await expect.poll(() => cover.evaluate((img) => (img as HTMLImageElement).naturalWidth)).toBe(96);
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

for (const leave of ['Cancel', 'Save']) {
	test(`a photo removed before ${leave} is gone from the recipe page`, async ({ page }) => {
		const { recipe } = await editorWithPhotos(page, 3);
		// A slow delete: the recipe page loads while it is on its way unless
		// the editor waits for it.
		await page.route('**/api/v1/recipes/*/images/*', async (route) => {
			if (route.request().method() === 'DELETE') {
				await new Promise((resolve) => setTimeout(resolve, 500));
			}
			return route.continue();
		});

		await page.getByRole('button', { name: 'Photo 2 options' }).click();
		await page.getByRole('menuitem', { name: 'Remove' }).click();
		await expect(tiles(page)).toHaveCount(2);
		await page.getByRole('button', { name: leave, exact: true }).click();

		await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
		await expect(page.getByRole('button', { name: 'Open photo 1 of 2' })).toBeVisible();
	});
}

test('a cover whose delete fails comes back as the cover', async ({ page }) => {
	const { recipe, ids } = await editorWithPhotos(page, 3);
	await page.route('**/api/v1/recipes/*/images/*', (route) =>
		route.request().method() === 'DELETE'
			? route.fulfill({ status: 500, json: { title: 'Internal Server Error', status: 500 } })
			: route.continue()
	);

	await page.getByRole('button', { name: 'Photo 1 options' }).click();
	await page.getByRole('menuitem', { name: 'Remove' }).click();
	await expect(tiles(page)).toHaveCount(2);
	// The delete goes out when the undo toast closes.
	await expect(page.getByText('Photo could not be removed.')).toBeVisible({ timeout: 10_000 });

	await expect(tiles(page)).toHaveCount(3);
	expect(await order(page)).toEqual(ids);
	const stored = (await (await page.request.get(`/api/v1/recipes/${recipe.id}`)).json()) as Recipe;
	expect(stored.images[0].id).toBe(ids[0]);
	await expect(tiles(page).first().getByText('Cover', { exact: true })).toBeVisible();
});

// Only a click opens a photo: the release that ends a drag, or a press held
// until the photo lifted, puts it down instead.
test('a photo opens on a click, not when a drag or a held press ends', async ({
	page,
	isMobile
}) => {
	await editorWithPhotos(page, 3);
	const dialog = page.getByRole('dialog');

	await drag(page, isMobile, tiles(page).nth(2), tiles(page).nth(1));
	await page.waitForTimeout(400);
	await expect(dialog).toBeHidden();

	const at = await center(tiles(page).nth(1));
	if (isMobile) {
		const cdp = await page.context().newCDPSession(page);
		await cdp.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [at] });
		await page.waitForTimeout(400);
		await cdp.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
		await cdp.detach();
	} else {
		await page.mouse.move(at.x, at.y);
		await page.mouse.down();
		await page.waitForTimeout(300);
		await page.mouse.up();
	}
	await page.waitForTimeout(400);
	await expect(dialog).toBeHidden();

	await (isMobile
		? tiles(page).nth(1).locator('img').tap()
		: tiles(page).nth(1).locator('img').click());
	await expect(dialog).toBeVisible();
});

test('a press on a photo’s menu button never picks the photo up', async ({ page, isMobile }) => {
	const { ids } = await editorWithPhotos(page, 3);
	let saves = 0;
	page.on('request', (request) => {
		if (isOrderSave(request)) saves++;
	});

	await drag(
		page,
		isMobile,
		page.getByRole('button', { name: 'Photo 3 options' }),
		tiles(page).nth(1)
	);
	await page.waitForTimeout(300);

	expect(saves).toBe(0);
	expect(await order(page)).toEqual(ids);
});

test('a long press on a photo selects nothing and opens no menu', async ({ page, isMobile }) => {
	test.skip(!isMobile, 'Chrome’s long-press is a touch gesture');
	await editorWithPhotos(page, 3);

	const { selection, prevented, unprevented, lifted } = await longPress(page, tiles(page).nth(1));

	expect(lifted).toBe(true);
	expect(prevented).not.toEqual([]);
	expect(selection).toBe('');
	expect(unprevented).toEqual([]);
});

// The phone's "..." sits over the photo. At 200% text its menu was wider than
// the screen: the page grew under it, the fixed bar reflowed, the button moved
// out from under the finger and the tap's click opened the photo behind it.
test('the more-actions tap opens the menu and not the photo', async ({ page, isMobile }) => {
	test.skip(!isMobile, 'a tap over the phone gallery');
	const token = uniqueToken();
	const recipe = await createRecipe(page, { ...loadFixture(1), title: `Menu ${token}` });
	await uploadImage(page, recipe.id, tinyPng([200, 120, 40]));

	await page.goto(`/recipes/${recipe.slug}`);
	await expect(page.getByRole('button', { name: 'Open photo 1 of 1' })).toBeVisible();
	await page.evaluate(() => (document.documentElement.style.fontSize = '200%'));
	const more = page.getByRole('button', { name: 'More actions' });
	// Squeezed by the large text, the bar's buttons stay circles.
	const box = await more.boundingBox();
	expect(Math.abs(box!.width - box!.height)).toBeLessThan(1);

	await more.tap();
	await expect(page.getByRole('menu')).toBeVisible();
	await expect(page.getByRole('dialog')).toHaveCount(0);
	expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(
		page.viewportSize()!.width
	);
});

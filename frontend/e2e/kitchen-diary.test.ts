import { expect, test, type Locator, type Page, type TestInfo } from '@playwright/test';
import {
	addDiaryPhoto,
	createRecipe,
	createUser,
	loadFixture,
	login,
	setRecipesLockedByDefault,
	signOut,
	tinyPng,
	uniqueToken
} from './helpers';

/**
 * Signs whoever is in out and `username` in, from the overview: the recipe
 * page hides the phone's bottom nav, which holds the sign-out.
 */
async function switchTo(page: Page, testInfo: TestInfo, username: string) {
	await page.goto('/');
	await signOut(page, testInfo);
	await login(page, username);
	await expect(page).toHaveURL('/');
}

const diary = (page: Page) => page.getByRole('tabpanel', { name: /^Comments/ });
const diaryTab = (page: Page) => page.getByRole('tab', { name: /^Comments/ });
const dot = (page: Page) => page.getByRole('button', { name: /new comments/ });

/** Opens the recipe page, then its diary tab, and waits until that marked the diary seen. */
async function openRecipe(page: Page, slug: string) {
	await page.goto(`/recipes/${slug}`);
	const seen = page.waitForResponse(
		(r) => r.url().endsWith('/comments/seen') && r.request().method() === 'PUT'
	);
	await diaryTab(page).click();
	expect((await seen).status()).toBe(204);
}

test('members talk in the comments and the author sees the dot', async ({ page }, testInfo) => {
	const token = uniqueToken();
	const mara = `mara${token}`.slice(0, 20);
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username: mara, role: 'user' });
	const recipe = await createRecipe(page, { ...loadFixture(1), title: `Diary ${token}` });

	// A member writes on somebody else's recipe, then edits it in place.
	// An empty diary has nothing to mark seen, opened or not.
	await switchTo(page, testInfo, mara);
	let seenCalls = 0;
	const count = (r: { url(): string; method(): string }) => {
		if (r.url().endsWith('/comments/seen') && r.method() === 'PUT') seenCalls++;
	};
	page.on('request', count);
	await page.goto(`/recipes/${recipe.slug}`);
	await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
	expect(seenCalls).toBe(0);
	await diaryTab(page).click();
	await expect(diary(page).getByText('No comments yet')).toBeVisible();
	expect(seenCalls).toBe(0);
	page.off('request', count);
	await expect(diaryTab(page)).toHaveAccessibleName(/No comments yet/);
	// With a keyboard Shift+Enter breaks the line and Enter writes the entry;
	// on a touch screen Enter breaks the line and the send button writes.
	const touch = testInfo.project.name === 'mobile';
	const composer = diary(page).getByPlaceholder('What did you notice?');
	await composer.fill('Capers only');
	// An IME's Enter only confirms its composition: nothing is written (a
	// second entry would break the single-entry checks below).
	await composer.dispatchEvent('keydown', { key: 'Enter', isComposing: true });
	await expect(composer).toHaveValue('Capers only');
	await composer.press(touch ? 'Enter' : 'Shift+Enter');
	await composer.pressSequentially('at the very end.');
	await expect(composer).toHaveValue('Capers only\nat the very end.');
	if (touch) {
		await diary(page).getByRole('button', { name: 'Send' }).click();
	} else {
		await composer.press('Enter');
	}
	const entry = diary(page).getByRole('listitem');
	await expect(entry).toHaveText(/Capers only\s+at the very end\./);
	await expect(diaryTab(page)).toHaveAccessibleName(/1 comment/);
	await expect(composer).toHaveValue('');
	// Ready for the next one, keyboard still up on a phone.
	await expect(composer).toBeFocused();
	await expect(diary(page).getByText('Comment sent')).toBeAttached();

	await entry.getByRole('button', { name: 'Comment options' }).click();
	await page.getByRole('menuitem', { name: 'Edit' }).click();
	const field = entry.getByRole('textbox');
	await expect(field).toBeFocused();
	await expect(field).toHaveValue('Capers only\nat the very end.');
	await field.fill('Capers only at the very end, or they go mushy.');
	if (touch) {
		await field.press('Enter');
		await expect(field).toHaveValue('Capers only at the very end, or they go mushy.\n');
		await field.press('Backspace');
		await entry.getByRole('button', { name: 'Save' }).click();
	} else {
		await field.press('Enter');
	}
	await expect(entry).toHaveText(/or they go mushy\./);
	await expect(entry.getByText('edited')).toBeVisible();

	// The recipe's author sees the dot on the card until opening the diary;
	// the tab carries it too, and drops it once opened - once a visit.
	await switchTo(page, testInfo, 'admin');
	await page.goto(`/?q=${token}`);
	await expect(dot(page)).toBeVisible();
	await page.goto(`/recipes/${recipe.slug}`);
	await expect(diaryTab(page)).toHaveAccessibleName(/1 comment\s*New/);
	await page.goto(`/?q=${token}`);
	await expect(dot(page)).toBeVisible();
	await openRecipe(page, recipe.slug);
	await expect(entry.getByText('New', { exact: true })).toBeAttached();
	await expect(diaryTab(page)).not.toHaveAccessibleName(/New/);
	page.on('request', count);
	await page.getByRole('tab', { name: 'Method' }).click();
	await diaryTab(page).click();
	await expect(diary(page)).toBeVisible();
	expect(seenCalls).toBe(0);
	page.off('request', count);
	await page.goto(`/?q=${token}`);
	await expect(page.getByRole('heading', { level: 3, name: `Diary ${token}` })).toBeVisible();
	await expect(dot(page)).toHaveCount(0);

	// The author answers with Ctrl/Cmd+Enter; an own entry is never new.
	await openRecipe(page, recipe.slug);
	await expect(entry.getByText('New', { exact: true })).toHaveCount(0);
	await composer.fill('Noted, thanks.');
	await composer.press('ControlOrMeta+Enter');
	await expect(diaryTab(page)).toHaveAccessibleName(/2 comments/);

	// An admin deletes somebody else's entry, after confirming.
	await diary(page)
		.getByRole('listitem')
		.filter({ hasText: 'mushy' })
		.getByRole('button', { name: 'Comment options' })
		.click();
	await expect(page.getByRole('menuitem', { name: 'Edit' })).toHaveCount(0);
	await page.getByRole('menuitem', { name: 'Delete' }).click();
	await page.getByRole('dialog').getByRole('button', { name: 'Delete' }).click();
	await expect(diary(page).getByText('mushy')).toHaveCount(0);
	await expect(diaryTab(page)).toHaveAccessibleName(/1 comment/);
	// The removed entry's menu held the focus; the composer takes it.
	await expect(composer).toBeFocused();

	// Her only entry is gone, and with it her part in the conversation: the
	// author's answer brings her no dot.
	await switchTo(page, testInfo, mara);
	await page.goto(`/?q=${token}`);
	await expect(page.getByRole('heading', { level: 3, name: `Diary ${token}` })).toBeVisible();
	await expect(dot(page)).toHaveCount(0);
});

test('a tablet sets a long ingredient list in two columns, a short one in one', async ({
	page
}) => {
	const token = uniqueToken();
	await page.setViewportSize({ width: 720, height: 1000 });
	await login(page);
	await expect(page).toHaveURL('/');
	const fixture = loadFixture(1);
	const long = await createRecipe(page, { ...fixture, title: `Columns ${token}` });
	const short = await createRecipe(page, {
		...fixture,
		title: `Column ${token}`,
		ingredientGroups: [
			{ name: null, ingredients: fixture.ingredientGroups[0].ingredients.slice(0, 3) }
		],
		// The steps name ingredients the short list no longer has.
		steps: fixture.steps.map((step) => ({ ...step, references: [] }))
	});
	const lefts = async () =>
		new Set(
			await page
				.getByRole('checkbox')
				.evaluateAll((boxes) => boxes.map((b) => Math.round(b.getBoundingClientRect().left)))
		).size;

	await page.goto(`/recipes/${long.slug}`);
	await expect(page.getByRole('checkbox').first()).toBeVisible();
	expect(await lefts()).toBe(2);
	// Checking off works in the right column too.
	const last = page.getByRole('checkbox').last();
	await last.click();
	await expect(last).toBeChecked();

	await page.goto(`/recipes/${short.slug}`);
	await expect(page.getByRole('checkbox').first()).toBeVisible();
	expect(await lefts()).toBe(1);
});

const png = (name: string, tint: [number, number, number]) => ({
	name,
	mimeType: 'image/png',
	buffer: tinyPng(tint)
});

test('members attach photos to comments', async ({ page }) => {
	const token = uniqueToken();
	await login(page);
	await expect(page).toHaveURL('/');
	const recipe = await createRecipe(page, { ...loadFixture(1), title: `Photo diary ${token}` });
	await page.goto(`/recipes/${recipe.slug}`);
	await diaryTab(page).click();

	const uploads: string[] = [];
	page.on('request', (r) => {
		if (r.url().includes('/comment-images')) uploads.push(r.url());
	});
	const composer = diary(page).getByPlaceholder('What did you notice?');
	await diary(page)
		.getByLabel('Add photos')
		.setInputFiles([png('one.png', [200, 120, 40]), png('two.png', [40, 120, 200])]);
	const pending = diary(page).getByRole('list', { name: 'Photos in this comment' });
	await expect(pending.getByRole('listitem')).toHaveCount(2);
	await expect(diary(page).getByPlaceholder('Something about your photo?')).toBeVisible();
	// Nothing goes up before Send.
	expect(uploads).toEqual([]);

	await pending.getByRole('listitem').nth(1).getByRole('button', { name: 'Remove photo' }).click();
	await expect(pending.getByRole('listitem')).toHaveCount(1);

	// A GIF is turned away with a toast and does not join the tiles.
	await diary(page)
		.getByLabel('Add photos')
		.setInputFiles({ name: 'loop.gif', mimeType: 'image/gif', buffer: Buffer.from('GIF89a') });
	await expect(page.getByText('Only JPEG, PNG and WebP are supported.')).toBeVisible();
	await expect(pending.getByRole('listitem')).toHaveCount(1);

	await diary(page).getByPlaceholder('Something about your photo?').fill('The crust');
	await diary(page).getByRole('button', { name: 'Send' }).click();
	// The pending tiles are list items too; an entry is the one with a menu.
	const entry = diary(page)
		.getByRole('listitem')
		.filter({ has: page.getByRole('button', { name: 'Comment options' }) });
	await expect(entry).toHaveText(/The crust/);
	await expect(entry.getByRole('button', { name: 'Open photo 1 of 1' })).toBeVisible();
	expect(uploads).toHaveLength(1);
	await expect(composer).toHaveValue('');

	// A narrow phone: the bubble holds its photos.
	await page.setViewportSize({ width: 320, height: 800 });
	const bubble = entry.locator('.diary-paper');
	await expect.poll(() => bubble.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true);

	await entry.getByRole('button', { name: 'Open photo 1 of 1' }).click();
	await expect(page.getByRole('dialog')).toBeVisible();
	await page.keyboard.press('Escape');
	await expect(page.getByRole('dialog')).toHaveCount(0);

	// The photo is served until the comment is deleted.
	const listed = await page.request.get(`/api/v1/comments?recipeId=${recipe.id}`);
	const {
		comments: [{ images }]
	} = (await listed.json()) as { comments: { images: { id: string }[] }[] };
	const url = `/images/${recipe.id}/comments/${images[0].id}/thumb.jpg`;
	expect((await page.request.get(url)).status()).toBe(200);

	// Editing takes the photo off the comment.
	await entry.getByRole('button', { name: 'Comment options' }).click();
	await page.getByRole('menuitem', { name: 'Edit' }).click();
	// The edit form replaces the bubble, menu and all.
	await diary(page).getByRole('button', { name: 'Remove photo' }).click();
	await diary(page).getByRole('button', { name: 'Save' }).click();
	await expect(entry.getByRole('button', { name: /^Open photo/ })).toHaveCount(0);
	await expect(entry.getByText('edited')).toBeVisible();

	// A comment with a photo, deleted, takes the file with it.
	await diary(page)
		.getByLabel('Add photos')
		.setInputFiles(png('three.png', [90, 90, 90]));
	await diary(page).getByPlaceholder('Something about your photo?').fill('Second look');
	await diary(page).getByRole('button', { name: 'Send' }).click();
	const second = entry.filter({ hasText: 'Second look' });
	await expect(second.getByRole('button', { name: 'Open photo 1 of 1' })).toBeVisible();
	const again = await page.request.get(`/api/v1/comments?recipeId=${recipe.id}`);
	const { comments: all } = (await again.json()) as {
		comments: { body: string; images: { id: string }[] }[];
	};
	const kept = all.find((c) => c.body === 'Second look')!.images[0].id;
	const keptUrl = `/images/${recipe.id}/comments/${kept}/thumb.jpg`;
	expect((await page.request.get(keptUrl)).status()).toBe(200);
	await second.getByRole('button', { name: 'Comment options' }).click();
	await page.getByRole('menuitem', { name: 'Delete' }).click();
	await page.getByRole('dialog').getByRole('button', { name: 'Delete' }).click();
	await expect(second).toHaveCount(0);
	expect((await page.request.get(keptUrl)).status()).toBe(404);
});

/** Signs in, creates a recipe and opens its diary. */
async function freshDiary(page: Page, title: string) {
	await login(page);
	await expect(page).toHaveURL('/');
	const recipe = await createRecipe(page, {
		...loadFixture(1),
		title: `${title} ${uniqueToken()}`
	});
	await page.goto(`/recipes/${recipe.slug}`);
	await diaryTab(page).click();
	return recipe;
}

const entries = (page: Page) =>
	diary(page)
		.getByRole('listitem')
		.filter({ has: page.getByRole('button', { name: 'Comment options' }) });

// 422 as well: an upload's own refusal (a photo too small, say) must not be
// read as the comment's, which would unstage the photos that went up.
for (const status of [500, 422]) {
	test(`a failed upload (${status}) marks its tile and the retry sends only that photo`, async ({
		page
	}) => {
		await freshDiary(page, 'Retry');
		const pending = diary(page).getByRole('list', { name: 'Photos in this comment' });

		// The fourth photo takes the picker away; its tile's remove button takes the focus.
		await diary(page)
			.getByLabel('Add photos')
			.setInputFiles([1, 2, 3, 4].map((n) => png(`${n}.png`, [n * 50, 90, 90])));
		await expect(pending.getByRole('button', { name: 'Remove photo 4' })).toBeFocused();
		await pending.getByRole('button', { name: 'Remove photo 4' }).click();
		await pending.getByRole('button', { name: 'Remove photo 3' }).click();
		await expect(pending.getByRole('listitem')).toHaveCount(2);

		let uploads = 0;
		await page.route('**/comment-images', (route) => {
			uploads++;
			return uploads === 2 ? route.fulfill({ status, json: { error: 'boom' } }) : route.fallback();
		});
		await diary(page).getByPlaceholder('Something about your photo?').fill('Two shots');
		await diary(page).getByRole('button', { name: 'Send' }).click();
		const failed = pending.getByRole('button', { name: /could not be uploaded/ });
		await expect(failed).toHaveCount(1);
		await expect(
			pending
				.getByRole('listitem')
				.nth(1)
				.getByRole('button', { name: /^Photo 2 could not/ })
		).toBeVisible();
		await expect(
			diary(page)
				.getByRole('alert')
				.getByText('Photo 2 could not be uploaded. Tap it to try again.')
		).toBeVisible();
		expect(uploads).toBe(2);

		await diary(page).getByRole('button', { name: 'Send' }).click();
		await expect(entries(page).getByRole('button', { name: 'Open photo 2 of 2' })).toBeVisible();
		expect(uploads).toBe(3);
	});
}

test('a comment refused for a gone photo reloads the diary and uploads it again', async ({
	page
}) => {
	const recipe = await freshDiary(page, 'Gone');
	let uploads = 0;
	page.on('request', (r) => {
		if (r.url().endsWith('/comment-images')) uploads++;
	});
	// The first comment lands, but its answer says a photo is gone: the
	// reloaded list shows it once, and the photo goes up again.
	let posts = 0;
	await page.route(
		(url) => url.pathname === `/api/v1/recipes/${recipe.id}/comments`,
		async (route) => {
			if (route.request().method() !== 'POST' || ++posts > 1) return route.fallback();
			await route.fetch();
			return route.fulfill({ status: 422, json: { error: 'unknown image' } });
		}
	);
	await diary(page)
		.getByLabel('Add photos')
		.setInputFiles(png('one.png', [200, 120, 40]));
	const field = diary(page).getByPlaceholder('Something about your photo?');
	await field.fill('Once more');
	const reloaded = page.waitForResponse(
		(r) => r.url().includes(`/comments?recipeId=${recipe.id}`) && r.request().method() === 'GET'
	);
	await diary(page).getByRole('button', { name: 'Send' }).click();
	await expect(page.getByText('Your comment could not be saved.')).toBeVisible();
	await reloaded;
	await expect(entries(page)).toHaveCount(1);
	await expect(field).toHaveValue('Once more');
	expect(uploads).toBe(1);

	await diary(page).getByRole('button', { name: 'Send' }).click();
	await expect(entries(page)).toHaveCount(2);
	expect(uploads).toBe(2);
});

/** The composer's photos, apart from the posted comments' collages. */
const composerPhotos = (page: Page) =>
	diary(page)
		.locator('form', { has: page.getByRole('button', { name: 'Send' }) })
		.getByRole('list', { name: 'Photos in this comment' });

/** Drags `from` onto `to` with a mouse, in steps, so the sortable sees the way. */
async function drag(page: Page, from: Locator, to: Locator) {
	// Both ends on screen: a press outside the viewport never lands.
	await to.scrollIntoViewIfNeeded();
	await from.scrollIntoViewIfNeeded();
	const a = (await from.boundingBox())!;
	const b = (await to.boundingBox())!;
	const [ax, ay, bx, by] = [
		a.x + a.width / 2,
		a.y + a.height / 2,
		b.x + b.width / 2,
		b.y + b.height / 2
	];
	await page.mouse.move(ax, ay);
	await page.mouse.down();
	for (let i = 1; i <= 12; i++)
		await page.mouse.move(ax + ((bx - ax) * i) / 12, ay + ((by - ay) * i) / 12);
	await page.mouse.up();
}

/** Pastes PNGs into `field`, one paste event per entry of `counts`, all in one task. */
async function pasteImages(field: Locator, counts: number[]) {
	await field.evaluate(
		(el, [data, counts]) => {
			const bytes = Uint8Array.from(atob(data as string), (c) => c.charCodeAt(0));
			for (const count of counts as number[]) {
				const transfer = new DataTransfer();
				for (let i = 1; i <= count; i++)
					transfer.items.add(new File([bytes], `${i}.png`, { type: 'image/png' }));
				el.dispatchEvent(
					new ClipboardEvent('paste', { clipboardData: transfer, bubbles: true, cancelable: true })
				);
			}
		},
		[tinyPng().toString('base64'), counts]
	);
}

/** Three photos told apart by their size: 64x64, 64x96, 96x64. */
const threeShapes = () => [
	{ name: 'square.png', mimeType: 'image/png', buffer: tinyPng([200, 120, 40], 64, 64) },
	{ name: 'tall.png', mimeType: 'image/png', buffer: tinyPng([40, 120, 200], 64, 96) },
	{ name: 'wide.png', mimeType: 'image/png', buffer: tinyPng([90, 160, 90], 96, 64) }
];

async function postedWidths(page: Page, recipeId: string, body: string) {
	const listed = await page.request.get(`/api/v1/comments?recipeId=${recipeId}`);
	const { comments } = (await listed.json()) as {
		comments: { body: string; images: { width: number }[] }[];
	};
	return comments.find((c) => c.body === body)!.images.map((i) => i.width);
}

test('reorders picked photos and posts them in that order', async ({ page }) => {
	const recipe = await freshDiary(page, 'Collage');
	await diary(page).getByLabel('Add photos').setInputFiles(threeShapes());
	const cells = composerPhotos(page).getByRole('listitem');
	await expect(cells).toHaveCount(3);
	await expect(cells.first().locator('img')).toHaveAttribute('width', '64');

	await diary(page).getByPlaceholder('Something about your photo?').fill('Wide first');

	// Photo 3 onto the large cell: it grows into it, the others move along.
	await drag(page, cells.nth(2), cells.nth(0));
	// Send at once, in the drop's own moment: the sortable eats only the
	// drop's click, never the next press.
	const send = (await diary(page).getByRole('button', { name: 'Send' }).boundingBox())!;
	await page.mouse.click(send.x + send.width / 2, send.y + send.height / 2);
	const entry = entries(page).filter({ hasText: 'Wide first' });
	await expect(entry.getByRole('button', { name: 'Open photo 3 of 3' })).toBeVisible();
	// The large cell is photo 3, posted as it was shown.
	await expect(entry.locator('img').first()).toHaveAttribute('width', '96');
	expect(await postedWidths(page, recipe.id, 'Wide first')).toEqual([96, 64, 64]);

	await page.reload();
	await diaryTab(page).click();
	await expect(entries(page).locator('img').first()).toHaveAttribute('width', '96');
});

test('moves a picked photo by Alt+Arrow', async ({ page }) => {
	await freshDiary(page, 'Keys');
	await diary(page).getByLabel('Add photos').setInputFiles(threeShapes());
	const cells = composerPhotos(page).getByRole('listitem');
	await expect(cells.nth(2).locator('img')).toHaveAttribute('width', '96');
	const widths = () =>
		cells.locator('img').evaluateAll((imgs) => imgs.map((i) => i.getAttribute('width')));

	await composerPhotos(page).getByRole('button', { name: 'Open photo 2 of 3' }).focus();
	await page.keyboard.press('Alt+ArrowRight');
	await expect.poll(widths).toEqual(['64', '96', '64']);
	await expect(page.getByText('Photo moved to position 3 of 3', { exact: true })).toBeAttached();

	// Already first: nothing moves.
	await composerPhotos(page).getByRole('button', { name: 'Open photo 1 of 3' }).focus();
	await page.keyboard.press('Alt+ArrowLeft');
	await page.waitForTimeout(300);
	expect(await widths()).toEqual(['64', '96', '64']);
});

test('a press on a picked photo’s remove button never picks the photo up', async ({ page }) => {
	await freshDiary(page, 'Remove press');
	await diary(page).getByLabel('Add photos').setInputFiles(threeShapes());
	const cells = composerPhotos(page).getByRole('listitem');
	await expect(cells).toHaveCount(3);
	const widths = () =>
		cells.locator('img').evaluateAll((imgs) => imgs.map((i) => i.getAttribute('width')));
	const before = await widths();

	await drag(page, diary(page).getByRole('button', { name: 'Remove photo 3' }), cells.nth(0));

	await expect(composerPhotos(page).locator('[data-lifted]')).toHaveCount(0);
	expect(await widths()).toEqual(before);
});

test('retrying a failed photo posts the comment', async ({ page }) => {
	await freshDiary(page, 'Tap retry');
	await diary(page).getByLabel('Add photos').setInputFiles(threeShapes());
	let uploads = 0;
	await page.route('**/comment-images', (route) =>
		++uploads === 2 ? route.fulfill({ status: 500, json: { error: 'boom' } }) : route.fallback()
	);
	await diary(page).getByPlaceholder('Something about your photo?').fill('Third time lucky');
	await diary(page).getByRole('button', { name: 'Send' }).click();
	const retry = composerPhotos(page).getByRole('button', { name: /^Photo 2 could not/ });
	await expect(retry).toBeVisible();
	await expect(diary(page).getByRole('alert')).toHaveText(
		'Photo 2 could not be uploaded. Tap it to try again.'
	);
	// The text and the other photos stay; tapping the photo sends without another Send.
	await retry.click();
	await expect(entries(page).filter({ hasText: 'Third time lucky' })).toHaveCount(1);
	await expect(entries(page).getByRole('button', { name: 'Open photo 3 of 3' })).toBeVisible();
	await expect(composerPhotos(page)).toHaveCount(0);
	await expect(diary(page).getByRole('alert')).toHaveCount(0);
	expect(uploads).toBe(4);
});

test('reorders the photos of a posted comment when editing', async ({ page }) => {
	const recipe = await freshDiary(page, 'Edit order');
	await diary(page).getByLabel('Add photos').setInputFiles(threeShapes().slice(0, 2));
	await diary(page).getByPlaceholder('Something about your photo?').fill('Two of them');
	await diary(page).getByRole('button', { name: 'Send' }).click();
	const entry = entries(page);
	await expect(entry.getByRole('button', { name: 'Open photo 2 of 2' })).toBeVisible();
	await expect(entry.getByText('edited')).toHaveCount(0);

	await entry.getByRole('button', { name: 'Comment options' }).click();
	await page.getByRole('menuitem', { name: 'Edit' }).click();
	const form = diary(page)
		.getByRole('listitem')
		.filter({ has: page.getByRole('textbox', { name: 'Your comment' }) });
	const cells = form.getByRole('list', { name: 'Photos in this comment' }).getByRole('listitem');
	await expect(cells.first().locator('img')).toHaveAttribute('height', '64');
	await drag(page, cells.nth(1), cells.nth(0));
	await form.getByRole('button', { name: 'Save' }).click();

	await expect(entry.getByText('edited')).toBeVisible();
	await expect(entry.locator('img').first()).toHaveAttribute('height', '96');
	await page.reload();
	await diaryTab(page).click();
	await expect(entries(page).locator('img').first()).toHaveAttribute('height', '96');
	await expect(entries(page).getByText('edited')).toBeVisible();
	expect(await postedWidths(page, recipe.id, 'Two of them')).toEqual([64, 64]);
});

test('pastes photos into the composer up to four', async ({ page }) => {
	await freshDiary(page, 'Paste');
	const field = diary(page).getByPlaceholder('What did you notice?');
	await pasteImages(field, [5]);
	await expect(composerPhotos(page).getByRole('listitem')).toHaveCount(4);
	await expect(page.getByText('Up to 4 photos per comment')).toBeVisible();
	await expect(diary(page).getByLabel('Add photos')).toBeDisabled();
});

test('two quick pastes stop at four photos', async ({ page }) => {
	await freshDiary(page, 'Paste twice');
	// Both land before either file is decoded: the limit counts the first.
	await pasteImages(diary(page).getByPlaceholder('What did you notice?'), [3, 3]);
	await expect(composerPhotos(page).getByRole('listitem')).toHaveCount(4);
	await expect(page.getByText('Up to 4 photos per comment')).toBeVisible();
});

test('shows a picked photo large before sending', async ({ page }) => {
	const recipe = await freshDiary(page, 'Preview');
	await diary(page).getByLabel('Add photos').setInputFiles(threeShapes());
	await composerPhotos(page).getByRole('button', { name: 'Open photo 2 of 3' }).click();
	const dialog = page.getByRole('dialog');
	await expect(dialog.getByText('2 / 3')).toBeVisible();
	// Named after the recipe, as every lightbox is.
	await expect(dialog).toHaveAccessibleName(recipe.title);
	await expect(dialog).toHaveAccessibleDescription(`Photos of ${recipe.title}`);
	await expect(dialog.getByRole('link', { name: 'Download the original' })).toHaveCount(0);
	await expect(dialog.getByRole('button', { name: 'Add to recipe photos' })).toHaveCount(0);
});

test('an edit cannot be cancelled while it saves', async ({ page }) => {
	const recipe = await freshDiary(page, 'Edit busy');
	await addDiaryPhoto(page, recipe.id, 'Before', tinyPng());
	await page.reload();
	await diaryTab(page).click();
	const entry = entries(page);
	await entry.getByRole('button', { name: 'Comment options' }).click();
	await page.getByRole('menuitem', { name: 'Edit' }).click();
	// The edit form stands in the entry's list item; the composer has a picker too.
	const form = diary(page)
		.getByRole('listitem')
		.filter({ has: page.getByRole('textbox', { name: 'Your comment' }) });
	await form.getByLabel('Add photos').setInputFiles(png('two.png', [40, 120, 200]));

	let release = () => {};
	const gate = new Promise<void>((resolve) => (release = resolve));
	await page.route('**/comment-images', async (route) => {
		await gate;
		await route.fallback();
	});
	let patches = 0;
	page.on('request', (r) => {
		if (r.method() === 'PATCH' && r.url().includes('/comments/')) patches++;
	});
	await diary(page).getByRole('button', { name: 'Save' }).click();
	const cancel = diary(page).getByRole('button', { name: 'Cancel' });
	await expect(cancel).toBeDisabled();
	const field = diary(page).getByRole('textbox', { name: 'Your comment' });
	await expect(field).toHaveAttribute('readonly');
	await field.press('Escape');
	await expect(field).toBeVisible();
	expect(patches).toBe(0);

	release();
	await expect(entries(page).getByRole('button', { name: 'Open photo 2 of 2' })).toBeVisible();
	await expect(field).toHaveCount(0);
	expect(patches).toBe(1);
});

test('the author adds a comment photo to the recipe photos', async ({
	page,
	browser
}, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'the household lock is instance-wide');
	try {
		const token = uniqueToken();
		const ben = `ben${token}`.slice(0, 20);
		await login(page);
		await expect(page).toHaveURL('/');
		await createUser(page, { username: ben, role: 'user' });
		const recipe = await createRecipe(page, { ...loadFixture(1), title: `Gallery ${token}` });
		const recipeImages = async () =>
			(
				(await (await page.request.get(`/api/v1/recipes/${recipe.id}`)).json()) as {
					images: unknown[];
				}
			).images.length;
		expect(await recipeImages()).toBe(0);

		await page.goto(`/recipes/${recipe.slug}`);
		await diaryTab(page).click();
		await diary(page)
			.getByLabel('Add photos')
			.setInputFiles(png('one.png', [200, 120, 40]));
		await diary(page).getByPlaceholder('Something about your photo?').fill('Look at this');
		await diary(page).getByRole('button', { name: 'Send' }).click();
		await diary(page).getByRole('button', { name: 'Open photo 1 of 1' }).click();

		const dialog = page.getByRole('dialog');
		// Only the recipe reloads: the cover shows, the comments stay as they were.
		let listed = 0;
		page.on('request', (r) => {
			if (r.url().includes('/comments?recipeId=')) listed++;
		});
		await dialog.getByRole('button', { name: 'Add to recipe photos' }).click();
		await expect(page.getByText('Added to the recipe photos')).toBeVisible();
		const done = dialog.getByRole('button', { name: 'In the recipe photos' });
		await expect(done).toBeDisabled();
		// The disabled button dropped the focus; Close took it.
		await expect(dialog.getByRole('button', { name: 'Close' })).toBeFocused();
		expect(await recipeImages()).toBe(1);
		await page.keyboard.press('Escape');
		await expect(page.getByRole('img', { name: `Cover photo of Gallery ${token}` })).toBeVisible();
		expect(listed).toBe(0);

		// A member who marked the recipe tasty adds a photo too: the reload of the
		// photos keeps their circle.
		const benContext = await browser.newContext();
		try {
			const benPage = await benContext.newPage();
			await login(benPage, ben);
			await expect(benPage).toHaveURL('/');
			await addDiaryPhoto(benPage, recipe.id, 'Mine too', tinyPng([40, 120, 200]));
			await benPage.goto(`/recipes/${recipe.slug}`);
			await benPage.getByRole('button', { name: 'Tasty', exact: true, pressed: false }).click();
			const circle = benPage.getByRole('button', { name: `Tasty for: ${ben}` });
			await expect(circle).toBeVisible();
			await benPage.getByRole('tab', { name: /^Comments/ }).click();
			await diary(benPage)
				.getByRole('listitem')
				.filter({ hasText: 'Mine too' })
				.getByRole('button', { name: 'Open photo 1 of 1' })
				.click();
			await benPage
				.getByRole('dialog')
				.getByRole('button', { name: 'Add to recipe photos' })
				.click();
			await expect(
				benPage.getByRole('dialog').getByRole('button', { name: 'In the recipe photos' })
			).toBeVisible();
			await benPage.keyboard.press('Escape');
			await expect(circle).toBeVisible();
			await expect(
				benPage.getByRole('button', { name: 'Tasty', exact: true, pressed: true })
			).toBeVisible();
		} finally {
			await benContext.close();
		}

		// A member on a locked recipe may look, not add.
		await setRecipesLockedByDefault(page, true);
		const locked = await createRecipe(page, { ...loadFixture(1), title: `Locked ${token}` });
		await addDiaryPhoto(page, locked.id, 'Nice', tinyPng());
		const memberContext = await browser.newContext();
		try {
			const member = await memberContext.newPage();
			await login(member, ben);
			await expect(member).toHaveURL('/');
			await member.goto(`/recipes/${locked.slug}`);
			await member.getByRole('tab', { name: /^Comments/ }).click();
			await expect(diary(member).getByText('Nice')).toBeVisible();
			await diary(member).getByRole('button', { name: 'Open photo 1 of 1' }).click();
			await expect(member.getByRole('dialog')).toBeVisible();
			await expect(
				member.getByRole('dialog').getByRole('button', { name: 'Add to recipe photos' })
			).toHaveCount(0);
		} finally {
			await memberContext.close();
		}
	} finally {
		await setRecipesLockedByDefault(page, false);
	}
});

// A comment notification, its toast and the comment mail all link to
// ?tab=comments: the page opens there scrolled to the comments, not on the
// cover with the comments far below the ingredients on a phone.
test('?tab=comments scrolls to the comments', async ({ page }) => {
	await login(page);
	const recipe = await createRecipe(page, loadFixture(0));
	await page.goto(`/recipes/${recipe.slug}?tab=comments`);
	await expect(diary(page)).toBeVisible();
	const tab = diaryTab(page);
	await expect(tab).toBeInViewport();
	// The tab sits near the top, under the fixed header, or the page could
	// not scroll further (a short recipe ends below it).
	await expect
		.poll(() =>
			tab.evaluate((el) => {
				const atEnd = scrollY + innerHeight >= document.documentElement.scrollHeight - 1;
				return el.getBoundingClientRect().top < 200 || (atEnd && scrollY > 0);
			})
		)
		.toBe(true);
	// Without the parameter the page opens at its top.
	await page.goto(`/recipes/${recipe.slug}`);
	await expect.poll(() => page.evaluate(() => scrollY)).toBe(0);
});

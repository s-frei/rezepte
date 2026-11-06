import { expect, test, type Locator } from '@playwright/test';
import {
	addDiaryEntry,
	createRecipe,
	createUser,
	loadFixture,
	login,
	openFromPeopleList,
	search,
	setOwnAvatarViaApi,
	tinyPng,
	uniqueToken
} from './helpers';

test('shows the picture in the profile and falls back when it fails', async ({ page }) => {
	const name = `pic${uniqueToken()}`.toLowerCase();
	await login(page);
	await createUser(page, { username: name, role: 'user' });
	await login(page, name);
	const avatarId = await setOwnAvatarViaApi(page, tinyPng([30, 140, 90]));

	await page.goto('/settings');
	const img = page.locator(`img[src$="/${avatarId}.jpg"]:visible`).first();
	// `:visible` because the desktop top bar's menu trigger is hidden on a phone.
	await expect(img).toBeVisible();

	// A picture that cannot load must not leave a broken-image icon.
	await page.route(`**/avatars/**`, (route) => route.fulfill({ status: 404 }));
	await page.reload();
	await expect(page.locator(`img[src$="/${avatarId}.jpg"]:visible`)).toHaveCount(0);
	await expect(
		page.getByText(name.charAt(0).toUpperCase(), { exact: true }).locator('visible=true').first()
	).toBeVisible();
});

test('crops and sets the own picture, then removes it', async ({ page }) => {
	const name = `crop${uniqueToken()}`.toLowerCase();
	await login(page);
	await createUser(page, { username: name, role: 'user' });
	await login(page, name);
	await page.goto('/settings');

	// Without a photo the camera on the circle opens the file picker at once.
	const chooser = page.waitForEvent('filechooser');
	await page.getByRole('button', { name: 'Add photo' }).click();
	await (
		await chooser
	).setFiles({
		name: 'me.png',
		mimeType: 'image/png',
		buffer: tinyPng([200, 60, 60])
	});
	const dialog = page.getByRole('dialog', { name: 'Crop the photo' });
	await expect(dialog).toBeVisible();
	// tinyPng is 64 px, the smallest the service takes, so the test keeps
	// zoom 1: any zoom would cut the square below 64 px and earn a 422.
	await expect(dialog.getByRole('application', { name: 'Photo section' })).toBeVisible();
	const put = page.waitForRequest(
		(r) => r.method() === 'PUT' && r.url().includes('/auth/me/avatar?crop=')
	);
	await dialog.getByRole('button', { name: 'Save' }).click();
	await put;
	await expect(page.getByText('Photo saved')).toBeVisible();
	// By its own file: the color picker's holders line shows other people's
	// faces, and some of them have pictures too.
	const me = (await (await page.request.get('/api/v1/auth/me')).json()) as { id: string };
	const own = page.locator(`img[src^="/avatars/${me.id}/"]:visible`);
	await expect(own.first()).toBeVisible();
	// The badge became a menu trigger; focus stayed on it.
	await expect(page.getByRole('button', { name: 'Change photo' })).toBeFocused();

	// With one, the camera opens a menu: change or remove.
	await page.getByRole('button', { name: 'Change photo' }).click();
	await expect(page.getByRole('menuitem', { name: 'Change photo' })).toBeVisible();
	await page.getByRole('menuitem', { name: 'Remove photo' }).click();
	await expect(page.getByText('Photo removed')).toBeVisible();
	await expect(own).toHaveCount(0);
	await expect(page.getByRole('button', { name: 'Add photo' })).toBeFocused();
});

test('a second file is cropped by its own size', async ({ page }) => {
	const name = `swap${uniqueToken()}`.toLowerCase();
	await login(page);
	await createUser(page, { username: name, role: 'user' });
	await login(page, name);
	await page.goto('/settings');
	const input = page.locator('input[type=file]');
	const dialog = page.getByRole('dialog', { name: 'Crop the photo' });

	// A tall file centers with a vertical offset that a wide file cannot take.
	await input.setInputFiles({
		name: 'tall.png',
		mimeType: 'image/png',
		buffer: tinyPng([200, 60, 60], 64, 256)
	});
	await expect(dialog.getByRole('button', { name: 'Save' })).toBeEnabled();
	await dialog.getByRole('button', { name: 'Cancel' }).click();
	await expect(dialog).toBeHidden();

	await input.setInputFiles({
		name: 'wide.png',
		mimeType: 'image/png',
		buffer: tinyPng([60, 60, 200], 256, 64)
	});
	const put = page.waitForRequest(
		(r) => r.method() === 'PUT' && r.url().includes('/auth/me/avatar?crop=')
	);
	await dialog.getByRole('button', { name: 'Save' }).click();
	const [x, y, size] = new URL((await put).url()).searchParams.get('crop')!.split(',').map(Number);
	// The wide file's centered square: a quarter of its width, all of its height.
	expect(y).toBe(0);
	expect(size).toBe(1);
	expect(x).toBeCloseTo(0.375, 3);
	await expect(page.getByText('Photo saved')).toBeVisible();
});

test('a file over 10 MB is refused before the dialog', async ({ page }) => {
	await login(page);
	await page.goto('/settings');
	await page.locator('input[type=file]').setInputFiles({
		name: 'big.png',
		mimeType: 'image/png',
		buffer: Buffer.alloc(10 * 1024 * 1024 + 1)
	});
	await expect(page.getByText('That photo is larger than 10 MB.')).toBeVisible();
	await expect(page.getByRole('dialog', { name: 'Crop the photo' })).toHaveCount(0);
});

test("the owner sets another account's picture", async ({ page }) => {
	const name = `other${uniqueToken()}`.toLowerCase();
	await login(page);
	await createUser(page, { username: name, role: 'user' });
	await page.goto('/settings/users');
	const row = page
		.getByRole('region', { name: 'People' })
		.getByRole('listitem')
		.filter({ has: page.getByText(name, { exact: true }) });
	await openFromPeopleList(row);
	// The camera's menu hands its pick to this input; setting it directly
	// skips the system file dialog.
	const head = page.getByRole('region', { name, exact: true });
	await expect(head.getByRole('button', { name: 'Add photo' })).toBeVisible();
	await head.locator('input[type=file]').setInputFiles({
		name: 'x.png',
		mimeType: 'image/png',
		buffer: tinyPng([60, 60, 200])
	});
	await page
		.getByRole('dialog', { name: 'Crop the photo' })
		.getByRole('button', { name: 'Save' })
		.click();
	await expect(page.getByText('Photo saved')).toBeVisible();
});

test('opens the author card from a recipe card and filters by author', async ({ page }) => {
	const token = uniqueToken();
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Card ${token}` });
	await page.goto('/');
	await search(page, token);
	const card = page.getByRole('article', { name: recipe.title });
	await card.getByRole('button', { name: /Added by/ }).click();
	const link = page.getByRole('link', { name: /Recipes by/ });
	await expect(link).toBeVisible();
	await link.click();
	await expect(page).toHaveURL(/\?author=admin/);
	// The card stays in the filtered list; its popover must not stay open over it.
	await expect(card).toBeVisible();
	await expect(link).toBeHidden();
});

test('a picture fills its whole circle', async ({ page }) => {
	const name = `fill${uniqueToken()}`.toLowerCase();
	await login(page);
	await createUser(page, { username: name, role: 'user' });
	await login(page, name);
	const avatarId = await setOwnAvatarViaApi(page, tinyPng([30, 140, 90]));

	await page.goto('/settings');
	const img = page.locator(`img[src$="/${avatarId}.jpg"]:visible`).first();
	await expect(img).toBeVisible();
	const pic = await img.boundingBox();
	const circle = await img.locator('..').boundingBox();
	expect(pic && circle).toBeTruthy();
	expect(Math.abs(pic!.width - circle!.width)).toBeLessThan(0.5);
	expect(Math.abs(pic!.height - circle!.height)).toBeLessThan(0.5);
	expect(Math.abs(pic!.y + pic!.height - (circle!.y + circle!.height))).toBeLessThan(0.5);
});

/** The outline around the circle holding `img`, and the color it should have. */
function ring(img: Locator, tone: string) {
	return img.locator('..').evaluate((el, tone) => {
		const probe = document.createElement('span');
		probe.style.color = `var(${tone})`;
		document.body.append(probe);
		const want = getComputedStyle(probe).color;
		probe.remove();
		const cs = getComputedStyle(el);
		return {
			style: cs.outlineStyle,
			width: cs.outlineWidth,
			offset: cs.outlineOffset,
			matches: cs.outlineColor === want
		};
	}, tone);
}

test('a photo is ringed in its color from md up, a small mark is not', async ({ page }) => {
	const name = `ring${uniqueToken()}`.toLowerCase();
	await login(page);
	await createUser(page, { username: name, role: 'user' });
	await login(page, name);
	const avatarId = await setOwnAvatarViaApi(page, tinyPng([30, 140, 90]));
	const me = (await (await page.request.get('/api/v1/auth/me')).json()) as { color: string };
	const tone = `--color-user-${me.color}-foreground`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Ring ${uniqueToken()}` });
	await addDiaryEntry(page, recipe.id, 'Ringed or not?');
	const pic = (scope: Locator) => scope.locator(`img[src$="/${avatarId}.jpg"]:visible`).first();

	// The profile head: past its 4px cut-out, a 2.5px ring - which Chrome
	// floors to whole device pixels, 2px on this 1x screen.
	await page.goto('/settings');
	await expect(pic(page.locator('main'))).toBeVisible();
	const head = await ring(pic(page.locator('main')), tone);
	expect(head).toMatchObject({ style: 'solid', offset: '4px', matches: true });
	expect(parseFloat(head.width)).toBeGreaterThanOrEqual(2);

	// A diary entry's 28px mark stays plain.
	await page.goto(`/recipes/${recipe.slug}`);
	await page.getByRole('tab', { name: /^Comments/ }).click();
	const small = pic(page.getByRole('tabpanel', { name: /^Comments/ }));
	await expect(small).toBeVisible();
	expect((await ring(small, tone)).style).toBe('none');
});

test('a mark in a line of text grows with the text', async ({ page }) => {
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Grow ${uniqueToken()}` });
	await page.goto(`/recipes/${recipe.slug}`);
	// The colophon's "Added by" face: an `xs` mark beside the name.
	const mark = page
		.locator('button')
		.filter({ hasText: 'Admin' })
		.locator('span[aria-hidden="true"]')
		.first();
	await expect(mark).toBeVisible();
	expect((await mark.boundingBox())!.width).toBeCloseTo(20, 0);
	// Android's 200% font size grows px text: the circle follows its letter.
	await page.addStyleTag({ content: ':root { --text-micro: 24px }' });
	await expect.poll(async () => Math.round((await mark.boundingBox())!.width)).toBe(40);
});

import { expect, test } from '@playwright/test';
import {
	createRecipe,
	createUser,
	loadFixture,
	login,
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

	await page.locator('input[type=file]').setInputFiles({
		name: 'me.png',
		mimeType: 'image/png',
		buffer: tinyPng([200, 60, 60])
	});
	const dialog = page.getByRole('dialog', { name: 'Crop your photo' });
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
	await expect(page.locator('img[src^="/avatars/"]:visible').first()).toBeVisible();

	await page.getByRole('button', { name: 'Remove photo' }).click();
	await expect(page.getByText('Photo removed')).toBeVisible();
	await expect(page.locator('img[src^="/avatars/"]:visible')).toHaveCount(0);
});

test("the owner sets another account's picture", async ({ page, isMobile }) => {
	const name = `other${uniqueToken()}`.toLowerCase();
	await login(page);
	await createUser(page, { username: name, role: 'user' });
	await page.goto('/settings/users');
	// A wide screen has the button in the row, a phone behind the row's sheet.
	const row = page
		.getByRole('region', { name: 'People' })
		.getByRole('listitem')
		.filter({ has: page.getByText(name, { exact: true }) });
	if (isMobile) {
		await row.getByRole('button', { name: `Manage ${name}` }).click();
		await page
			.getByRole('dialog', { name, exact: true })
			.getByRole('button', { name: 'Edit profile' })
			.click();
	} else {
		await row.getByRole('button', { name: 'Edit profile' }).click();
	}
	const edit = page.getByRole('dialog');
	await edit.locator('input[type=file]').setInputFiles({
		name: 'x.png',
		mimeType: 'image/png',
		buffer: tinyPng([60, 60, 200])
	});
	await page
		.getByRole('dialog', { name: 'Crop your photo' })
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

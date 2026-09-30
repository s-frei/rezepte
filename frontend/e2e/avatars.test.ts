import { expect, test } from '@playwright/test';
import { createUser, login, setOwnAvatarViaApi, tinyPng, uniqueToken } from './helpers';

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

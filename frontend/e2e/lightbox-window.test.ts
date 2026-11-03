import { expect, test } from '@playwright/test';
import { createRecipe, loadFixture, login, tinyPng, uniqueToken, uploadImage } from './helpers';

// Headless Chromium hides its scrollbars; this test needs the classic ones a
// desktop with a mouse shows, so it has a file of its own (launch options
// cannot be set per test).
test.use({ launchOptions: { ignoreDefaultArgs: ['--hide-scrollbars'] } });

// Desktop only (the mobile project ignores this file): the desktop viewport
// is the one that keeps a scrollbar gutter.
test('the lightbox covers the whole window and the page stays put', async ({ page }) => {
	await login(page);
	await page.setViewportSize({ width: 1280, height: 900 });
	const recipe = await createRecipe(page, { ...loadFixture(1), title: `Cover ${uniqueToken()}` });
	await uploadImage(page, recipe.id, tinyPng([200, 120, 40]));

	await page.goto(`/recipes/${recipe.slug}`);
	// A page that scrolls, so the scrollbar's room matters.
	await page.addStyleTag({ content: 'body { min-height: 3000px }' });
	const mainLeft = () => page.locator('main').evaluate((el) => el.getBoundingClientRect().left);
	const before = await mainLeft();

	await page.getByRole('button', { name: 'Open photo 1 of 1' }).click();
	const dialog = page.getByRole('dialog');
	await expect(dialog).toBeVisible();
	expect(await mainLeft()).toBe(before);
	const box = await dialog.evaluate((el) => {
		const { left, width } = el.getBoundingClientRect();
		return { left, width };
	});
	expect(box).toEqual({ left: 0, width: 1280 });

	await page.keyboard.press('Escape');
	await expect(dialog).toBeHidden();
	expect(await mainLeft()).toBe(before);
});

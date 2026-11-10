import { expect, test, type Locator } from '@playwright/test';
import { login } from './helpers';

// A broken image still has a box, so each check also asks whether it decoded.
async function loaded(img: Locator): Promise<boolean> {
	return img.evaluate(
		(el) => (el as HTMLImageElement).complete && (el as HTMLImageElement).naturalWidth > 0
	);
}

test('the login page shows the wordmark, named Rezepte, over the kitchen scene', async ({
	page
}) => {
	await page.context().clearCookies();
	await page.goto('/login');
	await expect(page.getByRole('img', { name: 'Rezepte' })).toBeVisible();
	const scene = page.locator('[data-auth-plate] img');
	await expect(scene).toBeVisible();
	await expect(scene).toHaveAttribute('src', /rezepte-kitchen/);
	expect(await loaded(scene)).toBe(true);
});

// The mascot in the kitchen painting: from the ribbon to the waving hand, as
// fractions of the painting's width and height.
const FIGURE = { left: 0.218, right: 0.538, top: 0.112, bottom: 0.845 };

test('the plate shows the mascot whole, 8px clear of every edge', async ({ page }, testInfo) => {
	test.skip(!testInfo.project.name.startsWith('mobile'), 'the plate is the phone layout');
	await page.context().clearCookies();
	await page.goto('/login');
	const scene = page.locator('[data-auth-plate] img');
	for (const size of [
		{ width: 360, height: 780 },
		{ width: 320, height: 568 },
		{ width: 1000, height: 800 },
		{ width: 1023, height: 800 }
	]) {
		await page.setViewportSize(size);
		await expect(scene).toBeVisible();
		await expect.poll(() => loaded(scene)).toBe(true);
		const img = (await scene.boundingBox())!;
		const view = (await page.locator('[data-auth-view]').boundingBox())!;
		const figure = {
			left: img.x + img.width * FIGURE.left,
			right: img.x + img.width * FIGURE.right,
			top: img.y + img.height * FIGURE.top,
			bottom: img.y + img.height * FIGURE.bottom
		};
		const at = `at ${size.width}x${size.height}`;
		expect(figure.left - view.x, at).toBeGreaterThanOrEqual(8);
		expect(view.x + view.width - figure.right, at).toBeGreaterThanOrEqual(8);
		expect(figure.top - view.y, at).toBeGreaterThanOrEqual(8);
		expect(view.y + view.height - figure.bottom, at).toBeGreaterThanOrEqual(8);
	}
});

// Under the sheet the painting goes on to the plate's foot, so the sheet lies
// over it, and the mascot stands 8px clear of the plate and of the sheet's top.
test('the sheet lies over the painting, the mascot clear of it', async ({ page }, testInfo) => {
	test.skip(!testInfo.project.name.startsWith('mobile'), 'the sheet is the phone layout');
	await page.context().clearCookies();
	await page.goto('/login');
	await page.locator('[data-auth-cover]').getByRole('button', { name: 'Sign in' }).click();
	const sheet = page.getByRole('dialog', { name: 'Sign in' });
	await expect(sheet).toBeVisible();
	const scene = page.locator('[data-auth-plate] img');
	for (const size of [
		{ width: 360, height: 780 },
		{ width: 390, height: 844 },
		{ width: 412, height: 915 },
		{ width: 1000, height: 800 }
	]) {
		await page.setViewportSize(size);
		await expect(scene).toBeVisible();
		await expect.poll(() => loaded(scene)).toBe(true);
		// Every move has ended.
		await expect.poll(() => page.evaluate(() => document.getAnimations().length)).toBe(0);
		const img = (await scene.boundingBox())!;
		const plate = (await page.locator('[data-auth-plate]').boundingBox())!;
		const top = (await sheet.boundingBox())!.y;
		const figure = {
			left: img.x + img.width * FIGURE.left,
			right: img.x + img.width * FIGURE.right,
			top: img.y + img.height * FIGURE.top,
			bottom: img.y + img.height * FIGURE.bottom
		};
		const at = `at ${size.width}x${size.height}`;
		expect(top, at).toBeLessThan(plate.y + plate.height);
		expect(img.y + img.height, at).toBeCloseTo(plate.y + plate.height, 0);
		expect(figure.left - plate.x, at).toBeGreaterThanOrEqual(8);
		expect(plate.x + plate.width - figure.right, at).toBeGreaterThanOrEqual(8);
		expect(figure.top - plate.y, at).toBeGreaterThanOrEqual(8);
		expect(top - figure.bottom, at).toBeGreaterThanOrEqual(8);
	}
});

test('the top bar links home with the mascot and the wordmark', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'the top bar is desktop-only');
	await login(page);
	const home = page.getByRole('banner').getByRole('link', { name: 'Rezepte', exact: true });
	await expect(home).toBeVisible();
	const logo = home.locator('img');
	await expect(logo).toHaveAttribute('src', /rezepte-mascot/);
	// The link names the logo, so the picture itself stays silent.
	await expect(logo).toHaveAttribute('alt', '');
	expect(await loaded(logo)).toBe(true);
	await expect(home.locator('span[aria-hidden="true"]')).toHaveCSS('mask-image', /url\(/);
});

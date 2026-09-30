import { readFile } from 'node:fs/promises';
import {
	expect,
	test,
	type Download,
	type Locator,
	type Page,
	type TestInfo
} from '@playwright/test';
import {
	createRecipe,
	loadFixture,
	login,
	openRecipeMenu,
	tinyPng,
	uniqueToken,
	uploadImage
} from './helpers';

// The browsers under test may or may not offer a share sheet for files, and
// a headless one cannot show it. Each test pins the behavior it needs:
// `noShareSheet` takes file sharing away, so "Card" falls back to a download.
async function noShareSheet(page: Page) {
	await page.addInitScript(() => {
		Object.defineProperty(Navigator.prototype, 'canShare', { value: undefined });
		Object.defineProperty(Navigator.prototype, 'share', { value: undefined });
	});
}

const isPhone = (testInfo: TestInfo) => Boolean(testInfo.project.use.isMobile);

/** Opens the pass-on sheet the way each viewport offers it first. */
async function openPassOn(page: Page, testInfo: TestInfo): Promise<Locator> {
	if (isPhone(testInfo)) {
		await page.getByRole('button', { name: 'Share' }).click();
	} else {
		await page.getByRole('button', { name: 'Pass on' }).click();
	}
	const sheet = page.getByRole('dialog', { name: 'Pass on recipe' });
	await expect(sheet.getByRole('img', { name: /^Recipe card:/ })).toBeVisible({ timeout: 20_000 });
	return sheet;
}

async function pngSize(download: Download): Promise<{ width: number; height: number }> {
	const bytes = await readFile(await download.path());
	expect(bytes.subarray(1, 4).toString('ascii')).toBe('PNG');
	return { width: bytes.readUInt32BE(16), height: bytes.readUInt32BE(20) };
}

async function saveCard(page: Page, sheet: Locator, testInfo: TestInfo): Promise<Download> {
	const downloading = page.waitForEvent('download');
	await sheet.getByRole('button', { name: isPhone(testInfo) ? 'Card' : 'Save image' }).click();
	return downloading;
}

test('passes a recipe on as a card PNG', async ({ page }, testInfo) => {
	await noShareSheet(page);
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Card ${uniqueToken()}` });
	await uploadImage(page, recipe.id, tinyPng([60, 140, 70], 320, 240));
	await page.goto(`/recipes/${recipe.slug}`);

	const sheet = await openPassOn(page, testInfo);
	const download = await saveCard(page, sheet, testInfo);
	expect(download.suggestedFilename()).toBe(`${recipe.slug}.png`);
	const { width, height } = await pngSize(download);
	expect(width).toBeGreaterThanOrEqual(420);
	expect(width).toBeLessThanOrEqual(1260);
	expect(height).toBeGreaterThan(width);
	expect(width * height).toBeLessThanOrEqual(16_000_000);
});

test('renders a card without a photo, at the chosen servings', async ({ page }, testInfo) => {
	await noShareSheet(page);
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Plain ${uniqueToken()}` });
	const chosen = recipe.servings + 1;
	// The stepper persists its choice per recipe; the card reads it on opening.
	await page.addInitScript(
		([id, servings]) => localStorage.setItem(`rezepte-servings:${id}`, String(servings)),
		[recipe.id, chosen] as const
	);
	await page.goto(`/recipes/${recipe.slug}`);

	await openPassOn(page, testInfo);
	const card = page.locator('[inert]');
	await expect(card.getByText(`${chosen} servings`)).toHaveCount(1);
	await expect(card.locator('img[src*="/images/"]')).toHaveCount(0);
});

test('cancelling the share sheet keeps it open', async ({ page }, testInfo) => {
	test.skip(!isPhone(testInfo), 'the phone sheet is where "Card" opens the share sheet');
	await page.addInitScript(() => {
		Object.defineProperty(Navigator.prototype, 'canShare', { value: () => true });
		Object.defineProperty(Navigator.prototype, 'share', {
			value: () => Promise.reject(new DOMException('cancelled', 'AbortError'))
		});
	});
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Abort ${uniqueToken()}` });
	await page.goto(`/recipes/${recipe.slug}`);

	const sheet = await openPassOn(page, testInfo);
	let downloaded = false;
	page.on('download', () => (downloaded = true));
	await sheet.getByRole('button', { name: 'Card' }).click();
	await expect(sheet).toBeVisible();
	expect(downloaded).toBe(false);
});

test('the menu offers passing on too', async ({ page }) => {
	await noShareSheet(page);
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Menu ${uniqueToken()}` });
	await page.goto(`/recipes/${recipe.slug}`);
	await openRecipeMenu(page);
	await page.getByRole('menuitem', { name: 'Pass on' }).click();
	await expect(page.getByRole('dialog', { name: 'Pass on recipe' })).toBeVisible();
});

test('falls back to the placeholder when the cover does not load', async ({ page }, testInfo) => {
	await noShareSheet(page);
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Gone ${uniqueToken()}` });
	await uploadImage(page, recipe.id, tinyPng([140, 60, 70], 320, 240));
	await page.route('**/images/**/detail.jpg', (route) => route.fulfill({ status: 404 }));
	await page.goto(`/recipes/${recipe.slug}`);

	await openPassOn(page, testInfo);
	await expect(page.locator('[inert] img[src*="/images/"]')).toHaveCount(0);
});

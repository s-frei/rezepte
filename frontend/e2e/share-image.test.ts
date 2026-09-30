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
// `noShareSheet` takes file sharing away, so "Image" falls back to a download.
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
	await expect(sheet.getByRole('img', { name: /^Recipe image:/ })).toBeVisible({ timeout: 20_000 });
	return sheet;
}

async function pngSize(download: Download): Promise<{ width: number; height: number }> {
	const bytes = await readFile(await download.path());
	expect(bytes.subarray(1, 4).toString('ascii')).toBe('PNG');
	return { width: bytes.readUInt32BE(16), height: bytes.readUInt32BE(20) };
}

async function saveCard(page: Page, sheet: Locator, testInfo: TestInfo): Promise<Download> {
	const downloading = page.waitForEvent('download');
	await sheet.getByRole('button', { name: isPhone(testInfo) ? 'Image' : 'Save image' }).click();
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
	test.skip(!isPhone(testInfo), 'the phone sheet is where "Image" opens the share sheet');
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
	await expect(sheet.getByText('Press and hold the image to share it.')).toHaveCount(0);
	let downloaded = false;
	page.on('download', () => (downloaded = true));
	await sheet.getByRole('button', { name: 'Image' }).click();
	// Longer than the sheet's 320ms send animation and a download's start.
	await page.waitForTimeout(800);
	await expect(sheet).toBeVisible();
	await expect(page.getByText('Image saved')).toHaveCount(0);
	expect(downloaded).toBe(false);
});

test('a second tap while the share sheet is up sends nothing more', async ({ page }, testInfo) => {
	test.skip(!isPhone(testInfo), 'the phone sheet is where "Image" opens the share sheet');
	// The first share stays open, as the system sheet does; a second call
	// meanwhile is refused the way browsers refuse it.
	await page.addInitScript(() => {
		let pending = false;
		Object.defineProperty(Navigator.prototype, 'canShare', { value: () => true });
		Object.defineProperty(Navigator.prototype, 'share', {
			value: () => {
				if (pending) return Promise.reject(new DOMException('busy', 'InvalidStateError'));
				pending = true;
				return new Promise(() => {});
			}
		});
	});
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Twice ${uniqueToken()}` });
	await page.goto(`/recipes/${recipe.slug}`);

	const sheet = await openPassOn(page, testInfo);
	let downloaded = false;
	page.on('download', () => (downloaded = true));
	const card = sheet.getByRole('button', { name: 'Image' });
	await card.click();
	await card.click({ force: true });
	await page.waitForTimeout(800);
	expect(downloaded).toBe(false);
	await expect(sheet).toBeVisible();
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

// Plain http on a home network is not a secure context: browsers then offer
// neither the share sheet nor the clipboard API. `insecure` takes both away
// and records what the fallback copy put on the clipboard.
async function insecure(page: Page, { copyWorks = true } = {}) {
	await page.addInitScript((works) => {
		Object.defineProperty(Navigator.prototype, 'canShare', { value: undefined });
		Object.defineProperty(Navigator.prototype, 'share', { value: undefined });
		Object.defineProperty(Navigator.prototype, 'clipboard', { get: () => undefined });
		Document.prototype.execCommand = function (command: string) {
			if (command !== 'copy' || !works) return false;
			const field = document.activeElement as HTMLTextAreaElement | null;
			(window as unknown as { copied?: string }).copied = field?.value;
			return true;
		};
	}, copyWorks);
}

test('without https, Link still copies the link', async ({ page }, testInfo) => {
	test.skip(!isPhone(testInfo), 'the phone sheet has Link');
	await insecure(page);
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Http ${uniqueToken()}` });
	await page.goto(`/recipes/${recipe.slug}`);

	const sheet = await openPassOn(page, testInfo);
	await sheet.getByRole('button', { name: 'Link' }).click();
	await expect(page.getByText('Link copied')).toBeVisible();
	const copied = await page.evaluate(() => (window as unknown as { copied?: string }).copied);
	expect(copied).toContain(`/recipes/${recipe.slug}`);
});

test('without https, the sheet says how to share the image', async ({ page }, testInfo) => {
	test.skip(!isPhone(testInfo), 'the hint is for phones');
	await insecure(page);
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Hint ${uniqueToken()}` });
	await page.goto(`/recipes/${recipe.slug}`);

	const sheet = await openPassOn(page, testInfo);
	await expect(sheet.getByText('Press and hold the image to share it.')).toBeVisible();
});

test('when nothing can copy, the text is shown instead', async ({ page }, testInfo) => {
	test.skip(!isPhone(testInfo), 'the phone sheet has Link');
	await insecure(page, { copyWorks: false });
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Show ${uniqueToken()}` });
	await page.goto(`/recipes/${recipe.slug}`);

	const sheet = await openPassOn(page, testInfo);
	await sheet.getByRole('button', { name: 'Link' }).click();
	await expect(page.getByText('Copying is not available here')).toBeVisible();
	await expect(page.getByText(new RegExp(`/recipes/${recipe.slug}`))).toBeVisible();
});

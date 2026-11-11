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
	addDiaryEntry,
	createRecipe,
	loadFixture,
	login,
	openLinkTab,
	openRecipeMenu,
	tinyPng,
	uniqueToken,
	uploadImage
} from './helpers';

// The browsers under test may or may not offer a share sheet for files, and
// a headless one cannot show it. Each test pins the behavior it needs:
// `noShareSheet` takes the share sheet away, so the phone's primary turns into
// what the browser can do instead (Copy image, Copy link) and Save is the download.
async function noShareSheet(page: Page) {
	await page.addInitScript(() => {
		Object.defineProperty(Navigator.prototype, 'canShare', { value: undefined });
		Object.defineProperty(Navigator.prototype, 'share', { value: undefined });
	});
}

const isPhone = (testInfo: TestInfo) => Boolean(testInfo.project.use.isMobile);

/** Opens the pass-on sheet the way each viewport offers it first. */
async function openSheet(page: Page, testInfo: TestInfo): Promise<Locator> {
	await page.getByRole('button', { name: isPhone(testInfo) ? 'Share' : 'Pass on' }).click();
	const sheet = page.getByRole('dialog', { name: 'Pass on recipe' });
	await expect(sheet).toBeVisible();
	return sheet;
}

/** Opens the sheet on its Image tab, the first one, once the image is drawn. */
async function openPassOn(page: Page, testInfo: TestInfo): Promise<Locator> {
	const sheet = await openSheet(page, testInfo);
	await expect(sheet.getByRole('img', { name: /^Recipe image:/ })).toBeVisible({ timeout: 20_000 });
	return sheet;
}

async function pngSize(download: Download): Promise<{ width: number; height: number }> {
	const bytes = await readFile(await download.path());
	expect(bytes.subarray(1, 4).toString('ascii')).toBe('PNG');
	return { width: bytes.readUInt32BE(16), height: bytes.readUInt32BE(20) };
}

async function saveCard(page: Page, sheet: Locator): Promise<Download> {
	const downloading = page.waitForEvent('download');
	await sheet.getByRole('button', { name: 'Save image', exact: true }).click();
	return downloading;
}

test('passes a recipe on as a card PNG', async ({ page }, testInfo) => {
	await noShareSheet(page);
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Card ${uniqueToken()}` });
	await uploadImage(page, recipe.id, tinyPng([60, 140, 70], 320, 240));
	await page.goto(`/recipes/${recipe.slug}`);

	const sheet = await openPassOn(page, testInfo);
	const download = await saveCard(page, sheet);
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
	await addDiaryEntry(page, recipe.id, 'Only for the household.');
	const chosen = recipe.servings + 1;
	// The stepper persists its choice per recipe; the card reads it on opening.
	await page.addInitScript(
		([id, servings]) => localStorage.setItem(`rezepte-servings:${id}`, String(servings)),
		[recipe.id, chosen] as const
	);
	await page.goto(`/recipes/${recipe.slug}`);

	await openPassOn(page, testInfo);
	const card = page.locator('[data-testid="share-card"]');
	await expect(card.getByText(`${chosen} servings`)).toHaveCount(1);
	await expect(card.locator('img[src*="/images/"]')).toHaveCount(0);
	// The comments stay in the household.
	await expect(card.getByRole('tab', { name: /^Comments/ })).toHaveCount(0);
	await expect(card.getByText('Only for the household.')).toHaveCount(0);
});

test('canceling the share sheet keeps it open', async ({ page }, testInfo) => {
	test.skip(!isPhone(testInfo), 'the phone sheet is where "Share image" opens the share sheet');
	await page.addInitScript(() => {
		Object.defineProperty(Navigator.prototype, 'canShare', { value: () => true });
		Object.defineProperty(Navigator.prototype, 'share', {
			value: () => Promise.reject(new DOMException('canceled', 'AbortError'))
		});
	});
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Abort ${uniqueToken()}` });
	await page.goto(`/recipes/${recipe.slug}`);

	const sheet = await openPassOn(page, testInfo);
	await expect(sheet.getByText('Press and hold the image to share it.')).toHaveCount(0);
	let downloaded = false;
	page.on('download', () => (downloaded = true));
	await sheet.getByRole('button', { name: 'Share image', exact: true }).click();
	// Longer than the sheet's 320ms send animation and a download's start.
	await page.waitForTimeout(800);
	await expect(sheet).toBeVisible();
	await expect(page.getByText('Image saved')).toHaveCount(0);
	expect(downloaded).toBe(false);
});

test('a refused share sheet saves the image instead', async ({ page }, testInfo) => {
	test.skip(!isPhone(testInfo), 'the phone sheet is where "Share image" opens the share sheet');
	await page.addInitScript(() => {
		Object.defineProperty(Navigator.prototype, 'canShare', { value: () => true });
		Object.defineProperty(Navigator.prototype, 'share', {
			value: () => Promise.reject(new DOMException('refused', 'NotAllowedError'))
		});
	});
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Refused ${uniqueToken()}` });
	await page.goto(`/recipes/${recipe.slug}`);

	const sheet = await openPassOn(page, testInfo);
	const downloading = page.waitForEvent('download');
	await sheet.getByRole('button', { name: 'Share image', exact: true }).click();
	expect((await downloading).suggestedFilename()).toBe(`${recipe.slug}.png`);
	await expect(sheet).toBeHidden();
});

test('a second tap while the share sheet is up sends nothing more', async ({ page }, testInfo) => {
	test.skip(!isPhone(testInfo), 'the phone sheet is where "Share image" opens the share sheet');
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
	const card = sheet.getByRole('button', { name: 'Share image', exact: true });
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
	await expect(page.locator('[data-testid="share-card"] img[src*="/images/"]')).toHaveCount(0);
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
	test.skip(!isPhone(testInfo), 'the phone sheet turns Share link into Copy link');
	await insecure(page);
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Http ${uniqueToken()}` });
	await page.goto(`/recipes/${recipe.slug}`);

	const sheet = await openLinkTab(page);
	await sheet.getByRole('button', { name: 'Copy link', exact: true }).click();
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

test('a narrow desktop window gets no press-and-hold hint', async ({ page }, testInfo) => {
	test.skip(isPhone(testInfo), 'a mouse, not a finger');
	await insecure(page);
	await page.setViewportSize({ width: 360, height: 780 });
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Mouse ${uniqueToken()}` });
	await page.goto(`/recipes/${recipe.slug}`);

	const sheet = await openSheet(page, { project: { use: { isMobile: true } } } as TestInfo);
	await expect(sheet.getByRole('img', { name: /^Recipe image:/ })).toBeVisible({ timeout: 20_000 });
	await expect(sheet.getByText('Press and hold the image to share it.')).toHaveCount(0);
	await expect(sheet.getByRole('button', { name: 'Save image', exact: true })).toBeVisible();
});

test('when nothing can copy, the text is shown instead', async ({ page }, testInfo) => {
	test.skip(!isPhone(testInfo), 'the phone sheet turns Share link into Copy link');
	await insecure(page, { copyWorks: false });
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Show ${uniqueToken()}` });
	await page.goto(`/recipes/${recipe.slug}`);

	const sheet = await openLinkTab(page);
	await sheet.getByRole('button', { name: 'Copy link', exact: true }).click();
	const toast = page.locator('[data-sonner-toast]');
	await expect(toast.getByText('Copying is not available here')).toBeVisible();
	await expect(toast.getByText(new RegExp(`/recipes/${recipe.slug}`))).toBeVisible();
});

test('says so when the image cannot be copied', async ({ page }, testInfo) => {
	test.skip(isPhone(testInfo), 'Copy image is on the desktop dialog');
	await noShareSheet(page);
	await page.addInitScript(() => {
		Object.defineProperty(Navigator.prototype, 'clipboard', {
			get: () => ({
				write: () => Promise.reject(new DOMException('denied', 'NotAllowedError')),
				writeText: () => Promise.resolve()
			})
		});
	});
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Copy ${uniqueToken()}` });
	await page.goto(`/recipes/${recipe.slug}`);

	const sheet = await openPassOn(page, testInfo);
	await sheet.getByRole('button', { name: 'Copy image' }).click();
	await expect(page.getByText('The image could not be copied.')).toBeVisible();
});

// From 768px each tab draws its own switch: an arrow key that switches the
// tab keeps focus on the switch, not on Close, where Enter would close it.
test('the tab switch keeps focus across tabs', async ({ page }, testInfo) => {
	test.skip(isPhone(testInfo), 'the dialog is from 768px');
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Keys ${uniqueToken()}` });
	await page.goto(`/recipes/${recipe.slug}`);

	const sheet = await openSheet(page, testInfo);
	await sheet.getByRole('radio', { name: 'Image', exact: true }).focus();
	await page.keyboard.press('ArrowRight');
	await expect(sheet.getByRole('radio', { name: 'Link', exact: true })).toBeFocused();
	await page.keyboard.press('ArrowLeft');
	await expect(sheet.getByRole('radio', { name: 'Image', exact: true })).toBeFocused();
});

test('the sheet remembers the last tab', async ({ page }, testInfo) => {
	await noShareSheet(page);
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Tab ${uniqueToken()}` });
	await page.goto(`/recipes/${recipe.slug}`);

	let sheet = await openSheet(page, testInfo);
	await expect(sheet.getByRole('radio', { name: 'Image', exact: true })).toBeChecked();
	await sheet.getByRole('radio', { name: 'Link', exact: true }).click();
	await page.keyboard.press('Escape');
	await expect(sheet).toBeHidden();
	// A fresh page: what remembers the tab is the device, not the open page.
	await page.reload();

	sheet = await openSheet(page, testInfo);
	await expect(sheet.getByRole('radio', { name: 'Link', exact: true })).toBeChecked();
	// The ticket, not its chip: with public sharing on (instance-wide, other
	// files flip it) a phone's fit drops the chip first.
	await expect(sheet.locator('[data-ticket]')).toBeVisible();
});

test('the Link tab without public sharing shows the household ticket', async ({ page }) => {
	// Answered, not set: public sharing is instance-wide and public-shares.test.ts flips it.
	await page.route('**/api/v1/settings', async (route) => {
		const response = await route.fetch();
		await route.fulfill({ response, json: { ...(await response.json()), publicShares: false } });
	});
	await noShareSheet(page);
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `House ${uniqueToken()}` });
	await page.goto(`/recipes/${recipe.slug}`);

	const sheet = await openLinkTab(page);
	await expect(sheet.getByText('Accessible after signing in')).toBeVisible();
	// The ticket shows host and path: no scheme, no share link query.
	const address = `[^/:\\s]+(:\\d+)?/recipes/${recipe.slug}`;
	await expect(sheet.getByText(new RegExp(`^${address}$`))).toBeVisible();
	// It is hidden from screen readers, so a line of its own reads both out.
	await expect(sheet.getByText(new RegExp(`^${recipe.title}, ${address}$`))).toBeAttached();
	await expect(sheet.getByRole('radio', { name: /For everyone/ })).toHaveCount(0);
	await expect(sheet.getByRole('button', { name: 'Copy link', exact: true })).toBeVisible();
	await expect(sheet.getByRole('button', { name: 'Share link', exact: true })).toHaveCount(0);
});

// A button says what it does: where the browser has no share sheet, the
// phone's primary copies instead of promising to share and then downloading.
test('without a share sheet the phone offers copying, not sharing', async ({ page }, testInfo) => {
	test.skip(!isPhone(testInfo), 'the desktop dialog never promised to share');
	await noShareSheet(page);
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Honest ${uniqueToken()}` });
	await page.goto(`/recipes/${recipe.slug}`);

	const sheet = await openPassOn(page, testInfo);
	await expect(sheet.getByRole('button', { name: 'Share image', exact: true })).toHaveCount(0);
	await expect(sheet.getByRole('button', { name: 'Copy image', exact: true })).toBeVisible();
	await expect(sheet.getByRole('button', { name: 'Save image', exact: true })).toBeVisible();
	await sheet.getByRole('radio', { name: 'Link', exact: true }).click();
	await expect(sheet.getByRole('button', { name: 'Share link', exact: true })).toHaveCount(0);
	await expect(sheet.getByRole('button', { name: 'Copy link', exact: true })).toHaveCount(1);
});

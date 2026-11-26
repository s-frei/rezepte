import { readFile } from 'node:fs/promises';
import { expect, test, type Locator, type Page, type TestInfo } from '@playwright/test';
import type { RecipeInput } from '../src/lib/api/recipes';
import {
	createRecipe,
	login,
	observePrints,
	openRecipeMenu,
	prints,
	setCover,
	tinyPng,
	uniqueToken,
	uploadImage
} from './helpers';

// The pass-on sheet's PDF tab: page 1 of the PDF as a preview, the options,
// and Download / Print / Share.

const isPhone = (testInfo: TestInfo) => Boolean(testInfo.project.use.isMobile);

function bread(title: string): RecipeInput {
	return {
		title,
		description: 'A plain bread for the PDF specs.',
		servings: 4,
		prepMinutes: 20,
		cookMinutes: 40,
		sourceUrl: null,
		sourceName: null,
		tags: [],
		ingredientGroups: [
			{
				name: null,
				ingredients: [{ quantity: 400, unit: 'g', name: 'flour', note: null }]
			}
		],
		steps: [{ text: 'Bake until golden.', references: [], times: [] }]
	};
}

/** Takes the share sheet away, so the phone's primary is the download. */
async function noShareSheet(page: Page) {
	await page.addInitScript(() => {
		Object.defineProperty(Navigator.prototype, 'canShare', { value: undefined });
		Object.defineProperty(Navigator.prototype, 'share', { value: undefined });
	});
}

/** Answers the settings with `pdf` changed, for this page only. */
async function pdfSettings(page: Page, pdf: { available: boolean }) {
	await page.route('**/api/v1/settings', async (route) => {
		const response = await route.fetch();
		await route.fulfill({ response, json: { ...(await response.json()), pdf } });
	});
}

/** Counts the PDF requests this page sends, and keeps the previews' bodies. */
function countPdfRequests(page: Page) {
	const seen = { count: 0, previews: [] as { options: Record<string, unknown> }[] };
	page.on('request', (request) => {
		const path = new URL(request.url()).pathname;
		if (path === '/api/v1/pdf/preview') seen.previews.push(request.postDataJSON());
		else if (path === '/api/v1/pdf') seen.count++;
	});
	return seen;
}

async function openRecipe(page: Page, photo = false) {
	await login(page);
	const created = await createRecipe(page, bread(`PDF bread ${uniqueToken()}`));
	if (photo) {
		const image = await uploadImage(page, created.id, tinyPng());
		await setCover(page, created.id, image.id);
	}
	await page.goto(`/recipes/${created.slug}`);
	await expect(page.getByRole('heading', { level: 1, name: created.title })).toBeVisible();
	return created;
}

async function openPdfTab(page: Page, testInfo: TestInfo): Promise<Locator> {
	await page
		.getByRole('button', { name: isPhone(testInfo) ? 'Share' : 'Pass on', exact: true })
		.click();
	const sheet = page.getByRole('dialog', { name: 'Pass on recipe' });
	await sheet.getByRole('radio', { name: 'PDF', exact: true }).click();
	return sheet;
}

const preview = (sheet: Locator) => sheet.locator('[data-preview] img');

test('the PDF tab previews page 1 and follows the options', async ({ page }, testInfo) => {
	const requests = countPdfRequests(page);
	await openRecipe(page, true);
	const sheet = await openPdfTab(page, testInfo);

	await expect(preview(sheet)).toHaveAttribute('src', /^blob:/);
	await expect.poll(() => requests.previews.length).toBe(1);
	expect(requests.previews[0].options).toMatchObject({
		bundle: 'single',
		paper: 'a4',
		photo: true
	});
	const first = await preview(sheet).getAttribute('src');
	await sheet.getByRole('radio', { name: isPhone(testInfo) ? 'A5' : 'Card · A5' }).click();
	await expect(preview(sheet)).not.toHaveAttribute('src', first!);
	expect(requests.previews.at(-1)?.options).toMatchObject({ paper: 'a5' });
	// The image is the PNG the server rendered.
	const width = await preview(sheet).evaluate((img: HTMLImageElement) => img.naturalWidth);
	expect(width).toBe(559);
	await sheet.getByRole('switch', { name: 'Footer' }).click();
	await expect.poll(() => requests.previews.at(-1)?.options.footer).toBe(false);

	// Remembered on this device, and the tab with them.
	await page.reload();
	const again = await openPdfTab(page, testInfo);
	await expect(preview(again)).toBeVisible();
	expect(requests.previews.at(-1)?.options).toMatchObject({ paper: 'a5', footer: false });
	await expect(again.getByRole('switch', { name: 'Footer' })).toHaveAttribute(
		'aria-checked',
		'false'
	);
	expect(requests.count).toBe(0);
});

test('Download saves the PDF at the page servings', async ({ page }, testInfo) => {
	await noShareSheet(page);
	const created = await openRecipe(page);
	await page.getByRole('button', { name: 'Fewer servings' }).click();
	await page.getByRole('button', { name: 'Fewer servings' }).click();
	const sheet = await openPdfTab(page, testInfo);

	const requested = page.waitForRequest('**/api/v1/pdf');
	const downloading = page.waitForEvent('download');
	await sheet.getByRole('button', { name: 'Download PDF' }).click();
	const body = (await requested).postDataJSON();
	expect(JSON.stringify(body.recipes[0].ingredientGroups)).toContain('"200"');
	expect(body.options).toMatchObject({ bundle: 'single', paper: 'a4' });

	const download = await downloading;
	// Playwright's WebKit saves `a.download` with spaces as underscores.
	expect(download.suggestedFilename().replaceAll('_', ' ')).toBe(`${created.title}.pdf`);
	const bytes = await readFile(await download.path());
	expect(bytes.subarray(0, 4).toString('ascii')).toBe('%PDF');
});

test('Print downloads the PDF where the browser has no PDF viewer', async ({ page }, testInfo) => {
	// The default headless shell has none, as Android Chrome (`printRoute`).
	test.skip(testInfo.project.name === 'webkit', 'WebKit has a PDF viewer');
	await observePrints(page);
	const created = await openRecipe(page);
	expect(await page.evaluate(() => navigator.pdfViewerEnabled)).toBe(false);
	const downloading = page.waitForEvent('download');
	await openRecipeMenu(page);
	await page.getByRole('menuitem', { name: /^Print/ }).click();
	const download = await downloading;
	expect(download.suggestedFilename()).toBe(`${created.title}.pdf`);
	const bytes = await readFile(await download.path());
	expect(bytes.subarray(0, 4).toString('ascii')).toBe('%PDF');
	expect(await prints(page)).toEqual([]);
});

type ShareWindow = { shared: { name: string; type: string }[][]; tries: number };

/**
 * A share sheet for files whose `navigator.share` fails with `refusals`, in
 * order, before it shares; every shared file list lands in `window.shared`.
 */
async function shareSheet(page: Page, refusals: string[]) {
	await page.addInitScript((refusals) => {
		const w = window as unknown as ShareWindow;
		w.shared = [];
		w.tries = 0;
		Object.defineProperty(Navigator.prototype, 'canShare', { value: () => true });
		Object.defineProperty(Navigator.prototype, 'share', {
			value: (data: ShareData) => {
				const refusal = refusals[w.tries++];
				if (refusal) return Promise.reject(new DOMException(refusal, refusal));
				w.shared.push((data.files ?? []).map((file) => ({ name: file.name, type: file.type })));
				return Promise.resolve();
			}
		});
	}, refusals);
}

/** Answers `/api/v1/pdf` with a tiny PDF, once `release` resolves. */
async function stubPdf(page: Page, release: Promise<void> = Promise.resolve()) {
	await page.route('**/api/v1/pdf', async (route) => {
		await release;
		await route.fulfill({
			body: '%PDF-1.7\n%%EOF\n',
			headers: {
				'Content-Type': 'application/pdf',
				'Content-Disposition': 'attachment; filename="bread.pdf"'
			}
		});
	});
}

const shared = (page: Page) => page.evaluate(() => (window as unknown as ShareWindow).shared);

test('a refused share keeps the PDF for a second tap', async ({ page }, testInfo) => {
	test.skip(!isPhone(testInfo), 'the phone sheet leads with Share PDF');
	// Safari refuses once the tap's activation was spent on the request.
	await shareSheet(page, ['NotAllowedError', 'NotAllowedError']);
	const requests = countPdfRequests(page);
	await stubPdf(page);
	await openRecipe(page);
	const sheet = await openPdfTab(page, testInfo);

	await sheet.getByRole('button', { name: 'Share PDF' }).click();
	await expect(sheet.getByRole('button', { name: 'Share now' })).toBeVisible();
	// A changed option makes the held PDF stale: it is dropped.
	await sheet.getByRole('switch', { name: 'Photo' }).click();
	await expect(sheet.getByRole('button', { name: 'Share PDF' })).toBeVisible();

	await sheet.getByRole('button', { name: 'Share PDF' }).click();
	const now = sheet.getByRole('button', { name: 'Share now' });
	await expect(now).toBeVisible();
	await now.click();
	await expect.poll(() => shared(page)).toEqual([[{ name: 'bread.pdf', type: 'application/pdf' }]]);
	await expect(sheet.getByRole('button', { name: 'Share PDF' })).toBeVisible();
	expect(requests.count).toBe(2);
});

test('a canceled share changes nothing, and a tap while rendering sends nothing more', async ({
	page
}, testInfo) => {
	test.skip(!isPhone(testInfo), 'the phone sheet leads with Share PDF');
	await shareSheet(page, ['AbortError']);
	const requests = countPdfRequests(page);
	let release = () => {};
	await stubPdf(page, new Promise<void>((resolve) => (release = resolve)));
	await openRecipe(page);
	const sheet = await openPdfTab(page, testInfo);

	await sheet.getByRole('button', { name: 'Share PDF' }).click();
	const busy = sheet.getByRole('button', { name: 'Creating PDF' });
	await expect(busy).toBeVisible();
	await busy.click();
	release();
	await expect(sheet.getByRole('button', { name: 'Share PDF' })).toBeVisible();
	await expect.poll(() => page.evaluate(() => (window as unknown as ShareWindow).tries)).toBe(1);
	expect(await shared(page)).toEqual([]);
	await expect(page.getByText('could not be shared')).toHaveCount(0);
	await expect(sheet).toBeVisible();
	expect(requests.count).toBe(1);
});

test('an option changed while rendering drops that PDF', async ({ page }, testInfo) => {
	test.skip(!isPhone(testInfo), 'the phone sheet leads with Share PDF');
	await shareSheet(page, []);
	let release = () => {};
	await stubPdf(page, new Promise<void>((resolve) => (release = resolve)));
	await openRecipe(page);
	const sheet = await openPdfTab(page, testInfo);

	await sheet.getByRole('button', { name: 'Share PDF' }).click();
	await expect(sheet.getByRole('button', { name: 'Creating PDF' })).toBeVisible();
	await sheet.getByRole('switch', { name: 'Photo' }).click();
	release();
	await expect(sheet.getByRole('button', { name: 'Share PDF' })).toBeVisible();
	expect(await page.evaluate(() => (window as unknown as ShareWindow).tries)).toBe(0);
	expect(await shared(page)).toEqual([]);
});

test('a failing share says so', async ({ page }, testInfo) => {
	test.skip(!isPhone(testInfo), 'the phone sheet leads with Share PDF');
	await shareSheet(page, ['DataError']);
	await stubPdf(page);
	await openRecipe(page);
	const sheet = await openPdfTab(page, testInfo);
	await sheet.getByRole('button', { name: 'Share PDF' }).click();
	await expect(page.getByText('The PDF could not be shared.')).toBeVisible();
	await expect(sheet.getByRole('button', { name: 'Share PDF' })).toBeVisible();
});

test('without PDFs on this instance there is no PDF tab and no Print', async ({
	page
}, testInfo) => {
	// The tab this device used last was PDF: the sheet opens on Image instead.
	await page.addInitScript(() => localStorage.setItem('rezepte-share-tab', 'pdf'));
	await pdfSettings(page, { available: false });
	const requests = countPdfRequests(page);
	await openRecipe(page);
	await openRecipeMenu(page);
	await expect(page.getByRole('menuitem', { name: /^Print/ })).toHaveCount(0);
	await page.keyboard.press('Escape');
	await page
		.getByRole('button', { name: isPhone(testInfo) ? 'Share' : 'Pass on', exact: true })
		.click();
	const sheet = page.getByRole('dialog', { name: 'Pass on recipe' });
	await expect(sheet.getByRole('radio', { name: 'Image', exact: true })).toBeChecked();
	await expect(sheet.getByRole('radio', { name: 'PDF', exact: true })).toHaveCount(0);
	expect(requests).toMatchObject({ count: 0, previews: [] });
});

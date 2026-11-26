import { expect, test, type Page, type TestInfo } from '@playwright/test';
import type { RecipeInput } from '../src/lib/api/recipes';
import { createRecipe, login, observePrints, openRecipeMenu, prints, uniqueToken } from './helpers';

// Print on the recipe page is the server's PDF: the menu's Print and Ctrl+P
// render it with the remembered options at the page's servings and print it.
// Desktop Chromium prints it in a frame (`printRoute`). The `mobile` project
// is phone emulation on the full Chromium, which has a PDF viewer, so it takes
// the tab route of phones with one (iOS, Firefox on Android); Android Chrome
// has none and downloads, which share-pdf.test.ts checks on the headless
// shell. The PDF's own layout is tested in service/internal/pdf.

const isPhone = (testInfo: TestInfo) => Boolean(testInfo.project.use.isMobile);

// The full Chromium: the default headless shell has no PDF viewer.
test.use({ channel: 'chromium' });

function bread(title: string): RecipeInput {
	return {
		title,
		description: 'A plain bread for the print specs.',
		servings: 4,
		prepMinutes: 20,
		cookMinutes: 40,
		sourceUrl: null,
		sourceName: 'Grandma',
		tags: [],
		ingredientGroups: [
			{ name: null, ingredients: [{ quantity: 400, unit: 'g', name: 'flour', note: null }] }
		],
		steps: [{ text: 'Bake until golden.', references: [], times: [] }]
	};
}

/** Every PDF request body this page sends; previews go to their own route. */
function pdfRequests(page: Page) {
	const bodies: { options: Record<string, unknown>; recipes: unknown[] }[] = [];
	page.on('request', (request) => {
		if (new URL(request.url()).pathname === '/api/v1/pdf') bodies.push(request.postDataJSON());
	});
	return bodies;
}

async function openRecipe(page: Page) {
	await login(page);
	const created = await createRecipe(page, bread(`Print bread ${uniqueToken()}`));
	await page.goto(`/recipes/${created.slug}`);
	await expect(page.getByRole('heading', { level: 1, name: created.title })).toBeVisible();
	return created;
}

/** The PDF is printed: in a frame on desktop, in a new tab under phone emulation. */
async function expectPrinted(page: Page, testInfo: TestInfo, print: () => Promise<void>) {
	if (isPhone(testInfo)) {
		const popup = page.waitForEvent('popup');
		await print();
		await expect.poll(async () => (await popup).url()).toMatch(/^blob:/);
	} else {
		await print();
		await expect.poll(() => prints(page)).toEqual(['frame']);
	}
	expect(await prints(page)).not.toContain('page');
}

test('the menu prints the PDF with the remembered options at the page servings', async ({
	page
}, testInfo) => {
	await observePrints(page);
	await page.addInitScript(() =>
		localStorage.setItem('rezepte-print', JSON.stringify({ format: 'a5', photo: false }))
	);
	const bodies = pdfRequests(page);
	await openRecipe(page);
	await page.getByRole('button', { name: 'Fewer servings' }).click();
	await page.getByRole('button', { name: 'Fewer servings' }).click();
	await openRecipeMenu(page);
	const item = page.getByRole('menuitem', { name: /^Print/ });
	// The shortcut hint shows on desktop only, as the platform spells it
	// (the ⌘ is an icon, so the text is just "P" there).
	const apple = await page.evaluate(() => /Mac|iPhone|iPad/.test(navigator.platform));
	if (isPhone(testInfo)) await expect(item.locator('kbd')).toBeHidden();
	else await expect(item.locator('kbd')).toHaveText(apple ? 'P' : 'Ctrl+P');
	expect((await item.innerText()).split('\n')[0].trim()).toBe('Print');

	await expectPrinted(page, testInfo, () => item.click());
	expect(bodies).toHaveLength(1);
	expect(bodies[0].options).toMatchObject({
		bundle: 'single',
		paper: 'a5',
		photo: false,
		footer: true
	});
	expect(JSON.stringify(bodies[0].recipes)).toContain('"display":"200"');
});

test('Ctrl+P prints the PDF, not the page', async ({ page }, testInfo) => {
	test.skip(isPhone(testInfo), 'a keyboard shortcut');
	await observePrints(page);
	const bodies = pdfRequests(page);
	await openRecipe(page);
	const apple = await page.evaluate(() => /Mac/.test(navigator.platform));
	await expectPrinted(page, testInfo, () => page.keyboard.press(apple ? 'Meta+P' : 'Control+P'));
	expect(bodies).toHaveLength(1);
});

test("the PDF tab's Print prints the PDF", async ({ page }, testInfo) => {
	await observePrints(page);
	const bodies = pdfRequests(page);
	await openRecipe(page);
	await page
		.getByRole('button', { name: isPhone(testInfo) ? 'Share' : 'Pass on', exact: true })
		.click();
	const sheet = page.getByRole('dialog', { name: 'Pass on recipe' });
	await sheet.getByRole('radio', { name: 'PDF', exact: true }).click();
	await expectPrinted(page, testInfo, () =>
		sheet.getByRole('button', { name: 'Print', exact: true }).click()
	);
	expect(bodies).toHaveLength(1);
});

test('a failed render says so and prints nothing', async ({ page }, testInfo) => {
	await observePrints(page);
	await page.route('**/api/v1/pdf', (route) =>
		route.fulfill({ status: 500, contentType: 'application/problem+json', body: '{}' })
	);
	await openRecipe(page);
	const popup = isPhone(testInfo) ? page.waitForEvent('popup') : null;
	await openRecipeMenu(page);
	await page.getByRole('menuitem', { name: /^Print/ }).click();
	await expect(page.getByText('The PDF could not be created.')).toBeVisible();
	// The tab opened for it closes again.
	if (popup) await expect.poll(async () => (await popup).isClosed()).toBe(true);
	expect(await prints(page)).toEqual([]);
});

test('a second print while one renders joins it', async ({ page }, testInfo) => {
	test.skip(isPhone(testInfo), 'a keyboard shortcut');
	await observePrints(page);
	let release = () => {};
	const held = new Promise<void>((resolve) => (release = resolve));
	await page.route('**/api/v1/pdf', async (route) => {
		await held;
		await route.fallback();
	});
	const bodies = pdfRequests(page);
	await openRecipe(page);
	await page.getByRole('button', { name: 'Pass on', exact: true }).click();
	const sheet = page.getByRole('dialog', { name: 'Pass on recipe' });
	await sheet.getByRole('radio', { name: 'PDF', exact: true }).click();
	await sheet.getByRole('button', { name: 'Print', exact: true }).click();
	await expect.poll(() => bodies.length).toBe(1);
	const apple = await page.evaluate(() => /Mac/.test(navigator.platform));
	await page.keyboard.press(apple ? 'Meta+P' : 'Control+P');
	release();
	await expect.poll(() => prints(page)).toEqual(['frame']);
	expect(bodies).toHaveLength(1);
});

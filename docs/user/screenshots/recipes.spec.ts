import { expect, test, type Page, type TestInfo } from '@playwright/test';
import { isMobile, prepare, shot } from './helpers';

const DETAIL = '/recipes/shepherd-s-pie';

test('recipe-detail', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto(DETAIL);
	await expect(page.getByRole('heading', { level: 1, name: "Shepherd's Pie" })).toBeVisible();
	await shot(page, 'recipe-detail');
});

test('lightbox', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto(DETAIL);
	// The demo gives Shepherd's Pie three photos, so the label is
	// "Open photo 1 of 3"; the regex survives any count.
	await page.getByRole('button', { name: /^Open photo 1 of/ }).click();
	await expect(page.getByRole('dialog')).toBeVisible();
	await shot(page, 'lightbox');
});

test('editor', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/recipes/new');
	// Role locators, like frontend/e2e: every editor list item repeats the
	// label of the field inside it, which leaves getByLabel ambiguous.
	await page.getByRole('textbox', { name: 'Title', exact: true }).fill('Onion Tart');
	await shot(page, 'editor');
});

test('editor-ingredients', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto(`${DETAIL}/edit`);
	await page.getByRole('heading', { name: 'Ingredients' }).scrollIntoViewIfNeeded();
	await shot(page, 'editor-ingredients');
});

test('editor-images', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto(`${DETAIL}/edit`);
	await page.getByRole('heading', { name: 'Photos' }).scrollIntoViewIfNeeded();
	await shot(page, 'editor-images');
});

test('editor-editing', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto(`${DETAIL}/edit`);
	// The hint names what Default means once the household setting has
	// loaded; before that it only says that it follows it.
	await expect(page.getByText('right now everyone may edit this recipe')).toBeVisible();
	await page.getByRole('radiogroup', { name: 'Who may edit this recipe' }).scrollIntoViewIfNeeded();
	await shot(page, 'editor-editing');
});

/** Opens the import dialog: the top bar's button, or on a phone "+" and then "Import". */
async function openImport(page: Page, testInfo: TestInfo) {
	await page.goto('/');
	if (isMobile(testInfo)) {
		await page.getByRole('button', { name: 'New', exact: true }).click();
		await page
			.getByRole('dialog')
			.getByRole('button', { name: /^Import/ })
			.click();
	} else {
		await page.getByRole('banner').getByRole('button', { name: 'Import', exact: true }).click();
	}
	await expect(page.getByRole('dialog', { name: 'Import recipe' })).toBeVisible();
}

test('import-dialog', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await openImport(page, testInfo);
	const dialog = page.getByRole('dialog', { name: 'Import recipe' });
	await dialog
		.getByLabel('Link or recipe text')
		.fill('https://www.example.com/recipes/onion-tart');
	await expect(dialog.getByText('to example.com')).toBeVisible();
	await expect(dialog.getByRole('button', { name: 'Import', exact: true })).toBeEnabled();
	await shot(page, 'import-dialog');
});

test('import-draft', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await openImport(page, testInfo);
	const dialog = page.getByRole('dialog', { name: 'Import recipe' });
	await dialog
		.getByLabel('Link or recipe text')
		.fill(
			'Onion Tart\nIngredients\n500 g onions\n200 g flour\nsalt to taste\nMethod\nSoften the onions.\n\nBake for 40 minutes.'
		);
	await dialog.getByRole('button', { name: 'Import', exact: true }).click();
	await expect(page).toHaveURL('/recipes/new');
	await expect(dialog).toHaveCount(0);
	await expect(page.getByText('1 ingredient to check')).toBeVisible();
	if (isMobile(testInfo)) {
		// The hint, not the count: on a phone the rows stack and push it far down.
		await page.getByRole('button', { name: 'Looks right' }).scrollIntoViewIfNeeded();
	} else {
		// Through the rail, so the section it highlights is the one in view.
		const rail = page.getByRole('navigation', { name: 'Sections' });
		await rail.getByRole('button', { name: 'Ingredients' }).click();
		await expect(rail.getByRole('button', { name: 'Ingredients' })).toHaveAttribute(
			'aria-current',
			'true'
		);
		await expect(page.getByRole('heading', { name: /^Ingredients/ })).toBeInViewport();
		// Let the smooth scroll come to rest before the shot.
		await page.waitForFunction(
			() =>
				new Promise((done) => {
					const y = window.scrollY;
					requestAnimationFrame(() => requestAnimationFrame(() => done(window.scrollY === y)));
				})
		);
	}
	await shot(page, 'import-draft');
});

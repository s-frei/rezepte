import { expect, test, type Page } from '@playwright/test';
import { createRecipe, loadFixture, login, uniqueToken } from './helpers';

test.beforeEach(async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
});

// The credit line is the paragraph that starts with the prefix; found by
// its text because it has no role of its own.
const credit = (page: Page) => page.getByRole('main').getByText(/^Adapted from /);

async function open(page: Page, source: { sourceName: string | null; sourceUrl: string | null }) {
	const recipe = await createRecipe(page, {
		...loadFixture(0),
		title: `Source ${uniqueToken()}`,
		...source
	});
	await page.goto(`/recipes/${recipe.slug}`);
	await expect(page.getByRole('heading', { level: 1, name: recipe.title })).toBeVisible();
	return recipe;
}

test('a name alone is credited as text', async ({ page }) => {
	await open(page, { sourceName: 'Aunt Erika', sourceUrl: null });
	await expect(credit(page)).toHaveText('Adapted from Aunt Erika');
	await expect(credit(page).getByRole('link')).toHaveCount(0);
});

test('a link alone is credited by its host', async ({ page }) => {
	await open(page, { sourceName: null, sourceUrl: 'https://www.example.test/pie' });
	const link = credit(page).getByRole('link', { name: 'example.test' });
	await expect(link).toHaveAttribute('href', 'https://www.example.test/pie');
});

test('a name with a link is the link', async ({ page }) => {
	await open(page, { sourceName: 'Aunt Erika', sourceUrl: 'https://www.example.test/pie' });
	const link = credit(page).getByRole('link', { name: 'Aunt Erika' });
	await expect(link).toHaveAttribute('href', 'https://www.example.test/pie');
	await expect(link).toHaveAttribute('target', '_blank');
});

test('no source means no credit line, and no source pill', async ({ page }) => {
	await open(page, { sourceName: null, sourceUrl: null });
	await expect(page.getByText(/Adapted from/)).toHaveCount(0);
	await expect(page.getByRole('link', { name: 'Source' })).toHaveCount(0);
});

test('a long name wraps on a phone instead of widening the page', async ({ page }) => {
	await page.setViewportSize({ width: 360, height: 780 });
	await open(page, { sourceName: 'Kochbuch '.repeat(22).trim(), sourceUrl: null });
	await expect(credit(page)).toBeVisible();
	const width = await page.evaluate(() => document.documentElement.scrollWidth);
	expect(width).toBeLessThanOrEqual(360);
});

test('the editor keeps its prefix in front of what is typed', async ({ page }) => {
	const recipe = await open(page, { sourceName: null, sourceUrl: null });
	await page.goto(`/recipes/${recipe.slug}/edit`);

	const name = page.getByRole('textbox', { name: 'Source', exact: true });
	await expect(name).toHaveValue('');
	const prefix = page.locator('#editor-sourceName-prefix');
	await expect(prefix).toHaveText('Adapted from');
	await expect(name).toHaveAccessibleDescription(/Adapted from/);

	// Typed text starts after the prefix, never under it.
	const padding = await name.evaluate((el) => parseFloat(getComputedStyle(el).paddingLeft));
	const prefixWidth = (await prefix.boundingBox())!.width;
	expect(padding).toBeGreaterThan(prefixWidth);

	await name.fill('Aunt Erika');
	await page
		.getByRole('textbox', { name: 'Link', exact: true })
		.fill('https://www.example.test/pie');
	await page.getByRole('button', { name: 'Save', exact: true }).click();

	await expect(page).toHaveURL(new RegExp(`/recipes/${recipe.slug}$`));
	await expect(credit(page).getByRole('link', { name: 'Aunt Erika' })).toBeVisible();
});

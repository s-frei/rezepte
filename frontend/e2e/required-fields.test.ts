import { expect, test, type Locator, type Page } from '@playwright/test';
import { login, openNewRecipe } from './helpers';

// The star and the stroke are aria-hidden, so they are found by their data
// attributes rather than by role; the field they belong to is found by role,
// which also proves the star stays out of its accessible name.
const markOf = (container: Locator) => container.locator('[data-required-mark]');
const strokeOf = (container: Locator) => container.locator('[data-required-stroke]');
const fieldBox = (page: Page, id: string) =>
	page.locator(`#${id}`).locator('xpath=ancestor::div[2]');

test.beforeEach(async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
});

test('the editor marks what a recipe needs and lets go once it is there', async ({ page }) => {
	await openNewRecipe(page);

	const title = page.getByRole('textbox', { name: 'Title', exact: true });
	await expect(title).toHaveAttribute('aria-required', 'true');
	await expect(page.getByText('Required field')).toBeVisible();

	const titleBox = fieldBox(page, 'editor-title');
	await expect(markOf(titleBox)).toHaveAttribute('data-state', 'empty');
	await expect(strokeOf(titleBox)).toBeVisible();

	// Servings start at 4, so they start satisfied.
	const servingsBox = fieldBox(page, 'editor-servings');
	await expect(markOf(servingsBox)).toHaveAttribute('data-state', 'filled');

	// Optional fields carry nothing.
	await expect(markOf(fieldBox(page, 'editor-sourceUrl'))).toHaveCount(0);

	const ingredients = page.getByRole('heading', { name: 'Ingredients', level: 2 });
	await expect(markOf(ingredients)).toHaveAttribute('data-state', 'empty');

	await title.fill('Kürbissuppe');
	await expect(markOf(titleBox)).toHaveAttribute('data-state', 'filled');
	await expect(strokeOf(titleBox)).toHaveAttribute('data-state', 'filled');

	await page.getByRole('textbox', { name: 'Ingredient', exact: true }).fill('Kürbis');
	await expect(markOf(ingredients)).toHaveAttribute('data-state', 'filled');
});

test('an editor error leaves the field as soon as someone types in it', async ({ page }) => {
	await openNewRecipe(page);

	const servings = page.getByRole('textbox', { name: 'Servings', exact: true });
	await servings.fill('');
	await page.getByRole('button', { name: 'Save', exact: true }).click();

	const title = page.getByRole('textbox', { name: 'Title', exact: true });
	await expect(title).toHaveAttribute('aria-invalid', 'true');
	await expect(servings).toHaveAttribute('aria-invalid', 'true');
	const ingredients = page.getByRole('heading', { name: 'Ingredients', level: 2 });
	await expect(markOf(ingredients)).toHaveAttribute('data-state', 'error');
	// A failed save hands the field to the red border; the stroke steps back.
	await expect(markOf(fieldBox(page, 'editor-title'))).toHaveAttribute('data-state', 'error');
	await expect(strokeOf(fieldBox(page, 'editor-title'))).toHaveAttribute('data-state', 'error');

	await title.fill('K');
	await expect(title).not.toHaveAttribute('aria-invalid', 'true');
	// Only the field typed in lets go; the others keep their error.
	await expect(servings).toHaveAttribute('aria-invalid', 'true');

	// A name in an ingredient row answers the section's "at least one".
	await page.getByRole('textbox', { name: 'Ingredient', exact: true }).fill('Kürbis');
	await expect(markOf(ingredients)).toHaveAttribute('data-state', 'filled');
});

test('adding an account marks username and password, not the display name', async ({ page }) => {
	await page.goto('/settings/users');
	await page.getByRole('button', { name: 'Add account' }).click();
	const dialog = page.getByRole('dialog');

	const username = dialog.getByRole('textbox', { name: 'Username' });
	await expect(username).toHaveAttribute('aria-required', 'true');
	await expect(dialog.getByLabel('Password', { exact: true })).toHaveAttribute(
		'aria-required',
		'true'
	);
	// Typed twice, like every other form that sets a password.
	await expect(dialog.getByLabel('Repeat the new password')).toHaveAttribute(
		'aria-required',
		'true'
	);
	// The star is the convention now; "(optional)" would be the opposite one.
	const displayName = dialog.getByRole('textbox', { name: 'Display name', exact: true });
	await expect(displayName).not.toHaveAttribute('aria-required', 'true');

	await dialog.getByRole('button', { name: 'Add', exact: true }).click();
	await expect(username).toHaveAttribute('aria-invalid', 'true');
	await username.fill('a');
	await expect(username).not.toHaveAttribute('aria-invalid', 'true');
});

test('creating a token marks the name and clears its error on typing', async ({ page }) => {
	await page.goto('/settings/api');
	await page.getByRole('button', { name: 'Create token' }).first().click();
	const dialog = page.getByRole('dialog');

	const name = dialog.getByRole('textbox', { name: 'Name' });
	await expect(name).toHaveAttribute('aria-required', 'true');

	await dialog.getByRole('button', { name: 'Create', exact: true }).click();
	await expect(name).toHaveAttribute('aria-invalid', 'true');
	await name.fill('n');
	await expect(name).not.toHaveAttribute('aria-invalid', 'true');
});

test('the password change marks nothing, every field is needed', async ({ page }) => {
	await page.goto('/settings');
	const current = page.getByLabel('Current password');
	await expect(page.locator('[data-required-mark]')).toHaveCount(0);

	await page.getByRole('button', { name: 'Save password' }).click();
	await expect(current).toHaveAttribute('aria-invalid', 'true');
	await current.fill('x');
	await expect(current).not.toHaveAttribute('aria-invalid', 'true');
});

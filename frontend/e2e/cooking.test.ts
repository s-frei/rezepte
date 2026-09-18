import { expect, test, type Page } from '@playwright/test';
import type { RecipeInput } from '../src/lib/api/recipes';
import { createRecipe, login, uniqueToken } from './helpers';

// Written for `mise run e2e`, but NOT executed in Phase 5: Chromium cannot
// start on the development machine. Role-based locators throughout so the
// suite runs unchanged once a browser is available.

/** A 4-servings recipe with round numbers, so scaled quantities are easy to assert. */
function scalingRecipe(title: string): RecipeInput {
	return {
		title,
		description: '',
		servings: 4,
		prepMinutes: null,
		cookMinutes: null,
		sourceUrl: null,
		tags: [],
		ingredientGroups: [
			{
				name: null,
				ingredients: [
					{ quantity: 200, unit: 'g', name: 'Mehl', note: null },
					{ quantity: 1, unit: 'Prise', name: 'Salz', note: null }
				]
			}
		],
		steps: ['Mehl und Salz mischen.', 'Wasser dazugeben.', 'Kneten und ruhen lassen.']
	};
}

/**
 * Follows whichever cook-mode entry point the viewport shows (top bar link or
 * sticky CTA). Both are in the DOM on every project - the top bar is only
 * `hidden md:flex` - so the visible one has to be picked, not the first one.
 */
async function startCookMode(page: Page) {
	await page
		.getByRole('link', { name: /^Kochmodus( starten)?$/ })
		.filter({ visible: true })
		.first()
		.click();
	await expect(page).toHaveURL(/\/recipes\/[^/]+\/cook$/);
}

test.beforeEach(async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
});

test('scales quantities with the servings stepper and keeps the choice', async ({ page }) => {
	const recipe = await createRecipe(page, scalingRecipe(`Skaliert ${uniqueToken()}`));

	await page.goto(`/recipes/${recipe.slug}`);
	await expect(page.getByText('4 Portionen', { exact: true })).toBeVisible();
	await expect(page.getByText('200 g', { exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Zurücksetzen' })).toHaveCount(0);

	const more = page.getByRole('button', { name: 'Mehr Portionen' });
	for (let i = 0; i < 4; i++) {
		await more.click();
	}
	await expect(page.getByText('8 Portionen', { exact: true })).toBeVisible();
	await expect(page.getByText('400 g', { exact: true })).toBeVisible();
	await expect(page.getByText('2 Prise', { exact: true })).toBeVisible();
	await expect(page.getByText('Für 8 Portionen · ×2')).toBeVisible();

	// The choice lives in localStorage under the recipe id, so a reload keeps it.
	await page.reload();
	await expect(page.getByText('8 Portionen', { exact: true })).toBeVisible();
	await expect(page.getByText('400 g', { exact: true })).toBeVisible();

	await page.getByRole('button', { name: 'Zurücksetzen' }).click();
	await expect(page.getByText('4 Portionen', { exact: true })).toBeVisible();
	await expect(page.getByText('200 g', { exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Zurücksetzen' })).toHaveCount(0);

	// "Zurücksetzen" removes the stored choice, it does not store the baseline:
	// after a reload the page is back to the recipe's own servings.
	await page.reload();
	await expect(page.getByText('4 Portionen', { exact: true })).toBeVisible();
	await expect(page.getByText('200 g', { exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Zurücksetzen' })).toHaveCount(0);
});

test('rounds scaled quantities to kitchen fractions', async ({ page }) => {
	const recipe = await createRecipe(page, {
		...scalingRecipe(`Brüche ${uniqueToken()}`),
		ingredientGroups: [
			{
				name: null,
				ingredients: [
					{ quantity: 1, unit: 'EL', name: 'Öl', note: null },
					{ quantity: 150, unit: 'ml', name: 'Milch', note: null }
				]
			}
		]
	});

	await page.goto(`/recipes/${recipe.slug}`);
	// 4 → 3 servings: 1 EL → ¾ EL, 150 ml → 112.5 → 113 ml.
	await page.getByRole('button', { name: 'Weniger Portionen' }).click();
	await expect(page.getByText('3 Portionen', { exact: true })).toBeVisible();
	await expect(page.getByText('¾ EL', { exact: true })).toBeVisible();
	await expect(page.getByText('113 ml', { exact: true })).toBeVisible();
	await expect(page.getByText('Für 3 Portionen · ×0,75')).toBeVisible();
});

test('opens cook mode from the recipe and walks the steps with the keyboard', async ({ page }) => {
	const recipe = await createRecipe(page, scalingRecipe(`Kochen ${uniqueToken()}`));

	await page.goto(`/recipes/${recipe.slug}`);
	await startCookMode(page);

	// Full screen: the app shell (bottom nav is a <nav>) is not rendered.
	await expect(page.getByRole('navigation')).toHaveCount(0);
	await expect(page.getByText('Schritt 1 von 3')).toBeVisible();
	await expect(page.getByText('Mehl und Salz mischen.')).toBeVisible();
	const progress = page.getByRole('progressbar', { name: 'Fortschritt' });
	await expect(progress).toHaveAttribute('aria-valuenow', '1');
	await expect(progress).toHaveAttribute('aria-valuemax', '3');
	await expect(page.getByRole('button', { name: 'Zurück', exact: true })).toBeDisabled();

	await page.keyboard.press('ArrowRight');
	await expect(page.getByText('Schritt 2 von 3')).toBeVisible();
	await expect(page.getByText('Wasser dazugeben.')).toBeVisible();
	await expect(progress).toHaveAttribute('aria-valuenow', '2');

	await page.keyboard.press('ArrowLeft');
	await expect(page.getByText('Schritt 1 von 3')).toBeVisible();
	await expect(progress).toHaveAttribute('aria-valuenow', '1');

	// The buttons do the same as the keys; the last step swaps "Weiter" for "Fertig".
	await page.getByRole('button', { name: 'Weiter' }).click();
	await page.getByRole('button', { name: 'Weiter' }).click();
	await expect(page.getByText('Schritt 3 von 3')).toBeVisible();
	await expect(page.getByRole('button', { name: 'Weiter' })).toHaveCount(0);
	await expect(page.getByRole('button', { name: 'Fertig' })).toBeVisible();
	// ArrowRight on the last step stays put.
	await page.keyboard.press('ArrowRight');
	await expect(page.getByText('Schritt 3 von 3')).toBeVisible();

	await page.keyboard.press('Escape');
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
});

test('"Fertig" and the back button return to the recipe', async ({ page }) => {
	const recipe = await createRecipe(page, scalingRecipe(`Fertig ${uniqueToken()}`));

	await page.goto(`/recipes/${recipe.slug}/cook`);
	await page.getByRole('button', { name: 'Zurück zum Rezept' }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);

	await page.goto(`/recipes/${recipe.slug}/cook`);
	await page.getByRole('button', { name: 'Weiter' }).click();
	await page.getByRole('button', { name: 'Weiter' }).click();
	await page.getByRole('button', { name: 'Fertig' }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
});

test('toggles the ingredient sheet and shares the servings with the recipe page', async ({
	page
}) => {
	const recipe = await createRecipe(page, scalingRecipe(`Zutaten ${uniqueToken()}`));

	await page.goto(`/recipes/${recipe.slug}/cook`);
	await page.getByRole('button', { name: 'Zutaten · 2 Stück' }).click();
	const sheet = page.getByRole('dialog', { name: 'Zutaten' });
	await expect(sheet).toBeVisible();
	await expect(sheet.getByText('Mehl')).toBeVisible();
	await expect(sheet.getByText('200 g', { exact: true })).toBeVisible();

	await sheet.getByRole('button', { name: 'Mehr Portionen' }).click();
	await expect(sheet.getByText('5 Portionen', { exact: true })).toBeVisible();
	await expect(sheet.getByText('250 g', { exact: true })).toBeVisible();

	// Ticks are the same session state the detail page uses.
	await sheet.getByRole('checkbox', { name: 'Salz' }).click();
	await expect(sheet.getByRole('checkbox', { name: 'Salz' })).toBeChecked();

	// Tapping the backdrop closes the sheet.
	await page.getByRole('button', { name: 'Zutaten schließen' }).click();
	await expect(sheet).toBeHidden();
	await expect(page).toHaveURL(/\/cook$/);

	// Escape closes the sheet first and only then leaves cook mode.
	await page.getByRole('button', { name: 'Zutaten · 2 Stück' }).click();
	await expect(sheet).toBeVisible();
	await page.keyboard.press('Escape');
	await expect(sheet).toBeHidden();
	await expect(page).toHaveURL(/\/cook$/);

	// The arrows are inert while the sheet is open: the cook is reading the list.
	await page.getByRole('button', { name: 'Zutaten · 2 Stück' }).click();
	await expect(sheet).toBeVisible();
	await page.keyboard.press('ArrowRight');
	await expect(page.getByText('Schritt 1 von 3')).toBeVisible();
	await page.keyboard.press('Escape');
	await expect(sheet).toBeHidden();

	await page.goto(`/recipes/${recipe.slug}`);
	await expect(page.getByText('5 Portionen', { exact: true })).toBeVisible();
	await expect(page.getByText('250 g', { exact: true })).toBeVisible();
	await expect(page.getByRole('checkbox', { name: 'Salz' })).toBeChecked();
});

test('swipes between steps on touch', async ({ page, isMobile }) => {
	test.skip(!isMobile, 'pointer swipe is exercised on the mobile project only');
	const recipe = await createRecipe(page, scalingRecipe(`Wischen ${uniqueToken()}`));

	await page.goto(`/recipes/${recipe.slug}/cook`);
	const step = page.getByText('Mehl und Salz mischen.');
	const box = await step.boundingBox();
	if (!box) {
		throw new Error('step text has no bounding box');
	}
	const y = box.y + box.height / 2;
	// 120px leftwards: well past the 50px threshold.
	await page.mouse.move(box.x + 200, y);
	await page.mouse.down();
	await page.mouse.move(box.x + 80, y, { steps: 6 });
	await page.mouse.up();
	await expect(page.getByText('Schritt 2 von 3')).toBeVisible();

	// 20px is below the threshold and must not page.
	await page.mouse.move(box.x + 100, y);
	await page.mouse.down();
	await page.mouse.move(box.x + 120, y, { steps: 2 });
	await page.mouse.up();
	await expect(page.getByText('Schritt 2 von 3')).toBeVisible();
});

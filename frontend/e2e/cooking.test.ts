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
					{ quantity: 200, unit: 'g', name: 'Flour', note: null },
					{ quantity: 1, unit: 'pinch', name: 'Salt', note: null }
				]
			}
		],
		steps: [
			{ text: 'Mix the flour and salt.', references: [] },
			{ text: 'Add the water.', references: [] },
			{ text: 'Knead and leave to rest.', references: [] }
		]
	};
}

/**
 * Follows whichever cook-mode entry point the viewport shows (top bar link or
 * sticky CTA). Both are in the DOM on every project - the top bar is only
 * `hidden md:flex` - so the visible one has to be picked, not the first one.
 */
async function startCookMode(page: Page) {
	await page
		.getByRole('link', { name: /^(Start cook|Cook) mode$/ })
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
	const recipe = await createRecipe(page, scalingRecipe(`Scaled ${uniqueToken()}`));

	await page.goto(`/recipes/${recipe.slug}`);
	await expect(page.getByText('4 servings', { exact: true })).toBeVisible();
	await expect(page.getByText('200 g', { exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Reset' })).toHaveCount(0);

	const more = page.getByRole('button', { name: 'More servings' });
	for (let i = 0; i < 4; i++) {
		await more.click();
	}
	await expect(page.getByText('8 servings', { exact: true })).toBeVisible();
	await expect(page.getByText('400 g', { exact: true })).toBeVisible();
	await expect(page.getByText('2 pinch', { exact: true })).toBeVisible();
	await expect(page.getByText('For 8 servings · ×2')).toBeVisible();

	// The choice lives in localStorage under the recipe id, so a reload keeps it.
	await page.reload();
	await expect(page.getByText('8 servings', { exact: true })).toBeVisible();
	await expect(page.getByText('400 g', { exact: true })).toBeVisible();

	await page.getByRole('button', { name: 'Reset' }).click();
	await expect(page.getByText('4 servings', { exact: true })).toBeVisible();
	await expect(page.getByText('200 g', { exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Reset' })).toHaveCount(0);

	// "Reset" removes the stored choice, it does not store the baseline:
	// after a reload the page is back to the recipe's own servings.
	await page.reload();
	await expect(page.getByText('4 servings', { exact: true })).toBeVisible();
	await expect(page.getByText('200 g', { exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Reset' })).toHaveCount(0);
});

test('rounds scaled quantities to kitchen fractions', async ({ page }) => {
	const recipe = await createRecipe(page, {
		...scalingRecipe(`Fractions ${uniqueToken()}`),
		ingredientGroups: [
			{
				name: null,
				ingredients: [
					{ quantity: 1, unit: 'tbsp', name: 'Oil', note: null },
					{ quantity: 150, unit: 'ml', name: 'Milk', note: null }
				]
			}
		]
	});

	await page.goto(`/recipes/${recipe.slug}`);
	// 4 → 3 servings: 1 tbsp → ¾ tbsp, 150 ml → 112.5 → 113 ml. The factor's
	// decimal separator follows the language; locale.test.ts covers the
	// German comma.
	await page.getByRole('button', { name: 'Fewer servings' }).click();
	await expect(page.getByText('3 servings', { exact: true })).toBeVisible();
	await expect(page.getByText('¾ tbsp', { exact: true })).toBeVisible();
	await expect(page.getByText('113 ml', { exact: true })).toBeVisible();
	await expect(page.getByText('For 3 servings · ×0.75')).toBeVisible();
});

test('opens cook mode from the recipe and walks the steps with the keyboard', async ({ page }) => {
	const recipe = await createRecipe(page, scalingRecipe(`Cooking ${uniqueToken()}`));

	await page.goto(`/recipes/${recipe.slug}`);
	await startCookMode(page);

	// Full screen: the app shell (bottom nav is a <nav>) is not rendered.
	await expect(page.getByRole('navigation')).toHaveCount(0);
	await expect(page.getByText('Step 1 of 3')).toBeVisible();
	await expect(page.getByText('Mix the flour and salt.')).toBeVisible();
	const progress = page.getByRole('progressbar', { name: 'Progress' });
	await expect(progress).toHaveAttribute('aria-valuenow', '1');
	await expect(progress).toHaveAttribute('aria-valuemax', '3');
	await expect(page.getByRole('button', { name: 'Back', exact: true })).toBeDisabled();

	await page.keyboard.press('ArrowRight');
	await expect(page.getByText('Step 2 of 3')).toBeVisible();
	await expect(page.getByText('Add the water.')).toBeVisible();
	await expect(progress).toHaveAttribute('aria-valuenow', '2');

	await page.keyboard.press('ArrowLeft');
	await expect(page.getByText('Step 1 of 3')).toBeVisible();
	await expect(progress).toHaveAttribute('aria-valuenow', '1');

	// The buttons do the same as the keys; the last step swaps "Next" for "Done".
	await page.getByRole('button', { name: 'Next' }).click();
	await page.getByRole('button', { name: 'Next' }).click();
	await expect(page.getByText('Step 3 of 3')).toBeVisible();
	await expect(page.getByRole('button', { name: 'Next' })).toHaveCount(0);
	await expect(page.getByRole('button', { name: 'Done' })).toBeVisible();
	// ArrowRight on the last step stays put.
	await page.keyboard.press('ArrowRight');
	await expect(page.getByText('Step 3 of 3')).toBeVisible();

	await page.keyboard.press('Escape');
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
});

test('"Done" and the back button return to the recipe', async ({ page }) => {
	const recipe = await createRecipe(page, scalingRecipe(`Done ${uniqueToken()}`));

	await page.goto(`/recipes/${recipe.slug}/cook`);
	await page.getByRole('button', { name: 'Back to the recipe' }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);

	await page.goto(`/recipes/${recipe.slug}/cook`);
	await page.getByRole('button', { name: 'Next' }).click();
	await page.getByRole('button', { name: 'Next' }).click();
	await page.getByRole('button', { name: 'Done' }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
});

test('toggles the ingredient sheet and shares the servings with the recipe page', async ({
	page
}) => {
	const recipe = await createRecipe(page, scalingRecipe(`Sheet ${uniqueToken()}`));

	await page.goto(`/recipes/${recipe.slug}/cook`);
	await page.getByRole('button', { name: 'Ingredients · 2' }).click();
	const sheet = page.getByRole('dialog', { name: 'Ingredients' });
	await expect(sheet).toBeVisible();
	await expect(sheet.getByText('Flour')).toBeVisible();
	await expect(sheet.getByText('200 g', { exact: true })).toBeVisible();

	await sheet.getByRole('button', { name: 'More servings' }).click();
	await expect(sheet.getByText('5 servings', { exact: true })).toBeVisible();
	await expect(sheet.getByText('250 g', { exact: true })).toBeVisible();

	// Ticks are the same session state the detail page uses.
	await sheet.getByRole('checkbox', { name: 'Salt' }).click();
	await expect(sheet.getByRole('checkbox', { name: 'Salt' })).toBeChecked();

	// Tapping the backdrop closes the sheet.
	await page.getByRole('button', { name: 'Close the ingredients' }).click();
	await expect(sheet).toBeHidden();
	await expect(page).toHaveURL(/\/cook$/);

	// Escape closes the sheet first and only then leaves cook mode.
	await page.getByRole('button', { name: 'Ingredients · 2' }).click();
	await expect(sheet).toBeVisible();
	await page.keyboard.press('Escape');
	await expect(sheet).toBeHidden();
	await expect(page).toHaveURL(/\/cook$/);

	// The arrows are inert while the sheet is open: the cook is reading the list.
	await page.getByRole('button', { name: 'Ingredients · 2' }).click();
	await expect(sheet).toBeVisible();
	await page.keyboard.press('ArrowRight');
	await expect(page.getByText('Step 1 of 3')).toBeVisible();
	await page.keyboard.press('Escape');
	await expect(sheet).toBeHidden();

	await page.goto(`/recipes/${recipe.slug}`);
	await expect(page.getByText('5 servings', { exact: true })).toBeVisible();
	await expect(page.getByText('250 g', { exact: true })).toBeVisible();
	await expect(page.getByRole('checkbox', { name: 'Salt' })).toBeChecked();
});

test('swipes between steps on touch', async ({ page, isMobile }) => {
	test.skip(!isMobile, 'pointer swipe is exercised on the mobile project only');
	const recipe = await createRecipe(page, scalingRecipe(`Swipe ${uniqueToken()}`));

	await page.goto(`/recipes/${recipe.slug}/cook`);
	const step = page.getByText('Mix the flour and salt.');
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
	await expect(page.getByText('Step 2 of 3')).toBeVisible();

	// 20px is below the threshold and must not page.
	await page.mouse.move(box.x + 100, y);
	await page.mouse.down();
	await page.mouse.move(box.x + 120, y, { steps: 2 });
	await page.mouse.up();
	await expect(page.getByText('Step 2 of 3')).toBeVisible();
});

/** The computed font size of the paragraph that sets the current step. */
async function stepFontSize(page: Page, text: string) {
	return page
		.getByText(text)
		.evaluate((node) => getComputedStyle(node.closest('p') ?? node).fontSize);
}

test('sets the step type size for this visit to cook mode only', async ({ page, isMobile }) => {
	// A phone gets the smaller end of the scale, a large screen the larger;
	// the default in the middle is the same on both.
	const largest = isMobile ? '40px' : '52px';
	const smallest = isMobile ? '19px' : '22px';
	const recipe = await createRecipe(page, scalingRecipe(`Type ${uniqueToken()}`));

	await page.goto(`/recipes/${recipe.slug}`);
	await startCookMode(page);
	expect(await stepFontSize(page, 'Mix the flour and salt.')).toBe('30px');

	await page.getByRole('button', { name: 'Type size' }).click();
	const sizes = page.getByRole('radiogroup', { name: 'Type size' });
	await expect(sizes.getByRole('radio', { name: 'Pica', exact: true })).toBeChecked();
	await sizes.getByRole('radio', { name: 'Double Pica' }).click();
	await expect(sizes.getByRole('radio', { name: 'Double Pica' })).toBeChecked();
	expect(await stepFontSize(page, 'Mix the flour and salt.')).toBe(largest);
	await sizes.getByRole('radio', { name: 'Brevier' }).click();
	expect(await stepFontSize(page, 'Mix the flour and salt.')).toBe(smallest);
	await sizes.getByRole('radio', { name: 'Double Pica' }).click();

	// Escape closes the specimen, not cook mode.
	await page.keyboard.press('Escape');
	await expect(sizes).toBeHidden();
	await expect(page).toHaveURL(/\/cook$/);

	// The size holds across steps.
	await page.keyboard.press('ArrowRight');
	await expect(page.getByText('Step 2 of 3')).toBeVisible();
	expect(await stepFontSize(page, 'Add the water.')).toBe(largest);

	// Leaving cook mode forgets it: the next visit starts at the default again.
	await page.getByRole('button', { name: 'Back to the recipe' }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	await startCookMode(page);
	expect(await stepFontSize(page, 'Mix the flour and salt.')).toBe('30px');
});

test('keeps the start of a long step readable at the largest size', async ({ page }) => {
	const opening = 'Heat the oven to 200 degrees';
	const long = `${opening}, ${'then stir the sauce slowly while it thickens, '.repeat(8)}and serve.`;
	const recipe = await createRecipe(page, {
		...scalingRecipe(`Long ${uniqueToken()}`),
		steps: [{ text: long, references: [] }]
	});

	await page.goto(`/recipes/${recipe.slug}/cook`);
	await page.getByRole('button', { name: 'Type size' }).click();
	await page.getByRole('radio', { name: 'Double Pica' }).click();
	await page.keyboard.press('Escape');

	// A centered, overflowing step would push its first line above the
	// scroll container, out of reach; it must start at the top instead.
	const step = page.getByText(opening, { exact: false });
	const box = await step.boundingBox();
	if (!box) {
		throw new Error('step text has no bounding box');
	}
	const scroller = await step.evaluate(
		(node) => node.closest('section')!.getBoundingClientRect().top
	);
	expect(box.y).toBeGreaterThanOrEqual(scroller);
});

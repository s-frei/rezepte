import { expect, test } from '@playwright/test';
import { createRecipe, login, selectInStep, uniqueToken } from './helpers';

// Also run under WebKit (see playwright.config.ts): Safari focuses no button
// on click, and the "Mark as time" popover has to survive that.

test.beforeEach(async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
});

const RECIPE = {
	description: '',
	servings: 4,
	prepMinutes: null,
	cookMinutes: null,
	sourceUrl: null,
	sourceName: null,
	tags: [],
	ingredientGroups: [
		{ name: null, ingredients: [{ quantity: 400, unit: 'ml', name: 'Saft', note: null }] }
	]
};

test('step times: accepts, dismisses and renders them', async ({ page, browserName }) => {
	const recipe = await createRecipe(page, {
		...RECIPE,
		title: `Zeiten ${uniqueToken()}`,
		steps: [
			{ text: 'Simmer for 15 minutes.', references: [], times: [] },
			{ text: 'Rest for 20 to 25 minutes, then bake 10 min.', references: [], times: [] },
			{ text: 'Let it stand for 2 songs.', references: [], times: [] }
		]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	const step1 = page.getByRole('textbox', { name: 'Step 1', exact: true });
	const step2 = page.getByRole('textbox', { name: 'Step 2', exact: true });
	const fifteen = page.locator('[data-ref-word="15 minutes"]');
	const ten = page.locator('[data-ref-word="10 min"]');
	await expect(fifteen).toHaveClass('time-proposal');
	await expect(page.getByText('Saving accepts 3 suggestions')).toBeVisible();

	// The caret inside a proposed time brings up its card.
	await selectInStep(step1, '5 minutes', 'caret');
	const card = page.getByRole('group', { name: 'Time "15 minutes"' });
	await expect(card).toContainText('15 min');
	await card.getByRole('button', { name: 'Accept' }).click();
	await expect(fifteen).toHaveClass('time-link');
	await expect(card.getByRole('button', { name: 'Remove' })).toBeVisible();

	await selectInStep(step2, '0 min', 'caret');
	await page
		.getByRole('group', { name: 'Time "10 min"' })
		.getByRole('button', { name: 'Dismiss' })
		.click();
	await expect(ten).toHaveCount(0);
	// "20 to 25 minutes" stays open, and the save takes it.
	await expect(page.getByText('Saving accepts 1 suggestion')).toBeVisible();

	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	const time = (phrase: string) => page.locator('[data-time]', { hasText: phrase });
	await expect(time('15 minutes')).toBeVisible();
	await expect(time('20 to 25 minutes')).toBeVisible();
	await expect(time('10 min')).toHaveCount(0);
	await expect(page.getByRole('main')).toContainText('then bake 10 min.');

	await page.goto(`/recipes/${recipe.slug}/cook`);
	await expect(time('15 minutes')).toBeVisible();

	// A selection without a number is not a time.
	await page.goto(`/recipes/${recipe.slug}/edit`);
	const status = (text: string) => page.getByRole('status').filter({ hasText: text });
	await selectInStep(step2, 'bake', 'word');
	await page.getByRole('button', { name: 'Mark as time' }).click();
	await expect(status('Select a duration with a number first')).toHaveCount(1);
	await expect(page.getByRole('dialog')).toHaveCount(0);

	// A duration the detector does not know is marked by hand, in the author's unit.
	const step3 = page.getByRole('textbox', { name: 'Step 3', exact: true });
	await selectInStep(step3, '2 songs', 'word');
	await page.getByRole('button', { name: 'Mark as time' }).click();
	const popover = page.getByRole('dialog', { name: 'Time "2 songs"' });
	await expect(popover.getByRole('radio', { name: 'Minutes' })).toBeChecked();
	await expect(popover.getByRole('radio', { name: 'Minutes' })).toBeFocused();
	// Escape closes it and hands the caret back to the step.
	await page.keyboard.press('Escape');
	await expect(popover).toHaveCount(0);
	await expect(step3).toBeFocused();
	// A press outside closes it and leaves focus where the press put it.
	await page.getByRole('button', { name: 'Mark as time' }).click();
	await expect(popover).toBeVisible();
	await step2.click();
	await expect(popover).toHaveCount(0);
	await expect(step2).toBeFocused();
	// Focused without a click: Chromium places a click's caret late when focus
	// comes from another step, and it would land on the selection set below.
	await step3.focus();
	await selectInStep(step3, '2 songs', 'word');
	await page.getByRole('button', { name: 'Mark as time' }).click();
	await popover.getByRole('radio', { name: 'Hours' }).click();
	await expect(popover).toContainText('2 hr');
	// The clicked radio takes focus, so the arrows go on from it (Safari focuses no button on click).
	if (browserName !== 'webkit') {
		await expect(popover.getByRole('radio', { name: 'Hours' })).toBeFocused();
	}
	await popover.getByRole('button', { name: 'Set' }).click();
	await expect(popover).toHaveCount(0);
	// Set hands the caret back to the step.
	await expect(step3).toBeFocused();
	await expect(page.locator('[data-ref-word="2 songs"]')).toHaveClass('time-link');

	// A marked stretch is not marked a second time (overlaps are covered in
	// step-times.test.ts; the test helper cannot select across the decoration).
	await selectInStep(step3, '2 songs', 'word');
	await page.getByRole('button', { name: 'Mark as time' }).click();
	await expect(status('This part of the step is already marked')).toHaveCount(1);
	await expect(page.getByRole('dialog')).toHaveCount(0);

	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	await expect(time('2 songs')).toBeVisible();
});

test('step times: one press opens the list at phone width', async ({ page, isMobile }) => {
	const recipe = await createRecipe(page, {
		...RECIPE,
		title: `Zeitenliste ${uniqueToken()}`,
		steps: [
			{
				text: 'Knead for 5 minutes, rest for 45 seconds, bake for 20 minutes, cool for 10 minutes and serve in 2 hours.',
				references: [],
				times: [
					{ phrase: '5 minutes', seconds: 300 },
					{ phrase: '45 seconds', seconds: 45 }
				]
			}
		]
	});
	await page.setViewportSize({ width: 360, height: 780 });
	await page.goto(`/recipes/${recipe.slug}/edit`);
	const toggle = page.getByRole('button', { name: '2 times, 3 suggestions' });
	// Focusing the toggle would start editing, and the buttons that brings
	// would push the toggle onto the next line, away from under the finger.
	if (isMobile) await toggle.tap();
	else await toggle.click();
	await expect(toggle).toHaveAttribute('aria-expanded', 'true');
	await expect(page.getByText('45 sec', { exact: true })).toBeVisible();
});

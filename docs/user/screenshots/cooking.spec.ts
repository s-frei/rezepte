import { expect, test } from '@playwright/test';
import { prepare, shot } from './helpers';

// Servings scaling and cooking mode: a "servings" stepper on the detail
// page (buttons labelled "Fewer servings" / "More servings", value
// "<n> servings"), a full-screen /recipes/[slug]/cook and a collapsible
// ingredient sheet ("Ingredients · <n>"). The locators below match that
// markup. Keep the test names - they are the contract with
// <Screenshot name=…/>.

const DETAIL = '/recipes/shepherd-s-pie';

test('servings', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto(DETAIL);
	// The sample is stored for 4 servings, so two steps up land on 6.
	const increase = page.getByRole('button', { name: 'More servings' });
	await increase.click();
	await increase.click();
	// `exact` keeps this off the scaled-quantity hint ("For 6 servings · ×1.5").
	await expect(page.getByText('6 servings', { exact: true })).toBeVisible();
	await shot(page, 'servings');
});

test('cook-mode', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto(`${DETAIL}/cook`);
	// "potatoes" carries an ingredient reference, so its quantity is inserted
	// right after it and splits that text node. Match across the split so this
	// also proves the reference itself renders - the word AND its quantity.
	await expect(page.getByText(/potatoes\s*\(1\s*kg\),\s*then boil/)).toBeVisible();
	await shot(page, 'cook-mode');
});

test('cook-mode-sheet', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto(`${DETAIL}/cook`);
	await page.getByRole('button', { name: /^Ingredients/ }).click();
	await expect(page.getByRole('checkbox', { name: 'minced lamb' })).toBeVisible();
	await shot(page, 'cook-mode-sheet');
});

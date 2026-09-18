import { expect, test } from '@playwright/test';
import { prepare, shot } from './helpers';

// Servings scaling and cooking mode shipped in Phase 5: a "Portionen"
// stepper on the detail page (buttons labelled "Weniger Portionen" /
// "Mehr Portionen", value "<n> Portionen"), a full-screen
// /recipes/[slug]/cook and a collapsible ingredient sheet
// ("Zutaten · <n> Stück"). The locators below match that markup. What has
// not happened is a run: Chromium cannot start on this development
// machine, so these specs have never actually executed. Keep the test
// names - they are the contract with <Screenshot name=…/>.

const DETAIL = '/recipes/koenigsberger-klopse';

test('servings', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto(DETAIL);
	// The samples are stored for 4 portions, so two steps up land on 6.
	const increase = page.getByRole('button', { name: 'Mehr Portionen' });
	await increase.click();
	await increase.click();
	// `exact` keeps this off the scaled-quantity hint ("Für 6 Portionen · ×1,5").
	await expect(page.getByText('6 Portionen', { exact: true })).toBeVisible();
	await shot(page, 'servings');
});

test('cook-mode', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto(`${DETAIL}/cook`);
	await expect(page.getByText('Zwiebel fein würfeln')).toBeVisible();
	await shot(page, 'cook-mode');
});

test('cook-mode-sheet', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto(`${DETAIL}/cook`);
	await page.getByRole('button', { name: /^Zutaten/ }).click();
	await expect(page.getByRole('checkbox', { name: 'Rinderhackfleisch' })).toBeVisible();
	await shot(page, 'cook-mode-sheet');
});

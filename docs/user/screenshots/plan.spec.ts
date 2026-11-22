import { expect, test, type Page } from '@playwright/test';
import { prepare, shot } from './helpers';

// Meal plans as the demo seeds them for its admin: "My plan", the shared
// "Household" with the demo week (one empty day, a leftover and a takeaway,
// a three-recipe day, two recipes on Someday) and the staples salt, oil and
// pepper, Mila's "Holiday cabin", "Office lunch" with nothing this week
// and Jonas's invitation. Nothing here writes, so the order of the shots does
// not matter. Keep the test names - they are the contract with
// <Screenshot name=…/>.

async function openHousehold(page: Page) {
	await page.goto('/plans');
	await page.getByRole('article', { name: 'Household' }).getByRole('link', { name: 'Household' }).click();
	await expect(page.getByRole('heading', { level: 1, name: 'Household' })).toBeVisible();
	await page.waitForLoadState('networkidle');
}

async function planMenu(page: Page, item: string) {
	await page.getByRole('main').getByRole('button', { name: 'More actions' }).click();
	await page.getByRole('menuitem', { name: item }).click();
}

test('plans-overview', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/plans');
	await expect(page.getByRole('article', { name: 'Household' })).toBeVisible();
	await expect(page.getByText(/invites you to the plan/)).toBeVisible();
	await shot(page, 'plans-overview');
});

test('plan-week', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await openHousehold(page);
	await expect(page.getByRole('region', { name: 'Someday' })).toBeVisible();
	await page.mouse.move(0, 0);
	await shot(page, 'plan-week');
});

test('plan-to-list', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await openHousehold(page);
	await page.getByRole('main').getByRole('button', { name: /^Put on the list/ }).click();
	const dialog = page.getByRole('dialog', { name: 'Put on the list' });
	await expect(dialog.getByRole('button', { name: /^Add \d+ recipe/ })).toBeEnabled();
	await expect(dialog).not.toHaveAttribute('aria-busy', 'true');
	await page.mouse.move(0, 0);
	await shot(page, 'plan-to-list');
});

test('plan-staples', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await openHousehold(page);
	await planMenu(page, 'We always have it');
	const dialog = page.getByRole('dialog', { name: 'We always have it' });
	await expect(dialog.getByText('pepper', { exact: true })).toBeVisible();
	await page.mouse.move(0, 0);
	await shot(page, 'plan-staples');
});

test('plan-calendar', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await openHousehold(page);
	await planMenu(page, 'Subscribe in calendar');
	const dialog = page.getByRole('dialog', { name: 'Subscribe in calendar' });
	await expect(dialog.getByRole('button', { name: 'Create link' })).toBeVisible();
	await page.mouse.move(0, 0);
	await shot(page, 'plan-calendar');
});

test('plan-share', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await openHousehold(page);
	const share = page.getByRole('button', { name: 'Share', exact: true }).filter({ visible: true });
	if (await share.count()) await share.first().click();
	else await planMenu(page, 'Share');
	const dialog = page.getByRole('dialog', { name: 'Share “Household”' });
	await expect(dialog.getByText('Waiting for an answer')).toBeVisible();
	await page.mouse.move(0, 0);
	await shot(page, 'plan-share');
});

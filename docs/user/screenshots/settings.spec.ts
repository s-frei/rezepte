import { expect, test } from '@playwright/test';
import { ensureUser, prepare, shot } from './helpers';

test('settings', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings');
	await expect(page.getByRole('heading', { name: 'Change password' })).toBeVisible();
	await shot(page, 'settings');
});

test('api', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings/api');
	// The figures arrive from two fetches after the page renders; waiting for
	// one of them keeps the picture from catching the skeleton.
	await expect(page.getByRole('term').filter({ hasText: 'Endpoints' })).toBeVisible();
	await shot(page, 'api');
});

test('token-mcp', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings/api');
	await page.getByRole('button', { name: 'Create token' }).first().click();
	const dialog = page.getByRole('dialog');
	await dialog.getByLabel('Name').fill('MCP on the laptop');
	await dialog.getByRole('radio', { name: 'Full' }).click();
	await dialog.getByRole('button', { name: 'Create', exact: true }).click();
	const reveal = page.getByRole('dialog');
	await expect(reveal.getByRole('tab', { name: 'Claude Code' })).toBeVisible();
	await shot(page, 'token-mcp');
});

test('users', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	// The demo seeds one account; the second row on the picture is created
	// here, so the four projects can run against the same demo instance.
	await ensureUser(page, 'mila', 'user');
	await page.goto('/settings/users');
	await expect(page.getByRole('listitem').filter({ hasText: 'mila' })).toBeVisible();
	await shot(page, 'users');
});

test('user-create', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings/users');
	await page.getByRole('button', { name: 'Add member' }).click();
	await expect(page.getByRole('dialog')).toBeVisible();
	await shot(page, 'user-create');
});

test('settings-recipe-editing', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings/users');
	// The card reads the setting after the page renders; the switch appears
	// once it has, so waiting for it keeps the skeleton off the picture.
	const toggle = page.getByRole('switch', { name: 'Only authors and admins edit recipes' });
	await expect(toggle).toBeVisible();
	// To the bottom, so the phone's bottom navigation does not cover the card.
	await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
	await shot(page, 'settings-recipe-editing');
});

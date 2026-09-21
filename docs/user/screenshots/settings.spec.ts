import { expect, test } from '@playwright/test';
import { ensureUser, prepare, shot } from './helpers';

test('settings', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings');
	await expect(page.getByRole('heading', { name: 'Passwort ändern' })).toBeVisible();
	await shot(page, 'settings');
});

test('api', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings/api');
	// The figures arrive from two fetches after the page renders; waiting for
	// one of them keeps the picture from catching the skeleton.
	await expect(page.getByRole('term').filter({ hasText: 'Endpunkte' })).toBeVisible();
	await shot(page, 'api');
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
	await page.getByRole('button', { name: 'Benutzer anlegen' }).click();
	await expect(page.getByRole('dialog')).toBeVisible();
	await shot(page, 'user-create');
});

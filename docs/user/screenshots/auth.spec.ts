import { test } from '@playwright/test';
import { prepare, shot } from './helpers';

test('login', async ({ page }, testInfo) => {
	await prepare(page, testInfo);
	await page.goto('/login');
	await shot(page, 'login');
});

test('first-login', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings');
	await page.getByRole('heading', { name: 'Passwort ändern' }).waitFor();
	await shot(page, 'first-login');
});

import { expect, test } from '@playwright/test';
import { login } from './helpers';

test('redirects anonymous users to login', async ({ page }) => {
	await page.goto('/');
	await expect(page).toHaveURL(/\/login\?next=%2F/);
});

test('rejects a wrong password', async ({ page }) => {
	await login(page, 'admin', 'wrong');
	await expect(page.getByRole('alert')).toHaveText('Benutzername oder Passwort ist falsch.');
	await expect(page).toHaveURL(/\/login/);
});

test('logs in, survives reload, logs out', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await expect(page.getByText('Angemeldet als admin')).toBeVisible();
	await page.reload();
	await expect(page.getByText('Angemeldet als admin')).toBeVisible();
	await page.getByRole('button', { name: 'Abmelden' }).click();
	await expect(page).toHaveURL(/\/login/);
});

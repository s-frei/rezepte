import { expect, test } from '@playwright/test';
import { login, signOut } from './helpers';

test('redirects anonymous users to login', async ({ page }) => {
	await page.goto('/');
	await expect(page).toHaveURL(/\/login\?next=%2F/);
});

test('rejects a wrong password', async ({ page }) => {
	await login(page, 'admin', 'wrong');
	await expect(page.getByRole('alert')).toHaveText('Benutzername oder Passwort ist falsch.');
	await expect(page).toHaveURL(/\/login/);
});

test('logs in, survives reload, logs out', async ({ page }, testInfo) => {
	await login(page);
	// Being on the overview is the proof: an anonymous visitor is bounced to
	// /login by the session guard, so the headline can only render signed in.
	await expect(page).toHaveURL('/');
	const headline = page.getByRole('heading', { name: 'Was kochen wir heute?' });
	await expect(headline).toBeVisible();
	await page.reload();
	await expect(page).toHaveURL('/');
	await expect(headline).toBeVisible();
	await signOut(page, testInfo);
	await expect(page).toHaveURL(/\/login/);
});

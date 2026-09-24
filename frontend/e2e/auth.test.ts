import { expect, test } from '@playwright/test';
import { login, signOut } from './helpers';

test('redirects anonymous users to login', async ({ page }) => {
	await page.goto('/');
	await expect(page).toHaveURL(/\/login\?next=%2F/);
});

test('rejects a wrong password', async ({ page }) => {
	await login(page, 'admin', 'wrong');
	await expect(page.getByRole('alert')).toHaveText('That username or password is wrong.');
	await expect(page).toHaveURL(/\/login/);
});

test('explains a sign-in locked after too many failures', async ({ page }, testInfo) => {
	// A name of its own per project and run, so the lock never reaches the
	// accounts other specs sign in with. Unknown names are locked like real ones.
	const username = `locked-${testInfo.project.name}-${Date.now()}`;
	for (let attempt = 0; attempt < 5; attempt++) {
		await login(page, username, 'wrong');
		await expect(page.getByRole('alert')).toBeVisible();
	}
	await login(page, username, 'wrong');
	await expect(page.getByRole('alert')).toHaveText(
		'Too many failed attempts. Wait a little, then try again.'
	);
});

test('logs in, survives reload, logs out', async ({ page }, testInfo) => {
	await login(page);
	// Being on the overview is the proof: an anonymous visitor is bounced to
	// /login by the session guard, so the headline can only render signed in.
	await expect(page).toHaveURL('/');
	const headline = page.getByRole('heading', { name: 'What are we cooking today?' });
	await expect(headline).toBeVisible();
	await page.reload();
	await expect(page).toHaveURL('/');
	await expect(headline).toBeVisible();
	await signOut(page, testInfo);
	await expect(page).toHaveURL(/\/login/);
});

// A password manager fills and saves by these attributes alone, and a phone
// keyboard that capitalized "sam" to "Sam" would fail the login.
test('the login form speaks to password managers', async ({ page }) => {
	await page.goto('/login');
	const username = page.getByLabel('Username');
	await expect(username).toHaveAttribute('autocomplete', 'username');
	await expect(username).toHaveAttribute('autocapitalize', 'none');
	await expect(username).toHaveAttribute('spellcheck', 'false');
	await expect(page.getByLabel('Password')).toHaveAttribute('autocomplete', 'current-password');
});

test('the password change names the account it belongs to', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings');
	const account = page.locator('input[autocomplete="username"]');
	await expect(account).toHaveValue('admin');
	await expect(account).toBeHidden();
	await expect(page.getByLabel('New password', { exact: true })).toHaveAttribute(
		'autocomplete',
		'new-password'
	);
});

import { expect, test } from '@playwright/test';
import { devPassword, login, pinLocale, signOut } from './helpers';

test('redirects anonymous users to login', async ({ page }) => {
	await page.goto('/');
	await expect(page).toHaveURL(/\/login\?next=%2F/);
});

// SvelteKit 3's goto rejects a path without a route, and the login form read
// that rejection as a failed sign-in: signed in, yet told it went wrong.
test('a sign-in towards a page that does not exist lands on its 404', async ({ page }) => {
	await pinLocale(page);
	await page.goto('/nope');
	await expect(page).toHaveURL('/login?next=%2Fnope');
	// The form, not the API: the redirect after it is what broke. A lock is
	// the throttle race the login helper describes, so it waits and retries.
	for (let attempt = 0; attempt < 6; attempt++) {
		await page.getByLabel('Username').fill('admin');
		await page.getByLabel('Password').fill(devPassword('admin'));
		await page.getByRole('button', { name: 'Sign in' }).click();
		const outcome = await Promise.race([
			page.waitForURL('/nope').then(() => 'in'),
			page
				.getByRole('alert')
				.waitFor()
				.then(async () => page.getByRole('alert').textContent())
		]);
		if (outcome === 'in' || !outcome?.startsWith('Too many')) break;
		await page.waitForTimeout(250 * (attempt + 1));
	}
	await expect(page).toHaveURL('/nope');
	await expect(page.getByRole('heading', { name: "This page isn't in the book" })).toBeVisible();
	await expect(page.getByRole('alert')).toHaveCount(0);
});

// A failed session load (the server unreachable) fails the root layout, which
// SvelteKit would answer with its bare built-in page; ours offers a retry. An
// unknown path takes another way through SvelteKit (+layout.ts), so both.
test('an unreachable server shows the error page, and trying again recovers', async ({ page }) => {
	await pinLocale(page);
	await login(page);
	await page.route('**/api/v1/auth/me', (route) => route.abort());
	for (const path of ['/', '/nope']) {
		await page.goto(path);
		await expect(page.getByRole('heading', { name: 'Something went wrong' })).toBeVisible();
	}
	await page.unroute('**/api/v1/auth/me');
	await page.getByRole('button', { name: 'Try again' }).click();
	await expect(page.getByRole('heading', { name: "This page isn't in the book" })).toBeVisible();
	await page.getByRole('link', { name: 'Back to recipes' }).click();
	await expect(page.getByRole('heading', { name: 'Something went wrong' })).toHaveCount(0);
	await expect(page).toHaveURL('/');
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

// The page has one job, so it saves the click into its first field.
test('the login form starts in the username field', async ({ page }) => {
	await page.goto('/login');
	await expect(page.getByLabel('Username')).toBeFocused();
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

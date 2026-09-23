import { expect, test } from '@playwright/test';
import { createUser, devPassword, login, pinLocale, uniqueToken } from './helpers';

// This spec, uniquely, bootstraps a throwaway member for every test rather
// than logging in as `admin`. Everywhere else, `admin`'s own stored locale is
// German (mise run e2e's REZEPTE_LOCALE=de), so `pinLocale` and the account
// agree and nothing here is a fight. Switching languages, though, writes to
// the account, not just the browser, and `admin` is read by every other spec
// running in parallel against the same database - flipping it to English
// mid-suite would fail them, intermittently, depending on scheduling. A
// throwaway user's locale is nobody else's problem, so there is nothing to
// put back afterwards.

// A block of its own, because `test.use` sets the browser language for every
// test in its scope and the rest of this file wants the default one. The
// German browser is the whole point: English is the base locale, so a test
// run in an English browser cannot tell the preferredLanguage strategy from
// the baseLocale fallback - both answer "en" whether the strategy works or
// not. Only a browser asking for something other than the base locale does.
test.describe('a German browser at the login screen', () => {
	test.use({ locale: 'de-DE' });

	test('the login screen follows the browser language', async ({ page }) => {
		await page.context().clearCookies();
		await page.goto('/login');
		// No account yet, so no stored language: the browser's setting decides.
		await expect(page.getByRole('button', { name: 'Anmelden' })).toBeVisible();
	});
});

test('a signed-in account whose locale is English sees the English UI', async ({ page }) => {
	const username = `lang${uniqueToken()}`;
	// admin, only to create the throwaway member through the API - its own
	// language is irrelevant here and login() leaves it pinned to German.
	await login(page);
	await expect(page).toHaveURL('/');
	// mise run e2e bootstraps this instance with REZEPTE_LOCALE=de (see
	// mise/tasks/e2e.sh), so every account defaults to German unless it says
	// otherwise - this one has to say otherwise to be the English account
	// this test is about.
	await createUser(page, { username, role: 'user', locale: 'en' });

	await page.context().clearCookies();
	await pinLocale(page, 'en');
	await page.goto('/login');
	await page.getByLabel('Username').fill(username);
	await page.getByLabel('Password').fill(devPassword(username));
	await page.getByRole('button', { name: 'Sign in' }).click();
	// The overview headline, not the top bar's "New recipe" link: that link
	// is desktop-only, and the phone project shows the bottom nav's "New"
	// instead, so the headline is the one thing both viewports render.
	await expect(page.getByRole('heading', { name: 'What are we cooking today?' })).toBeVisible();
});

test('switching to German changes the UI and survives a reload', async ({ page }) => {
	const username = `lang${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	// Starts English (see the locale note above) so the click below is an
	// actual switch, not a no-op.
	await createUser(page, { username, role: 'user', locale: 'en' });

	await page.context().clearCookies();
	await pinLocale(page, 'en');
	await page.goto('/login');
	await page.getByLabel('Username').fill(username);
	await page.getByLabel('Password').fill(devPassword(username));
	await page.getByRole('button', { name: 'Sign in' }).click();
	await expect(page.getByRole('heading', { name: 'What are we cooking today?' })).toBeVisible();

	await page.goto('/settings');
	await expect(page.getByRole('heading', { name: 'Settings', level: 1 })).toBeVisible();
	// Bits UI renders the select trigger as a plain <button> carrying only
	// the aria-label (tokens.test.ts documents the same thing for the expiry
	// picker); the list items are real role="option"s.
	//
	// The account is still English here, so the list is written in English -
	// the German entry reads "German", with "Deutsch" beside it as the
	// endonym, and only reads "Deutsch" alone after the reload.
	await page.getByRole('button', { name: 'Interface language' }).click();
	await page.getByRole('option', { name: /^German/ }).click();
	await expect(page.getByRole('heading', { name: 'Einstellungen', level: 1 })).toBeVisible();

	await page.reload();
	await expect(page.getByRole('heading', { name: 'Einstellungen', level: 1 })).toBeVisible();
});

test('a stale locale cookie gives way to the account on the next session load', async ({
	page
}) => {
	const username = `lang${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'user', locale: 'en' });

	await page.context().clearCookies();
	await pinLocale(page, 'en');
	await page.goto('/login');
	await page.getByLabel('Username').fill(username);
	await page.getByLabel('Password').fill(devPassword(username));
	await page.getByRole('button', { name: 'Sign in' }).click();
	await expect(page.getByRole('heading', { name: 'What are we cooking today?' })).toBeVisible();

	// What a second device looks like once the language was changed on the
	// first one: a live session whose locale cookie no longer matches the
	// account row. Nothing rewrites that cookie by itself - it lives a year -
	// so the SPA has to notice that GET /auth/me disagrees with it.
	await pinLocale(page, 'de');
	await page.goto('/settings');
	await expect(page.getByRole('heading', { name: 'Settings', level: 1 })).toBeVisible();

	// And it stuck: the account's language went back into the cookie, so the
	// next load is not another round of the same.
	const cookie = (await page.context().cookies()).find((c) => c.name === 'PARAGLIDE_LOCALE');
	expect(cookie?.value).toBe('en');
});

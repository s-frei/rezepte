import { expect, test } from '@playwright/test';
import {
	createRecipe,
	createUser,
	devPassword,
	loadFixture,
	login,
	pinLocale,
	signOut,
	uniqueToken
} from './helpers';

// This spec, uniquely, bootstraps a throwaway member for every test rather
// than working as `admin`. Everywhere else, `admin`'s own stored locale is
// the instance default, English, so `pinLocale` and the account agree and
// nothing here is a fight. Switching languages, though, writes to the
// account, not just the browser, and `admin` is read by every other spec
// running in parallel against the same database - flipping it to German
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

// The one pair of locale changes that happens without a reload: the login
// response and the sign-out each rewrite the cookie and then navigate
// client-side. `<html lang>` has to follow both, or screen readers and
// `hyphens-auto` keep working to the previous language.
test.describe('an English browser signing a German account in and out', () => {
	test.use({ locale: 'en-US' });

	test('<html lang> follows login and sign-out', async ({ page }, testInfo) => {
		const username = `lang${uniqueToken()}`;
		await login(page);
		await expect(page).toHaveURL('/');
		await createUser(page, { username, role: 'user', locale: 'de' });

		await page.context().clearCookies();
		await page.goto('/login');
		await expect(page.locator('html')).toHaveAttribute('lang', 'en');
		await page.getByLabel('Username').fill(username);
		await page.getByLabel('Password').fill(devPassword(username));
		await page.getByRole('button', { name: 'Sign in' }).click();
		await expect(page.getByRole('heading', { name: 'Was kochen wir heute?' })).toBeVisible();
		await expect(page.locator('html')).toHaveAttribute('lang', 'de');

		await signOut(page, testInfo, 'de');
		await expect(page.getByRole('button', { name: 'Sign in' })).toBeVisible();
		await expect(page.locator('html')).toHaveAttribute('lang', 'en');
	});
});

test('a German account reads German, decimal comma included', async ({ page }) => {
	const username = `lang${uniqueToken()}`;
	// admin, only to create the throwaway member through the API.
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'user', locale: 'de' });

	// login() pins English for the form; the login response then hands over
	// the account's own language.
	await page.context().clearCookies();
	await login(page, username);
	// The overview headline, not the top bar's "Neues Rezept" link: that link
	// is desktop-only, and the phone project shows the bottom nav's "Neu"
	// instead, so the headline is the one thing both viewports render.
	await expect(page.getByRole('heading', { name: 'Was kochen wir heute?' })).toBeVisible();

	// Numbers follow the language too: format.ts hands getLocale() to
	// Intl.NumberFormat, so a scaling factor reads with a comma here and a
	// point in English (cooking.test.ts). 4 → 3 servings is ×0.75.
	const recipe = await createRecipe(page, loadFixture(0, 'de'));
	await page.goto(`/recipes/${recipe.slug}`);
	await page.getByRole('button', { name: 'Weniger Portionen' }).click();
	await expect(page.getByText('Für 3 Portionen · ×0,75')).toBeVisible();
});

test('switching to German changes the UI and survives a reload', async ({ page }) => {
	const username = `lang${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	// Named although it is the default, because the click below has to be an
	// actual switch, not a no-op.
	await createUser(page, { username, role: 'user', locale: 'en' });

	await page.context().clearCookies();
	await login(page, username);
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
	await login(page, username);
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

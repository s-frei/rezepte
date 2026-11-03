import { expect, test } from '@playwright/test';
import { devPassword, login, pinLocale, uniqueToken } from './helpers';

// "Add account" defaults to the "Send a setup link" mode (CreateUserDialog),
// so these tests never touch the password fields the older account-creation
// tests in settings.test.ts exercise.

test('an invited person sets their own password through the setup link', async ({
	page,
	browser
}) => {
	// uniqueToken(), not Date.now(): desktop and mobile run this file at the
	// same time against one database, and a bare timestamp can collide.
	const name = `invite${uniqueToken()}`;
	await login(page);
	await page.goto('/settings/users');
	await page.getByRole('button', { name: 'Add account' }).click();
	await page.getByLabel('Username').fill(name);
	await page.getByRole('button', { name: 'Add', exact: true }).click();
	// Named, not just `getByRole('dialog')`: the create dialog it replaces can
	// still be mid-close, and the two would otherwise both match.
	const linkDialog = page.getByRole('dialog', { name: `Setup link for ${name}` });
	const url = await linkDialog.getByRole('textbox').inputValue();
	expect(url).toContain('/welcome#');

	const invited = await (await browser.newContext()).newPage();
	await invited.goto(url);
	await expect(invited.getByRole('heading', { name: `Welcome, ${name}` })).toBeVisible();
	await invited.getByLabel('New password', { exact: true }).fill(devPassword(name));
	await invited.getByLabel('Repeat the new password').fill(devPassword(name));
	await invited.getByRole('button', { name: 'Save and sign in' }).click();
	await expect(invited).toHaveURL('/');

	await invited.goto(url);
	await expect(invited.getByText('This link no longer works')).toBeVisible();
});

test('an admin revokes an open setup link', async ({ page, browser }) => {
	const name = `revoke${uniqueToken()}`;
	await login(page);
	await page.goto('/settings/users');
	await page.getByRole('button', { name: 'Add account' }).click();
	await page.getByLabel('Username').fill(name);
	await page.getByRole('button', { name: 'Add', exact: true }).click();
	const linkDialog = page.getByRole('dialog', { name: `Setup link for ${name}` });
	const url = await linkDialog.getByRole('textbox').inputValue();
	await page.keyboard.press('Escape');

	// The status note under the name is text only; "Revoke link" is an item
	// in the card's "..." menu on every screen. Its accessible name carries
	// the username (users_setup_link_revoke_aria), since a flat list otherwise
	// has one identically-worded "Revoke link" per row.
	const row = page.getByRole('listitem').filter({ hasText: name });
	await expect(row.getByText(/Setup link open until/)).toBeVisible();
	await row.getByRole('button', { name: `More actions for ${name}` }).click();
	await page.getByRole('menuitem', { name: `Revoke setup link for ${name}` }).click();
	await expect(row.getByText('Not set up yet')).toBeVisible();

	const invited = await (await browser.newContext()).newPage();
	await invited.goto(url);
	await expect(invited.getByText('This link no longer works')).toBeVisible();
});

test('a setup link opened while signed in says whose it is', async ({ page }) => {
	const name = `signedin${uniqueToken()}`;
	await login(page);
	await page.goto('/settings/users');
	await page.getByRole('button', { name: 'Add account' }).click();
	await page.getByLabel('Username').fill(name);
	await page.getByRole('button', { name: 'Add', exact: true }).click();
	const linkDialog = page.getByRole('dialog', { name: `Setup link for ${name}` });
	const url = await linkDialog.getByRole('textbox').inputValue();

	await page.goto(url);
	await expect(page.getByRole('heading', { name: `Welcome, ${name}` })).toBeVisible();
	await expect(
		page.getByText(new RegExp(`^You are signed in as .+\\. This link is for ${name}\\.$`))
	).toBeVisible();
	// Nothing else changes: the form is still there to finish the link.
	await expect(page.getByRole('button', { name: 'Save and sign in' })).toBeVisible();
});

// The page clears the token from the address bar. Clearing it with a null
// history state took SvelteKit's entry away, so a back onto the page changed
// only the address and left the login form on screen.
test('back onto a setup link shows its page again', async ({ browser }) => {
	const page = await (await browser.newContext()).newPage();
	await pinLocale(page);
	await page.goto(`/welcome#unknown${uniqueToken()}`);
	await expect(page.getByRole('heading', { name: 'This link no longer works' })).toBeVisible();
	await page.getByRole('link', { name: 'Back to sign-in' }).click();
	await expect(page).toHaveURL('/login');

	await page.goBack();
	await expect(page).toHaveURL('/welcome');
	await expect(page.getByRole('heading', { name: 'This link no longer works' })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Sign in' })).toHaveCount(0);
});

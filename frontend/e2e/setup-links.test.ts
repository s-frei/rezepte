import { expect, test } from '@playwright/test';
import { devPassword, login, uniqueToken } from './helpers';

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

	// The status line and its "Revoke link" action sit under the name, so
	// they show on the row itself on both viewports - unlike Reset password
	// and Delete, which move into the sheet on a phone. The button's
	// accessible name carries the username (users_setup_link_revoke_aria),
	// since a flat list otherwise has one identically-worded "Revoke link"
	// per row.
	const row = page.getByRole('listitem').filter({ hasText: name });
	await expect(row.getByText(/Setup link open until/)).toBeVisible();
	await row.getByRole('button', { name: `Revoke setup link for ${name}` }).click();
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

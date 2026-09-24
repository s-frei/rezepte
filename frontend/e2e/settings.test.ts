import { expect, test } from '@playwright/test';
import {
	createRecipe,
	createUser,
	devPassword,
	devPasswordNext,
	loadFixture,
	login,
	search,
	uniqueToken
} from './helpers';

// Follows the conventions of recipes.test.ts: unique usernames per test,
// because desktop and mobile run against one binary and one database.

test('a member changes the own password and logs in with it', async ({ page }) => {
	const username = `pw${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'user' });

	await page.context().clearCookies();
	await login(page, username);
	await expect(page).toHaveURL('/');
	await page.goto('/settings');
	await page.getByLabel('Current password').fill(devPassword(username));
	await page.getByLabel('New password', { exact: true }).fill(devPasswordNext(username));
	await page.getByLabel('Repeat the new password').fill(devPasswordNext(username));
	await page.getByRole('button', { name: 'Save password' }).click();
	await expect(page.getByText('Password changed')).toBeVisible();

	await page.context().clearCookies();
	await login(page, username);
	await expect(page.getByRole('alert')).toHaveText('That username or password is wrong.');
	await login(page, username, devPasswordNext(username));
	await expect(page).toHaveURL('/');
});

test('a wrong current password shows an inline error', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings');
	await page.getByLabel('Current password').fill('definitely-wrong');
	// The instance owner, whose own password this test never changes - it is
	// rejected on the current one.
	await page.getByLabel('New password', { exact: true }).fill(devPasswordNext('admin'));
	await page.getByLabel('Repeat the new password').fill(devPasswordNext('admin'));
	await page.getByRole('button', { name: 'Save password' }).click();
	await expect(page.getByText('The current password is wrong')).toBeVisible();
});

test('the new password is rated as it is typed, and only advised on', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings');
	const next = page.getByLabel('New password', { exact: true });
	await next.fill('password1');
	await expect(page.getByText('Strength: very weak')).toBeVisible();
	await next.fill('tangerine-orbit-velvet-harbor-91');
	await expect(page.getByText('Strength: strong')).toBeVisible();
	await next.fill('');
	await expect(page.getByText(/^Strength:/)).toHaveCount(0);
});

test('members are sent from the users page to their own settings', async ({ page }) => {
	const username = `m${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'user' });
	await page.context().clearCookies();
	await login(page, username);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');
	await expect(page).toHaveURL('/settings');
	await expect(page.getByRole('link', { name: 'Members' })).toHaveCount(0);
});

test('the owner creates, promotes, resets and deletes a user', async ({ page }) => {
	const username = `u${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');

	await page.getByRole('button', { name: 'Add member' }).click();
	const dialog = page.getByRole('dialog');
	await dialog.getByLabel('Username').fill(username);
	await dialog.getByLabel('Password', { exact: true }).fill(devPassword(username));
	await dialog.getByRole('radio', { name: /Member/ }).click();
	await dialog.getByRole('button', { name: 'Add', exact: true }).click();

	// Scoped to the user list: svelte-sonner renders its "<name> added"
	// toast as a listitem as well, which makes a bare getByRole ambiguous.
	const row = page
		.getByRole('list', { name: 'Members' })
		.getByRole('listitem')
		.filter({ hasText: username });
	await expect(row).toBeVisible();

	// Bits UI renders the select trigger as a plain <button>; it carries no
	// `role="combobox"`, only the aria-label "<name>'s role".
	const roleSelect = row.getByRole('button', { name: `${username}'s role` });
	await expect(roleSelect).toHaveText(/Member/);
	await roleSelect.click();
	await page.getByRole('option', { name: 'Admin' }).click();
	await expect(roleSelect).toHaveText(/Admin/);
	await expect(page.getByText(`${username}'s role changed`)).toBeVisible();

	await row.getByRole('button', { name: `Reset ${username}'s password` }).click();
	await page.getByRole('dialog').getByLabel('New password').fill(devPasswordNext(username));
	await page.getByRole('dialog').getByRole('button', { name: 'Reset', exact: true }).click();
	await expect(page.getByText('Password reset')).toBeVisible();

	await row.getByRole('button', { name: 'Delete' }).click();
	await page.getByRole('dialog').getByRole('button', { name: 'Delete' }).click();
	await expect(row).toHaveCount(0);
});

test('the owner sorts the member list by role', async ({ page, isMobile }) => {
	// The sortable head is the desktop grid's; under `md` the rows are stacked
	// cards with no head at all and keep the default order by name.
	test.skip(isMobile, 'the sort head exists on the desktop layout only');
	const username = `z${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'user' });
	await page.goto('/settings/users');

	const rows = page.getByRole('list', { name: 'Members' }).getByRole('listitem');
	await expect(rows.filter({ hasText: username })).toBeVisible();

	// The head cell, not a row's role select, which is named "<name>'s
	// role". The sort state is part of the head button's own name, which is
	// how it reaches a screen reader - `aria-sort` would need a
	// `columnheader`, and these rows are cards, not a table.
	const byRole = page.getByRole('button', { name: /^Role( sorted (a|de)scending)?$/ });
	await expect(byRole).toHaveAccessibleName('Role');

	// Rank, owner first - whatever the other tests' accounts are called.
	await byRole.click();
	await expect(byRole).toHaveAccessibleName('Role sorted ascending');
	await expect(rows.first()).toContainText('Owner');
	// The same header again turns the order around.
	await byRole.click();
	await expect(byRole).toHaveAccessibleName('Role sorted descending');
	await expect(rows.last()).toContainText('Owner');
});

test('a second admin cannot touch the instance owner', async ({ page }) => {
	const username = `a${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'admin' });

	await page.context().clearCookies();
	await login(page, username);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');

	// The owner's row: a badge instead of a role control, and no actions.
	// Scoped to the user list and matched on the exact username, because the
	// `admin` the e2e task seeds also reads as "Admin" in every promoted
	// user's role pill, and svelte-sonner's toasts are listitems too.
	const owner = page
		.getByRole('list', { name: 'Members' })
		.getByRole('listitem')
		.filter({ has: page.getByText('admin', { exact: true }) });
	await expect(owner.getByText('Owner')).toBeVisible();
	await expect(owner.getByRole('button', { name: /delete/i })).toHaveCount(0);
	await expect(owner.getByRole('button', { name: /password/i })).toHaveCount(0);
});

test('a plain admin sees member roles as badges', async ({ page }) => {
	const admin = `a${uniqueToken()}`;
	const member = `m${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username: admin, role: 'admin' });
	await createUser(page, { username: member, role: 'user' });

	await page.context().clearCookies();
	await login(page, admin);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');

	// Changing a role is the owner's alone, so the member's role is text and
	// not a control - while delete and reset stay this admin's to use.
	const row = page
		.getByRole('list', { name: 'Members' })
		.getByRole('listitem')
		.filter({ has: page.getByText(member, { exact: true }) });
	await expect(row.getByText('Member', { exact: true })).toBeVisible();
	await expect(row.getByRole('button', { name: /role/i })).toHaveCount(0);
	await expect(row.getByRole('button', { name: `Delete ${member}` })).toBeVisible();
});

test('only the owner can hand out the admin role', async ({ page }) => {
	const admin = `a${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username: admin, role: 'admin' });

	await page.context().clearCookies();
	await login(page, admin);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');
	await page.getByRole('button', { name: 'Add member' }).click();
	// The dialog offers "Member" only.
	const dialog = page.getByRole('dialog');
	await expect(dialog.getByRole('radio', { name: 'Member' })).toBeVisible();
	await expect(dialog.getByRole('radio', { name: 'Admin' })).toHaveCount(0);
});

test('the command palette opens with Ctrl+K and jumps to a recipe', async ({ page }) => {
	const token = uniqueToken();
	await login(page);
	await expect(page).toHaveURL('/');
	const fixture = loadFixture(3);
	fixture.title = `Fish and Chips ${token}`;
	const recipe = await createRecipe(page, fixture);

	await page.keyboard.press('Control+k');
	const dialog = page.getByRole('dialog', { name: 'Command palette' });
	await expect(dialog).toBeVisible();
	await dialog.getByRole('combobox').fill(token);
	await dialog.getByRole('option', { name: fixture.title }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	await expect(dialog).toHaveCount(0);
});

test('a member renames themselves and picks a color, and their cards follow', async ({ page }) => {
	const token = uniqueToken();
	const username = `col${token}`;
	// The token rides along in the display name as well: both projects run
	// this test against the same database, so two accounts would otherwise
	// answer to "Added by Sam".
	const displayName = `Sam ${token}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'user' });

	await page.context().clearCookies();
	await login(page, username);
	await expect(page).toHaveURL('/');
	await createRecipe(page, { ...loadFixture(0), title: `Color test ${token}` });

	await page.goto('/settings');
	await page.getByLabel('Display name').fill(displayName);
	await page.getByRole('button', { name: 'Save name' }).click();
	await expect(page.getByText('Profile saved')).toBeVisible();

	// Picking a swatch is the save, and it raises a second toast carrying the
	// same words - which would make the locator above ambiguous. So the round
	// trip is read off the profile card's own avatar instead: it follows the
	// session, which only changes once the server has answered.
	await page.getByRole('radio', { name: 'Sage' }).click();
	const avatar = page.locator('section[aria-labelledby="settings-profile"] span.size-14');
	await expect(avatar).toHaveClass(/bg-user-sage/);

	// The recipe was written before the rename, and its card carries the name
	// and the color the account holds now: both are joined from `users` on
	// every read rather than copied onto the recipe.
	await page.goto('/');
	await search(page, token);
	const circle = page.getByLabel(`Added by ${displayName}`).locator('span').first();
	await expect(circle).toHaveClass(/bg-user-sage/);
	await expect(circle).toHaveText('S');
});

test('an admin cannot rename a member, the owner can', async ({ page }) => {
	const token = uniqueToken();
	const member = `ren${token}`;
	const admin = `adm${token}`;
	const displayName = `Renamed ${token}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username: member, role: 'user' });
	await createUser(page, { username: admin, role: 'admin' });

	// Managing a member is administration; renaming them is not, so an admin
	// who is not the owner never gets the action. The refusal underneath it
	// has no path through the UI and is covered by the Go handler test.
	await page.context().clearCookies();
	await login(page, admin);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');
	// Scoped to the user list and matched on the login name - which stays put
	// while the display name is what this test changes; svelte-sonner renders
	// its toasts as listitems too.
	const asAdmin = page
		.getByRole('list', { name: 'Members' })
		.getByRole('listitem')
		.filter({ hasText: member });
	await expect(asAdmin).toBeVisible();
	await expect(asAdmin.getByRole('button', { name: 'Edit profile' })).toHaveCount(0);

	// The `admin` the e2e task seeds is the instance owner, and renaming
	// somebody else is theirs alone.
	await page.context().clearCookies();
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');
	const row = page
		.getByRole('list', { name: 'Members' })
		.getByRole('listitem')
		.filter({ hasText: member });
	await row.getByRole('button', { name: 'Edit profile' }).click();
	const dialog = page.getByRole('dialog');
	await dialog.getByLabel('Display name').fill(displayName);
	await dialog.getByRole('radio', { name: 'Teal' }).click();
	await dialog.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page.getByText('Profile saved')).toBeVisible();
	await expect(row).toContainText(displayName);
	await expect(row.locator('span.size-9')).toHaveClass(/bg-user-teal/);
});

test('the settings page links to the user guide in a new tab', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings');
	const help = page.getByRole('link', { name: /Help & guide/ });
	await expect(help).toBeVisible();
	await expect(help).toHaveAttribute('href', 'https://s-frei.github.io/rezepte/guide/');
	await expect(help).toHaveAttribute('target', '_blank');
});

test('a phone reaches the settings pages and sign-out through the contents sheet', async ({
	page
}, testInfo) => {
	test.skip(
		!testInfo.project.name.startsWith('mobile'),
		'the running head is the phone layout only'
	);

	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings');

	await page.getByRole('button', { name: 'Profile, open contents' }).click();
	const sheet = page.getByRole('dialog', { name: 'Contents' });
	await expect(sheet.getByRole('link')).toHaveText(['Profile', 'API', 'Members']);
	await expect(sheet.getByRole('link', { name: 'Profile' })).toHaveAttribute(
		'aria-current',
		'page'
	);

	await sheet.getByRole('link', { name: 'API' }).click();
	await expect(page).toHaveURL('/settings/api');
	await expect(sheet).toBeHidden();

	await page.getByRole('button', { name: 'API, open contents' }).click();
	await sheet.getByRole('button', { name: 'Sign out' }).click();
	await expect(page).toHaveURL(/\/login/);
});

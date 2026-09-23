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
	await page.getByLabel('Aktuelles Passwort').fill(devPassword(username));
	await page.getByLabel('Neues Passwort', { exact: true }).fill(devPasswordNext(username));
	await page.getByLabel('Neues Passwort wiederholen').fill(devPasswordNext(username));
	await page.getByRole('button', { name: 'Passwort speichern' }).click();
	await expect(page.getByText('Passwort geändert')).toBeVisible();

	await page.context().clearCookies();
	await login(page, username);
	await expect(page.getByRole('alert')).toHaveText('Benutzername oder Passwort ist falsch.');
	await login(page, username, devPasswordNext(username));
	await expect(page).toHaveURL('/');
});

test('a wrong current password shows an inline error', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings');
	await page.getByLabel('Aktuelles Passwort').fill('definitely-wrong');
	// The instance owner, whose own password this test never changes - it is
	// rejected on the current one.
	await page.getByLabel('Neues Passwort', { exact: true }).fill(devPasswordNext('admin'));
	await page.getByLabel('Neues Passwort wiederholen').fill(devPasswordNext('admin'));
	await page.getByRole('button', { name: 'Passwort speichern' }).click();
	await expect(page.getByText('Das aktuelle Passwort ist falsch')).toBeVisible();
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
	await expect(page.getByRole('link', { name: 'Benutzer' })).toHaveCount(0);
});

test('the owner creates, promotes, resets and deletes a user', async ({ page }) => {
	const username = `u${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');

	await page.getByRole('button', { name: 'Benutzer anlegen' }).click();
	const dialog = page.getByRole('dialog');
	await dialog.getByLabel('Benutzername').fill(username);
	await dialog.getByLabel('Passwort', { exact: true }).fill(devPassword(username));
	await dialog.getByRole('radio', { name: /Mitglied/ }).click();
	await dialog.getByRole('button', { name: 'Anlegen' }).click();

	// Scoped to the user list: svelte-sonner renders its "<name> angelegt"
	// toast as a listitem as well, which makes a bare getByRole ambiguous.
	const row = page
		.getByRole('list', { name: 'Benutzer' })
		.getByRole('listitem')
		.filter({ hasText: username });
	await expect(row).toBeVisible();

	// Bits UI renders the select trigger as a plain <button>; it carries no
	// `role="combobox"`, only the aria-label "Rolle von <name>".
	const roleSelect = row.getByRole('button', { name: 'Rolle' });
	await expect(roleSelect).toHaveText(/Mitglied/);
	await roleSelect.click();
	await page.getByRole('option', { name: 'Admin' }).click();
	await expect(roleSelect).toHaveText(/Admin/);
	await expect(page.getByText(`Rolle von ${username} geändert`)).toBeVisible();

	await row.getByRole('button', { name: `Passwort von ${username} zurücksetzen` }).click();
	await page.getByRole('dialog').getByLabel('Neues Passwort').fill(devPasswordNext(username));
	await page.getByRole('dialog').getByRole('button', { name: 'Zurücksetzen' }).click();
	await expect(page.getByText('Passwort zurückgesetzt')).toBeVisible();

	await row.getByRole('button', { name: 'Löschen' }).click();
	await page.getByRole('dialog').getByRole('button', { name: 'Löschen' }).click();
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

	const rows = page.getByRole('list', { name: 'Benutzer' }).getByRole('listitem');
	await expect(rows.filter({ hasText: username })).toBeVisible();

	// The head cell, not a row's role select, which is named "Rolle von
	// <name>". The sort state is part of the head button's own name, which is
	// how it reaches a screen reader - `aria-sort` would need a
	// `columnheader`, and these rows are cards, not a table.
	const byRole = page.getByRole('button', { name: /^Rolle( (auf|ab)steigend sortiert)?$/ });
	await expect(byRole).toHaveAccessibleName('Rolle');

	// Rank, owner first - whatever the other tests' accounts are called.
	await byRole.click();
	await expect(byRole).toHaveAccessibleName('Rolle aufsteigend sortiert');
	await expect(rows.first()).toContainText('Inhaber');
	// The same header again turns the order around.
	await byRole.click();
	await expect(byRole).toHaveAccessibleName('Rolle absteigend sortiert');
	await expect(rows.last()).toContainText('Inhaber');
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
		.getByRole('list', { name: 'Benutzer' })
		.getByRole('listitem')
		.filter({ has: page.getByText('admin', { exact: true }) });
	await expect(owner.getByText('Inhaber')).toBeVisible();
	await expect(owner.getByRole('button', { name: /löschen/i })).toHaveCount(0);
	await expect(owner.getByRole('button', { name: /Passwort von/i })).toHaveCount(0);
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
		.getByRole('list', { name: 'Benutzer' })
		.getByRole('listitem')
		.filter({ has: page.getByText(member, { exact: true }) });
	await expect(row.getByText('Mitglied')).toBeVisible();
	await expect(row.getByRole('button', { name: 'Rolle' })).toHaveCount(0);
	await expect(row.getByRole('button', { name: `${member} löschen` })).toBeVisible();
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
	await page.getByRole('button', { name: 'Benutzer anlegen' }).click();
	// The dialog offers "Mitglied" only.
	const dialog = page.getByRole('dialog');
	await expect(dialog.getByRole('radio', { name: 'Mitglied' })).toBeVisible();
	await expect(dialog.getByRole('radio', { name: 'Admin' })).toHaveCount(0);
});

test('the command palette opens with Ctrl+K and jumps to a recipe', async ({ page }) => {
	const token = uniqueToken();
	await login(page);
	await expect(page).toHaveURL('/');
	const fixture = loadFixture(3); // Käsespätzle
	fixture.title = `Käsespätzle ${token}`;
	const recipe = await createRecipe(page, fixture);

	await page.keyboard.press('Control+k');
	const dialog = page.getByRole('dialog', { name: 'Befehlspalette' });
	await expect(dialog).toBeVisible();
	await dialog.getByRole('combobox').fill(token);
	await dialog.getByRole('option', { name: fixture.title }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	await expect(dialog).toHaveCount(0);
});

test('a member renames themselves and picks a colour, and their cards follow', async ({ page }) => {
	const token = uniqueToken();
	const username = `col${token}`;
	// The token rides along in the display name as well: both projects run
	// this test against the same database, so two accounts would otherwise
	// answer to "Angelegt von Sam".
	const displayName = `Sam ${token}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'user' });

	await page.context().clearCookies();
	await login(page, username);
	await expect(page).toHaveURL('/');
	await createRecipe(page, { ...loadFixture(0), title: `Farbtest ${token}` });

	await page.goto('/settings');
	await page.getByLabel('Anzeigename').fill(displayName);
	await page.getByRole('button', { name: 'Namen speichern' }).click();
	await expect(page.getByText('Profil gespeichert')).toBeVisible();

	// Picking a swatch is the save, and it raises a second toast carrying the
	// same words - which would make the locator above ambiguous. So the round
	// trip is read off the profile card's own avatar instead: it follows the
	// session, which only changes once the server has answered.
	await page.getByRole('radio', { name: 'Salbei' }).click();
	const avatar = page.locator('section[aria-labelledby="settings-profile"] span.size-14');
	await expect(avatar).toHaveClass(/bg-user-sage/);

	// The recipe was written before the rename, and its card carries the name
	// and the colour the account holds now: both are joined from `users` on
	// every read rather than copied onto the recipe.
	await page.goto('/');
	await search(page, token);
	const circle = page.getByLabel(`Angelegt von ${displayName}`).locator('span').first();
	await expect(circle).toHaveClass(/bg-user-sage/);
	await expect(circle).toHaveText('S');
});

test('an admin cannot rename a member, the owner can', async ({ page }) => {
	const token = uniqueToken();
	const member = `ren${token}`;
	const admin = `adm${token}`;
	const displayName = `Umbenannt ${token}`;
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
		.getByRole('list', { name: 'Benutzer' })
		.getByRole('listitem')
		.filter({ hasText: member });
	await expect(asAdmin).toBeVisible();
	await expect(asAdmin.getByRole('button', { name: 'Profil bearbeiten' })).toHaveCount(0);

	// The `admin` the e2e task seeds is the instance owner, and renaming
	// somebody else is theirs alone.
	await page.context().clearCookies();
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');
	const row = page
		.getByRole('list', { name: 'Benutzer' })
		.getByRole('listitem')
		.filter({ hasText: member });
	await row.getByRole('button', { name: 'Profil bearbeiten' }).click();
	const dialog = page.getByRole('dialog');
	await dialog.getByLabel('Anzeigename').fill(displayName);
	await dialog.getByRole('radio', { name: 'Petrol' }).click();
	await dialog.getByRole('button', { name: 'Speichern' }).click();
	await expect(page.getByText('Profil gespeichert')).toBeVisible();
	await expect(row).toContainText(displayName);
	await expect(row.locator('span.size-9')).toHaveClass(/bg-user-teal/);
});

test('the settings page links to the user guide in a new tab', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings');
	const help = page.getByRole('link', { name: /Hilfe & Anleitung/ });
	await expect(help).toBeVisible();
	await expect(help).toHaveAttribute('href', 'https://s-frei.github.io/rezepte/guide/');
	await expect(help).toHaveAttribute('target', '_blank');
});

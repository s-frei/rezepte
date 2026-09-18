import { expect, test } from '@playwright/test';
import { createRecipe, createUser, loadFixture, login, uniqueToken } from './helpers';

// NOTE: written for the Phase 7 flows but not executed in this phase
// (Chromium cannot start on the dev machine). It must type-check
// (`bun run check` covers e2e/) and follows the conventions of
// recipes.test.ts: unique usernames per test because desktop and mobile
// share one binary and one database.

test('a member changes the own password and logs in with it', async ({ page }) => {
	const username = `pw${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, password: 'old-password-1', role: 'user' });

	await page.context().clearCookies();
	await login(page, username, 'old-password-1');
	await expect(page).toHaveURL('/');
	await page.goto('/settings');
	await page.getByLabel('Aktuelles Passwort').fill('old-password-1');
	await page.getByLabel('Neues Passwort', { exact: true }).fill('new-password-2');
	await page.getByLabel('Neues Passwort wiederholen').fill('new-password-2');
	await page.getByRole('button', { name: 'Passwort speichern' }).click();
	await expect(page.getByText('Passwort geändert')).toBeVisible();

	await page.context().clearCookies();
	await login(page, username, 'old-password-1');
	await expect(page.getByRole('alert')).toHaveText('Benutzername oder Passwort ist falsch.');
	await login(page, username, 'new-password-2');
	await expect(page).toHaveURL('/');
});

test('a wrong current password shows an inline error', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings');
	await page.getByLabel('Aktuelles Passwort').fill('definitely-wrong');
	await page.getByLabel('Neues Passwort', { exact: true }).fill('new-password-2');
	await page.getByLabel('Neues Passwort wiederholen').fill('new-password-2');
	await page.getByRole('button', { name: 'Passwort speichern' }).click();
	await expect(page.getByText('Das aktuelle Passwort ist falsch')).toBeVisible();
});

test('members are sent from the users page to their own settings', async ({ page }) => {
	const username = `m${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, password: 'member-password', role: 'user' });
	await page.context().clearCookies();
	await login(page, username, 'member-password');
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');
	await expect(page).toHaveURL('/settings');
	await expect(page.getByRole('link', { name: 'Benutzer' })).toHaveCount(0);
});

test('an admin creates, promotes, resets and deletes a user', async ({ page }) => {
	const username = `u${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');

	await page.getByRole('button', { name: 'Benutzer anlegen' }).click();
	const dialog = page.getByRole('dialog');
	await dialog.getByLabel('Benutzername').fill(username);
	await dialog.getByLabel('Passwort', { exact: true }).fill('first-password-1');
	await dialog.getByRole('radio', { name: /Mitglied/ }).click();
	await dialog.getByRole('button', { name: 'Anlegen' }).click();

	const row = page.getByRole('listitem').filter({ hasText: username });
	await expect(row).toBeVisible();

	const roleSelect = row.getByRole('combobox', { name: 'Rolle' });
	await expect(roleSelect).toHaveText(/Mitglied/);
	await roleSelect.click();
	await page.getByRole('option', { name: 'Admin' }).click();
	await expect(roleSelect).toHaveText(/Admin/);
	await expect(page.getByText(`Rolle von ${username} geändert`)).toBeVisible();

	await row.getByRole('button', { name: `Passwort von ${username} zurücksetzen` }).click();
	await page.getByRole('dialog').getByLabel('Neues Passwort').fill('second-password-2');
	await page.getByRole('dialog').getByRole('button', { name: 'Zurücksetzen' }).click();
	await expect(page.getByText('Passwort zurückgesetzt')).toBeVisible();

	await row.getByRole('button', { name: 'Löschen' }).click();
	await page.getByRole('dialog').getByRole('button', { name: 'Löschen' }).click();
	await expect(row).toHaveCount(0);
});

test('the last admin cannot be deleted', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	// Other tests may temporarily add admins, so the "only one admin" state is
	// pinned through the API response (same approach as the overview's
	// empty-state test).
	await page.route(
		(url) => url.pathname === '/api/v1/users',
		(route) =>
			route.fulfill({
				json: {
					items: [
						{
							id: 'only-admin',
							username: 'admin',
							role: 'admin',
							createdAt: '2026-01-01T00:00:00Z'
						}
					]
				}
			})
	);
	await page.goto('/settings/users');
	const row = page.getByRole('listitem').filter({ hasText: 'admin' });
	await expect(row.getByRole('button', { name: 'Löschen' })).toBeDisabled();
	await expect(row.getByText('Der letzte Admin kann nicht gelöscht werden.')).toBeVisible();
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

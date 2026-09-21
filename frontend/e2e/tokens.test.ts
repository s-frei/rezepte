import { expect, test } from '@playwright/test';
import { login, uniqueToken } from './helpers';

test('an admin creates, sees once and revokes an API token', async ({ page }) => {
	const name = `token-${uniqueToken()}`;
	await login(page);
	// Wait for the post-login redirect before navigating again: firing
	// page.goto() while the login POST is still in flight cancels it (the
	// server logs "insert session: context canceled") and leaves the page on
	// /login - every other spec in this suite follows the same order.
	await expect(page).toHaveURL('/');
	await page.goto('/settings/api');

	// .first(): with no tokens yet (a fresh e2e database), the empty state
	// repeats the same "Token erstellen" button as the header, so the plain
	// role query is ambiguous - both open the identical dialog.
	await page.getByRole('button', { name: 'Token erstellen' }).first().click();
	// Scoped to the dialog from here on: "Erstellen" is otherwise a substring
	// match of the page's own "Token erstellen" button(s) (getByRole's name
	// match is substring, not exact, unless asked), so the bare query is
	// ambiguous once the dialog is open too.
	const createDialog = page.getByRole('dialog');
	await createDialog.getByLabel('Name').fill(name);
	// The expiry control is a Bits UI Select, not a native <select>: Select.svelte
	// (settings.test.ts documents the same thing for the role picker) renders the
	// trigger as a plain <button> carrying only the aria-label passed as `label`
	// ("Gültigkeit" here), not role="combobox" - confirmed against
	// select-trigger.svelte in bits-ui, which renders `<button>` with no role
	// override. The popover items are real `role="option"`s.
	await createDialog.getByRole('button', { name: 'Gültigkeit' }).click();
	await page.getByRole('option', { name: 'Kein Ablauf' }).click();
	await createDialog.getByRole('button', { name: 'Erstellen', exact: true }).click();

	// Shown exactly once, in the dialog that only its own button closes. Scoped
	// to that dialog: the new list item's own prefix line (e.g. "rzp_1zqB…")
	// also starts with "rzp_" and is visible in the list at the same time.
	const revealDialog = page.getByRole('dialog');
	const secret = revealDialog.getByText(/^rzp_/);
	await expect(secret).toBeVisible();
	const raw = (await secret.textContent()) ?? '';
	expect(raw.startsWith('rzp_')).toBe(true);
	await page.keyboard.press('Escape');
	await expect(secret).toBeVisible();
	await page.getByRole('button', { name: 'Ich habe ihn gespeichert' }).click();
	await expect(secret).toBeHidden();

	// The list shows the prefix, never the secret. TokenTable/TokenRow are a
	// CSS grid (`<ul>`/`<li>`), not a `<table>`, so this is a listitem rather
	// than a row - see settings.test.ts's own user-list flow for the same
	// list/listitem pattern.
	const row = page
		.getByRole('list', { name: 'API' })
		.getByRole('listitem')
		.filter({ hasText: name });
	await expect(row).toBeVisible();
	await expect(page.getByText(raw, { exact: true })).toBeHidden();

	await row.getByRole('button', { name: `Token „${name}“ widerrufen` }).click();
	// exact:true, scoped to the confirm dialog: the icon button just clicked
	// carries an aria-label ("Token „...“ widerrufen") that also substring-
	// matches a bare "Widerrufen" query.
	await page.getByRole('dialog').getByRole('button', { name: 'Widerrufen', exact: true }).click();
	await expect(row).toBeHidden();
});

test('a member cannot reach the API tab', async ({ page }) => {
	const username = `member${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');
	await page.getByRole('button', { name: 'Benutzer anlegen' }).click();
	// Scoped to the dialog: the user list behind it holds per-row "Passwort von
	// <name> zurücksetzen" buttons whose aria-label also contains "Passwort",
	// which getByLabel matches page-wide regardless of the dialog on top
	// (settings.test.ts's own user flow scopes the same way).
	const dialog = page.getByRole('dialog');
	await dialog.getByLabel('Benutzername').fill(username);
	await dialog.getByLabel('Passwort', { exact: true }).fill('member-password-1');
	await dialog.getByRole('button', { name: 'Anlegen' }).click();
	// Wait for the dialog to close (only on a successful create) before
	// clearing cookies: doing so immediately after the click can race the
	// create request itself, same as the login-then-navigate race above.
	await expect(dialog).toBeHidden();

	await page.context().clearCookies();
	await login(page, username, 'member-password-1');
	await expect(page).toHaveURL('/');
	await page.goto('/settings/api');
	await expect(page).toHaveURL('/settings');
});

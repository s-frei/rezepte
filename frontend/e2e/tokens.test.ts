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
	// repeats the same "Create token" button as the header, so the plain
	// role query is ambiguous - both open the identical dialog.
	await page.getByRole('button', { name: 'Create token' }).first().click();
	// Scoped to the dialog from here on: "Create" is otherwise a substring
	// match of the page's own "Create token" button(s) (getByRole's name
	// match is substring, not exact, unless asked), so the bare query is
	// ambiguous once the dialog is open too.
	const createDialog = page.getByRole('dialog');
	await createDialog.getByLabel('Name').fill(name);
	// The expiry control is a Bits UI Select, not a native <select>: Select.svelte
	// (settings.test.ts documents the same thing for the role picker) renders the
	// trigger as a plain <button> carrying only the aria-label passed as `label`
	// ("Lifetime" here), not role="combobox" - confirmed against
	// select-trigger.svelte in bits-ui, which renders `<button>` with no role
	// override. The popover items are real `role="option"`s.
	await createDialog.getByRole('button', { name: 'Lifetime' }).click();
	await page.getByRole('option', { name: 'No expiry' }).click();
	await createDialog.getByRole('button', { name: 'Create', exact: true }).click();

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
	await page.getByRole('button', { name: 'I have saved it' }).click();
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

	await row.getByRole('button', { name: `Revoke the token "${name}"` }).click();
	// exact:true, scoped to the confirm dialog: the icon button just clicked
	// carries an aria-label ('Revoke the token "..."') that also substring-
	// matches a bare "Revoke" query.
	await page.getByRole('dialog').getByRole('button', { name: 'Revoke', exact: true }).click();
	await expect(row).toBeHidden();
});

test('a Full recipes token carries the delete scope', async ({ page }) => {
	const name = `full-${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/api');
	await page.getByRole('button', { name: 'Create token' }).first().click();
	const dialog = page.getByRole('dialog');
	await dialog.getByLabel('Name').fill(name);
	await dialog.getByRole('radio', { name: 'Full' }).click();
	await dialog.getByRole('button', { name: 'Create', exact: true }).click();

	const reveal = page.getByRole('dialog');
	await expect(reveal.getByRole('tab', { name: 'Claude Code' })).toBeVisible();
	await expect(
		reveal.getByText(/claude mcp add --transport http rezepte http:\/\/.+\/mcp/)
	).toBeVisible();
	await reveal.getByRole('tab', { name: 'JSON' }).click();
	await expect(reveal.getByText('"mcpServers"')).toBeVisible();

	await page.getByRole('dialog').getByRole('button', { name: 'I have saved it' }).click();

	// Scoped to this token's own row: the desktop and mobile projects run at
	// the same time against one shared account, and an unscoped text match
	// can catch another project's row with the identical scope list.
	const row = page
		.getByRole('list', { name: 'API' })
		.getByRole('listitem')
		.filter({ hasText: name });
	await expect(row.getByText('recipes:read, recipes:write, recipes:delete')).toBeVisible();
});

test('a users-only token gets no MCP snippet', async ({ page }) => {
	const name = `users-${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/api');
	await page.getByRole('button', { name: 'Create token' }).first().click();
	const dialog = page.getByRole('dialog');
	await dialog.getByLabel('Name').fill(name);
	// exact: true on 'Read' - a substring match also catches 'Read & write'.
	await dialog.getByRole('radio', { name: 'No access' }).first().click();
	await dialog.getByRole('radio', { name: 'Read', exact: true }).nth(1).click();
	await dialog.getByRole('button', { name: 'Create', exact: true }).click();

	const reveal = page.getByRole('dialog');
	await expect(reveal.getByText(/^rzp_/)).toBeVisible();
	await expect(reveal.getByRole('tab', { name: 'Claude Code' })).toHaveCount(0);
	await reveal.getByRole('button', { name: 'I have saved it' }).click();
});

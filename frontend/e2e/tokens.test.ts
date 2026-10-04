import { expect, test } from '@playwright/test';
import { createUser, login, uniqueToken } from './helpers';

test('an admin creates, sees once and revokes an API token', async ({ page }) => {
	const name = `token-${uniqueToken()}`;
	await login(page);
	// Wait for the post-login redirect before navigating again: firing
	// page.goto() while the login POST is still in flight cancels it (the
	// server logs "insert session: context canceled") and leaves the page on
	// /login - every other spec in this suite follows the same order.
	await expect(page).toHaveURL('/');
	await page.goto('/settings/api');

	// One create button, in the token card, with or without tokens.
	await page.getByRole('button', { name: 'Create token' }).click();
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

	// Shown exactly once, in the dialog that only its own button closes.
	const revealDialog = page.getByRole('dialog');
	const secret = revealDialog.getByText(/^rzp_/);
	await expect(secret).toBeVisible();
	const raw = (await secret.textContent()) ?? '';
	expect(raw.startsWith('rzp_')).toBe(true);
	await page.keyboard.press('Escape');
	await expect(secret).toBeVisible();
	await page.getByRole('button', { name: 'I have saved it' }).click();
	await expect(secret).toBeHidden();

	// With a valid token, step one of the AI-assistant recipe is done.
	await expect(page.getByText('Done. Your tokens are listed below.')).toBeVisible();

	// The list never shows the secret. TokenTable/TokenRow are a `<ul>`/`<li>`,
	// not a `<table>`, so this is a listitem rather than a row - see
	// settings.test.ts's own user-list flow for the same list/listitem pattern.
	const row = page
		.getByRole('list', { name: 'API tokens' })
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

test('a Delete recipes token carries the delete scope', async ({ page }) => {
	const name = `full-${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/api');
	await page.getByRole('button', { name: 'Create token' }).click();
	const dialog = page.getByRole('dialog');
	await dialog.getByLabel('Name').fill(name);
	const recipes = dialog.getByRole('radiogroup', { name: 'Recipes' });
	await recipes.getByRole('radio', { name: 'Delete' }).click();
	// The levels stack, so the sentence under the ladder names all three.
	await expect(recipes).toHaveAccessibleDescription('Can read, write and delete recipes.');
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
		.getByRole('list', { name: 'API tokens' })
		.getByRole('listitem')
		.filter({ hasText: name });
	// One plaque per area, naming the deepest level rather than the scopes.
	await expect(row.getByRole('listitem').filter({ hasText: 'Recipes' })).toContainText('Delete');
	await expect(row.getByRole('listitem').filter({ hasText: 'Accounts' })).toContainText('None');
});

test('a users-only token gets no MCP snippet', async ({ page }) => {
	const name = `users-${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/api');
	await page.getByRole('button', { name: 'Create token' }).click();
	const dialog = page.getByRole('dialog');
	await dialog.getByLabel('Name').fill(name);
	await dialog
		.getByRole('radiogroup', { name: 'Recipes' })
		.getByRole('radio', { name: 'None' })
		.click();
	await dialog
		.getByRole('radiogroup', { name: 'Accounts' })
		.getByRole('radio', { name: 'Read' })
		.click();
	await dialog.getByRole('button', { name: 'Create', exact: true }).click();

	const reveal = page.getByRole('dialog');
	await expect(reveal.getByText(/^rzp_/)).toBeVisible();
	await expect(reveal.getByRole('tab', { name: 'Claude Code' })).toHaveCount(0);
	await reveal.getByRole('button', { name: 'I have saved it' }).click();
});

test('a token with no access says so before it is submitted', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/api');
	await page.getByRole('button', { name: 'Create token' }).click();
	const dialog = page.getByRole('dialog');
	const hint = dialog
		.getByRole('alert')
		.filter({ hasText: 'Please choose at least one kind of access' });
	const recipes = dialog.getByRole('radiogroup', { name: 'Recipes' });

	// Accounts start at None, so taking Recipes down to None leaves nothing.
	await expect(hint).toBeHidden();
	await recipes.getByRole('radio', { name: 'None' }).click();
	await expect(hint).toBeVisible();
	await recipes.getByRole('radio', { name: 'Read' }).click();
	await expect(hint).toBeHidden();
});

test("an admin sees another admin's token only in the household view", async ({ page }) => {
	const issuer = `iss${uniqueToken()}`;
	const theirs = `theirs-${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username: issuer, role: 'admin' });

	// The other admin issues a token of their own.
	await page.context().clearCookies();
	await login(page, issuer);
	await expect(page).toHaveURL('/');
	const origin = new URL(page.url()).origin;
	const created = await page.request.post('/api/v1/tokens', {
		headers: { Origin: origin, 'Content-Type': 'application/json' },
		data: { name: theirs, scopes: ['recipes:read'] }
	});
	expect(created.ok()).toBeTruthy();

	await page.context().clearCookies();
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/api');
	const list = page.getByRole('region', { name: 'API tokens' });
	// Own tokens by default: theirs is not in the list.
	await expect(list.getByRole('heading', { name: theirs })).toHaveCount(0);

	await list.getByRole('switch', { name: 'Everyone in the household' }).click();
	await expect(page).toHaveURL(/all=1/);
	const tag = list
		.getByRole('listitem')
		.filter({ has: page.getByRole('heading', { name: theirs }) });
	await expect(tag).toBeVisible();
	// The household view names the issuer on the tag...
	await expect(tag).toContainText(issuer);

	// ...and narrows to one issuer by chip.
	await list.getByRole('button', { name: new RegExp(`^${issuer}`) }).click();
	await expect(page).toHaveURL(new RegExp(`user=`));
	await expect(
		list.getByRole('list', { name: 'API tokens' }).getByRole('heading', { level: 3 })
	).toHaveText([theirs]);
});

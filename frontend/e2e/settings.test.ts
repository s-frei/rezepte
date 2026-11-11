import { expect, test, type Locator, type Page } from '@playwright/test';
import {
	createRecipe,
	createUser,
	devPassword,
	devPasswordNext,
	loadFixture,
	login,
	search,
	setCover,
	tinyPng,
	uniqueToken,
	uploadImage
} from './helpers';

// Follows the conventions of recipes.test.ts: unique usernames per test,
// because desktop and mobile run against one binary and one database.

/** The people card. Toasts render as listitems too, but outside it, so its rows are the only listitems inside. */
function people(page: Page): Locator {
	return page.getByRole('region', { name: 'People' });
}

/** One person's row, matched on the login name, which the row always shows. */
function personRow(page: Page, username: string): Locator {
	return people(page)
		.getByRole('listitem')
		.filter({ has: page.getByText(username, { exact: true }) });
}

/**
 * Opens a person's page from their card, the one way to their account's
 * actions: the card is one link.
 */
async function openPerson(page: Page, username: string): Promise<void> {
	await personRow(page, username).getByRole('link').click();
	await expect(page).toHaveURL(`/settings/users/${username}`);
}

/**
 * Opens the own password dialog from its row on the profile page, and waits
 * for Bits UI to focus its first field, a frame after it opens: a fill of a
 * later field that frame lands in would type into the first one.
 */
async function openPasswordDialog(page: Page): Promise<void> {
	await page.getByRole('button', { name: 'Change password' }).click();
	const dialog = page.getByRole('dialog', { name: 'Change password' });
	await expect(dialog).toBeVisible();
	await expect.poll(() => dialog.evaluate((el) => el.contains(document.activeElement))).toBe(true);
}

test('a member changes the own password and logs in with it', async ({ page }) => {
	const username = `pw${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'user' });

	await page.context().clearCookies();
	await login(page, username);
	await expect(page).toHaveURL('/');
	await page.goto('/settings');
	await openPasswordDialog(page);
	await page.getByLabel('Current password').fill(devPassword(username));
	await page.getByLabel('New password', { exact: true }).fill(devPasswordNext(username));
	await page.getByLabel('Repeat the new password').fill(devPasswordNext(username));
	await page.getByRole('dialog').getByRole('button', { name: 'Save', exact: true }).click();
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
	await openPasswordDialog(page);
	await page.getByLabel('Current password').fill('definitely-wrong');
	// The instance owner, whose own password this test never changes - it is
	// rejected on the current one.
	await page.getByLabel('New password', { exact: true }).fill(devPasswordNext('admin'));
	await page.getByLabel('Repeat the new password').fill(devPasswordNext('admin'));
	await page.getByRole('dialog').getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page.getByText('The current password is wrong')).toBeVisible();
});

test('the own password change refuses a new password typed differently twice', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings');
	await openPasswordDialog(page);
	// Stopped before the API, so the owner's password is never at risk here.
	await page.getByLabel('Current password').fill(devPassword('admin'));
	await page.getByLabel('New password', { exact: true }).fill(devPasswordNext('admin'));
	const repeat = page.getByLabel('Repeat the new password');
	await repeat.fill(`${devPasswordNext('admin')}x`);
	await page.getByRole('dialog').getByRole('button', { name: 'Save', exact: true }).click();

	await expect(page.getByText('The passwords do not match')).toBeVisible();
	await repeat.fill(devPasswordNext('admin'));
	await expect(repeat).not.toHaveAttribute('aria-invalid', 'true');
});

test('fixing the first password field clears the mismatch too', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings');
	await openPasswordDialog(page);
	await page.getByLabel('Current password').fill(devPassword('admin'));
	const next = page.getByLabel('New password', { exact: true });
	const repeat = page.getByLabel('Repeat the new password');
	await next.fill(`${devPasswordNext('admin')}x`);
	await repeat.fill(devPasswordNext('admin'));
	await page.getByRole('dialog').getByRole('button', { name: 'Save', exact: true }).click();
	await expect(repeat).toHaveAttribute('aria-invalid', 'true');

	// The mismatch is about the pair, so retyping either half answers it.
	await next.fill(devPasswordNext('admin'));
	await expect(repeat).not.toHaveAttribute('aria-invalid', 'true');
});

test('the new password is rated as it is typed, and only advised on', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings');
	await openPasswordDialog(page);
	const next = page.getByLabel('New password', { exact: true });
	await next.fill('password1');
	await expect(page.getByText('Strength: very weak')).toBeVisible();
	await next.fill('tangerine-orbit-velvet-harbor-91');
	await expect(page.getByText('Strength: strong')).toBeVisible();
	await next.fill('');
	await expect(page.getByText(/^Strength:/)).toHaveCount(0);
});

test('a member sees everyone, read-only', async ({ page }) => {
	const username = `m${uniqueToken()}`;
	const adminName = `a${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'user' });
	await createUser(page, { username: adminName, role: 'admin' });
	await page.context().clearCookies();
	await login(page, username);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');
	await expect(page).toHaveURL('/settings/users');

	// Everyone, grouped under their role: the owner, the admin and the member
	// themselves, each with the login name - who takes part, not only who
	// wrote a recipe.
	await expect(people(page).getByRole('list', { name: 'Owner' })).toContainText('admin');
	await expect(people(page).getByRole('list', { name: 'Admins' })).toContainText(adminName);
	await expect(people(page).getByRole('list', { name: 'Members' })).toContainText(username);
	await expect(
		page.getByText('Everyone with an account here. Admins manage accounts.')
	).toBeVisible();

	// Nothing to act on: no way to add an account, and no card but the own
	// one leads anywhere. The one button a card keeps is its circle, which
	// only shows who that person is. The counts are there for everyone.
	await expect(page.getByRole('button', { name: 'Add account' })).toHaveCount(0);
	await expect(people(page).getByRole('button', { name: /^(?!About )/ })).toHaveCount(0);
	await expect(people(page).getByRole('button', { name: `About ${adminName}` })).toBeVisible();
	await expect(personRow(page, adminName)).toContainText('No shopping list');
	await expect(personRow(page, adminName)).toContainText('No recipes');
	await expect(page.getByText(/What is on a list only the people/)).toBeVisible();
	// An address and its state are an admin's business: not even the symbol.
	await expect(people(page)).not.toContainText('Address');
});

test('a member sees the list without links', async ({ page }) => {
	const username = `m${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'user' });
	await createUser(page, { username: `n${uniqueToken()}`, role: 'user' });
	await page.context().clearCookies();
	await login(page, username);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');

	await expect(personRow(page, 'admin')).toBeVisible();
	await expect(people(page).locator('a[href^="/settings/users/"]')).toHaveCount(0);
	await expect(people(page).getByRole('link')).toHaveCount(1);
});

test('the own card leads to the profile page', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');

	const own = personRow(page, 'admin');
	await expect(own).toContainText('you');
	await expect(own.getByRole('link')).toHaveAttribute('href', '/settings');
	await own.getByRole('link').click();
	await expect(page).toHaveURL('/settings');
});

test("an admin reads a member's counts on the card and the rest on their page", async ({
	page
}) => {
	const username = `d${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'user', email: `${username}@example.com` });
	await page.context().clearCookies();
	await login(page, username);
	const fixture = loadFixture(0);
	fixture.title = `Soup ${username}`;
	await createRecipe(page, fixture);
	await page.context().clearCookies();
	await login(page);
	await page.goto('/settings/users');

	const row = personRow(page, username);
	await expect(row).toContainText('No shopping list');
	// The newest recipe's cover and title, and how many there are.
	await expect(row.locator('[data-thumb]')).toHaveCount(1);
	await expect(row).toContainText(`Soup ${username}`);
	await expect(row).toContainText('So far the only recipe');
	// The whole card is one link to the person's page, with no controls inside.
	await expect(row.getByRole('link')).toHaveCount(1);
	await expect(row.getByRole('link')).toHaveAttribute('href', `/settings/users/${username}`);
	await expect(row.getByRole('button')).toHaveCount(0);
	await expect(row.getByRole('combobox')).toHaveCount(0);
	await openPerson(page, username);
	const card = page.getByRole('region', { name: username, exact: true });
	await expect(card.getByText(`${username}@example.com`)).toBeVisible();
	const overview = card.getByRole('group', { name: 'Overview' });
	await expect(overview).toContainText('1 recipe');
	await expect(overview).toContainText(`latest “Soup ${username}”`);
	await expect(overview).toContainText('No shopping list');
	await expect(card.getByRole('button', { name: `${username}'s role` })).toBeVisible();
});

test('the card shows the four newest covers, newest first, and no recipes calmly', async ({
	page
}) => {
	const cook = `c${uniqueToken()}`;
	const idle = `i${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username: cook, role: 'user' });
	await createUser(page, { username: idle, role: 'user' });
	await page.context().clearCookies();
	await login(page, cook);
	const fixture = loadFixture(0);
	for (const n of [1, 2, 3, 4, 5]) {
		fixture.title = `Dish ${n} ${cook}`;
		const recipe = await createRecipe(page, fixture);
		if (n === 5)
			await setCover(page, recipe.id, (await uploadImage(page, recipe.id, tinyPng())).id);
	}
	await page.goto('/settings/users');

	const row = personRow(page, cook);
	const thumbs = row.locator('[data-thumb]');
	await expect(thumbs).toHaveCount(4);
	// The newest has a photo, an image the title beside it names.
	await expect(thumbs.first()).toHaveAttribute('alt', '');
	await expect(row).toContainText(`Dish 5 ${cook}`);
	await expect(row).toContainText('The newest of 5 recipes');
	await expect(personRow(page, idle)).toContainText('No recipes yet');
	await expect(personRow(page, idle).locator('[data-thumb]')).toHaveCount(0);
});

test('the owner creates, promotes, resets and deletes a user', async ({ page }) => {
	const username = `u${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');

	await page.getByRole('button', { name: 'Add account' }).click();
	const dialog = page.getByRole('dialog');
	await dialog.getByLabel('Username').fill(username);
	// "Send a setup link" is the default mode; this test wants the old
	// admin-hands-over-a-password form.
	await dialog.getByRole('radio', { name: 'Set a password now' }).click();
	await dialog.getByLabel('Password', { exact: true }).fill(devPassword(username));
	await dialog.getByLabel('Repeat the new password').fill(devPassword(username));
	await dialog.getByRole('radio', { name: /Member/ }).click();
	await dialog.getByRole('button', { name: 'Add', exact: true }).click();
	await expect(people(page).getByRole('list', { name: 'Members' })).toContainText(username);

	// The role is a select on the person's page - Bits UI renders its
	// trigger as a plain <button> named "<name>'s role".
	await openPerson(page, username);
	const card = page.getByRole('region', { name: username, exact: true });
	await card.getByRole('button', { name: `${username}'s role` }).click();
	await page.getByRole('option', { name: 'Admin' }).click();
	await expect(page.getByText(`${username} is now Admin`)).toBeVisible();
	await expect(card.getByRole('button', { name: `${username}'s role` })).toBeFocused();

	await card.getByRole('button', { name: `Reset ${username}'s password` }).click();
	const reset = page.getByRole('dialog', { name: `Reset ${username}'s password` });
	await reset.getByLabel('New password', { exact: true }).fill(devPasswordNext(username));
	await reset.getByLabel('Repeat the new password').fill(devPasswordNext(username));
	await reset.getByRole('button', { name: 'Reset', exact: true }).click();
	await expect(page.getByText('Password reset')).toBeVisible();

	await card.getByRole('button', { name: 'Delete account', exact: true }).click();
	await page
		.getByRole('dialog', { name: 'Delete account?' })
		.getByRole('button', { name: 'Delete' })
		.click();
	await expect(page).toHaveURL('/settings/users');
	await expect(people(page).getByRole('list', { name: 'Admins' })).toBeVisible();
	await expect(personRow(page, username)).toHaveCount(0);
});

test('a reset password has to be typed the same twice', async ({ page }) => {
	const username = `u${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'user' });
	await page.goto(`/settings/users/${username}`);

	await page.getByRole('button', { name: `Reset ${username}'s password` }).click();
	const dialog = page.getByRole('dialog', { name: `Reset ${username}'s password` });
	const repeat = dialog.getByLabel('Repeat the new password');

	await dialog.getByLabel('New password', { exact: true }).fill(devPasswordNext(username));
	await repeat.fill(`${devPasswordNext(username)}x`);
	await dialog.getByRole('button', { name: 'Reset', exact: true }).click();
	await expect(dialog.getByText('The passwords do not match')).toBeVisible();
	await expect(repeat).toHaveAttribute('aria-invalid', 'true');

	// The mismatch answered the last attempt; retyping the field clears it.
	await repeat.fill(devPasswordNext(username));
	await expect(repeat).not.toHaveAttribute('aria-invalid', 'true');

	await dialog.getByRole('button', { name: 'Reset', exact: true }).click();
	await expect(page.getByText('Password reset')).toBeVisible();
});

test('adding an account refuses a password typed differently twice', async ({ page }) => {
	const username = `u${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');

	await page.getByRole('button', { name: 'Add account' }).click();
	const dialog = page.getByRole('dialog', { name: 'Add account' });
	await dialog.getByRole('radio', { name: 'Set a password now' }).click();
	const repeat = dialog.getByLabel('Repeat the new password');
	await dialog.getByLabel('Username').fill(username);
	await dialog.getByLabel('Password', { exact: true }).fill(devPassword(username));
	await repeat.fill(`${devPassword(username)}x`);
	await dialog.getByRole('button', { name: 'Add', exact: true }).click();

	await expect(dialog.getByText('The passwords do not match')).toBeVisible();
	await repeat.fill(devPassword(username));
	await expect(repeat).not.toHaveAttribute('aria-invalid', 'true');
	await dialog.getByRole('button', { name: 'Add', exact: true }).click();
	await expect(people(page).getByRole('list', { name: 'Members' })).toContainText(username);
});

test('every field that sets a password says how long it has to be', async ({ page }) => {
	const username = `u${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'user' });

	await page.goto('/settings');
	await openPasswordDialog(page);
	await expect(page.getByLabel('New password', { exact: true })).toHaveAccessibleDescription(
		'At least 8 characters'
	);

	await page.goto('/settings/users');
	await page.getByRole('button', { name: 'Add account' }).click();
	const add = page.getByRole('dialog', { name: 'Add account' });
	await add.getByRole('radio', { name: 'Set a password now' }).click();
	await expect(add.getByLabel('Password', { exact: true })).toHaveAccessibleDescription(
		'At least 8 characters'
	);
	await page.keyboard.press('Escape');
	await expect(add).toBeHidden();

	await page.goto(`/settings/users/${username}`);
	await page.getByRole('button', { name: `Reset ${username}'s password` }).click();
	await expect(
		page
			.getByRole('dialog', { name: `Reset ${username}'s password` })
			.getByLabel('New password', { exact: true })
	).toHaveAccessibleDescription('At least 8 characters');
});

test('a role change is undone from its toast', async ({ page }) => {
	const username = `r${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'user' });
	await page.goto('/settings/users');

	await openPerson(page, username);
	await page.getByRole('button', { name: `${username}'s role` }).click();
	await page.getByRole('option', { name: 'Admin' }).click();
	await expect(page.getByText(`${username} is now Admin`)).toBeVisible();

	await page.getByRole('button', { name: 'Undo' }).click();
	await expect(page.getByText(`${username} is now Member`)).toBeVisible();
	await page
		.getByRole('navigation', { name: 'Path' })
		.getByRole('link', { name: 'People' })
		.click();
	await expect(people(page).getByRole('list', { name: 'Members' })).toContainText(username);
});

test('the people list groups everyone under their role', async ({ page }) => {
	const member = `z${uniqueToken()}`;
	const admin = `y${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username: member, role: 'user' });
	await createUser(page, { username: admin, role: 'admin' });
	await page.goto('/settings/users');

	// Owner first, then admins, then members - the headings in that order,
	// and each person under the one that names their role.
	const headings = people(page).getByRole('heading', { level: 3 });
	await expect(headings).toHaveText([/^Owner/, /^Admins/, /^Members/]);
	await expect(people(page).getByRole('list', { name: 'Owner' })).toContainText('admin');
	await expect(people(page).getByRole('list', { name: 'Admins' })).toContainText(admin);
	await expect(people(page).getByRole('list', { name: 'Members' })).toContainText(member);
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

	// The owner's card leads nowhere: there is nothing on it to act on. Its
	// circle still opens who the person is.
	await expect(people(page).getByRole('list', { name: 'Owner' })).toContainText('admin');
	const owner = personRow(page, 'admin');
	await expect(owner.getByRole('link')).toHaveCount(0);
	await expect(owner.getByRole('button')).toHaveCount(1);
	await expect(owner.getByText('Owner', { exact: true })).toBeVisible();
});

test('a plain admin manages a member but cannot change their role', async ({ page }) => {
	const admin = `a${uniqueToken()}`;
	const other = `o${uniqueToken()}`;
	const member = `m${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username: admin, role: 'admin' });
	await createUser(page, { username: other, role: 'admin' });
	await createUser(page, { username: member, role: 'user' });

	await page.context().clearCookies();
	await login(page, admin);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');

	// Another admin is the owner's to manage: their card leads nowhere.
	await expect(personRow(page, other).getByRole('link')).toHaveCount(0);
	// Changing a role is the owner's alone, so there is no role control -
	// while delete and reset stay this admin's to use.
	await openPerson(page, member);
	await expect(page.getByRole('button', { name: `${member}'s role` })).toHaveCount(0);
	await expect(page.getByRole('button', { name: 'Delete account', exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: `Reset ${member}'s password` })).toBeVisible();
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
	await page.getByRole('button', { name: 'Add account' }).click();
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

test('the palette stays open while the overview finishes loading', async ({ page }) => {
	const token = uniqueToken();
	await login(page);
	await expect(page).toHaveURL('/');
	const fixture = loadFixture(3);
	fixture.title = `Slow list ${token}`;
	await createRecipe(page, fixture);

	// The overview's list answers only after the palette is open; the
	// palette's own search (`limit=8`) goes through.
	let release = () => {};
	const released = new Promise<void>((resolve) => (release = resolve));
	let arrived = () => {};
	const held = new Promise<void>((resolve) => (arrived = resolve));
	await page.route(
		(url) => url.pathname === '/api/v1/recipes' && !url.searchParams.has('limit'),
		async (route) => {
			arrived();
			await released;
			await route.continue();
		}
	);
	await page.goto('/');
	await held;
	await page.keyboard.press('Control+k');
	const dialog = page.getByRole('dialog', { name: 'Command palette' });
	await expect(dialog).toBeVisible();

	const listed = page.waitForResponse(
		(response) =>
			new URL(response.url()).pathname === '/api/v1/recipes' &&
			!new URL(response.url()).searchParams.has('limit')
	);
	release();
	await listed;
	await dialog.getByRole('combobox').fill(token);
	await expect(dialog.getByRole('option', { name: fixture.title })).toBeVisible();
});

test('the palette stays open when a search it interrupted lands', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	// The search's list answers only after the palette is open, and its URL
	// differs from the one the palette opened over.
	let release = () => {};
	const released = new Promise<void>((resolve) => (release = resolve));
	let arrived = () => {};
	const held = new Promise<void>((resolve) => (arrived = resolve));
	await page.route(
		(url) => url.pathname === '/api/v1/recipes' && url.searchParams.get('q') === 'soup',
		async (route) => {
			arrived();
			await released;
			await route.continue();
		}
	);
	await page.getByRole('combobox', { name: 'Search recipes' }).fill('soup');
	await page.keyboard.press('Enter');
	await held;
	await page.keyboard.press('Control+k');
	const dialog = page.getByRole('dialog', { name: 'Command palette' });
	await expect(dialog).toBeVisible();

	release();
	await expect(page).toHaveURL(/q=soup/);
	// Still there to type into once the navigation has settled.
	await page.waitForLoadState('networkidle');
	await dialog.getByRole('combobox').fill('settings');
	await expect(dialog.getByRole('option').first()).toBeVisible();
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
	await createUser(page, { username, role: 'user', color: 'amber' });

	await page.context().clearCookies();
	await login(page, username);
	await expect(page).toHaveURL('/');
	await createRecipe(page, { ...loadFixture(0), title: `Color test ${token}` });

	await page.goto('/settings');
	// The pencil turns the name into a field in place.
	await page.getByRole('button', { name: 'Change name' }).click();
	await page.getByLabel('Display name').fill(displayName);
	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page.getByRole('heading', { name: displayName, level: 2 })).toBeVisible();

	// Picking a swatch is the save, after a short debounce; the band follows
	// the pick at once, so the round trip is read off the request instead.
	const saved = page.waitForResponse(
		(r) => r.url().endsWith('/api/v1/auth/me/profile') && r.request().method() === 'PATCH'
	);
	await page.getByRole('radio', { name: 'Sage' }).click();
	expect((await saved).ok()).toBe(true);

	// The recipe was written before the rename, and its card carries the name
	// and the color the account holds now: both are joined from `users` on
	// every read rather than copied onto the recipe.
	await page.goto('/');
	await search(page, token);
	const circle = page.getByLabel(`Added by ${displayName}`).locator('span').first();
	await expect(circle).toHaveClass(/bg-user-sage/);
	await expect(circle).toHaveText('S');
});

test('the profile page shows who else wears the own color', async ({ page }) => {
	const token = uniqueToken();
	const member = `amb${token}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username: member, role: 'user', color: 'amber' });
	// Unique, because every test shares one database: others may wear amber
	// too, so the visible line may count Sofia among "N more". The
	// screen-reader list names everyone in full.
	const sofia = `Sofia Safran ${token}`;
	await createUser(page, {
		username: `sof${token}`,
		role: 'user',
		displayName: sofia,
		color: 'amber'
	});

	await page.context().clearCookies();
	await login(page, member);
	await page.goto('/settings');
	await expect(
		page.getByRole('radiogroup', { name: 'Color', exact: true })
	).toHaveAccessibleDescription(new RegExp(`Amber.*also .*${sofia}`));
	await expect(page.getByText('Sharing a color is fine.')).toBeVisible();
});

test('a 64-character display name fits the profile head on a 320px phone', async ({ page }) => {
	const username = `long${uniqueToken()}`;
	const displayName = 'Konstantin '.repeat(6).slice(0, 64);
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'user', displayName });
	await page.context().clearCookies();
	await page.setViewportSize({ width: 320, height: 780 });
	await login(page, username);
	await page.goto('/settings');

	await expect(page.getByRole('heading', { name: displayName.trim(), level: 2 })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Change name' })).toBeInViewport();
	expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(320);
});

test('the e-mail row says when there is no address and offers a send for an unconfirmed one', async ({
	page
}) => {
	const token = uniqueToken();
	const bare = `bare${token}`;
	const unconfirmed = `unc${token}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username: bare, role: 'user' });
	await createUser(page, {
		username: unconfirmed,
		role: 'user',
		email: `${unconfirmed}@example.com`
	});

	await page.context().clearCookies();
	await login(page, bare);
	await page.goto('/settings');
	const account = page.getByRole('group', { name: 'Your account' });
	await expect(account.getByText('none', { exact: true })).toBeVisible();

	await page.context().clearCookies();
	await login(page, unconfirmed);
	// Mail is on or off on the shared instance; the row offers a send only
	// while it is on, so it is on here, and the send itself is answered here.
	await page.route('**/api/v1/settings', async (route) => {
		if (route.request().method() !== 'GET') return route.continue();
		const response = await route.fetch();
		return route.fulfill({ response, json: { ...(await response.json()), mailEnabled: true } });
	});
	let resent = false;
	await page.route('**/api/v1/auth/me/email/confirmation', (route) => {
		resent = true;
		return route.fulfill({ status: 204 });
	});
	await page.goto('/settings');
	await expect(account.getByText(`${unconfirmed}@example.com`, { exact: true })).toBeVisible();
	await expect(account.getByText('unconfirmed', { exact: true })).toBeVisible();
	// With mail on the account was created with a confirmation already out
	// ("Send again"); with it off, none went ("Send").
	await account.getByRole('button', { name: /^(Send|Send again)$/ }).click();
	await expect(
		page.getByText(`Confirmation mail sent to ${unconfirmed}@example.com`)
	).toBeVisible();
	expect(resent).toBe(true);
});

test("an admin changes a member's address but not their name, the owner can", async ({ page }) => {
	const token = uniqueToken();
	const member = `ren${token}`;
	const admin = `adm${token}`;
	const displayName = `Renamed ${token}`;
	await login(page);
	await expect(page).toHaveURL('/');
	// Not teal, so picking Teal below is a change whatever the least worn color is.
	await createUser(page, { username: member, role: 'user', color: 'rose' });
	await createUser(page, { username: admin, role: 'admin' });

	// Managing a member is administration and takes in their address;
	// renaming them is not, so an admin who is not the owner gets the
	// address row and a head without a pencil. The refusal underneath it has
	// no path through the UI and is covered by the Go handler test.
	await page.context().clearCookies();
	await login(page, admin);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');
	await openPerson(page, member);
	await expect(page.getByRole('button', { name: 'Change name' })).toHaveCount(0);
	await page.getByRole('button', { name: `Change ${member}'s email` }).click();
	const emailDialog = page.getByRole('dialog', { name: 'Email address' });
	await emailDialog.getByLabel('Email').fill(`${member}@example.com`);
	// Mail is on or off on the shared instance, depending on which file ran first.
	await emailDialog.getByRole('button', { name: /^Save( and send mail)?$/ }).click();
	await expect(
		page.getByText(/^(Email address saved|Saved\. Confirmation mail sent to .*)$/)
	).toBeVisible();
	await expect(emailDialog).toBeHidden();
	// The new address is unconfirmed at once, on the page and on the card,
	// without a reload.
	await expect(page.getByText('unconfirmed', { exact: true })).toBeVisible();
	await page.goBack();
	await expect(personRow(page, member)).toContainText('Address not confirmed yet');

	// The `admin` the e2e task seeds is the instance owner, and renaming
	// somebody else is theirs alone.
	await page.context().clearCookies();
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');
	await openPerson(page, member);
	await page.getByRole('button', { name: 'Change name' }).click();
	await page.getByLabel('Display name').fill(displayName);
	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page.getByText(`Renamed to ${displayName}`)).toBeVisible();
	// The color is saved 400ms after the last pick; wait for that write
	// instead of racing the toast's own timeout on a loaded machine.
	const saved = page.waitForResponse(
		(r) => r.request().method() === 'PATCH' && r.url().includes('/api/v1/users/')
	);
	await page.getByRole('radio', { name: 'Teal' }).click();
	expect((await saved).ok()).toBe(true);
	await expect(page.getByText(`${displayName} now wears Teal`)).toBeVisible();
	await page.goBack();
	const row = personRow(page, member);
	await expect(row).toContainText(displayName);
	await expect(row.locator('span.size-9')).toHaveClass(/bg-user-teal/);
});

test('the settings page links to the user guide in a new tab', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings');
	const help = page.getByRole('link', { name: /Help & guide/ });
	await expect(help).toBeVisible();
	await expect(help).toHaveAttribute('href', 'https://getrezepte.app/guide/');
	await expect(help).toHaveAttribute('target', '_blank');
});

test('the settings page says which Rezepte runs and where it comes from', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings');
	const about = page.getByRole('region', { name: 'About Rezepte' });
	await expect(about).toContainText(/Version \S+/);
	await expect(about.getByRole('link', { name: /Source code/ })).toHaveAttribute(
		'href',
		'https://github.com/s-frei/rezepte'
	);
	await expect(about.getByRole('link', { name: /Changelog/ })).toHaveAttribute(
		'href',
		'https://getrezepte.app/changelog/'
	);
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
	await expect(sheet.getByRole('link')).toHaveText([
		'Profile',
		'Notifications',
		'Shared links',
		'API',
		'People',
		'Email',
		'Sign-in',
		'Import & export'
	]);
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

test('the add-account dialog names who already wears the picked color', async ({ page }) => {
	const username = `f${uniqueToken()}`;
	// Unique, because every test shares one database: other people may wear
	// rose too, so the visible line may count Finn among "N more". The
	// screen-reader list names everyone in full.
	const displayName = `Finn Fenchel ${username}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'user', displayName, color: 'rose' });
	await page.goto('/settings/users');
	await page.getByRole('button', { name: 'Add account' }).click();

	const dialog = page.getByRole('dialog', { name: 'Add account' });
	await dialog.getByRole('radio', { name: 'Rose' }).click();
	await expect(
		dialog.getByRole('radiogroup', { name: 'Color', exact: true })
	).toHaveAccessibleDescription(new RegExp(`Rose.*also .*${displayName}`));
	await expect(dialog).toContainText('also ');
	await expect(dialog).toContainText('Sharing a color is fine.');
});

test('the add-account dialog fits a short phone screen and scrolls', async ({ page }) => {
	// 360 wide, as docs/memory's verify-ui asks for, and short enough to
	// stand for a phone with its browser bar and keyboard up: the form is
	// taller than that, so it must scroll inside the dialog instead of
	// pushing its title and its buttons off the screen.
	await page.setViewportSize({ width: 360, height: 560 });
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');
	await page.getByRole('button', { name: 'Add account' }).click();

	const dialog = page.getByRole('dialog');
	await expect(dialog.getByRole('heading', { name: 'Add account' })).toBeInViewport();
	const box = await dialog.boundingBox();
	expect(box!.y).toBeGreaterThanOrEqual(0);
	expect(box!.y + box!.height).toBeLessThanOrEqual(560);

	const submit = dialog.getByRole('button', { name: 'Add', exact: true });
	await submit.scrollIntoViewIfNeeded();
	await expect(submit).toBeInViewport();
});

test('the account menu switches the theme and stays open', async ({ page, isMobile }) => {
	test.skip(isMobile, 'the account menu is the desktop top bar; a phone has the You sheet');
	await login(page);
	await expect(page).toHaveURL('/');

	await page.getByRole('button', { name: 'Account menu' }).click();
	const menu = page.getByRole('menu');
	const theme = menu.getByRole('group', { name: 'Appearance' });
	await expect(theme.getByRole('menuitemradio', { name: 'System' })).toHaveAttribute(
		'aria-checked',
		'true'
	);

	await theme.getByRole('menuitemradio', { name: 'Dark' }).click();
	await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
	await expect(menu).toBeVisible();

	await theme.getByRole('menuitemradio', { name: 'System' }).click();
	await expect(page.locator('html')).not.toHaveAttribute('data-theme');
});

test('mail card stacks left-aligned on a 320px phone', async ({ page }) => {
	// A configured card with a sender too long for one line, answered here
	// rather than saved: mail is instance-wide, and mail.test.ts owns it.
	await page.route('**/api/v1/settings/mail', (route) =>
		route.request().method() === 'GET'
			? route.fulfill({
					json: {
						source: 'settings',
						host: 'mail.example.org',
						port: 587,
						security: 'starttls',
						username: 'rezepte',
						passwordSet: true,
						from: 'notifications.for.the.family.kitchen@rezepte.example.org',
						fromName: '',
						publicUrlMissing: false
					}
				})
			: route.continue()
	);
	await page.setViewportSize({ width: 320, height: 700 });
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/mail');
	const values = page.getByRole('region', { name: 'Email' }).locator('dd');
	await expect(values).toHaveCount(3);
	const measured = await values.evaluateAll((dds) =>
		dds.map((dd) => ({
			align: getComputedStyle(dd).textAlign,
			fits: dd.scrollWidth <= dd.clientWidth
		}))
	);
	expect(measured).toEqual(measured.map(() => ({ align: 'left', fits: true })));

	// The address wraps where it reads well, before "@" or ".", never inside
	// a label: every line after the first starts with one of the two.
	const lineStarts = await values.first().evaluate((dd) => {
		const starts: string[] = [];
		const walker = document.createTreeWalker(dd, NodeFilter.SHOW_TEXT);
		let top: number | null = null;
		for (let node = walker.nextNode(); node; node = walker.nextNode()) {
			const text = node.textContent ?? '';
			for (let i = 0; i < text.length; i++) {
				const range = document.createRange();
				range.setStart(node, i);
				range.setEnd(node, i + 1);
				const rect = range.getBoundingClientRect();
				if (!rect.height || !text[i].trim()) continue;
				if (top !== null && rect.top > top + 2) starts.push(text[i]);
				top = rect.top;
			}
		}
		return starts;
	});
	expect(lineStarts.length).toBeGreaterThan(0);
	expect(lineStarts.every((c) => c === '@' || c === '.')).toBe(true);
});

// The menu sits in the top bar, which every page keeps, so nothing closed it
// when the back button left the page under it.
test('the account menu closes with the back button', async ({ page, isMobile }) => {
	test.skip(isMobile, 'the account menu is the desktop top bar');
	await login(page);
	const menuButton = page.getByRole('button', { name: 'Account menu' });
	await menuButton.click();
	await page.getByRole('menuitem', { name: 'Settings' }).click();
	await expect(page).toHaveURL('/settings');

	await menuButton.click();
	await expect(page.getByRole('menu')).toBeVisible();
	await page.goBack();
	await expect(page).toHaveURL('/');
	await expect(page.getByRole('menu')).toHaveCount(0);
});

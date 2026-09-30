import { expect, test, type Locator, type Page } from '@playwright/test';
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
 * Where an admin acts on one person: the row itself on a wide screen, the
 * sheet the row opens on a phone. The sheet is titled with the display name,
 * which these tests leave at the login name.
 */
async function controlsFor(page: Page, isMobile: boolean, username: string): Promise<Locator> {
	const row = personRow(page, username);
	if (!isMobile) return row;
	await row.getByRole('button', { name: `Manage ${username}` }).click();
	const sheet = page.getByRole('dialog', { name: username, exact: true });
	await expect(sheet).toBeVisible();
	return sheet;
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

test('the own password change refuses a new password typed differently twice', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings');
	// Stopped before the API, so the owner's password is never at risk here.
	await page.getByLabel('Current password').fill(devPassword('admin'));
	await page.getByLabel('New password', { exact: true }).fill(devPasswordNext('admin'));
	const repeat = page.getByLabel('Repeat the new password');
	await repeat.fill(`${devPasswordNext('admin')}x`);
	await page.getByRole('button', { name: 'Save password' }).click();

	await expect(page.getByText('The passwords do not match')).toBeVisible();
	await repeat.fill(devPasswordNext('admin'));
	await expect(repeat).not.toHaveAttribute('aria-invalid', 'true');
});

test('fixing the first password field clears the mismatch too', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings');
	await page.getByLabel('Current password').fill(devPassword('admin'));
	const next = page.getByLabel('New password', { exact: true });
	const repeat = page.getByLabel('Repeat the new password');
	await next.fill(`${devPasswordNext('admin')}x`);
	await repeat.fill(devPasswordNext('admin'));
	await page.getByRole('button', { name: 'Save password' }).click();
	await expect(repeat).toHaveAttribute('aria-invalid', 'true');

	// The mismatch is about the pair, so retyping either half answers it.
	await next.fill(devPasswordNext('admin'));
	await expect(repeat).not.toHaveAttribute('aria-invalid', 'true');
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

test('a member sees everyone, read-only', async ({ page, isMobile }) => {
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

	// Nothing to act on: no way to add an account, and not one control in
	// any row - no role select, no pencil, no reset, no delete, and on a
	// phone no row that opens anything but their own profile.
	await expect(page.getByRole('button', { name: 'Add account' })).toHaveCount(0);
	await expect(people(page).getByRole('button')).toHaveCount(0);
	await expect(page.getByText('This account cannot be removed.')).toHaveCount(0);
	if (isMobile) {
		await personRow(page, username).getByRole('link', { name: 'Open your profile' }).click();
		await expect(page).toHaveURL('/settings');
	}
});

test('the owner creates, promotes, resets and deletes a user', async ({ page, isMobile }) => {
	const username = `u${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');

	await page.getByRole('button', { name: 'Add account' }).click();
	const dialog = page.getByRole('dialog');
	await dialog.getByLabel('Username').fill(username);
	await dialog.getByLabel('Password', { exact: true }).fill(devPassword(username));
	await dialog.getByLabel('Repeat the new password').fill(devPassword(username));
	await dialog.getByRole('radio', { name: /Member/ }).click();
	await dialog.getByRole('button', { name: 'Add', exact: true }).click();
	await expect(people(page).getByRole('list', { name: 'Members' })).toContainText(username);

	// Promoting moves them under "Admins". On a wide screen the role is a
	// select in the row - Bits UI renders its trigger as a plain <button>
	// named "<name>'s role" - and on a phone a segmented control in the sheet.
	let controls = await controlsFor(page, isMobile, username);
	// Moving to another group re-creates the row, and neither the control
	// just used nor the keyboard focus may get lost on the way.
	const manage = personRow(page, username).getByRole('button', { name: `Manage ${username}` });
	if (isMobile) {
		const role = controls.getByRole('radiogroup', { name: `${username}'s role` });
		await role.getByRole('radio', { name: 'Admin' }).click();
		await expect(page.getByText(`${username} is now Admin`)).toBeVisible();
		await expect(controls).toBeVisible();
		await expect(role.getByRole('radio', { name: 'Admin' })).toHaveAttribute(
			'aria-checked',
			'true'
		);
		await page.keyboard.press('Escape');
		await expect(manage).toBeFocused();
	} else {
		await controls.getByRole('button', { name: `${username}'s role` }).click();
		await page.getByRole('option', { name: 'Admin' }).click();
		await expect(page.getByText(`${username} is now Admin`)).toBeVisible();
		await expect(
			personRow(page, username).getByRole('button', { name: `${username}'s role` })
		).toBeFocused();
	}
	await expect(people(page).getByRole('list', { name: 'Admins' })).toContainText(username);

	// The visible words are part of every name, so what a voice-control
	// user reads is what they can say.
	controls = await controlsFor(page, isMobile, username);
	await controls.getByRole('button', { name: 'Reset password' }).click();
	const reset = page.getByRole('dialog', { name: `Reset ${username}'s password` });
	await reset.getByLabel('New password', { exact: true }).fill(devPasswordNext(username));
	await reset.getByLabel('Repeat the new password').fill(devPasswordNext(username));
	await reset.getByRole('button', { name: 'Reset', exact: true }).click();
	await expect(page.getByText('Password reset')).toBeVisible();
	if (isMobile) await expect(manage).toBeFocused();

	controls = await controlsFor(page, isMobile, username);
	await controls
		.getByRole('button', { name: isMobile ? 'Delete account' : `Delete ${username}` })
		.click();
	await page
		.getByRole('dialog', { name: 'Delete account?' })
		.getByRole('button', { name: 'Delete' })
		.click();
	await expect(personRow(page, username)).toHaveCount(0);
	// The row focus would return to is gone; the card's heading takes it.
	await expect(people(page).getByRole('heading', { name: 'People', level: 2 })).toBeFocused();
});

test('a reset password has to be typed the same twice', async ({ page, isMobile }) => {
	const username = `u${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'user' });
	await page.goto('/settings/users');

	const controls = await controlsFor(page, isMobile, username);
	await controls.getByRole('button', { name: 'Reset password' }).click();
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

test('every field that sets a password says how long it has to be', async ({ page, isMobile }) => {
	const username = `u${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'user' });

	await page.goto('/settings');
	await expect(page.getByLabel('New password', { exact: true })).toHaveAccessibleDescription(
		'At least 8 characters'
	);

	await page.goto('/settings/users');
	await page.getByRole('button', { name: 'Add account' }).click();
	const add = page.getByRole('dialog', { name: 'Add account' });
	await expect(add.getByLabel('Password', { exact: true })).toHaveAccessibleDescription(
		'At least 8 characters'
	);
	await page.keyboard.press('Escape');
	await expect(add).toBeHidden();

	const controls = await controlsFor(page, isMobile, username);
	await controls.getByRole('button', { name: 'Reset password' }).click();
	await expect(
		page
			.getByRole('dialog', { name: `Reset ${username}'s password` })
			.getByLabel('New password', { exact: true })
	).toHaveAccessibleDescription('At least 8 characters');
});

test('a role change is undone from its toast', async ({ page, isMobile }) => {
	const username = `r${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'user' });
	await page.goto('/settings/users');

	const controls = await controlsFor(page, isMobile, username);
	if (isMobile) {
		await controls
			.getByRole('radiogroup', { name: `${username}'s role` })
			.getByRole('radio', { name: 'Admin' })
			.click();
	} else {
		await controls.getByRole('button', { name: `${username}'s role` }).click();
		await page.getByRole('option', { name: 'Admin' }).click();
	}
	await expect(people(page).getByRole('list', { name: 'Admins' })).toContainText(username);

	await page.getByRole('button', { name: 'Undo' }).click();
	await expect(page.getByText(`${username} is now Member`)).toBeVisible();
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

test('a second admin cannot touch the instance owner', async ({ page, isMobile }) => {
	const username = `a${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'admin' });

	await page.context().clearCookies();
	await login(page, username);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');

	// The owner's row offers nothing: no role control, no actions, and on a
	// phone no sheet to open.
	await expect(people(page).getByRole('list', { name: 'Owner' })).toContainText('admin');
	await expect(personRow(page, 'admin').getByRole('button')).toHaveCount(0);
	// A wide screen says why, where the other rows have their actions.
	if (!isMobile) {
		await expect(
			personRow(page, 'admin').getByText('This account cannot be removed.')
		).toBeVisible();
	}
});

test('a plain admin manages a member but cannot change their role', async ({ page, isMobile }) => {
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

	// Changing a role is the owner's alone, so there is no role control -
	// while delete and reset stay this admin's to use.
	const controls = await controlsFor(page, isMobile, member);
	await expect(controls.getByRole('button', { name: `${member}'s role` })).toHaveCount(0);
	await expect(controls.getByRole('radiogroup')).toHaveCount(0);
	await expect(
		controls.getByRole('button', { name: isMobile ? 'Delete account' : `Delete ${member}` })
	).toBeVisible();
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
	const avatar = page.locator('section[aria-labelledby="settings-profile"] span.size-24');
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

test('an admin cannot rename a member, the owner can', async ({ page, isMobile }) => {
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
	const asAdmin = await controlsFor(page, isMobile, member);
	await expect(
		asAdmin.getByRole('button', { name: isMobile ? 'Delete account' : `Delete ${member}` })
	).toBeVisible();
	await expect(asAdmin.getByRole('button', { name: 'Edit profile' })).toHaveCount(0);

	// The `admin` the e2e task seeds is the instance owner, and renaming
	// somebody else is theirs alone.
	await page.context().clearCookies();
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/users');
	const controls = await controlsFor(page, isMobile, member);
	await controls.getByRole('button', { name: 'Edit profile' }).click();
	const dialog = page.getByRole('dialog', { name: `Edit ${member}'s profile` });
	await dialog.getByLabel('Display name').fill(displayName);
	await dialog.getByRole('radio', { name: 'Teal' }).click();
	await dialog.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page.getByText('Profile saved')).toBeVisible();
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
	await expect(help).toHaveAttribute('href', 'https://s-frei.github.io/rezepte/guide/');
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
		'https://s-frei.github.io/rezepte/changelog/'
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
	await expect(sheet.getByRole('link')).toHaveText(['Profile', 'API', 'Shared links', 'People']);
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

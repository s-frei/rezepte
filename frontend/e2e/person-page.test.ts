import { expect, test, type Locator, type Page } from '@playwright/test';
import { createUser, devPassword, login, uniqueToken } from './helpers';

// Desktop and mobile run this file against one binary and one database, so
// every test makes its own people. Pages are opened by URL or from the
// people list; both lead to the same page.

/** The person page's card, named after the person. */
function card(page: Page, displayName: string): Locator {
	return page.getByRole('region', { name: displayName, exact: true });
}

/**
 * Answers `GET /users` with `lastActiveAt` set for one account, as the
 * service records it on a sign-in or a session's daily renewal.
 */
async function stubLastActive(page: Page, username: string, at: string | null): Promise<void> {
	await page.route('**/api/v1/users', async (route) => {
		if (route.request().method() !== 'GET') return route.fallback();
		const response = await route.fetch();
		const body = (await response.json()) as { items: { username: string }[] };
		for (const item of body.items) {
			if (item.username === username) Object.assign(item, { lastActiveAt: at });
		}
		await route.fulfill({ response, json: body });
	});
}

const daysAgo = (days: number) => new Date(Date.now() - days * 86_400_000).toISOString();

async function asOwner(page: Page): Promise<void> {
	await login(page);
	await expect(page).toHaveURL('/');
}

test('the owner opens a member from the list and goes back the same way', async ({ page }) => {
	const username = `pl${uniqueToken()}`;
	await asOwner(page);
	await createUser(page, { username, role: 'user' });
	await page.goto('/settings/users');

	// The whole card is the way on.
	const row = page.getByRole('listitem').filter({ has: page.getByText(username, { exact: true }) });
	await row.getByRole('link').click();
	await expect(page).toHaveURL(`/settings/users/${username}`);
	await expect(card(page, username)).toBeVisible();
	// The settings nav stays on People, and the trail names the person.
	const trail = page.getByRole('navigation', { name: 'Path' });
	await expect(trail).toContainText(username);
	await trail.getByRole('link', { name: 'People' }).click();
	await expect(page).toHaveURL('/settings/users');

	await page.goBack();
	await expect(page).toHaveURL(`/settings/users/${username}`);
	await page.goBack();
	await expect(page).toHaveURL('/settings/users');
});

test('the owner recolors a member on their page and undoes it', async ({ page }) => {
	const username = `pc${uniqueToken()}`;
	await asOwner(page);
	await createUser(page, { username, role: 'user', color: 'rose' });
	await page.goto(`/settings/users/${username}`);

	const head = card(page, username);
	await expect(head.getByRole('radio', { name: 'Rose' })).toBeChecked();
	await head.getByRole('radio', { name: 'Teal' }).click();
	await expect(page.getByText(`${username} now wears Teal`)).toBeVisible();
	await page.getByRole('button', { name: 'Undo' }).click();
	await expect(page.getByText(`${username} now wears Rose`)).toBeVisible();
	await expect(head.getByRole('radio', { name: 'Rose' })).toBeChecked();
	await page.reload();
	await expect(card(page, username).getByRole('radio', { name: 'Rose' })).toBeChecked();
});

test('a keyboard run across the colors saves once and undoes to the start', async ({ page }) => {
	const username = `pk${uniqueToken()}`;
	await asOwner(page);
	const user = await createUser(page, { username, role: 'user', color: 'rose' });
	await page.goto(`/settings/users/${username}`);

	let patches = 0;
	page.on('request', (r) => {
		if (r.method() === 'PATCH' && r.url().endsWith(`/api/v1/users/${user.id}`)) patches++;
	});
	const head = card(page, username);
	await head.getByRole('radio', { name: 'Rose' }).focus();
	// Rose, then Plum, Sage, Olive: one save for where the run lands.
	await page.keyboard.press('ArrowRight');
	await page.keyboard.press('ArrowRight');
	await page.keyboard.press('ArrowRight');
	await expect(page.getByText(`${username} now wears Olive`)).toBeVisible();
	await expect(page.getByText(/now wears (Plum|Sage)/)).toHaveCount(0);
	expect(patches).toBe(1);

	await page.getByRole('button', { name: 'Undo' }).click();
	await expect(page.getByText(`${username} now wears Rose`)).toBeVisible();
	await page.reload();
	await expect(card(page, username).getByRole('radio', { name: 'Rose' })).toBeChecked();
});

test('a color answer overtaken by a newer pick is dropped', async ({ page }) => {
	const username = `po${uniqueToken()}`;
	await asOwner(page);
	const user = await createUser(page, { username, role: 'user', color: 'rose' });
	await page.goto(`/settings/users/${username}`);

	// The first save reaches the service at once but answers late, after
	// the second one has landed.
	let first = true;
	await page.route(`**/api/v1/users/${user.id}`, async (route) => {
		if (route.request().method() !== 'PATCH' || !first) return route.fallback();
		first = false;
		const response = await route.fetch();
		await new Promise((r) => setTimeout(r, 1500));
		await route.fulfill({ response }).catch(() => {});
	});
	const head = card(page, username);
	await head.getByRole('radio', { name: 'Teal' }).click();
	await page.waitForTimeout(600);
	await head.getByRole('radio', { name: 'Sage' }).click();
	await expect(page.getByText(`${username} now wears Sage`)).toBeVisible();
	await page.waitForTimeout(1500);
	await expect(head.getByRole('radio', { name: 'Sage' })).toBeChecked();
	await expect(page.getByText(`${username} now wears Teal`)).toHaveCount(0);
});

test('a refused rename says why', async ({ page }) => {
	const username = `prf${uniqueToken()}`;
	await asOwner(page);
	const user = await createUser(page, { username, role: 'user' });
	await page.goto(`/settings/users/${username}`);

	await page.route(`**/api/v1/users/${user.id}`, (route) =>
		route.request().method() === 'PATCH'
			? route.fulfill({
					status: 409,
					contentType: 'application/problem+json',
					body: JSON.stringify({ title: 'Conflict', status: 409 })
				})
			: route.fallback()
	);
	await card(page, username).getByRole('button', { name: 'Change name' }).click();
	await page.getByLabel('Display name').fill(`Other ${username}`);
	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page.getByText('The owner cannot be changed.')).toBeVisible();
	await expect(page.getByText('Profile could not be saved.')).toHaveCount(0);
});

test('the owner renames a member inline', async ({ page }) => {
	const username = `pr${uniqueToken()}`;
	const name = `Renamed ${username}`;
	await asOwner(page);
	await createUser(page, { username, role: 'user' });
	await page.goto(`/settings/users/${username}`);

	await card(page, username).getByRole('button', { name: 'Change name' }).click();
	await page.getByLabel('Display name').fill(name);
	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page.getByText(`Renamed to ${name}`)).toBeVisible();
	await expect(card(page, name).getByRole('heading', { name })).toBeVisible();
	await expect(page.getByRole('navigation', { name: 'Path' })).toContainText(name);
});

test('a role change on the person page is undone from its toast', async ({ page }) => {
	const username = `pro${uniqueToken()}`;
	await asOwner(page);
	await createUser(page, { username, role: 'user' });
	await page.goto(`/settings/users/${username}`);

	const head = card(page, username);
	await head.getByRole('button', { name: `${username}'s role` }).click();
	await page.getByRole('option', { name: 'Admin' }).click();
	await expect(page.getByText(`${username} is now Admin`)).toBeVisible();
	await expect(head.getByText(`Admin, signs in as ${username}`)).toBeVisible();

	await page.getByRole('button', { name: 'Undo' }).click();
	await expect(page.getByText(`${username} is now Member`)).toBeVisible();
	await expect(head.getByText(`Member, signs in as ${username}`)).toBeVisible();
});

/** Holds the first PATCH to `id`'s account 1.5s after the service answered it. */
async function delayFirstPatch(page: Page, id: string): Promise<void> {
	let first = true;
	await page.route(`**/api/v1/users/${id}`, async (route) => {
		if (route.request().method() !== 'PATCH' || !first) return route.fallback();
		first = false;
		const response = await route.fetch();
		await new Promise((r) => setTimeout(r, 1500));
		await route.fulfill({ response }).catch(() => {});
	});
}

test('a role answer overtaken by a newer pick is dropped', async ({ page }) => {
	const username = `prq${uniqueToken()}`;
	await asOwner(page);
	const user = await createUser(page, { username, role: 'user' });
	await page.goto(`/settings/users/${username}`);
	await delayFirstPatch(page, user.id);

	const head = card(page, username);
	const role = head.getByRole('button', { name: `${username}'s role` });
	await role.click();
	await page.getByRole('option', { name: 'Admin' }).click();
	await role.click();
	await page.getByRole('option', { name: 'Member' }).click();
	await expect(page.getByText(`${username} is now Member`)).toBeVisible();
	await page.waitForTimeout(1500);
	await expect(head.getByText(`Member, signs in as ${username}`)).toBeVisible();
	await expect(page.getByText(`${username} is now Admin`)).toHaveCount(0);
});

test('a sharing answer overtaken by a newer flip is dropped', async ({ page }) => {
	const username = `prs${uniqueToken()}`;
	await asOwner(page);
	const user = await createUser(page, { username, role: 'user' });
	await page.goto(`/settings/users/${username}`);
	const sharing = card(page, username).getByRole('switch', { name: 'May share publicly' });
	const before = await sharing.isChecked();
	await delayFirstPatch(page, user.id);

	await sharing.click();
	await sharing.click();
	await page.waitForTimeout(2000);
	expect(await sharing.isChecked()).toBe(before);
});

test('an admin issues and revokes a setup link from the person page', async ({ page }) => {
	const username = `psl${uniqueToken()}`;
	await asOwner(page);
	await createUser(page, { username, role: 'user' });
	await page.goto(`/settings/users/${username}`);

	const head = card(page, username);
	await expect(head.getByText('Signs in with a password')).toBeVisible();
	await head.getByRole('button', { name: 'Create link' }).click();
	// mail.test.ts turns mail on for the shared instance; with mail on, a
	// step asks first where the link goes.
	const byHand = page.getByRole('radio', { name: 'Pass it on myself' });
	const dialog = page.getByRole('dialog', { name: `Setup link for ${username}` });
	await expect(byHand.or(page.locator('textarea'))).toBeVisible();
	if (await byHand.isVisible()) {
		await byHand.click();
		await page.getByRole('button', { name: 'Show link' }).click();
		// The step has finished closing behind the result.
		await expect(page.getByRole('dialog')).toHaveCount(1);
	}
	await expect(dialog.getByRole('textbox')).toHaveValue(/\/welcome#/);
	await page.keyboard.press('Escape');
	// Both the step and the result share the name while they play out.
	await expect(page.getByRole('dialog')).toHaveCount(0);

	await expect(head.getByText(/Setup link open until/)).toBeVisible();
	// "Create link" became "Manage"; focus moved over with it.
	await expect(head.getByRole('button', { name: 'Manage the setup link' })).toBeFocused();
	await head.getByRole('button', { name: 'Manage the setup link' }).click();
	await expect(page.getByRole('menuitem', { name: 'Create a new link' })).toBeVisible();
	await page.getByRole('menuitem', { name: 'Revoke' }).click();
	await expect(page.getByText('Setup link revoked')).toBeVisible();
	await expect(head.getByText('Signs in with a password')).toBeVisible();
	// "Manage" left with the link; "Create link" in its place takes focus.
	await expect(head.getByRole('button', { name: `Create link for ${username}` })).toBeFocused();
});

test('the owner resets a password from the person page', async ({ page }) => {
	const username = `prs${uniqueToken()}`;
	await asOwner(page);
	await createUser(page, { username, role: 'user' });
	await page.goto(`/settings/users/${username}`);

	await expect(card(page, username).getByText(`Signs ${username} out everywhere`)).toBeVisible();
	await card(page, username)
		.getByRole('button', { name: `Reset ${username}'s password` })
		.click();
	await expect(page.getByRole('dialog', { name: `Reset ${username}'s password` })).toBeVisible();
});

test('a reset gives an account without a password one on the person page', async ({ page }) => {
	const username = `pnp${uniqueToken()}`;
	await asOwner(page);
	const origin = new URL(page.url()).origin;
	const headers = { Origin: origin, 'Content-Type': 'application/json' };
	const created = await page.request.post('/api/v1/users', {
		headers,
		data: { username, role: 'user' }
	});
	const { id } = (await created.json()) as { id: string };
	await page.request.delete(`/api/v1/users/${id}/setup-link`, { headers });
	await page.goto(`/settings/users/${username}`);

	const head = card(page, username);
	await expect(head.getByText('Not set up yet')).toBeVisible();
	await head.getByRole('button', { name: `Reset ${username}'s password` }).click();
	const dialog = page.getByRole('dialog', { name: `Reset ${username}'s password` });
	await dialog.getByLabel('New password', { exact: true }).fill(devPassword(username));
	await dialog.getByLabel('Repeat the new password').fill(devPassword(username));
	await dialog.getByRole('button', { name: 'Reset', exact: true }).click();
	await expect(page.getByText('Password reset')).toBeVisible();
	await expect(head.getByText('Signs in with a password')).toBeVisible();
});

test('deleting from the person page returns to the list without the person', async ({ page }) => {
	const username = `pd${uniqueToken()}`;
	await asOwner(page);
	await createUser(page, { username, role: 'user' });
	await page.goto(`/settings/users/${username}`);

	await card(page, username).getByRole('button', { name: 'Delete account', exact: true }).click();
	await page
		.getByRole('dialog', { name: 'Delete account?' })
		.getByRole('button', { name: 'Delete' })
		.click();
	await expect(page).toHaveURL('/settings/users');
	await expect(page.getByText(`${username} deleted`)).toBeVisible();
	await expect(page.getByRole('region', { name: 'People' })).not.toContainText(username);
});

test('a plain admin sees a member read-only in the head but manages the account', async ({
	page
}) => {
	const admin = `pa${uniqueToken()}`;
	const member = `pm${uniqueToken()}`;
	await asOwner(page);
	await createUser(page, { username: admin, role: 'admin' });
	await createUser(page, { username: member, role: 'user', color: 'sage' });
	await page.context().clearCookies();
	await login(page, admin);
	await expect(page).toHaveURL('/');
	await page.goto(`/settings/users/${member}`);

	const head = card(page, member);
	await expect(head.getByRole('heading', { name: member })).toBeVisible();
	await expect(head.getByRole('button', { name: /^(Add|Change) photo$/ })).toHaveCount(0);
	await expect(head.getByRole('button', { name: 'Change name' })).toHaveCount(0);
	await expect(head.getByRole('radiogroup')).toHaveCount(0);
	await expect(head.getByText('Sage', { exact: true })).toBeVisible();
	// The role is the owner's to change; this admin reads it.
	await expect(head.getByRole('button', { name: `${member}'s role` })).toHaveCount(0);

	await expect(head.getByRole('button', { name: `Change ${member}'s email` })).toBeVisible();
	await expect(head.getByRole('button', { name: `Reset ${member}'s password` })).toBeVisible();
	await expect(head.getByRole('button', { name: `Create link for ${member}` })).toBeVisible();
	await expect(head.getByRole('switch', { name: 'May share publicly' })).toBeVisible();
	await expect(head.getByRole('button', { name: 'Delete account', exact: true })).toBeVisible();
});

test('typed person URLs redirect or say the person is unknown', async ({ page }) => {
	const admin = `pt${uniqueToken()}`;
	await asOwner(page);
	await createUser(page, { username: admin, role: 'admin' });
	await page.context().clearCookies();
	await login(page, admin);
	await expect(page).toHaveURL('/');

	await page.goto(`/settings/users/${admin}`);
	await expect(page).toHaveURL('/settings');
	await page.goto('/settings/users/admin');
	await expect(page).toHaveURL('/settings/users');
	await page.goto(`/settings/users/nobody${uniqueToken()}`);
	await expect(page.getByRole('heading', { name: 'No such account' })).toBeVisible();
	await page.getByRole('link', { name: 'Back to the people' }).click();
	await expect(page).toHaveURL('/settings/users');
});

test('the e-mail row on the person page shows no address and an unconfirmed one', async ({
	page
}) => {
	const username = `pe${uniqueToken()}`;
	await asOwner(page);
	await createUser(page, { username, role: 'user' });
	await page.goto(`/settings/users/${username}`);

	const head = card(page, username);
	await expect(head.getByText('No address')).toBeVisible();
	await head.getByRole('button', { name: `Change ${username}'s email` }).click();
	const dialog = page.getByRole('dialog', { name: 'Email address' });
	await dialog.getByLabel('Email').fill(`${username}@example.com`);
	// Mail is on or off on the shared instance, depending on which file ran first.
	await dialog.getByRole('button', { name: /^Save( and send mail)?$/ }).click();
	await expect(
		page.getByText(/^(Email address saved|Saved\. Confirmation mail sent to .*)$/)
	).toBeVisible();
	await expect(head.getByText(`${username}@example.com`)).toBeVisible();
	await expect(head.getByText('unconfirmed', { exact: true })).toBeVisible();
});

test('the overview says what they have, when they joined and were last active', async ({
	page
}) => {
	const username = `pf${uniqueToken()}`;
	await asOwner(page);
	await createUser(page, { username, role: 'user' });
	await stubLastActive(page, username, daysAgo(3));
	await page.goto(`/settings/users/${username}`);

	// First of the groups, before the account's.
	const groups = card(page, username).getByRole('group');
	await expect(groups.first()).toHaveAccessibleName('Overview');
	await expect(groups.first()).toContainText('No recipes');
	await expect(groups.first()).toContainText('No shopping list');
	await expect(groups.first()).toContainText(/Joined \w+ \d{4}/);
	await expect(groups.first()).toContainText('Active 3 days ago');
});

test('the people list shows an admin how long ago everyone was active', async ({ page }) => {
	const username = `pa${uniqueToken()}`;
	await asOwner(page);
	await createUser(page, { username, role: 'user' });
	await stubLastActive(page, username, daysAgo(45));
	await page.goto('/settings/users');

	const row = page.getByRole('listitem').filter({ has: page.getByText(username, { exact: true }) });
	// Dormant: 30 days or more, counted in weeks below 60 days.
	await expect(row.getByText('Active 6 weeks ago')).toBeVisible();
	await expect(row.locator('[data-activity]')).toHaveAttribute('data-activity', 'dormant');

	await page.unrouteAll({ behavior: 'ignoreErrors' });
	await stubLastActive(page, username, null);
	await page.reload();
	const fresh = page
		.getByRole('listitem')
		.filter({ has: page.getByText(username, { exact: true }) });
	await expect(fresh.getByText('No activity recorded yet')).toBeVisible();
	await expect(fresh.locator('[data-activity]')).toHaveAttribute('data-activity', 'none');
});

// 200% text as a phone's text-size setting gives it: the type tokens are px,
// so they are doubled along with the root font.
const doubleText = `html { font-size: 200% !important }
:root { --text-body-lg: 34px !important; --text-body-sm: 28px !important; --text-body: 30px !important;
  --text-caption: 26px !important; --text-card-sm: 32px !important; --text-heading: 44px !important;
  --text-display-sm: 60px !important; --text-label: 22px !important; --text-micro: 24px !important }`;

test('people cards keep their content inside with 200% text on a 360px phone', async ({ page }) => {
	const username = `pz${uniqueToken()}`;
	const displayName = 'Maximiliane Friederike Charlotte';
	await asOwner(page);
	await createUser(page, { username, role: 'user', displayName });
	await page.setViewportSize({ width: 360, height: 780 });
	await page.goto('/settings/users');
	await page.addStyleTag({ content: doubleText });

	const tile = page.locator(`[data-person="${username}"]`);
	await expect(tile).toBeVisible();
	// The card's link is named by the person, not by the circle's initial first.
	await expect(tile).toHaveAccessibleName(new RegExp(`^${displayName}`));

	const report = await page.locator('[data-person]').evaluateAll((cards) =>
		cards.flatMap((card) => {
			const box = card.getBoundingClientRect();
			const out: string[] = [];
			for (const el of card.querySelectorAll('*')) {
				const r = el.getBoundingClientRect();
				if (r.width && r.right > box.right + 0.5)
					out.push(
						`${card.getAttribute('data-person')}: "${(el as HTMLElement).innerText?.slice(0, 20)}" +${Math.round(r.right - box.right)}`
					);
			}
			// The name is never one letter per line: its column has the card's width.
			const name = card.querySelector('.text-body.font-medium') as HTMLElement;
			if (name.getBoundingClientRect().width < box.width * 0.6)
				out.push(`${card.getAttribute('data-person')}: name squeezed`);
			return out;
		})
	);
	expect(report).toEqual([]);

	// In a narrow card the role pill always takes its own line, under the lists.
	const placement = await page.locator('[data-person]').evaluateAll((cards) =>
		cards.map((card) => {
			const pill = card.querySelector('.rounded-pill.ml-auto')!;
			const lists = pill.parentElement!.firstElementChild!;
			return pill.getBoundingClientRect().top >= lists.getBoundingClientRect().bottom - 0.5;
		})
	);
	expect(placement.every(Boolean)).toBe(true);
	expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(360);
});

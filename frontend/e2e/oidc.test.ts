import { expect, test, type Page } from '@playwright/test';
import { createUser, login, signOut, uniqueToken } from './helpers';

// Exercises sign-in through the test OIDC provider (mise run oidc:up, see
// docs/memory/content/howtos/test-oidc.mdx). Every test below shares the two
// Dex accounts (mila and gast) this file's beforeEach and afterEach manage,
// so it runs serially and on the desktop project only - an identity links to
// one Rezepte account at a time, and running twice at once would race.
test.describe.configure({ mode: 'serial' });

test.beforeEach(async ({ request }, testInfo) => {
	test.skip(
		testInfo.project.name !== 'desktop',
		'the Dex accounts below are shared across workers; this file runs once, on desktop'
	);
	const res = await request.get('/api/v1/auth/oidc');
	test.skip(!(await res.json()).enabled, 'Dex is not running (mise run oidc:up)');
});

async function dexLogin(page: Page, email: string, password: string) {
	await page.getByLabel(/email/i).fill(email);
	await page.getByLabel(/password/i).fill(password);
	await page.getByRole('button', { name: /login/i }).click();
}

/** Disconnects the signed-in account's Dex identity, through the API with the page's session. */
async function unlinkDex(page: Page): Promise<void> {
	const origin = new URL(page.url()).origin;
	const response = await page.request.delete('/api/v1/auth/me/identity', {
		headers: { Origin: origin }
	});
	expect(
		response.ok(),
		`DELETE /api/v1/auth/me/identity failed: ${response.status()}`
	).toBeTruthy();
}

test('connect Dex in the profile, then sign in with it', async ({ page }, testInfo) => {
	// An account of its own, not the shared admin: disconnecting ends the
	// account's other sessions, and other workers are signed in as admin.
	const name = `dexlink${uniqueToken()}`;
	await login(page);
	await createUser(page, { username: name, role: 'user' });
	await page.context().clearCookies({ name: 'rezepte_session' });
	await login(page, name);
	await page.goto('/settings');
	await page.getByRole('button', { name: 'Continue with Dex' }).click();
	await dexLogin(page, 'mila@example.com', 'mila1234');
	// The toast says "Connected to Dex" too; the card's line is the lasting one.
	await expect(page.getByText(/^Connected to Dex since/)).toBeVisible();

	await signOut(page, testInfo);
	await page.getByRole('button', { name: 'Continue with Dex' }).click();
	// Dex keeps no login session of its own, so it asks again.
	await dexLogin(page, 'mila@example.com', 'mila1234');
	await expect(page).toHaveURL('/');

	// The Rezepte account (signed in as mila at Dex) keeps its password, so
	// the service allows this.
	await unlinkDex(page);
});

test('an unconnected Dex account is told how to connect', async ({ page }) => {
	await page.goto('/login');
	await page.getByRole('button', { name: 'Continue with Dex' }).click();
	await dexLogin(page, 'gast@example.com', 'gast1234');
	await expect(page.getByText(/not connected to Rezepte yet/)).toBeVisible();
});

// The account-creation steps are copied from setup-links.test.ts on purpose
// (spec files do not import each other); keep the labels in sync with it.
// Deleting the invited account in afterEach also frees gast's identity again,
// through ON DELETE CASCADE.
let inviteeUsername: string | null = null;

test.afterEach(async ({ page }) => {
	if (!inviteeUsername) return;
	const name = inviteeUsername;
	inviteeUsername = null;
	const list = (await (await page.request.get('/api/v1/users')).json()) as {
		items: { id: string; username: string }[];
	};
	const created = list.items.find((u) => u.username === name);
	if (!created) return;
	const origin = new URL(page.url()).origin;
	await page.request.delete(`/api/v1/users/${created.id}`, { headers: { Origin: origin } });
});

test('an invited person sets up their account with Dex', async ({ page, browser }) => {
	const name = `dex${uniqueToken()}`;
	await login(page);
	await page.goto('/settings/users');
	await page.getByRole('button', { name: 'Add account' }).click();
	await page.getByLabel('Username').fill(name);
	await page.getByRole('button', { name: 'Add', exact: true }).click();
	const linkDialog = page.getByRole('dialog', { name: `Setup link for ${name}` });
	const url = await linkDialog.getByRole('textbox').inputValue();
	inviteeUsername = name;

	const context = await browser.newContext();
	const invited = await context.newPage();
	await invited.goto(url);
	await invited.getByRole('button', { name: 'Continue with Dex' }).click();
	await dexLogin(invited, 'gast@example.com', 'gast1234');
	await expect(invited).toHaveURL('/');
	const me = await (await invited.request.get('/api/v1/auth/me')).json();
	expect(me.username).toBe(name);
	expect(me.hasPassword).toBe(false);
	await context.close();

	// The admin disconnects the new account's identity, after confirming;
	// without a password it is not set up any more. This file runs on
	// desktop only, so "Disconnect" is always in the row's "..." menu, not
	// the phone's sheet.
	await page.reload();
	const row = page.getByRole('listitem').filter({ hasText: name });
	const more = row.getByRole('button', { name: `More actions for ${name}` });
	await more.click();
	await page.getByRole('menuitem', { name: `Disconnect Dex from ${name}` }).click();
	const confirm = page.getByRole('dialog', { name: 'Disconnect Dex?' });
	await expect(confirm).toContainText(`${name} is signed out on every device`);
	await confirm.getByRole('button', { name: 'Disconnect Dex' }).click();
	await expect(page.getByText(`Dex disconnected from ${name}`)).toBeVisible();
	await expect(more).toBeFocused();
	await more.click();
	await expect(page.getByRole('menuitem', { name: `Disconnect Dex from ${name}` })).toHaveCount(0);
});

// The Sign-in page against Dex, which the e2e task saved as the provider.
// Nothing below confirms a switch or turns sign-in off: every other spec
// shares this instance and its provider.

test('switching the provider warns with the number of connected accounts', async ({ page }) => {
	const name = `dexswitch${uniqueToken()}`;
	await login(page);
	await createUser(page, { username: name, role: 'user' });
	await page.context().clearCookies({ name: 'rezepte_session' });
	await login(page, name);
	await page.goto('/settings');
	await page.getByRole('button', { name: 'Continue with Dex' }).click();
	await dexLogin(page, 'mila@example.com', 'mila1234');
	await expect(page.getByText(/^Connected to Dex since/)).toBeVisible();
	try {
		const owner = await page.context().browser()!.newContext();
		const admin = await owner.newPage();
		await login(admin);
		await admin.goto('/settings/sign-in');
		const before = await (await admin.request.get('/api/v1/settings/sign-in')).json();
		await admin
			.getByRole('region', { name: 'Sign-in' })
			.getByRole('button', { name: 'Edit' })
			.click();
		const dialog = admin.getByRole('dialog', { name: 'Set up sign-in' });
		await dialog.getByLabel('Issuer URL').fill('http://localhost:1/another');
		await dialog.getByRole('button', { name: 'Save', exact: true }).click();
		const confirm = admin.getByRole('dialog', { name: 'Switch provider?' });
		await expect(confirm).toContainText('1 account is connected to Dex');
		await confirm.getByRole('button', { name: 'Cancel' }).click();
		await expect(confirm).toBeHidden();
		expect(await (await admin.request.get('/api/v1/settings/sign-in')).json()).toEqual(before);
		await owner.close();
	} finally {
		await unlinkDex(page);
	}
});

test('changing only the name saves without a warning', async ({ page }) => {
	await login(page);
	await page.goto('/settings/sign-in');
	const card = page.getByRole('region', { name: 'Sign-in' });
	const rename = async (to: string) => {
		await card.getByRole('button', { name: 'Edit' }).click();
		const dialog = page.getByRole('dialog', { name: 'Set up sign-in' });
		await dialog.getByLabel('Button name').fill(to);
		await expect(dialog).toContainText(`On the sign-in page: Continue with ${to}`);
		await dialog.getByRole('button', { name: 'Save', exact: true }).click();
		await expect(dialog).toBeHidden();
		await expect(card.locator('dd').first()).toContainText(to);
	};
	await rename('Dex at home');
	await expect(page.getByRole('dialog', { name: 'Switch provider?' })).toHaveCount(0);
	await rename('Dex');
});

test('the redirect URI can be copied', async ({ page, context }) => {
	await context.grantPermissions(['clipboard-read', 'clipboard-write']);
	await login(page);
	await page.goto('/settings/sign-in');
	await page
		.getByRole('region', { name: 'Sign-in' })
		.getByRole('button', { name: 'Copy the redirect URI' })
		.click();
	await expect(page.getByText('Redirect URI copied')).toBeVisible();
	const copied = await page.evaluate(() => navigator.clipboard.readText());
	expect(copied).toBe(new URL('/api/v1/auth/oidc/callback', page.url()).href);
});

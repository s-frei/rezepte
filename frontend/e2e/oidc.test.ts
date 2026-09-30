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

	// The admin disconnects the new account's identity; without a password
	// it is not set up any more. This file runs on desktop only, so
	// "Disconnect" is always in the row's "..." menu, not the phone's sheet.
	await page.reload();
	const row = page.getByRole('listitem').filter({ hasText: name });
	await row.getByRole('button', { name: `More actions for ${name}` }).click();
	await page.getByRole('menuitem', { name: `Disconnect Dex from ${name}` }).click();
	await expect(page.getByText(`Dex disconnected from ${name}`)).toBeVisible();
	await row.getByRole('button', { name: `More actions for ${name}` }).click();
	await expect(page.getByRole('menuitem', { name: `Disconnect Dex from ${name}` })).toHaveCount(0);
});

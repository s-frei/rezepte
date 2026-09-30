import { expect, test, type Page } from '@playwright/test';
import { createUser, login, uniqueToken } from './helpers';

// The phone's half of an admin disconnecting someone's provider: on a phone
// the action sits in the person's sheet, not in a "..." menu, and it has to
// ask first there too. oidc.test.ts covers the desktop half. This file uses
// the Dex account jonas, which nothing else in the suite signs in with, and
// runs on the mobile project only, so the two files never race for one
// identity. Needs the test provider (mise run oidc:up).
test.beforeEach(async ({ request }, testInfo) => {
	test.skip(testInfo.project.name !== 'mobile', 'the desktop half lives in oidc.test.ts');
	const res = await request.get('/api/v1/auth/oidc');
	test.skip(!(await res.json()).enabled, 'Dex is not running (mise run oidc:up)');
});

async function dexLogin(page: Page, email: string, password: string) {
	await page.getByLabel(/email/i).fill(email);
	await page.getByLabel(/password/i).fill(password);
	await page.getByRole('button', { name: /login/i }).click();
}

test('an admin disconnects a provider from the phone sheet, after confirming', async ({
	page,
	browser
}) => {
	const name = `dexphone${uniqueToken()}`;
	await login(page);
	const account = await createUser(page, { username: name, role: 'user' });

	try {
		// The member connects jonas at Dex from their own profile.
		const context = await browser.newContext();
		const member = await context.newPage();
		await login(member, name);
		await member.goto('/settings');
		await member.getByRole('button', { name: 'Continue with Dex' }).click();
		await dexLogin(member, 'jonas@example.com', 'jonas1234');
		await expect(member.getByText(/^Connected to Dex since/)).toBeVisible();
		await context.close();

		await page.goto('/settings/users');
		const row = page.getByRole('listitem').filter({ hasText: name });
		await expect(row.getByText('Signs in with Dex')).toBeVisible();
		const manage = row.getByRole('button', { name: `Manage ${name}` });
		await manage.click();
		await page
			.getByRole('dialog')
			.getByRole('button', { name: `Disconnect Dex from ${name}` })
			.click();

		const confirm = page.getByRole('dialog', { name: 'Disconnect Dex?' });
		await expect(confirm).toContainText(`${name} is signed out on every device`);
		await confirm.getByRole('button', { name: 'Disconnect Dex' }).click();
		await expect(page.getByText(`Dex disconnected from ${name}`)).toBeVisible();
		await expect(manage).toBeFocused();
		await expect(row.getByText('Signs in with Dex')).toHaveCount(0);
	} finally {
		// Deleting the account frees jonas's Dex identity for the next run
		// (ON DELETE CASCADE), whether or not the test got that far.
		const origin = new URL(page.url()).origin;
		await page.request.delete(`/api/v1/users/${account.id}`, { headers: { Origin: origin } });
	}
});

import { expect, test, type Page } from '@playwright/test';
import { createUser, login, uniqueToken } from './helpers';

/** Signs whoever is in out and name in. */
async function switchTo(page: Page, name: string) {
	await page.context().clearCookies({ name: 'rezepte_session' });
	await login(page, name);
	await expect(page).toHaveURL('/');
}

// The mailed flow (comment mail, unsubscribe link, Undo) lives in
// mail.test.ts: mail settings are instance-wide, and only that file's serial
// run may change them.

test('a broken unsubscribe link says so and leads to the settings', async ({ page }) => {
	await page.goto('/unsubscribe?u=nobody&k=comment&s=made-up');
	await expect(page.getByRole('heading', { name: 'This link no longer works' })).toBeVisible();
	await page.getByRole('link', { name: 'Manage all notifications' }).click();
	await expect(page).toHaveURL(/\/login\?next=%2Fsettings%2Fnotifications$/);
});

test('without a confirmed address the page asks for one', async ({ page }) => {
	await login(page);
	const name = `noaddr${uniqueToken()}`;
	await createUser(page, { username: name, role: 'user' });
	await switchTo(page, name);
	await page.goto('/settings/notifications');
	await expect(page.getByText('Rezepte only writes to a confirmed address.')).toBeVisible();
	await expect(page.getByRole('switch')).toHaveCount(0);
	// Confirming takes a mail too, so a page without mail says so here as
	// well. Answered, not set: mail settings belong to mail.test.ts.
	await page.route('**/api/v1/settings', async (route) => {
		const response = await route.fetch();
		await route.fulfill({ response, json: { ...(await response.json()), mailEnabled: false } });
	});
	await page.reload();
	await expect(page.getByText("Mail isn't set up on this instance")).toBeVisible();
	await page.getByRole('link', { name: 'Confirm your address in your profile' }).click();
	await expect(page).toHaveURL('/settings');
});

test('the notifications page fits a 360px phone', async ({ page }) => {
	await page.setViewportSize({ width: 360, height: 780 });
	// The owner with a long confirmed address: every group and its switches show.
	await page.route('**/api/v1/auth/me', async (route) => {
		const response = await route.fetch();
		const user = await response.json();
		await route.fulfill({
			response,
			json: { ...user, email: 'a.rather.long.address@kitchen.example.org', emailVerified: true }
		});
	});
	await login(page);
	await page.goto('/settings/notifications');
	await expect(page.getByRole('switch', { name: 'Invitation accepted' })).toBeVisible();
	const scroll = await page.evaluate(() => document.documentElement.scrollWidth - innerWidth);
	expect(scroll).toBeLessThanOrEqual(0);
});

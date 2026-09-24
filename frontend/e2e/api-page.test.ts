import { expect, test } from '@playwright/test';
import { createUser, login, uniqueToken } from './helpers';

test('an admin finds the served spec next to the token section', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/api');

	await expect(page.getByRole('heading', { name: 'HTTP API' })).toBeVisible();
	// The card names the origin the page itself came from, whatever host or
	// port this instance happens to run on.
	const origin = new URL(page.url()).origin;
	await expect(page.getByText(`${origin}/api/v1`)).toBeVisible();

	// The figures come from the document this instance actually serves, so the
	// test asserts they are numbers rather than pinning today's count.
	await expect(page.getByRole('term').filter({ hasText: 'Endpoints' })).toBeVisible();
	await expect(page.locator('dd').first()).toHaveText(/^\d+$/);

	await expect(page.getByRole('link', { name: 'Interactive docs' })).toHaveAttribute(
		'href',
		'/api/v1/docs'
	);
	await expect(page.getByRole('link', { name: 'openapi.json' })).toHaveAttribute(
		'href',
		'/api/v1/openapi.json'
	);

	// The token section is still there, below the card.
	await expect(page.getByRole('heading', { name: 'API tokens' })).toBeVisible();
	// An admin is not told to ask an admin.
	await expect(page.getByText('Only an account with the admin role can issue one')).toHaveCount(0);
});

test('a member reaches the API page without the token section', async ({ page }, testInfo) => {
	const username = `api${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'user' });

	await page.context().clearCookies();
	await login(page, username);
	await expect(page).toHaveURL('/');

	await page.goto('/settings');
	// A phone lists the pages in the contents sheet behind the running head;
	// the desktop sidebar shows them outright.
	if (testInfo.project.name.startsWith('mobile')) {
		await page.getByRole('button', { name: 'Profile, open contents' }).click();
	}
	await expect(page.getByRole('link', { name: 'API' })).toBeVisible();

	await page.goto('/settings/api');
	await expect(page).toHaveURL('/settings/api');
	await expect(page.getByRole('heading', { name: 'HTTP API' })).toBeVisible();
	await expect(page.getByText('Only an account with the admin role can issue one')).toBeVisible();

	await expect(page.getByRole('heading', { name: 'API tokens' })).toHaveCount(0);
	await expect(page.getByRole('button', { name: 'Create token' })).toHaveCount(0);
	// The admin-only nav entry is still hidden from them.
	await expect(page.getByRole('link', { name: 'Members' })).toHaveCount(0);
});

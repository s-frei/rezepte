import { expect, test } from '@playwright/test';
import { createUser, login, uniqueToken } from './helpers';

test('an admin finds both ways in on the API page', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/api');

	await expect(page.getByRole('heading', { name: 'Access for programs' })).toBeVisible();
	const scripts = page.getByRole('region', { name: 'For your own scripts' });
	// The card names the origin the page itself came from, whatever host or
	// port this instance happens to run on.
	const origin = new URL(page.url()).origin;
	await expect(scripts.getByText(`${origin}/api/v1`, { exact: true })).toBeVisible();

	// The figure comes from the document this instance actually serves, so the
	// test asserts it is a number rather than pinning today's count.
	await expect(scripts.getByText(/^\d+ endpoints, the same ones this app uses\.$/)).toBeVisible();
	await expect(scripts.getByRole('link', { name: 'Interactive docs' })).toHaveAttribute(
		'href',
		'/api/v1/docs'
	);
	await expect(scripts.getByRole('link', { name: 'openapi.json' })).toHaveAttribute(
		'href',
		'/api/v1/openapi.json'
	);
	// A script may also sign in the way this app does, so the token is one of
	// two ingredients there.
	await expect(scripts.getByText('Username and password')).toBeVisible();

	// "Create token" has one home, the token card, whatever state the list is
	// in - other specs create tokens on this account at the same time.
	await expect(page.getByRole('button', { name: 'Create token' })).toHaveCount(1);
	// An admin is not told to ask an admin.
	await expect(page.getByText('Only an account with the admin role can issue one.')).toHaveCount(0);
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
	// The people list is theirs to read, so its nav entry is there too.
	await expect(page.getByRole('link', { name: 'People' })).toBeVisible();

	await page.goto('/settings/api');
	await expect(page).toHaveURL('/settings/api');
	await expect(page.getByRole('heading', { name: 'Access for programs' })).toBeVisible();
	await expect(page.getByText('Only an account with the admin role can issue one.')).toBeVisible();
	// Both recipes name the same token, so both say where it comes from.
	await expect(page.getByText('from an admin')).toHaveCount(2);

	await expect(page.getByRole('heading', { name: 'API tokens' })).toHaveCount(0);
	await expect(page.getByRole('button', { name: 'Create token' })).toHaveCount(0);
});

test('the API page shows the MCP endpoint', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/api');
	const card = page.getByRole('region', { name: 'For AI assistants' });
	await expect(card.getByText(/^http:\/\/.+\/mcp$/)).toBeVisible();
	await expect(card.getByRole('link', { name: 'Setup guide' })).toHaveAttribute(
		'href',
		'https://s-frei.github.io/rezepte/api/mcp/'
	);
});

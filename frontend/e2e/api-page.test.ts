import { expect, test } from '@playwright/test';
import { createUser, login, uniqueToken } from './helpers';

test('an admin finds the served spec next to the token section', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/api');

	await expect(page.getByRole('heading', { name: 'HTTP-API' })).toBeVisible();
	// The card names the origin the page itself came from, whatever host or
	// port this instance happens to run on.
	const origin = new URL(page.url()).origin;
	await expect(page.getByText(`${origin}/api/v1`)).toBeVisible();

	// The figures come from the document this instance actually serves, so the
	// test asserts they are numbers rather than pinning today's count.
	await expect(page.getByRole('term').filter({ hasText: 'Endpunkte' })).toBeVisible();
	await expect(page.locator('dd').first()).toHaveText(/^\d+$/);

	await expect(page.getByRole('link', { name: 'Interaktive Doku' })).toHaveAttribute(
		'href',
		'/api/v1/docs'
	);
	await expect(page.getByRole('link', { name: 'openapi.json' })).toHaveAttribute(
		'href',
		'/api/v1/openapi.json'
	);

	// The token section is still there, below the card.
	await expect(page.getByRole('heading', { name: 'API-Token' })).toBeVisible();
	// An admin is not told to ask an admin.
	await expect(page.getByText('Ausgestellt wird er von einem Konto')).toHaveCount(0);
});

test('a member reaches the API page without the token section', async ({ page }) => {
	const username = `api${uniqueToken()}`;
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username, role: 'user' });

	await page.context().clearCookies();
	await login(page, username);
	await expect(page).toHaveURL('/');

	await page.goto('/settings');
	// Each viewport renders its own nav variant - the chip row is `md:hidden`,
	// the sidebar `hidden md:flex` - and only the visible one is in the
	// accessibility tree, so this matches exactly one link either way.
	await expect(page.getByRole('link', { name: 'API' })).toBeVisible();

	await page.goto('/settings/api');
	await expect(page).toHaveURL('/settings/api');
	await expect(page.getByRole('heading', { name: 'HTTP-API' })).toBeVisible();
	await expect(page.getByText('Ausgestellt wird er von einem Konto')).toBeVisible();

	await expect(page.getByRole('heading', { name: 'API-Token' })).toHaveCount(0);
	await expect(page.getByRole('button', { name: 'Token erstellen' })).toHaveCount(0);
	// The admin-only nav entry is still hidden from them.
	await expect(page.getByRole('link', { name: 'Benutzer' })).toHaveCount(0);
});

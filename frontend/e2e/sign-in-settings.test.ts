import { expect, test } from '@playwright/test';
import { createUser, login, uniqueToken } from './helpers';

// The owner's Sign-in page, in whatever state the run left it: with Dex up
// (mise run oidc:up) the e2e task has saved Dex there, without it sign-in is
// off. Nothing here saves; the flows that need a provider that answers live
// in oidc.test.ts.

test('owner sees the sign-in page; an admin is sent to settings', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/sign-in');
	const card = page.getByRole('region', { name: 'Sign-in' });
	await expect(card.getByText('Redirect URI for the provider')).toBeVisible();
	await expect(card.getByText(/\/api\/v1\/auth\/oidc\/callback$/)).toBeVisible();

	const name = `signin${uniqueToken()}`;
	await createUser(page, { username: name, role: 'admin' });
	await page.context().clearCookies({ name: 'rezepte_session' });
	await login(page, name);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/sign-in');
	await expect(page).toHaveURL('/settings');
});

test('a wrong issuer shows a field error and saves nothing', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	// The card is told nobody is connected, so no switch warning stands
	// between the save and its error: oidc.test.ts may have an account linked
	// at Dex right now, and the warning has its own test there.
	await page.route('**/api/v1/settings/sign-in', async (route) => {
		if (route.request().method() !== 'GET') return route.continue();
		const response = await route.fetch();
		await route.fulfill({
			response,
			json: { ...(await response.json()), connected: 0, connectedWithoutPassword: 0 }
		});
	});
	const before = await (await page.request.get('/api/v1/settings/sign-in')).json();

	await page.goto('/settings/sign-in');
	const card = page.getByRole('region', { name: 'Sign-in' });
	await card.getByRole('button', { name: /^(Set up sign-in|Edit)$/ }).click();
	const dialog = page.getByRole('dialog', { name: 'Set up sign-in' });
	await dialog.getByLabel('Issuer URL').fill('http://localhost:1/nothing');
	await dialog.getByLabel('Client ID').fill('rezepte');
	await dialog.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(dialog.getByText('No OpenID provider answers at this address.')).toBeVisible();
	await expect(dialog.getByLabel('Issuer URL')).toHaveAttribute('aria-invalid', 'true');

	expect(await (await page.request.get('/api/v1/settings/sign-in')).json()).toEqual(before);
});

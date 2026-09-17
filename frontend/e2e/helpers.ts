import type { Page } from '@playwright/test';

/** Logs in as the seeded admin (password from the e2e task). */
export async function login(page: Page, username = 'admin', password = 'e2e-password') {
	await page.goto('/login');
	await page.getByLabel('Benutzername').fill(username);
	await page.getByLabel('Passwort').fill(password);
	await page.getByRole('button', { name: 'Anmelden' }).click();
}

import { expect, test, type Page } from '@playwright/test';
import { isMobile, prepare, shot } from './helpers';

// The demo never checks GitHub (state `off`), so the page gets the status a
// newer release would give. The service worker could answer before `page.route`.
test.use({ serviceWorkers: 'block' });

async function offerUpdate(page: Page) {
	await page.route('**/api/v1/settings/update', (route) =>
		route.fulfill({
			json: {
				current: '1.5.0',
				state: 'available',
				latest: {
					version: '1.6.0',
					publishedAt: '2026-09-10T09:00:00Z',
					changelogUrl: 'https://getrezepte.app/changelog/v1.6.0/'
				}
			}
		})
	);
}

test('update-note', async ({ page }, testInfo) => {
	await offerUpdate(page);
	await prepare(page, testInfo, { login: true });
	await expect(page.getByText('New version available')).toBeVisible();
	await shot(page, 'update-note');
});

// Desktop: the account menu; phone: the You sheet, whose foot the band is.
test('update-band', async ({ page }, testInfo) => {
	await offerUpdate(page);
	await prepare(page, testInfo, { login: true });
	if (isMobile(testInfo)) {
		await page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'You' }).click();
		await expect(page.getByRole('dialog').getByText('Update available')).toBeVisible();
	} else {
		await page.getByRole('button', { name: 'Account menu' }).click();
		await expect(page.getByRole('menu').getByText('Update available')).toBeVisible();
	}
	await shot(page, 'update-band');
});

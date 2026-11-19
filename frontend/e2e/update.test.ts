import { expect, test, type Page } from '@playwright/test';
import { createUser, login, pinLocale, uniqueToken } from './helpers';

const CHANGELOG = 'https://getrezepte.app/changelog/v1.6.0/';
const GUIDE = 'https://getrezepte.app/operations/updates/';

// The e2e server runs with the check off, so each test hands the page the
// status it wants. The service worker could answer before `page.route`.
test.use({ serviceWorkers: 'block' });

const available = (version = '1.6.0') => ({
	current: '1.5.0',
	state: 'available',
	latest: {
		version,
		publishedAt: '2026-11-10T09:00:00Z',
		changelogUrl: `https://getrezepte.app/changelog/v${version}/`
	}
});

async function stub(page: Page, body: object) {
	await page.route('**/api/v1/settings/update', (route) => route.fulfill({ json: body }));
}

const note = (page: Page) => page.getByRole('group', { name: /^Installed 1\.5\.0, new / });

// Desktop: the dropdown's items; phone: the You sheet's buttons and links.
async function openMenu(page: Page, isMobile: boolean) {
	if (isMobile) {
		await page
			.getByRole('navigation', { name: 'Main' })
			.getByRole('button', { name: 'You' })
			.click();
		return page.getByRole('dialog');
	}
	await page.getByRole('button', { name: 'Account menu' }).click();
	return page.getByRole('menu');
}

test('owner sees the note and the band, × keeps the band', async ({ page, isMobile }) => {
	await stub(page, available());
	await login(page);
	await page.goto('/');

	await expect(note(page)).toBeVisible();
	await expect(note(page)).toContainText('1.5.0');
	await expect(note(page)).toContainText('1.6.0');
	await expect(page.getByRole('link', { name: "What's new" })).toHaveAttribute('href', CHANGELOG);
	await expect(page.getByRole('link', { name: 'Instructions' })).toHaveAttribute('href', GUIDE);

	await page.getByRole('button', { name: 'Hide' }).click();
	await expect(note(page)).toBeHidden();

	const menu = await openMenu(page, isMobile);
	const band = menu.getByText('Update available');
	await expect(band).toBeVisible();
	if (isMobile) {
		await expect(menu.getByRole('link', { name: /Update available/ })).toHaveAttribute(
			'href',
			CHANGELOG
		);
	} else {
		// A real link, reached with the menu's keys and opened with Enter.
		const item = menu.getByRole('menuitem', { name: /Update available/ });
		await expect(item).toHaveAttribute('href', CHANGELOG);
		await page.keyboard.press('End');
		await expect(item).toBeFocused();
		const popup = page.waitForEvent('popup');
		await page.keyboard.press('Enter');
		expect((await popup).url()).toBe(CHANGELOG);
	}

	await page.reload();
	await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
	await expect(note(page)).toBeHidden();
});

test('a newer version brings the note back', async ({ page }) => {
	await page.addInitScript(() => localStorage.setItem('rezepte-update-dismissed', '1.6.0'));
	await stub(page, available('1.7.0'));
	await login(page);
	await page.goto('/');
	await expect(note(page)).toBeVisible();
	await expect(note(page)).toContainText('1.7.0');
});

test('up to date shows the chip and no note', async ({ page, isMobile }) => {
	await stub(page, { current: '1.5.0', state: 'current' });
	await login(page);
	await page.goto('/');
	await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
	await expect(note(page)).toHaveCount(0);
	const menu = await openMenu(page, isMobile);
	await expect(menu.getByText('Up to date')).toBeVisible();
	await expect(menu.getByText('Version 1.5.0')).toBeVisible();
});

test('a member never asks', async ({ page, isMobile }) => {
	const member = `member-${uniqueToken()}`;
	await login(page);
	await createUser(page, { username: member, role: 'user' });
	await page.context().clearCookies();
	const asked: string[] = [];
	page.on('request', (request) => {
		if (request.url().includes('/settings/update')) asked.push(request.url());
	});
	await login(page, member);
	await page.goto('/');
	await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
	await page.waitForLoadState('networkidle');
	const menu = await openMenu(page, isMobile);
	await expect(menu.getByText(/Up to date|Update available/)).toHaveCount(0);
	expect(asked).toEqual([]);
	expect((await page.request.get('/api/v1/settings/update')).status()).toBe(403);
});

test('fits at 320 px in German', async ({ page, isMobile }) => {
	test.skip(!isMobile, 'the narrow case is the phone project');
	await page.setViewportSize({ width: 320, height: 700 });
	await stub(page, available());
	await login(page);
	// The account's own language wins over the cookie, and admin is shared
	// with every parallel spec, so only this page is told it reads German.
	await page.route('**/api/v1/auth/me', async (route) => {
		if (route.request().method() !== 'GET') return route.fallback();
		const response = await route.fetch();
		await route.fulfill({ response, json: { ...(await response.json()), locale: 'de' } });
	});
	await pinLocale(page, 'de');
	await page.goto('/');
	await expect(page.getByRole('navigation', { name: 'Hauptmenü' })).toBeVisible();
	await expect(page.getByText('Neue Version verfügbar')).toBeVisible();
	expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);

	const box = page
		.getByRole('group', { name: /1\.5\.0.*1\.6\.0/ })
		.locator('xpath=ancestor::div[contains(@class,"shadow-card")][1]');
	await expect(box).toBeVisible();
	const fits = (loc: typeof box) => loc.evaluate((el) => el.scrollWidth <= el.clientWidth);
	expect(await fits(box)).toBe(true);
	const outer = (await box.boundingBox())!;
	for (const version of ['1.5.0', '1.6.0']) {
		const v = box.getByText(version, { exact: true });
		await expect(v).toBeVisible();
		const b = (await v.boundingBox())!;
		expect(b.x).toBeGreaterThanOrEqual(outer.x);
		expect(b.x + b.width).toBeLessThanOrEqual(outer.x + outer.width);
	}

	await page
		.getByRole('navigation', { name: 'Hauptmenü' })
		.getByRole('button', { name: 'Du' })
		.click();
	const band = page.getByRole('dialog').getByRole('link', { name: /1\.6\.0/ });
	await expect(band).toBeVisible();
	expect(await fits(band)).toBe(true);
	const bb = (await band.boundingBox())!;
	for (const version of ['1.5.0', '1.6.0']) {
		const v = band.getByText(version, { exact: true });
		await expect(v).toBeVisible();
		const b = (await v.boundingBox())!;
		expect(b.x).toBeGreaterThanOrEqual(bb.x);
		expect(b.x + b.width).toBeLessThanOrEqual(bb.x + bb.width);
	}
});

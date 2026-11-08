import { expect, test, type Page } from '@playwright/test';
import { FIXED_TIME, prepare, shot } from './helpers';

// The demo gives its admin and its two signed-in members (mila, jonas)
// public links of their own - its third member, noah, has no password and
// no links - but leaves public sharing off, as every instance starts
// (service/internal/demo/seed.go). These pictures show it on, so each test
// switches it on as the demo admin - the owner - and off again afterwards:
// the specs run one after another against the same instance, and every other
// picture expects it off.

async function setPublicShares(page: Page, on: boolean): Promise<void> {
	const origin = new URL(page.url()).origin;
	const res = await page.request.patch('/api/v1/settings', {
		headers: { Origin: origin, 'Content-Type': 'application/json' },
		data: { publicShares: on }
	});
	expect(res.ok(), `PATCH /api/v1/settings: ${res.status()}`).toBeTruthy();
}

test.beforeEach(async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await setPublicShares(page, true);
});

test.afterEach(async ({ page }) => {
	await setPublicShares(page, false);
});

test('public-link-sheet', async ({ page }, testInfo) => {
	// A recipe nobody has shared: create its link from the Link tab, so the
	// picture shows the ticket with its remaining time.
	await page.goto('/recipes/macaroni-cheese');
	if (testInfo.project.use.isMobile) {
		await page.getByRole('button', { name: 'Share', exact: true }).click();
	} else {
		await page.getByRole('button', { name: 'Pass on', exact: true }).click();
	}
	const sheet = page.getByRole('dialog', { name: 'Pass on recipe' });
	// The page's clock is frozen, the instance's is not: the new link's dates
	// are moved onto the frozen day, or the stub would count the days from
	// it to an expiry set from today.
	await page.route('**/api/v1/recipes/*/public-share', async (route) => {
		const response = await route.fetch();
		const body = await response.json();
		const day = 86_400_000;
		const start = FIXED_TIME.toISOString();
		await route.fulfill({
			response,
			json: {
				...body,
				createdAt: start,
				startsAt: start,
				expiresAt: new Date(FIXED_TIME.getTime() + 7 * day).toISOString()
			}
		});
	});
	await sheet.getByRole('radio', { name: 'Link', exact: true }).click();
	await sheet.getByRole('radio', { name: 'For everyone with the link' }).click();
	const created = page.waitForResponse(
		(r) => r.url().endsWith('/public-share') && r.request().method() === 'POST'
	);
	await sheet.getByRole('button', { name: 'Create public link' }).click();
	const share = (await (await created).json()) as { id: string };
	try {
		await expect(sheet.getByRole('button', { name: 'Revoke' })).toBeVisible();
		await shot(page, 'public-link-sheet');
	} finally {
		// The other pictures expect this recipe unshared.
		const origin = new URL(page.url()).origin;
		await page.request.delete(`/api/v1/shares/${share.id}`, { headers: { Origin: origin } });
	}
});

test('shared-links', async ({ page }) => {
	// The admin's view of everyone's links, so the person filter shows.
	await page.goto('/settings/shares?all=1');
	await expect(page.getByRole('heading', { name: 'Created by' })).toBeVisible();
	await shot(page, 'shared-links');
});

test('public-page', async ({ page, browser }, testInfo) => {
	const origin = new URL(page.url()).origin;
	const res = await page.request.get('/api/v1/shares', { headers: { Origin: origin } });
	expect(res.ok()).toBeTruthy();
	const { items } = (await res.json()) as { items: { path: string }[] };
	expect(items.length).toBeGreaterThan(0);

	// What a stranger sees: a context of its own, without the session, in the
	// project's own viewport and color scheme.
	const stranger = await browser.newContext({
		viewport: page.viewportSize(),
		deviceScaleFactor: testInfo.project.use.deviceScaleFactor,
		isMobile: testInfo.project.use.isMobile,
		hasTouch: testInfo.project.use.hasTouch,
		colorScheme: testInfo.project.use.colorScheme,
		baseURL: testInfo.project.use.baseURL
	});
	try {
		const strangerPage = await stranger.newPage();
		await prepare(strangerPage, testInfo);
		await strangerPage.goto(items[0].path);
		await expect(strangerPage.getByRole('contentinfo').getByRole('link', { name: /Get Rezepte/ })).toBeVisible();
		await shot(strangerPage, 'public-page');
	} finally {
		await stranger.close();
	}
});

test('public-sharing-card', async ({ page }) => {
	await page.goto('/settings/users');
	await expect(
		page.getByRole('switch', { name: 'Members may share recipes publicly' })
	).toBeVisible();
	await expect(page.getByRole('button', { name: 'Maximum lifetime' })).toBeVisible();
	// Centered, so neither the phone's sticky header nor its bottom navigation
	// covers the card.
	await page
		.getByRole('heading', { name: 'Public sharing' })
		.evaluate((el) => el.closest('section')?.scrollIntoView({ block: 'center' }));
	await shot(page, 'public-sharing-card');
});

test('recipe-card', async ({ page }, testInfo) => {
	await page.goto('/recipes/macaroni-cheese');
	if (testInfo.project.use.isMobile) {
		await page.getByRole('button', { name: 'Share' }).click();
	} else {
		await page.getByRole('button', { name: 'Pass on' }).click();
	}
	const sheet = page.getByRole('dialog', { name: 'Pass on recipe' });
	await expect(sheet.getByRole('img', { name: /^Recipe image:/ })).toBeVisible({ timeout: 20_000 });
	await shot(page, 'recipe-card');
});

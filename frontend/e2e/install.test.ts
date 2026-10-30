import { devices, expect, test, type Page } from '@playwright/test';
import { login } from './helpers';

test('Chromium finds the app installable', async ({ page, isMobile }) => {
	test.skip(isMobile, 'the installability audit is a desktop Chromium call');
	await login(page);
	const cdp = await page.context().newCDPSession(page);
	const { installabilityErrors } = await cdp.send('Page.getInstallabilityErrors');
	expect(installabilityErrors).toEqual([]);
});

// Chromium fires `beforeinstallprompt` only after its own engagement checks,
// so the test hands the app one, as the browser would: once the page has
// loaded, with a `prompt()` that records the call and an accepted choice.
async function offerPrompt(page: Page) {
	await page.addInitScript(() => {
		window.addEventListener('load', () => {
			const event = Object.assign(new Event('beforeinstallprompt', { cancelable: true }), {
				prompt: async () => {
					(window as unknown as { __prompted: boolean }).__prompted = true;
				},
				userChoice: Promise.resolve({ outcome: 'accepted' })
			});
			window.dispatchEvent(event);
		});
	});
}

const prompted = (page: Page) =>
	page.evaluate(() => (window as unknown as { __prompted?: boolean }).__prompted === true);

const banner = (page: Page) => page.getByText('Rezepte as an app').first();
const installButton = (page: Page) => page.getByRole('button', { name: 'Install', exact: true });

test('the banner offers the install on a phone and × hides it for good', async ({
	page,
	isMobile
}) => {
	test.skip(!isMobile, 'the banner is for phones only');
	await offerPrompt(page);
	await login(page);
	await page.goto('/');
	await installButton(page).click();
	await expect.poll(() => prompted(page)).toBe(true);

	// Accepting the dialog is not installing; only `appinstalled` ends the offer.
	await page.reload();
	await expect(banner(page)).toBeVisible();

	await page.getByRole('button', { name: 'Hide' }).click();
	await expect(banner(page)).toBeHidden();
	await page.reload();
	await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
	await expect(banner(page)).toBeHidden();

	await page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'You' }).click();
	await expect(page.getByRole('dialog').getByRole('button', { name: 'Install app' })).toBeVisible();
});

test('the settings card installs', async ({ page }) => {
	await offerPrompt(page);
	await login(page);
	await page.goto('/settings');
	const card = page.getByRole('region', { name: 'Rezepte as an app' });
	await expect(card.getByRole('listitem')).toHaveText([
		'Its own icon on the home screen',
		'Its own window, no browser bar',
		'Share → Rezepte from any app'
	]);
	await card.getByRole('button', { name: 'Install' }).click();
	await expect.poll(() => prompted(page)).toBe(true);
});

test('no hint without a captured prompt', async ({ page, isMobile }) => {
	test.skip(isMobile, 'desktop Chromium shows only what a captured prompt offers');
	await login(page);
	await page.goto('/settings');
	await expect(page.getByRole('heading', { name: 'Appearance' })).toBeVisible();
	await expect(page.getByRole('region', { name: 'Rezepte as an app' })).toHaveCount(0);
	await page.getByRole('button', { name: 'Account menu' }).click();
	await expect(page.getByRole('menuitem', { name: 'Sign out' })).toBeVisible();
	await expect(page.getByRole('menuitem', { name: 'Install app' })).toHaveCount(0);
	await page.keyboard.press('Escape');
	await page.goto('/');
	await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
	await expect(banner(page)).toHaveCount(0);
});

test('no hint inside the app', async ({ page }) => {
	// `emulateMedia` cannot set display-mode, so the query is answered here.
	await page.addInitScript(() => {
		const matchMedia = window.matchMedia.bind(window);
		window.matchMedia = (query: string) =>
			query === '(display-mode: standalone)'
				? ({ ...matchMedia(query), matches: true, media: query } as MediaQueryList)
				: matchMedia(query);
	});
	await offerPrompt(page);
	await login(page);
	await page.goto('/');
	await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
	await expect(banner(page)).toHaveCount(0);
	await page.goto('/settings');
	await expect(page.getByRole('heading', { name: 'Appearance' })).toBeVisible();
	await expect(page.getByRole('region', { name: 'Rezepte as an app' })).toHaveCount(0);
});

test('iPhone gets the steps', async ({ browser, isMobile }) => {
	test.skip(!isMobile, 'one phone run is enough');
	const context = await browser.newContext({ ...devices['iPhone 15'] });
	const page = await context.newPage();
	await login(page);
	await page.goto('/settings');
	const card = page.getByRole('region', { name: 'Rezepte as an app' });
	await expect(card.getByText('In Safari, in three steps')).toBeVisible();
	await expect(card.getByRole('listitem')).toHaveText([
		/Tap Share at the bottom/,
		/Add to Home Screen/,
		/Tap Add/
	]);

	await page.goto('/');
	await installButton(page).click();
	const sheet = page.getByRole('dialog', { name: 'Add Rezepte to your home screen' });
	await expect(sheet.getByRole('listitem')).toHaveCount(3);
	await context.close();
});

// Inside the app, on a device never asked for notifications: a push service
// that hands out a fake subscription, and a permission the test can grant.
async function insideTheApp(page: Page) {
	await page.addInitScript(() => {
		const matchMedia = window.matchMedia.bind(window);
		window.matchMedia = (query: string) =>
			query === '(display-mode: standalone)'
				? ({ ...matchMedia(query), matches: true, media: query } as MediaQueryList)
				: matchMedia(query);
		let permission: NotificationPermission = 'default';
		Object.defineProperty(Notification, 'permission', { get: () => permission });
		Notification.requestPermission = async () => (permission = 'granted');
		PushManager.prototype.getSubscription = async () => null;
		PushManager.prototype.subscribe = async () =>
			({
				endpoint: 'https://fcm.googleapis.com/fcm/send/e2e',
				toJSON: () => ({ keys: { p256dh: 'p', auth: 'a' } })
			}) as unknown as PushSubscription;
	});
	await page.route('**/api/v1/push/subscription', (route) => route.fulfill({ status: 204 }));
}

const pushNote = (page: Page) => page.getByText('Never miss a thing');

test('the installed app offers notifications and one tap turns them on', async ({ page }) => {
	await login(page);
	await page.goto('/');
	await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
	await expect(pushNote(page)).toHaveCount(0);

	await insideTheApp(page);
	await page.reload();
	const subscribed = page.waitForRequest('**/api/v1/push/subscription');
	await page.getByRole('button', { name: 'Turn on notifications' }).click();
	await subscribed;
	await expect(pushNote(page)).toBeHidden();
});

test('× hides the notifications note for good', async ({ page }) => {
	await insideTheApp(page);
	await login(page);
	await page.goto('/');
	await expect(pushNote(page)).toBeVisible();
	await page.getByRole('button', { name: 'Hide' }).click();
	await expect(pushNote(page)).toBeHidden();
	await page.reload();
	await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
	await expect(pushNote(page)).toHaveCount(0);
});

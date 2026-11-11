import { expect, test, type Page } from '@playwright/test';
import type { DeviceAccount } from '../src/lib/api/auth';
import { daysAgo, login } from './helpers';

// "Who's cooking?" against mocked device accounts, and the ways there: the
// entry row in the account menu and the phone's You sheet, the add mode of
// /login, and where a 401 leads. The service side lives in auth.test.ts and
// user-switch.test.ts; here only the screens' own behavior counts.
const LONG = 'Maximiliane-Friederike Sophie von Hohenzollern-Sigmaringen';

const accounts: DeviceAccount[] = [
	{
		id: 'u-jo',
		username: 'jo',
		displayName: 'Jo',
		color: 'rose',
		avatarId: null,
		expired: false,
		lastSeenAt: daysAgo(1)
	},
	{
		id: 'u-max',
		username: 'max',
		displayName: LONG,
		color: 'plum',
		avatarId: null,
		expired: true,
		lastSeenAt: daysAgo(31)
	}
];

/** Mocks the device's accounts and the switch; returns the calls made. */
async function mockDevice(
	page: Page,
	answer: { list?: DeviceAccount[]; status?: number; delay?: number } = {}
) {
	const calls: string[] = [];
	await page.route('**/api/v1/auth/accounts', (route) => {
		calls.push('GET /accounts');
		return route.fulfill({ json: answer.list ?? accounts });
	});
	await page.route('**/api/v1/auth/accounts/*', (route) => {
		calls.push(`${route.request().method()} ${new URL(route.request().url()).pathname}`);
		return route.fulfill({ status: 204 });
	});
	await page.route('**/api/v1/auth/switch', async (route) => {
		calls.push(`POST /switch ${route.request().postDataJSON().userId}`);
		if (answer.delay) await new Promise((done) => setTimeout(done, answer.delay));
		// The real session stays admin's; the screen only has to reload onto `next`.
		if (answer.status)
			await route.fulfill({ status: answer.status, json: { status: answer.status } });
		else await route.fulfill({ json: { id: 'u-jo', displayName: 'Jo' } });
	});
	return calls;
}

const cards = (page: Page) => page.getByRole('list').getByRole('listitem');

async function openEntry(page: Page, isMobile: boolean) {
	if (isMobile) {
		await page
			.getByRole('navigation', { name: 'Main' })
			.getByRole('button', { name: 'You', exact: true })
			.click();
		return page
			.getByRole('dialog')
			.getByRole('link', { name: /Who's cooking\?|Sign in another person/ });
	}
	await page.getByRole('button', { name: 'Account menu' }).click();
	return page.getByRole('menuitem', { name: /Who's cooking\?|Sign in another person/ });
}

test.beforeEach(async ({ page }) => {
	await page.emulateMedia({ reducedMotion: 'reduce' });
	await login(page);
});

test('lists the active person first, then the device, then "Add a person"', async ({ page }) => {
	await mockDevice(page);
	await page.goto('/switch');
	await expect(page.getByRole('heading', { name: "Who's cooking?" })).toBeVisible();
	await expect(cards(page)).toHaveCount(4);
	await expect(cards(page).nth(0)).toContainText('Cooking now');
	await expect(cards(page).nth(1)).toContainText('Jo');
	await expect(cards(page).nth(1)).toContainText('Here yesterday');
	await expect(cards(page).nth(2)).toContainText('Sign in again');
	await expect(cards(page).nth(3)).toContainText('Add a person');
});

test('an expired card asks for the password again, keeping where it was going', async ({
	page
}) => {
	await mockDevice(page);
	await page.goto('/switch?next=%2Flists');
	await expect(page.getByRole('link', { name: /Add a person/ })).toHaveAttribute(
		'href',
		'/login?add=1&next=%2Flists'
	);
	await page.getByTitle(LONG).click();
	await page.waitForURL(/\/login\?add=1&username=max&next=%2Flists$/);
});

test('editing shows a labeled × and removes after the confirmation', async ({ page }) => {
	const calls = await mockDevice(page);
	await page.goto('/switch');
	const toggle = page.getByRole('button', { name: 'Edit' });
	await toggle.click();
	await expect(page.getByRole('button', { name: 'Done' })).toHaveAttribute('aria-pressed', 'true');
	await expect(page.getByRole('button', { name: /^Remove / })).toHaveCount(2);
	await expect(page.getByText('Add a person')).toBeHidden();

	await page.getByRole('button', { name: 'Remove Jo' }).click();
	const dialog = page.getByRole('dialog');
	await expect(dialog).toContainText('Remove from this device?');
	await dialog.getByRole('button', { name: 'Remove', exact: true }).click();
	await expect(page.getByTitle('Jo', { exact: true })).toBeHidden();
	expect(calls).toContain('DELETE /api/v1/auth/accounts/u-jo');
});

test('a double tap on Remove removes once', async ({ page }) => {
	await page.emulateMedia({ reducedMotion: 'no-preference' });
	const calls = await mockDevice(page);
	await page.goto('/switch');
	await page.getByRole('button', { name: 'Edit' }).click();
	await page.getByRole('button', { name: 'Remove Jo' }).click();
	await page.getByRole('dialog').getByRole('button', { name: 'Remove', exact: true }).dblclick();
	await expect(page.getByTitle('Jo', { exact: true })).toBeHidden();
	await expect(page.getByRole('dialog')).toBeHidden();
	expect(calls.filter((c) => c.startsWith('DELETE'))).toEqual([
		'DELETE /api/v1/auth/accounts/u-jo'
	]);
});

test('picking a card switches and opens the app', async ({ page }) => {
	const calls = await mockDevice(page);
	await page.goto('/switch');
	await page.getByTitle('Jo', { exact: true }).click();
	await expect(page.getByText('Hello, Jo!')).toBeVisible();
	await page.waitForURL((url) => url.pathname === '/');
	expect(calls.filter((c) => c.startsWith('POST'))).toEqual(['POST /switch u-jo']);
});

test('a long name is clamped to two lines and 320px never scrolls sideways', async ({ page }) => {
	await mockDevice(page);
	await page.setViewportSize({ width: 320, height: 700 });
	await page.goto('/switch');
	const name = page.getByTitle(LONG).locator('[data-name]');
	await expect(name).toBeVisible();
	expect(await name.evaluate((el) => el.scrollHeight > el.clientHeight)).toBe(true);
	expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
		true
	);
});

test('after a remove the focus moves to the next card, then to the heading', async ({ page }) => {
	await mockDevice(page);
	await page.goto('/switch');
	await page.getByRole('button', { name: 'Edit' }).click();
	await page.getByRole('button', { name: 'Remove Jo' }).click();
	await page.getByRole('dialog').getByRole('button', { name: 'Remove', exact: true }).click();
	await expect(page.getByRole('button', { name: `Remove ${LONG}` })).toBeFocused();
	await page.keyboard.press('Enter');
	await page.getByRole('dialog').getByRole('button', { name: 'Remove', exact: true }).click();
	await expect(page.getByRole('heading', { name: "Who's cooking?" })).toBeFocused();
});

test('a session that ran out since the list loaded asks for the password', async ({ page }) => {
	await mockDevice(page, { status: 409 });
	await page.goto('/switch?next=%2Flists');
	await page.getByTitle('Jo', { exact: true }).click();
	await page.waitForURL(/\/login\?add=1&username=jo&next=%2Flists$/);
});

test('an account gone from the device reloads the list', async ({ page }) => {
	const calls = await mockDevice(page, { status: 404 });
	await page.goto('/switch');
	await page.getByTitle('Jo', { exact: true }).click();
	await expect.poll(() => calls.filter((c) => c === 'GET /accounts').length).toBe(2);
	await expect(page.getByText('Hello, Jo!')).toBeHidden();
	await page.getByTitle('Jo', { exact: true }).click();
	await expect.poll(() => calls.filter((c) => c === 'GET /accounts').length).toBe(3);
});

test('the pick runs its view transition and leaves nothing named behind', async ({ page }) => {
	await page.emulateMedia({ reducedMotion: 'no-preference' });
	await mockDevice(page, { delay: 3000 });
	await page.goto('/switch');
	await page.getByTitle('Jo', { exact: true }).click();
	await expect(page.getByText('Hello, Jo!')).toBeVisible();
	// The page behind the splash takes no keys.
	await expect(page.locator('main')).toHaveAttribute('inert');
	await expect
		.poll(() =>
			page.evaluate(() => ({
				switching: document.documentElement.dataset.switching,
				named: [...document.querySelectorAll<HTMLElement>('*')].filter(
					(el) => el.style.viewTransitionName
				).length
			}))
		)
		.toEqual({ switching: undefined, named: 0 });
	await expect(page.getByText('Hello, Jo!')).toBeVisible();
});

test('a pick onto a page outside the app leaves no arrival behind', async ({ page }) => {
	await page.emulateMedia({ reducedMotion: 'no-preference' });
	await mockDevice(page);
	await page.goto('/switch?next=%2Fapi%2Fv1%2Fsettings');
	await page.getByTitle('Jo', { exact: true }).click();
	await page.waitForURL((url) => url.pathname === '/api/v1/settings');
	expect(await page.evaluate(() => sessionStorage.getItem('rezepte-arrival'))).toBeNull();
});

test('the close × folds the cards away and goes back once', async ({ page, isMobile }) => {
	await page.emulateMedia({ reducedMotion: 'no-preference' });
	await mockDevice(page);
	// Within the app, so the page's own back stays in this document.
	await page.goto('/lists');
	await (await openEntry(page, isMobile)).click();
	await expect(cards(page)).toHaveCount(4);
	await page.evaluate(() => {
		const w = window as unknown as { backs: number[] };
		w.backs = [];
		addEventListener('popstate', () => w.backs.push(performance.now()));
	});
	// A double tap during the exit must not go back twice.
	await page.getByRole('button', { name: 'Close' }).dblclick();
	await expect(page).toHaveURL(/\/lists$/);
	await expect(page.getByRole('heading', { name: "Who's cooking?" })).toBeHidden();
	// Settled half a second after the first back, with that one only.
	await expect
		.poll(() =>
			page.evaluate(() => {
				const { backs } = window as unknown as { backs: number[] };
				return backs.length && performance.now() - backs[0] > 500 ? backs.length : 0;
			})
		)
		.toBe(1);
});

test("the × after the add mode's way back still returns to the app", async ({ page }) => {
	await mockDevice(page);
	await page.goto('/lists');
	await page.goto('/switch');
	await page.getByText('Add a person').click();
	await page.getByRole('link', { name: /Back to .Who's cooking\?./ }).click();
	await page.getByRole('button', { name: 'Close' }).click();
	await page.waitForURL((url) => url.pathname === '/lists');
});

test('the entry row names the other people, and offers adding when there are none', async ({
	page,
	isMobile
}) => {
	await mockDevice(page);
	let row = await openEntry(page, isMobile);
	await expect(row).toContainText("Who's cooking?");
	await expect(row.getByText('J', { exact: true })).toBeVisible();

	await page.reload();
	await mockDevice(page, { list: [] });
	row = await openEntry(page, isMobile);
	await expect(row).toContainText('Sign in another person');
});

test('the add mode of the login page prefills the name and goes back to the picker', async ({
	page,
	isMobile
}) => {
	await page.goto('/login?add=1&username=mila');
	await expect(page.getByRole('heading', { name: 'Sign in another person' })).toBeAttached();
	await expect(page.getByLabel('Username')).toHaveValue('mila');
	// A phone opens straight in the sheet but focuses nothing: without a tap
	// iOS would not raise the keyboard anyway.
	if (isMobile) {
		expect(await page.evaluate(() => document.activeElement === document.body)).toBe(true);
	} else {
		await expect(page.getByLabel('Password')).toBeFocused();
	}
	await expect(page.getByRole('link', { name: /Back to .Who's cooking\?./ })).toHaveAttribute(
		'href',
		'/switch'
	);
});

test('a 401 goes to the picker when the device knows someone, else to the login', async ({
	page
}) => {
	await page.route('**/api/v1/auth/me', (route) => route.fulfill({ status: 401, json: {} }));
	await mockDevice(page);
	await page.goto('/');
	await expect(page).toHaveURL(/\/switch\?next=%2F$/);

	await mockDevice(page, { list: [] });
	await page.goto('/');
	await expect(page).toHaveURL(/\/login\?next=%2F/);
});

// The shell is loaded; the next page's call is the one that finds the
// session gone, so this is the API client's 401, not the root load's.
test('a 401 mid-use goes to the picker and back to the page after the switch', async ({
	page,
	isMobile
}) => {
	test.skip(isMobile, 'the lists link it follows sits in the desktop top bar');
	await mockDevice(page);
	await page.route(
		(url) => url.pathname.startsWith('/api/v1/') && url.pathname !== '/api/v1/auth/accounts',
		(route) => route.fulfill({ status: 401, json: {} })
	);
	await page.getByRole('banner').getByRole('link', { name: 'Shopping lists' }).click();
	await expect(page).toHaveURL(/\/switch\?next=%2Flists$/);
	await expect(page.getByRole('heading', { name: "Who's cooking?" })).toBeVisible();
	// Nobody is active here, so there is no × that would bounce straight back.
	await expect(page.getByRole('button', { name: 'Close' })).toHaveCount(0);

	await page.unrouteAll();
	await mockDevice(page);
	await page.getByRole('button', { name: /Jo/ }).click();
	await page.waitForURL((url) => url.pathname === '/lists');
});

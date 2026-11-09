import { expect, test, type Page, type TestInfo } from '@playwright/test';
import {
	createRecipe,
	createUser,
	devPassword,
	devPasswordNext,
	loadFixture,
	login,
	uniqueToken
} from './helpers';

// Switching people on one device against the real service, no mocks: add a
// second person, switch both ways, sign out as them, remove them, and a cook
// timer riding through a switch. The device's accounts live in the browser
// context's cookie jar, so each test starts with an empty device.

test.use({ contextOptions: { reducedMotion: 'reduce' } });

const mobile = (info: TestInfo) => info.project.name.startsWith('mobile');

/** Opens the account menu (desktop) or the You sheet (phone); returns its surface. */
async function openMenu(page: Page, info: TestInfo) {
	if (mobile(info)) {
		await page
			.getByRole('navigation', { name: 'Main' })
			.getByRole('button', { name: 'You', exact: true })
			.click();
		return page.getByRole('dialog');
	}
	await page.getByRole('button', { name: 'Account menu' }).click();
	return page.getByRole('menu');
}

/** The signed-in person, as the menu's header names them. */
async function expectCooking(page: Page, info: TestInfo, name: string) {
	const menu = await openMenu(page, info);
	await expect(menu).toContainText(name);
	await page.keyboard.press('Escape');
	await expect(menu).toBeHidden();
}

async function openSwitch(page: Page, info: TestInfo) {
	const menu = await openMenu(page, info);
	const row = mobile(info)
		? menu.getByRole('link', { name: /Who's cooking\?/ })
		: menu.getByRole('menuitem', { name: /Who's cooking\?/ });
	await row.click();
	await expect(page.getByRole('heading', { name: "Who's cooking?" })).toBeVisible();
}

const card = (page: Page, name: string) => page.getByTitle(name, { exact: true });

test('people on one device: add, switch, sign out, remove', async ({ page }, info) => {
	const token = uniqueToken();
	const mila = `mila${token}`;
	await login(page);
	const me = (await (await page.request.get('/api/v1/auth/me')).json()) as { displayName: string };
	await createUser(page, { username: mila, role: 'user' });

	// Add mila through the menu row (nobody else is on the device yet).
	const menu = await openMenu(page, info);
	await (
		mobile(info)
			? menu.getByRole('link', { name: 'Sign in another person' })
			: menu.getByRole('menuitem', { name: 'Sign in another person' })
	).click();
	// Both entry rows lead to the picker; its last card adds a person.
	await page.getByText('Add a person').click();
	await expect(page).toHaveURL(/\/login\?add=1$/);
	await page.getByLabel('Username').fill(mila);
	await page.getByLabel('Password').fill(devPassword(mila));
	await page.getByRole('button', { name: 'Sign in' }).click();
	await page.waitForURL((url) => url.pathname === '/');
	await page.getByRole('main').waitFor();
	await expectCooking(page, info, mila);

	// Back to admin and to mila again, through the cards.
	await openSwitch(page, info);
	await expect(page.getByText('Cooking now')).toBeVisible();
	await card(page, me.displayName).click();
	await page.waitForURL((url) => url.pathname === '/');
	await page.getByRole('main').waitFor();
	await expectCooking(page, info, me.displayName);
	await openSwitch(page, info);
	await card(page, mila).click();
	await page.waitForURL((url) => url.pathname === '/');
	await page.getByRole('main').waitFor();
	await expectCooking(page, info, mila);

	// Signing out as mila leaves her card to sign in again; admin stays valid.
	const out = await openMenu(page, info);
	await (
		mobile(info)
			? out.getByRole('button', { name: 'Sign out' })
			: out.getByRole('menuitem', { name: 'Sign out' })
	).click();
	await page.waitForURL((url) => url.pathname === '/switch');
	await expect(card(page, mila).locator('..')).toContainText('Sign in again');
	await expect(card(page, me.displayName).locator('..')).not.toContainText('Sign in again');

	// Admin's card signs in without a password; then mila is removed.
	await card(page, me.displayName).click();
	await page.waitForURL((url) => url.pathname === '/');
	await expectCooking(page, info, me.displayName);
	await openSwitch(page, info);
	await page.getByRole('button', { name: 'Edit' }).click();
	await page.getByRole('button', { name: `Remove ${mila}` }).click();
	await page.getByRole('dialog').getByRole('button', { name: 'Remove', exact: true }).click();
	await expect(card(page, mila)).toBeHidden();
	await page.reload();
	await expect(page.getByRole('heading', { name: "Who's cooking?" })).toBeVisible();
	await expect(card(page, mila)).toHaveCount(0);
});

test('a running cook timer survives a switch', async ({ page }, info) => {
	const token = uniqueToken();
	const mila = `mila${token}`;
	await login(page);
	const me = (await (await page.request.get('/api/v1/auth/me')).json()) as { displayName: string };
	await createUser(page, { username: mila, role: 'user' });
	const fixture = loadFixture(0, 'de');
	const recipe = await createRecipe(page, { ...fixture, title: `${fixture.title} ${token}` });
	// Put mila on the device; she is then the one cooking.
	await page.goto('/switch');
	await page.getByText('Add a person').click();
	await page.getByLabel('Username').fill(mila);
	await page.getByLabel('Password').fill(devPassword(mila));
	await page.getByRole('button', { name: 'Sign in' }).click();
	await page.waitForURL((url) => url.pathname === '/');
	await page.getByRole('main').waitFor();

	await page.goto(`/recipes/${recipe.slug}/cook`);
	for (let i = 1; i < 3; i++) await page.getByRole('button', { name: 'Next' }).click();
	await page.getByRole('button', { name: /^Start timer: 15/ }).click();
	await expect(page.locator('[data-timer-dock]')).toBeVisible();

	// Cook mode has no menu, so the picker is reached by address.
	await page.goto('/switch');
	await card(page, me.displayName).click();
	await page.waitForURL((url) => url.pathname === '/');
	await page.getByRole('main').waitFor();
	await expectCooking(page, info, me.displayName);
	// The dock lives in cook mode only; the timer is still running there.
	await page.goto(`/recipes/${recipe.slug}/cook`);
	await expect(page.locator('[data-timer-dock]')).toBeVisible();
});

/** Signs out through the menu; with the person still known it lands on the picker. */
async function signOutHere(page: Page, info: TestInfo) {
	const menu = await openMenu(page, info);
	await (
		mobile(info)
			? menu.getByRole('button', { name: 'Sign out' })
			: menu.getByRole('menuitem', { name: 'Sign out' })
	).click();
	await page.waitForURL((url) => url.pathname === '/switch');
}

test('a deep link after the own session ran out lands there after signing in again', async ({
	page,
	browser
}) => {
	const token = uniqueToken();
	const mila = `mila${token}`;
	await login(page);
	await createUser(page, { username: mila, role: 'user' });
	const fixture = loadFixture(0);
	const recipe = await createRecipe(page, { ...fixture, title: `${fixture.title} ${token}` });
	// A device that knows mila only, whose session is gone: a password
	// change elsewhere ends every other session of hers.
	await page.context().clearCookies();
	await login(page, mila);
	const elsewhere = await browser.newContext();
	const there = await elsewhere.newPage();
	await login(there, mila);
	const changed = await there.request.patch('/api/v1/auth/me', {
		data: { currentPassword: devPassword(mila), password: devPasswordNext(mila) }
	});
	expect(changed.ok()).toBe(true);
	await elsewhere.close();

	await page.goto(`/recipes/${recipe.slug}`);
	await page.waitForURL((url) => url.pathname === '/switch');
	await expect(card(page, mila).locator('..')).toContainText('Sign in again');
	await card(page, mila).click();
	await page.waitForURL((url) => url.pathname === '/login');
	await page.getByLabel('Password').fill(devPasswordNext(mila));
	await page.getByRole('button', { name: 'Sign in' }).click();
	await page.waitForURL((url) => url.pathname === `/recipes/${recipe.slug}`);
});

test('another open tab follows a switch and a sign-out', async ({ page }, info) => {
	const mila = `mila${uniqueToken()}`;
	await login(page);
	const me = (await (await page.request.get('/api/v1/auth/me')).json()) as { displayName: string };
	await createUser(page, { username: mila, role: 'user' });
	await login(page, mila);
	const other = await page.context().newPage();
	await other.goto('/');
	await other.getByRole('main').waitFor();
	await expectCooking(other, info, mila);

	await page.goto('/switch');
	const reloaded = other.waitForEvent('load');
	await card(page, me.displayName).click();
	await reloaded;
	await expectCooking(other, info, me.displayName);

	await signOutHere(page, info);
	await other.waitForURL((url) => url.pathname === '/switch');
});

test.describe('with motion', () => {
	test.use({ contextOptions: { reducedMotion: 'no-preference' } });

	test('a pick carries the splash into the app and clears it', async ({ page }, info) => {
		const mila = `mila${uniqueToken()}`;
		await login(page);
		const me = (await (await page.request.get('/api/v1/auth/me')).json()) as {
			displayName: string;
		};
		await createUser(page, { username: mila, role: 'user' });
		// Signed in on the same page, so admin stays on the device beside her.
		await login(page, mila);

		await page.goto('/switch');
		await card(page, me.displayName).click();
		await page.waitForURL((url) => url.pathname === '/');
		// The reload opens under the splash's ground (app.html), which closes
		// into the avatar and is gone well within 2.5s.
		await expect(page.locator('html')).not.toHaveAttribute('data-arrival', { timeout: 2500 });
		expect(
			await page.evaluate(() => getComputedStyle(document.documentElement, '::after').content)
		).toBe('none');
		await expectCooking(page, info, me.displayName);

		// One-shot: a reload later shows nothing.
		await page.reload();
		await page.getByRole('main').waitFor();
		await expect(page.locator('html')).not.toHaveAttribute('data-arrival');
	});
});

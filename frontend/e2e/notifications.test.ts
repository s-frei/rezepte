import { expect, test, type Page } from '@playwright/test';
import {
	addDiaryEntry,
	createRecipe,
	createUser,
	loadFixture,
	login,
	uniqueToken
} from './helpers';

/** Signs whoever is in out and name in. */
async function switchTo(page: Page, name: string) {
	await page.context().clearCookies({ name: 'rezepte_session' });
	await login(page, name);
	await expect(page).toHaveURL('/');
}

// The mailed flow (comment mail, unsubscribe link, Undo) lives in
// mail.test.ts: mail settings are instance-wide, and only that file's serial
// run may change them.

test('a broken unsubscribe link says so and leads to the settings', async ({ page }) => {
	await page.goto('/unsubscribe?u=nobody&k=comment&s=made-up');
	await expect(page.getByRole('heading', { name: 'This link no longer works' })).toBeVisible();
	await page.getByRole('link', { name: 'Manage all notifications' }).click();
	await expect(page).toHaveURL(/\/login\?next=%2Fsettings%2Fnotifications$/);
});

test('without an address the page says so and keeps the switches', async ({ page }) => {
	// Answered, not set: mail settings belong to mail.test.ts.
	let mailEnabled = true;
	await page.route('**/api/v1/settings', async (route) => {
		const response = await route.fetch();
		await route.fulfill({ response, json: { ...(await response.json()), mailEnabled } });
	});
	await login(page);
	const name = `noaddr${uniqueToken()}`;
	await createUser(page, { username: name, role: 'user' });
	await switchTo(page, name);
	await page.goto('/settings/notifications');
	const mail = page.getByRole('region', { name: 'By mail' });
	await expect(mail.getByText('No address yet')).toBeVisible();
	// Greyed, but a preference outlives the missing address.
	await expect(
		mail.getByRole('group', { name: 'New comments' }).getByRole('radio', { name: 'Off' })
	).toBeEnabled();
	await expect(mail.getByRole('link', { name: 'Add in profile' })).toHaveAttribute(
		'href',
		'/settings'
	);
	// Confirming takes a mail too, so a page without mail says so instead;
	// only the owner can set mail up.
	mailEnabled = false;
	await page.reload();
	await expect(mail.getByText("Mail isn't set up on this instance")).toBeVisible();
	await expect(mail.getByRole('link')).toHaveCount(0);
});

test('the owner is offered to set up mail', async ({ page }) => {
	await page.route('**/api/v1/settings', async (route) => {
		const response = await route.fetch();
		await route.fulfill({ response, json: { ...(await response.json()), mailEnabled: false } });
	});
	await login(page);
	await page.goto('/settings/notifications');
	const mail = page.getByRole('region', { name: 'By mail' });
	await expect(mail.getByRole('link', { name: 'Set up mail' })).toHaveAttribute(
		'href',
		'/settings/mail'
	);
});

test('the notifications page fits a 360px phone', async ({ page }) => {
	await page.setViewportSize({ width: 360, height: 780 });
	// The owner with a long confirmed address: every group and its switches show.
	await page.route('**/api/v1/auth/me', async (route) => {
		const response = await route.fetch();
		const user = await response.json();
		await route.fulfill({
			response,
			json: { ...user, email: 'a.rather.long.address@kitchen.example.org', emailVerified: true }
		});
	});
	await login(page);
	await page.goto('/settings/notifications');
	await expect(
		page.getByRole('region', { name: 'Push' }).getByRole('group', { name: 'Mail delivery failing' })
	).toBeVisible();
	const scroll = await page.evaluate(() => document.documentElement.scrollWidth - innerWidth);
	expect(scroll).toBeLessThanOrEqual(0);
});

// The rest reads the page in German, as a member: "Per Mail" and "Push".

/** A German member, signed in; `push` is what this browser makes of push. */
async function germanMember(page: Page, push: NotificationPermission | 'unsupported' = 'denied') {
	// Playwright's Chromium has no push service: a fake subscription stands
	// in and the server's answer is stubbed, as in cook-timers.test.ts.
	await page.addInitScript((push) => {
		if (push === 'unsupported') {
			delete (window as { PushManager?: unknown }).PushManager;
			return;
		}
		let permission: NotificationPermission = push;
		Object.defineProperty(Notification, 'permission', { get: () => permission });
		Notification.requestPermission = async () => (permission = 'granted');
		PushManager.prototype.getSubscription = async () => null;
		PushManager.prototype.subscribe = async () =>
			({
				endpoint: 'https://fcm.googleapis.com/fcm/send/e2e',
				toJSON: () => ({ keys: { p256dh: 'p', auth: 'a' } })
			}) as unknown as PushSubscription;
	}, push);
	await page.route('**/api/v1/push/subscription', (route) => route.fulfill({ status: 204 }));
	await login(page);
	const name = `pushde${uniqueToken()}`;
	await createUser(page, { username: name, role: 'user', locale: 'de' });
	await page.context().clearCookies();
	await login(page, name);
	await expect(page).toHaveURL('/');
	await page.goto('/settings/notifications');
	return {
		mail: page.getByRole('region', { name: 'Per Mail' }),
		push: page.getByRole('region', { name: 'Push' })
	};
}

test('a kind is switched per channel', async ({ page }) => {
	const { mail, push } = await germanMember(page);
	const saved = page.waitForRequest(
		(r) => r.url().endsWith('/api/v1/auth/me/notifications') && r.method() === 'PUT'
	);
	const comments = push.getByRole('group', { name: 'Neue Kommentare' });
	await comments.getByRole('radio', { name: 'Aus' }).click();
	expect((await saved).postDataJSON()).toEqual({ kinds: { comment: { push: false } }, groups: {} });
	await expect(comments.getByRole('radio', { name: 'Aus' })).toHaveAttribute(
		'aria-checked',
		'true'
	);
	// Clicking the chosen side again changes nothing.
	await comments.getByRole('radio', { name: 'Aus' }).click();
	await expect(comments.getByRole('radio', { name: 'Aus' })).toHaveAttribute(
		'aria-checked',
		'true'
	);
	await page.reload();
	await expect(comments.getByRole('radio', { name: 'Aus' })).toHaveAttribute(
		'aria-checked',
		'true'
	);
	await expect(
		mail.getByRole('group', { name: 'Neue Kommentare' }).getByRole('radio', { name: 'An' })
	).toHaveAttribute('aria-checked', 'true');
});

test("a group's switch holds its kinds and keeps their choices", async ({ page }) => {
	const { push } = await germanMember(page);
	const recipes = push.getByRole('group', { name: 'Neue Rezepte' });
	await recipes.getByRole('radio', { name: 'Aus' }).click();
	await expect(recipes.getByRole('radio', { name: 'Aus' })).toHaveAttribute('aria-checked', 'true');
	const household = push.getByRole('switch', { name: 'Haushalt' });
	await household.click();
	await expect(household).toHaveAttribute('aria-checked', 'false');
	await expect(recipes.getByRole('radio', { name: 'An' })).toBeDisabled();
	await expect(
		push.getByRole('group', { name: 'Lecker' }).getByRole('radio', { name: 'Aus' })
	).toBeDisabled();
	await page.reload();
	await expect(recipes.getByRole('radio', { name: 'An' })).toBeDisabled();
	await household.click();
	await expect(recipes.getByRole('radio', { name: 'An' })).toBeEnabled();
	await expect(recipes.getByRole('radio', { name: 'Aus' })).toHaveAttribute('aria-checked', 'true');
	await expect(
		push.getByRole('group', { name: 'Neue Kommentare' }).getByRole('radio', { name: 'An' })
	).toHaveAttribute('aria-checked', 'true');
});

test('this device is enabled and sent a test', async ({ page }) => {
	const { push } = await germanMember(page, 'default');
	await page.route('**/api/v1/push/test', (route) => route.fulfill({ status: 204 }));
	await expect(push.getByText('Erlaube Benachrichtigungen, damit sie hier ankommen')).toBeVisible();
	await push.getByRole('button', { name: 'Auf diesem Gerät aktivieren' }).click();
	await expect(push.getByText('Empfängt Benachrichtigungen')).toBeVisible();
	const test = page.waitForRequest(
		(r) => r.url().endsWith('/api/v1/push/test') && r.method() === 'POST'
	);
	await push.getByRole('button', { name: 'Test senden' }).click();
	expect((await test).postDataJSON()).toEqual({
		endpoint: 'https://fcm.googleapis.com/fcm/send/e2e'
	});
	await expect(push.getByText(/^Gesendet\. Nichts angekommen\?/)).toBeVisible();
});

test('the live region for a sent test is there before it and speaks again on a repeat', async ({
	page
}) => {
	const { push } = await germanMember(page, 'granted');
	// Screen readers announce a change of a live region's text, not one
	// inserted along with it.
	const status = push.getByRole('status');
	await expect(status).toHaveCount(1);
	await expect(status).toHaveText('');
	let release!: () => void;
	let hold: Promise<void> | null = null;
	await page.route('**/api/v1/push/test', async (route) => {
		await hold;
		await route.fulfill({ status: 204 });
	});
	const send = push.getByRole('button', { name: 'Test senden' });
	await send.click();
	await expect(status).toHaveText(/^Gesendet\. Nichts angekommen\?/);
	hold = new Promise((done) => (release = done));
	await send.click();
	// Cleared while the repeat is on its way, so its answer is new text.
	await expect(status).toHaveText('');
	release();
	await expect(status).toHaveText(/^Gesendet\. Nichts angekommen\?/);
});

test('a device allowed before is enabled on load', async ({ page }) => {
	const { push } = await germanMember(page, 'granted');
	await expect(push.getByText('Empfängt Benachrichtigungen')).toBeVisible();
	await expect(push.getByRole('button', { name: 'Test senden' })).toBeVisible();
});

test('blocked notifications are explained', async ({ page }) => {
	const { push } = await germanMember(page, 'denied');
	await expect(push.getByText(/^Benachrichtigungen für Rezepte sind blockiert/)).toBeVisible();
	await expect(push.getByRole('button')).toHaveCount(0);
});

test('without push the device box says why, and the switches still work', async ({ page }) => {
	const { push } = await germanMember(page, 'unsupported');
	await expect(push.getByText(/Auf dem iPhone nur in der App vom Home-Bildschirm/)).toBeVisible();
	await expect(push.getByRole('button', { name: 'Auf diesem Gerät aktivieren' })).toHaveCount(0);
	const tasty = push.getByRole('group', { name: 'Lecker' });
	await tasty.getByRole('radio', { name: 'Aus' }).click();
	await expect(tasty.getByRole('radio', { name: 'Aus' })).toHaveAttribute('aria-checked', 'true');
});

test('a late reply does not undo a newer flip', async ({ page }) => {
	const { push } = await germanMember(page);
	// The first PUT's reply is held until the second one has answered.
	let second!: () => void;
	const secondDone = new Promise<void>((done) => (second = done));
	let puts = 0;
	await page.route('**/api/v1/auth/me/notifications', async (route) => {
		if (route.request().method() !== 'PUT') return route.continue();
		const first = ++puts === 1;
		const response = await route.fetch();
		if (first) await secondDone;
		await route.fulfill({ response });
		if (!first) second();
	});
	const comments = push.getByRole('group', { name: 'Neue Kommentare · Push', exact: true });
	const firstReply = page.waitForResponse(
		(r) => r.url().endsWith('/api/v1/auth/me/notifications') && r.request().method() === 'PUT'
	);
	await comments.getByRole('radio', { name: 'Aus' }).click();
	await comments.getByRole('radio', { name: 'An' }).click();
	await firstReply;
	await expect.poll(() => puts).toBe(2);
	// Both replies are in; the second flip's value stands.
	await page.waitForTimeout(200);
	await expect(comments.getByRole('radio', { name: 'An' })).toHaveAttribute('aria-checked', 'true');
	await page.reload();
	await expect(comments.getByRole('radio', { name: 'An' })).toHaveAttribute('aria-checked', 'true');
});

test("a held group's rows stay focusable but do not change", async ({ page }) => {
	const { mail, push } = await germanMember(page);
	// Each toggle is named by its kind and its channel.
	await expect(
		mail.getByRole('group', { name: 'Neue Rezepte · Per Mail', exact: true })
	).toBeVisible();
	await push.getByRole('switch', { name: 'Haushalt · Push', exact: true }).click();
	const recipes = push.getByRole('group', { name: 'Neue Rezepte · Push', exact: true });
	const off = recipes.getByRole('radio', { name: 'Aus' });
	await expect(off).toHaveAttribute('aria-disabled', 'true');
	await expect(off).toHaveAttribute('aria-checked', 'false');
	let puts = 0;
	page.on('request', (r) => {
		if (r.url().endsWith('/api/v1/auth/me/notifications') && r.method() === 'PUT') puts++;
	});
	await off.focus();
	await expect(off).toBeFocused();
	await page.keyboard.press('Space');
	await page.keyboard.press('Enter');
	await off.click({ force: true });
	await page.waitForTimeout(300);
	expect(puts).toBe(0);
	await expect(off).toHaveAttribute('aria-checked', 'false');
	// Tab reaches the held rows too: from the switch into the first kind's toggle.
	await push.getByRole('switch', { name: 'Haushalt · Push', exact: true }).focus();
	await page.keyboard.press('Tab');
	await expect(
		push.getByRole('group', { name: 'Neue Kommentare · Push', exact: true }).locator(':focus')
	).toHaveCount(1);
});

// The worker hands a push to a visible window with postMessage; the test
// dispatches that message itself, since Playwright's Chromium has no push
// service to send a real one.
test('a push in a visible window also shows as a toast that opens its page', async ({ page }) => {
	await login(page);
	await page.goto('/settings/notifications');
	const post = (data: unknown) =>
		page.evaluate((d) => {
			navigator.serviceWorker.dispatchEvent(new MessageEvent('message', { data: d }));
		}, data);
	// Not the worker's message, or a url leaving the app: no toast.
	await post({ type: 'other', title: 'Nope', body: '', url: '/', tag: 'x' });
	await post({
		type: 'rezepte-push',
		title: 'Away',
		body: '',
		url: 'https://evil.example/',
		tag: 'x'
	});
	await post({
		type: 'rezepte-push',
		title: 'Soup',
		body: '3 new comments',
		url: '/recipes/does-not-matter?tab=comments',
		tag: 'comment:r1'
	});
	const toast = page.locator('[data-sonner-toast]');
	await expect(toast).toHaveCount(1);
	await expect(toast).toContainText('Soup');
	await expect(toast).toContainText('3 new comments');
	await toast.getByRole('button', { name: 'Open' }).click();
	await expect(page).toHaveURL('/recipes/does-not-matter?tab=comments');
});

/** Hands the page what the worker posts for a push, as the test above does. */
const postToast = (page: Page, data: unknown) =>
	page.evaluate((d) => {
		navigator.serviceWorker.dispatchEvent(new MessageEvent('message', { data: d }));
	}, data);

test("a toast's Open on the recipe already open shows the new comment", async ({ page }) => {
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(1), title: `Toast ${uniqueToken()}` });
	await page.goto(`/recipes/${recipe.slug}`);
	await expect(page.getByRole('heading', { name: recipe.title })).toBeVisible();
	const body = `Fresh comment ${uniqueToken()}`;
	await addDiaryEntry(page, recipe.id, body);
	await postToast(page, {
		type: 'rezepte-push',
		title: recipe.title,
		body: '1 new comment',
		url: `/recipes/${recipe.slug}?tab=comments`,
		tag: `comment:${recipe.id}`
	});
	await page.locator('[data-sonner-toast]').getByRole('button', { name: 'Open' }).click();
	await expect(page.getByRole('tabpanel', { name: /^Comments/ }).getByText(body)).toBeVisible();
});

test('a later push under the same tag replaces its toast', async ({ page }) => {
	await login(page);
	await page.goto('/settings/notifications');
	// The app's listener is up once it has rendered, not at the load event.
	await expect(page.getByRole('region', { name: 'Push' })).toBeVisible();
	const push = (body: string) =>
		postToast(page, { type: 'rezepte-push', title: 'Soup', body, url: '/', tag: 'comment:r9' });
	await push('1 new comment');
	const toast = page.locator('[data-sonner-toast]');
	await expect(toast).toContainText('1 new comment');
	await push('2 new comments');
	await expect(toast).toHaveCount(1);
	await expect(toast).toContainText('2 new comments');
});

test('signed out, a push shows no toast', async ({ page }) => {
	await page.goto('/login');
	await expect(page.getByLabel('Username')).toBeVisible();
	await postToast(page, { type: 'rezepte-push', title: 'Soup', body: '', url: '/', tag: 'x' });
	await page.waitForTimeout(300);
	await expect(page.locator('[data-sonner-toast]')).toHaveCount(0);
	await expect(page).toHaveURL(/\/login/);
});

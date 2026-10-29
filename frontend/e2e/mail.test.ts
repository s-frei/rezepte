import {
	expect,
	test,
	type APIRequestContext,
	type BrowserContext,
	type Page
} from '@playwright/test';
import {
	addDiaryEntry,
	createRecipe,
	createUser,
	devPassword,
	devPasswordNext,
	loadFixture,
	login,
	uniqueToken
} from './helpers';
import {
	latestMailTo,
	linkIn,
	mailCountTo,
	mailpitSmtpPort,
	mailpitUrl,
	setupLinkIn
} from './mailpit';

// Mail is instance-wide, so this file configures it once and runs serially
// on one project, like oidc.test.ts with its shared Dex accounts.
test.describe.configure({ mode: 'serial' });

test.beforeEach(() => {
	test.skip(
		test.info().project.name !== 'desktop',
		'instance-wide mail settings; runs once, on desktop'
	);
	test.skip(!mailpitUrl, 'Mailpit is not running (mise run mail:up)');
});

async function configure(page: Page, port: number) {
	await page.goto('/settings/mail');
	const card = page.getByRole('region', { name: 'Email' });
	await card.getByRole('button', { name: /^(Set up mail|Edit)$/ }).click();
	const dialog = page.getByRole('dialog');
	await dialog.getByLabel('From address').fill('rezepte@example.com');
	await dialog.getByLabel('Server', { exact: true }).fill('localhost');
	await dialog.getByLabel('Port').fill(String(port));
	await dialog.getByRole('radio', { name: 'None' }).click();
	await dialog.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(card.getByText('Sending on')).toBeVisible();
	await expect(dialog).toBeHidden();
}

async function addAccount(page: Page, name: string, email?: string) {
	await page.goto('/settings/users');
	await page.getByRole('button', { name: 'Add account' }).click();
	const dialog = page.getByRole('dialog');
	await dialog.getByLabel('Username').fill(name);
	if (email) await dialog.getByLabel(/^Email/).fill(email);
	await dialog
		.getByRole('button', { name: email ? 'Create and send link' : 'Add', exact: true })
		.click();
}

test('the owner sets up mail and sends a test', async ({ page, request }) => {
	await login(page);
	await configure(page, mailpitSmtpPort);
	const to = `owner-${uniqueToken()}@example.com`;
	const card = page.getByRole('region', { name: 'Email' });
	await card.getByLabel('Send a test mail to').fill(to);
	await card.getByLabel('Send a test mail to').press('Enter');
	await expect(card.getByRole('status')).toContainText(`Test mail sent to ${to}`);
	expect((await latestMailTo(request, to)).subject).toBe('Rezepte can send mail');
});

test('the owner moves the new recipes mail; an admin never sees the mail page', async ({
	page,
	browser
}) => {
	await login(page);
	await configure(page, mailpitSmtpPort);
	const card = page.getByRole('region', { name: 'Email' });
	const hour = card.getByRole('button', { name: 'Daily new-recipes mail at' });
	await expect(hour).toContainText('5:00 PM');
	await hour.click();
	await page.getByRole('option', { name: '7:00 AM' }).click();
	await expect(page.getByText('Send time saved')).toBeVisible();
	await page.reload();
	await expect(hour).toContainText('7:00 AM');
	// Back to the default for the tests after this one.
	await hour.click();
	await page.getByRole('option', { name: '5:00 PM' }).click();
	await expect(page.getByText('Send time saved').first()).toBeVisible();

	// Settings that cannot be read hide only the hour, not the card.
	await page.route('**/api/v1/settings', (route) => route.fulfill({ status: 500 }));
	await page.reload();
	await expect(card.getByText('Sending on')).toBeVisible();
	await expect(card.getByLabel('Send a test mail to')).toBeVisible();
	await expect(hour).toHaveCount(0);
	await page.unroute('**/api/v1/settings');

	const name = `digest-${uniqueToken()}`;
	await createUser(page, { username: name, role: 'admin' });
	const other = await browser.newPage();
	await login(other, name);
	await other.goto('/settings/mail');
	await expect(other).toHaveURL(/\/settings$/);
	await other.close();
});

test('an invited person sets up their account from the mail', async ({
	page,
	request,
	browser
}) => {
	await login(page);
	const name = `lena${uniqueToken()}`;
	const address = `${name}@example.com`;
	await addAccount(page, name, address);
	await expect(
		page
			.getByRole('dialog')
			.getByRole('status')
			.filter({ hasText: `Sent to ${address}` })
	).toBeVisible();

	const mail = await latestMailTo(request, address);
	expect(mail.subject).toContain('invited you to Rezepte');
	const link = setupLinkIn(mail.text);

	const lena = await (await browser.newContext()).newPage();
	await lena.goto(link);
	await lena.getByLabel('New password', { exact: true }).fill(devPassword(name));
	await lena.getByLabel('Repeat the new password').fill(devPassword(name));
	await lena.getByRole('button', { name: 'Save and sign in' }).click();
	await expect(lena).toHaveURL('/');

	const users = await (await page.request.get('/api/v1/users')).json();
	const row = users.items.find((u: { username: string }) => u.username === name);
	expect(row.emailVerified).toBe(true);
});

test('sending the link from the row returns focus to its menu', async ({ page, request }) => {
	await login(page);
	const name = `focus${uniqueToken()}`;
	const address = `${name}@example.com`;
	await addAccount(page, name);
	await expect(page.getByRole('dialog', { name: `Setup link for ${name}` })).toBeVisible();
	await expect(page.getByRole('dialog', { name: 'Add account' })).toBeHidden();
	await page.keyboard.press('Escape');
	await expect(page.getByRole('dialog')).toBeHidden();

	const users = await (await page.request.get('/api/v1/users')).json();
	const id = users.items.find((u: { username: string }) => u.username === name).id;
	await page.locator(`#person-${id}-menu`).click();
	await page.getByRole('menuitem', { name: `Setup link for ${name}`, exact: true }).click();
	const step = page.getByRole('dialog', { name: new RegExp(`^Setup link for ${name}`, 'i') });
	await step.getByLabel('Email').fill(address);
	await step.getByRole('button', { name: 'Send link' }).click();
	await expect(page.getByText(`Sent to ${address}`)).toBeVisible();
	// The step dialog has finished closing behind the result.
	await expect(page.getByRole('dialog')).toHaveCount(1);
	expect((await latestMailTo(request, address)).subject).toContain('invited you to Rezepte');
	await page.keyboard.press('Escape');
	await expect(page.getByRole('dialog')).toBeHidden();
	await expect(page.locator(`#person-${id}-menu`)).toBeFocused();
});

test('passing the link on by hand mails nothing', async ({ page, request }) => {
	await login(page);
	const name = `hand${uniqueToken()}`;
	const address = `${name}@example.com`;
	await addAccount(page, name, address);
	const result = page.getByRole('dialog', { name: `Setup link for ${name}` });
	await expect(result.getByText(`Sent to ${address}`)).toBeVisible();
	// Add account's own close has to finish before Escape reaches the result.
	await expect(page.getByRole('dialog', { name: 'Add account' })).toBeHidden();
	await latestMailTo(request, address);
	const before = await mailCountTo(request, address);
	await page.keyboard.press('Escape');
	await expect(page.getByRole('dialog')).toBeHidden();

	const users = await (await page.request.get('/api/v1/users')).json();
	const id = users.items.find((u: { username: string }) => u.username === name).id;
	await page.locator(`#person-${id}-menu`).click();
	await page.getByRole('menuitem', { name: `Setup link for ${name}`, exact: true }).click();
	const step = page.getByRole('dialog', { name: new RegExp(`^Setup link for ${name}`, 'i') });
	await step.getByRole('radio', { name: 'Pass it on myself' }).click();
	await step.getByRole('button', { name: 'Show link' }).click();
	await expect(page.getByRole('dialog')).toHaveCount(1);
	await expect(result.locator('textarea')).toHaveValue(/\/welcome#/);
	await expect(result.getByText(`Sent to ${address}`)).toHaveCount(0);
	await expect(result.getByRole('status')).toHaveCount(0);
	// A send would be in Mailpit well within this; nothing may arrive.
	await page.waitForTimeout(500);
	expect(await mailCountTo(request, address)).toBe(before);
});

test('a failed send still shows the link', async ({ page }) => {
	await login(page);
	await configure(page, 1); // nothing listens on port 1
	const name = `max${uniqueToken()}`;
	await addAccount(page, name, `${name}@example.com`);
	const dialog = page.getByRole('dialog', { name: `Setup link for ${name}` });
	await expect(dialog.getByRole('alert')).toContainText('could not be sent');
	await expect(dialog.locator('textarea')).toHaveValue(/\/welcome#/);
	// Leave the instance configured for Mailpit for whatever runs next.
	await page.keyboard.press('Escape');
	await configure(page, mailpitSmtpPort);
});

/**
 * A fresh account with a password that sets its address in the profile and
 * opens the confirmation link: the page ends signed in as it.
 */
async function confirmedAccount(page: Page, request: APIRequestContext, prefix: string) {
	await login(page);
	const name = `${prefix}${uniqueToken()}`;
	await createUser(page, { username: name, role: 'user' });
	await page.context().clearCookies({ name: 'rezepte_session' });
	await login(page, name);
	const address = `${name}@example.com`;
	await page.goto('/settings');
	await page.getByLabel('Email', { exact: true }).fill(address);
	await page.getByRole('button', { name: 'Save profile' }).click();
	await expect(
		page.getByText(`Not confirmed yet. Confirmation mail sent to ${address}.`)
	).toBeVisible();
	const mail = await latestMailTo(request, address);
	expect(mail.subject).toBe('Confirm your email address for Rezepte');
	await page.goto(linkIn(mail.text, '/confirm-email'));
	await expect(page.getByText('Address confirmed', { exact: true })).toBeVisible();
	return { name, address };
}

/** Waits until the newest mail to address has subject, and returns it. */
async function mailWithSubject(request: APIRequestContext, address: string, subject: string) {
	await expect
		.poll(async () => (await latestMailTo(request, address)).subject, { timeout: 10_000 })
		.toBe(subject);
	return latestMailTo(request, address);
}

test("an admin changes a member's address and the new one gets a confirmation", async ({
	page,
	request
}) => {
	await login(page);
	const token = uniqueToken();
	const member = `addr${token}`;
	const admin = `addradm${token}`;
	await createUser(page, { username: member, role: 'user' });
	await createUser(page, { username: admin, role: 'admin' });
	await page.context().clearCookies({ name: 'rezepte_session' });
	await login(page, admin);
	await page.goto('/settings/users');
	const row = page.getByRole('listitem').filter({ hasText: member });
	await row.getByRole('button', { name: 'Edit profile' }).click();
	const dialog = page.getByRole('dialog', { name: `Edit ${member}'s profile` });
	const address = `${member}@example.com`;
	await dialog.getByLabel('Email').fill(address);
	await expect(dialog.getByText('A confirmation mail goes to the new address.')).toBeVisible();
	await dialog.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(row.getByRole('button', { name: 'Address not confirmed yet' })).toBeVisible();
	const mail = await latestMailTo(request, address);
	expect(mail.subject).toBe('Confirm your email address for Rezepte');
	expect(mail.text).toContain(admin);
});

test('a changed address is confirmed from its mail', async ({ page, request }) => {
	const { name } = await confirmedAccount(page, request, 'conf');
	await page.goto('/settings');
	await expect(page.getByText('Confirmed', { exact: true })).toBeVisible();
	await page.context().clearCookies({ name: 'rezepte_session' });
	await login(page);
	await page.goto('/settings/users');
	const row = page.getByRole('listitem').filter({ hasText: name });
	await expect(row.getByRole('button', { name: 'Address confirmed' })).toHaveCount(1);
});

test('a forgotten password is reset by username', async ({ page, request }) => {
	const { name, address } = await confirmedAccount(page, request, 'reset');
	await page.context().clearCookies({ name: 'rezepte_session' });
	await page.goto('/login');
	await page.getByLabel('Username').fill(name);
	// The link sits in the password label's row but comes last by keyboard.
	await page.locator('#username').focus();
	await page.keyboard.press('Tab');
	await expect(page.locator('#password')).toBeFocused();
	await page.keyboard.press('Tab');
	await expect(page.getByRole('button', { name: 'Sign in' })).toBeFocused();
	await page.keyboard.press('Tab');
	await expect(page.getByRole('link', { name: 'Forgot password?' })).toBeFocused();
	await page.getByRole('link', { name: 'Forgot password?' }).click();
	// Carried in navigation state, never in the address bar.
	await expect(page).toHaveURL('/forgot-password');
	await expect(page.getByLabel('Username or email')).toHaveValue(name);
	await page.getByRole('button', { name: 'Send link' }).click();
	await expect(page.getByRole('heading', { name: 'Check your inbox' })).toBeVisible();

	const mail = await mailWithSubject(request, address, 'A new password for Rezepte');
	await page.goto(linkIn(mail.text, '/welcome'));
	await expect(page.getByRole('heading', { name: `A new password for ${name}` })).toBeVisible();
	await page.getByLabel('New password', { exact: true }).fill(devPasswordNext(name));
	await page.getByLabel('Repeat the new password').fill(devPasswordNext(name));
	await page.getByRole('button', { name: 'Save password' }).click();
	await expect(page).toHaveURL('/');

	await page.context().clearCookies({ name: 'rezepte_session' });
	await login(page, name);
	await expect(page.getByRole('alert')).toHaveText('That username or password is wrong.');
	await login(page, name, devPasswordNext(name));
	await expect(page).toHaveURL('/');
});

test('a forgotten password is reset by address', async ({ page, request }) => {
	const { address } = await confirmedAccount(page, request, 'byaddr');
	await page.context().clearCookies({ name: 'rezepte_session' });
	await page.goto('/forgot-password');
	await page.getByLabel('Username or email').fill(address.toUpperCase());
	await page.getByRole('button', { name: 'Send link' }).click();
	await mailWithSubject(request, address, 'A new password for Rezepte');
});

test('an unknown name gets the same answer and no mail', async ({ page, request }) => {
	const address = `nobody${uniqueToken()}@example.com`;
	await page.goto('/forgot-password');
	await page.getByLabel('Username or email').fill(address);
	await page.getByRole('button', { name: 'Send link' }).click();
	// The form is gone, so focus moves to the answer.
	await expect(page.getByRole('heading', { name: 'Check your inbox' })).toBeFocused();
	await page.waitForTimeout(1000);
	expect(await mailCountTo(request, address)).toBe(0);
});

// Reached by URL while mail is off. Stubbed, since mail is instance-wide and
// this file keeps it on.
test('the forgot page says when resetting by mail is not set up', async ({ page }) => {
	await page.route('**/api/v1/auth/password', (route) =>
		route.fulfill({ json: { available: false } })
	);
	await page.goto('/forgot-password');
	await expect(page.getByText("Resetting by mail isn't set up here")).toBeVisible();
	await expect(page.getByLabel('Username or email')).toHaveCount(0);
});

test('the reset page offers no provider sign-in', async ({ page, request }) => {
	const oidc = await (await request.get('/api/v1/auth/oidc')).json();
	test.skip(!oidc.enabled, 'Dex is not running (mise run oidc:up)');
	const { name, address } = await confirmedAccount(page, request, 'nodex');
	await page.context().clearCookies({ name: 'rezepte_session' });
	// The provider is on, so the button would show if the page offered it.
	await page.goto('/login');
	await expect(page.getByRole('button', { name: `Continue with ${oidc.name}` })).toBeVisible();
	await page.goto('/forgot-password');
	await page.getByLabel('Username or email').fill(name);
	await page.getByRole('button', { name: 'Send link' }).click();
	const mail = await mailWithSubject(request, address, 'A new password for Rezepte');
	await page.goto(linkIn(mail.text, '/welcome'));
	await expect(page.getByRole('heading', { name: `A new password for ${name}` })).toBeVisible();
	await expect(page.getByRole('button', { name: /^Continue with/ })).toHaveCount(0);
});

test('a used or made-up confirmation link no longer works', async ({ page, request }) => {
	const { address } = await confirmedAccount(page, request, 'used');
	const mail = await latestMailTo(request, address);
	// The page is still on /confirm-email, and a goto that only changes the
	// fragment would not load it again.
	await page.goto('about:blank');
	await page.goto(linkIn(mail.text, '/confirm-email'));
	await expect(page.getByRole('heading', { name: 'This link no longer works' })).toBeVisible();
	await page.goto('about:blank');
	await page.goto('/confirm-email#made-up');
	await expect(page.getByRole('heading', { name: 'This link no longer works' })).toBeVisible();
	// A new link is sent from the profile: signed in it leads there,
	// signed out to the login first.
	await page.getByRole('link', { name: 'Go to your profile' }).click();
	await expect(page).toHaveURL(/\/settings$/);
	await page.context().clearCookies({ name: 'rezepte_session' });
	await page.goto('/confirm-email#made-up');
	await expect(page.getByText('Sign in and send yourself a new one')).toBeVisible();
	await expect(page.getByRole('link', { name: 'Sign in', exact: true })).toHaveAttribute(
		'href',
		'/login'
	);
});

test('sending again right after saving waits, and the first link still works', async ({
	page,
	request
}) => {
	await login(page);
	const name = `again${uniqueToken()}`;
	await createUser(page, { username: name, role: 'user' });
	await page.context().clearCookies({ name: 'rezepte_session' });
	await login(page, name);
	const address = `${name}@example.com`;
	await page.goto('/settings');
	await page.getByLabel('Email', { exact: true }).fill(address);
	await page.getByRole('button', { name: 'Save profile' }).click();
	await expect(
		page.getByText(`Not confirmed yet. Confirmation mail sent to ${address}.`)
	).toBeVisible();
	const mail = await latestMailTo(request, address);
	// The save's mail started the minute.
	await page.getByRole('button', { name: 'Send again' }).click();
	await expect(
		page.getByText('A mail just went out. Wait a minute before sending again.')
	).toBeVisible();
	await expect(
		page.getByText(`Not confirmed yet. Confirmation mail sent to ${address}.`)
	).toBeVisible();
	expect(await mailCountTo(request, address)).toBe(1);

	// Confirmed in another tab from the first mail: "Send again" then
	// catches up instead of claiming a failed send.
	const other = await page.context().newPage();
	await other.goto(linkIn(mail.text, '/confirm-email'));
	await expect(other.getByText('Address confirmed', { exact: true })).toBeVisible();
	await other.close();
	await page.getByRole('button', { name: 'Send again' }).click();
	await expect(page.getByText('Confirmed', { exact: true })).toBeVisible();
	await expect(page.getByText('could not be sent')).toHaveCount(0);
});

// The owner's PATCH writes the address before the name, so only checking
// every field first keeps a refused name from leaving the address behind.
test('a refused name keeps the old address too', async ({ page, request }) => {
	await login(page);
	const member = `refused${uniqueToken()}`;
	await createUser(page, { username: member, role: 'user' });
	const address = `${member}@example.com`;
	await page.goto('/settings/users');
	const row = page.getByRole('listitem').filter({ hasText: member });
	await row.getByRole('button', { name: 'Edit profile' }).click();
	const dialog = page.getByRole('dialog', { name: `Edit ${member}'s profile` });
	await dialog.getByLabel('Display name').fill('Bad\tName');
	await dialog.getByLabel('Email').fill(address);
	await dialog.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page.getByText('Profile could not be saved.')).toBeVisible();
	await page.reload();
	await page
		.getByRole('listitem')
		.filter({ hasText: member })
		.getByRole('button', { name: 'Edit profile' })
		.click();
	await expect(
		page.getByRole('dialog', { name: `Edit ${member}'s profile` }).getByLabel('Email')
	).toHaveValue('');
	expect(await mailCountTo(request, address)).toBe(0);
});

test('an admin cannot save an address the service refuses', async ({ page }) => {
	await login(page);
	const member = `badaddr${uniqueToken()}`;
	await createUser(page, { username: member, role: 'user' });
	await page.goto('/settings/users');
	const row = page.getByRole('listitem').filter({ hasText: member });
	await row.getByRole('button', { name: 'Edit profile' }).click();
	const dialog = page.getByRole('dialog', { name: `Edit ${member}'s profile` });
	// The browser lets the double dot through; the service does not.
	await dialog.getByLabel('Email').fill(`a..b@example.com`);
	await dialog.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(dialog.getByText('That is not an email address')).toBeVisible();
	await page.reload();
	await expect(
		page
			.getByRole('listitem')
			.filter({ hasText: member })
			.getByRole('button', {
				name: /^Address (not )?confirmed/
			})
	).toHaveCount(0);
});

test('a second request within five minutes mails nothing', async ({ page, request }) => {
	const { name, address } = await confirmedAccount(page, request, 'twice');
	await page.context().clearCookies({ name: 'rezepte_session' });
	for (let i = 0; i < 2; i++) {
		await page.goto('/forgot-password');
		await page.getByLabel('Username or email').fill(name);
		await page.getByRole('button', { name: 'Send link' }).click();
		await expect(page.getByRole('heading', { name: 'Check your inbox' })).toBeVisible();
		if (i === 0) await mailWithSubject(request, address, 'A new password for Rezepte');
	}
	await page.waitForTimeout(1000);
	// The confirmation and one reset.
	expect(await mailCountTo(request, address)).toBe(2);
});

// Dex's demo@ account: oidc.test.ts uses mila@ and gast@, oidc-mobile jonas@.
test('an account that signs in only with Dex gets a hint', async ({ page, request, browser }) => {
	const oidc = await (await request.get('/api/v1/auth/oidc')).json();
	test.skip(!oidc.enabled, 'Dex is not running (mise run oidc:up)');
	await login(page);
	const name = `dexhint${uniqueToken()}`;
	const origin = new URL(page.url()).origin;
	let created: { id?: string; setupLink?: { path: string } } | undefined;
	let context: BrowserContext | undefined;
	try {
		created = await (
			await page.request.post('/api/v1/users', {
				headers: { Origin: origin },
				data: { username: name, role: 'user' }
			})
		).json();
		context = await browser.newContext();
		const invited = await context.newPage();
		await invited.goto(created!.setupLink!.path);
		await invited.getByRole('button', { name: 'Continue with Dex' }).click();
		await invited.getByLabel(/email/i).fill('demo@example.com');
		await invited.getByLabel(/password/i).fill('demo1234');
		await invited.getByRole('button', { name: /login/i }).click();
		await expect(invited).toHaveURL('/');
		const users = await (await page.request.get('/api/v1/users')).json();
		const row = users.items.find((u: { username: string }) => u.username === name);
		// Dex vouches for its static accounts' addresses.
		expect(row).toMatchObject({
			email: 'demo@example.com',
			emailVerified: true,
			hasPassword: false
		});

		await page.goto('/forgot-password');
		await page.getByLabel('Username or email').fill(name);
		await page.getByRole('button', { name: 'Send link' }).click();
		const mail = await mailWithSubject(request, 'demo@example.com', 'Signing in to Rezepte');
		expect(mail.text).toContain('with Dex; you have no password');
	} finally {
		await context?.close();
		if (created?.id) {
			await page.request.delete(`/api/v1/users/${created.id}`, { headers: { Origin: origin } });
		}
	}
});

/** The URL after label in a mail's plain-text part. */
function urlAfter(text: string, label: string): string {
	const match = text.match(new RegExp(`${label}: (https?://\\S+)`));
	if (!match) throw new Error(`no "${label}" link in: ${text}`);
	return match[1];
}

test('a comment mails the cook, who unsubscribes and switches it back on', async ({
	page,
	browser,
	request
}) => {
	// The cook confirms an address, then writes a recipe.
	const { name: cook, address } = await confirmedAccount(page, request, 'cook');
	const title = `Notified ${uniqueToken()}`;
	const recipe = await createRecipe(page, { ...loadFixture(1), title });
	const guest = `guest${uniqueToken()}`;
	const switchTo = async (name: string) => {
		await page.context().clearCookies({ name: 'rezepte_session' });
		await login(page, name);
	};
	await switchTo('admin');
	await createUser(page, { username: guest, role: 'user' });

	await switchTo(guest);
	await addDiaryEntry(page, recipe.id, 'Lovely with capers.');
	const mail = await mailWithSubject(request, address, `${guest} commented on “${title}”`);

	// "View comment" opens the recipe on its comments.
	await switchTo(cook);
	await page.goto(urlAfter(mail.text, 'View comment'));
	await expect(page.getByRole('tab', { name: /^Comments/ })).toHaveAttribute(
		'aria-selected',
		'true'
	);

	// The unsubscribe link works without a session; opening it changes nothing.
	const strangers = await browser.newContext();
	try {
		const stranger = await strangers.newPage();
		await stranger.goto(urlAfter(mail.text, 'No more mails about new comments'));
		await expect(stranger.getByRole('heading', { name: 'Unsubscribe from mails' })).toBeVisible();
		await expect(stranger.getByText(cook, { exact: true })).toBeVisible();
		await stranger.getByRole('button', { name: 'No more mails about new comments' }).click();
		await expect(stranger.getByRole('heading', { name: 'Unsubscribed' })).toBeVisible();
		await expect(stranger.getByRole('link', { name: 'Manage all notifications' })).toHaveAttribute(
			'href',
			'/settings/notifications'
		);

		// A second comment mails nothing.
		const before = await mailCountTo(request, address);
		await switchTo(guest);
		await addDiaryEntry(page, recipe.id, 'And with lemon.');
		await page.waitForTimeout(1000);
		expect(await mailCountTo(request, address)).toBe(before);

		// The settings page shows it off; Undo on the link's page turns it on.
		await switchTo(cook);
		await page.goto('/settings/notifications');
		const byMail = page.getByRole('region', { name: 'By mail' });
		// The kind's "On" side: checked while the mail arrives.
		const comments = byMail
			.getByRole('radiogroup', { name: 'New comments' })
			.getByRole('radio', { name: 'On' });
		await expect(comments).toHaveAttribute('aria-checked', 'false');
		await expect(
			byMail.getByText('Confirmed. This is where Rezepte sends your mail.')
		).toBeVisible();
		await expect(page.getByText(address)).toBeVisible();
		await stranger.getByRole('button', { name: 'Undo' }).click();
		await expect(stranger.getByRole('heading', { name: 'Unsubscribe from mails' })).toBeVisible();
		await page.reload();
		await expect(comments).toHaveAttribute('aria-checked', 'true');

		// The toggle saves on flip.
		await byMail
			.getByRole('radiogroup', { name: 'New comments' })
			.getByRole('radio', { name: 'Off' })
			.click();
		await expect(comments).toHaveAttribute('aria-checked', 'false');
		await page.reload();
		await expect(comments).toHaveAttribute('aria-checked', 'false');
		// Opened again while off, the link offers only the group, whose new
		// recipes mail is still on.
		await stranger.reload();
		await expect(
			stranger.getByRole('button', { name: 'Unsubscribe from all household mails' })
		).toBeVisible();
		await expect(stranger.getByRole('button', { name: /No more mails/ })).toHaveCount(0);
		// With the group's switch off, it offers nothing.
		const household = byMail.getByRole('switch', { name: 'Household' });
		await household.click();
		await expect(household).toHaveAttribute('aria-checked', 'false');
		await stranger.reload();
		await expect(stranger.getByRole('heading', { name: 'Already unsubscribed' })).toBeVisible();
		await expect(stranger.getByRole('button', { name: /^Unsubscribe from all/ })).toHaveCount(0);
		// The group's switch is its own: on again, each kind keeps its choice.
		await household.click();
		await page.reload();
		await expect(household).toHaveAttribute('aria-checked', 'true');
		await expect(comments).toHaveAttribute('aria-checked', 'false');
		await expect(
			byMail.getByRole('radiogroup', { name: 'New recipes' }).getByRole('radio', { name: 'On' })
		).toHaveAttribute('aria-checked', 'true');
	} finally {
		await strangers.close();
	}
});

test('an unconfirmed address is named and the mail rows still switch', async ({ page }) => {
	await login(page);
	const name = `unconf${uniqueToken()}`;
	await createUser(page, { username: name, role: 'user' });
	await page.context().clearCookies({ name: 'rezepte_session' });
	await login(page, name);
	const address = `${name}@example.com`;
	await page.goto('/settings');
	await page.getByLabel('Email', { exact: true }).fill(address);
	await page.getByRole('button', { name: 'Save profile' }).click();
	await expect(
		page.getByText(`Not confirmed yet. Confirmation mail sent to ${address}.`)
	).toBeVisible();
	await page.goto('/settings/notifications');
	const byMail = page.getByRole('region', { name: 'By mail' });
	await expect(byMail.getByText(address)).toBeVisible();
	await expect(
		byMail.getByText('Not confirmed yet. Rezepte only writes to a confirmed address.')
	).toBeVisible();
	await expect(byMail.getByRole('link', { name: 'Confirm in profile' })).toHaveAttribute(
		'href',
		'/settings'
	);
	// Greyed, since nothing arrives yet, but the choice already counts.
	const off = byMail
		.getByRole('radiogroup', { name: 'New comments' })
		.getByRole('radio', { name: 'Off' });
	await off.click();
	await expect(off).toHaveAttribute('aria-checked', 'true');
});

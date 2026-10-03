import { expect, test, type Page } from '@playwright/test';
import { devPassword, login, uniqueToken } from './helpers';
import { latestMailTo, mailCountTo, mailpitSmtpPort, mailpitUrl, setupLinkIn } from './mailpit';

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

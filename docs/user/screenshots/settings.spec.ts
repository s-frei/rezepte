import path from 'node:path';
import { expect, test, type Page } from '@playwright/test';
import { isMobile, prepare, shot } from './helpers';

test('settings', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings');
	await expect(page.getByRole('heading', { name: 'Change password' })).toBeVisible();
	await shot(page, 'settings');
});

test('avatar-crop', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings');
	// Jonas's demo picture: embedded for the demo, but not set on anyone, so
	// the dialog shows a real photo and nothing is saved.
	await page
		.locator('input[type=file]')
		.setInputFiles(path.join(__dirname, '../../../service/internal/demo/avatars/jonas.jpg'));
	const dialog = page.getByRole('dialog', { name: 'Crop the photo' });
	await expect(dialog.getByRole('button', { name: 'Save' })).toBeEnabled();
	await shot(page, 'avatar-crop');
});

test('import-export', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings/transfer');
	await expect(page.getByRole('heading', { name: 'Export' })).toBeVisible();
	await page.getByRole('checkbox').nth(0).check();
	await page.getByRole('checkbox').nth(2).check();
	await shot(page, 'import-export');
});

test('import-export-import', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	// The demo's own first recipes, exported through the API: the slip then
	// shows what a real file looks like, "already here" marks included.
	const list = await (await page.request.get('/api/v1/recipes?limit=4&sort=title')).json();
	const zip = await page.request.post('/api/v1/export', {
		headers: {
			Origin: new URL(page.url()).origin,
			'Content-Type': 'application/json'
		},
		data: { recipeIds: list.items.map((r: { id: string }) => r.id) }
	});
	await page.goto('/settings/transfer');
	await page.getByLabel('Choose a file').setInputFiles({
		name: 'rezepte-2026-09-30.zip',
		mimeType: 'application/zip',
		buffer: await zip.body()
	});
	await expect(page.getByText('already here').first()).toBeVisible();
	await page.locator('#import').evaluate((el) => el.scrollIntoView({ block: 'start' }));
	await shot(page, 'import-export-import');
});

// The screenshot instance always has OIDC configured (docs/user/mise.toml),
// so the card is there, unconnected, with the neutral provider name.
test('settings-signin', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings');
	const button = page.getByRole('button', {
		name: 'Continue with single sign-on'
	});
	await expect(button).toBeVisible();
	// Centered, so neither the phone's sticky header nor its bottom navigation
	// covers the card.
	await page
		.getByRole('heading', { name: 'Sign-in' })
		.evaluate((el) => el.closest('section')?.scrollIntoView({ block: 'center' }));
	await shot(page, 'settings-signin');
});

test('api', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings/api');
	// The figures arrive from two fetches after the page renders; waiting for
	// one of them keeps the picture from catching the skeleton.
	await expect(page.getByRole('term').filter({ hasText: 'Endpoints' })).toBeVisible();
	await shot(page, 'api');
});

test('token-mcp', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings/api');
	await page.getByRole('button', { name: 'Create token' }).first().click();
	const dialog = page.getByRole('dialog');
	await dialog.getByLabel('Name').fill('MCP on the laptop');
	await dialog
		.getByRole('radiogroup', { name: 'Recipes' })
		.getByRole('radio', { name: 'Delete' })
		.click();
	await dialog.getByRole('button', { name: 'Create', exact: true }).click();
	const reveal = page.getByRole('dialog');
	await expect(reveal.getByRole('tab', { name: 'Claude Code' })).toBeVisible();
	await shot(page, 'token-mcp');
});

test('users', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	// The demo seeds its admin, two members (mila, jonas) and a third,
	// noah, with an open setup link and no password, so the list needs
	// nothing created here.
	await page.goto('/settings/users');
	await expect(page.getByRole('listitem').filter({ hasText: 'mila' })).toBeVisible();
	await shot(page, 'users');
});

test('user-create', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings/users');
	await page.getByRole('button', { name: 'Add account' }).click();
	const dialog = page.getByRole('dialog');
	await expect(dialog).toBeVisible();
	// Filled, so the picture shows what the address field is for.
	await dialog.getByLabel(/^Email/).fill('lena@example.org');
	await shot(page, 'user-create');
});

// "Add account" defaults to "Send a setup link", so finishing it here is
// what opens the dialog the shot is of. The screenshot instance has no mail
// server, so the create response gets the `mailedTo` a real send would add,
// and the shot shows the sent state.
test('setup-link-dialog', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	const address = 'lena@example.org';
	await page.route('**/api/v1/users', async (route) => {
		if (route.request().method() !== 'POST') return route.fallback();
		const response = await route.fetch();
		const body = await response.json();
		body.setupLink = { ...body.setupLink, mailedTo: address };
		await route.fulfill({ response, json: body });
	});
	await page.goto('/settings/users');
	await page.getByRole('button', { name: 'Add account' }).click();
	const username = `invited-${testInfo.project.name}`;
	await page.getByLabel('Username').fill(username);
	await page
		.getByRole('dialog')
		.getByLabel(/^Email/)
		.fill(address);
	await page.getByRole('button', { name: 'Add', exact: true }).click();
	// Named, not just `getByRole('dialog')`: the create dialog it replaces can
	// still be mid-close, and the two would otherwise both match.
	await expect(page.getByRole('dialog', { name: `Setup link for ${username}` })).toBeVisible();
	await expect(page.getByText(`Sent to ${address}`)).toBeVisible();
	await shot(page, 'setup-link-dialog');
	// Gone again, so the people screenshots of the shared instance never list it.
	const users = (await (await page.request.get('/api/v1/users')).json()) as {
		items: { id: string; username: string }[];
	};
	const id = users.items.find((u) => u.username === username)?.id;
	await page.request.delete(`/api/v1/users/${id}`, {
		headers: { Origin: new URL(page.url()).origin }
	});
});

// Mail is configured through the API, on the placeholder server a real
// instance would have: the screenshot instance has none of its own
// (RZP_DEMO_MAIL=off), and the card is the same whether or not it works.
async function configureMail(page: Page) {
	const response = await page.request.put('/api/v1/settings/mail', {
		headers: { Origin: new URL(page.url()).origin },
		data: {
			host: 'smtp.example.org',
			port: 587,
			security: 'starttls',
			username: 'rezepte@example.org',
			password: 'x',
			from: 'rezepte@example.org',
			fromName: 'Rezepte'
		}
	});
	expect(response.ok()).toBe(true);
}

async function turnMailOff(page: Page) {
	await page.request.delete('/api/v1/settings/mail', {
		headers: { Origin: new URL(page.url()).origin }
	});
}

test('mail-card', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings/mail');
	try {
		await configureMail(page);
		await page.reload();
		const card = page.getByRole('region', { name: 'Email' });
		await expect(card.getByText('Sending on')).toBeVisible();
		await shot(page, 'mail-card');
	} finally {
		await turnMailOff(page);
	}
});

test('mail-dialog', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings/mail');
	try {
		await configureMail(page);
		await page.reload();
		await page.getByRole('region', { name: 'Email' }).getByRole('button', { name: 'Edit' }).click();
		await expect(page.getByRole('dialog', { name: 'Set up mail' })).toBeVisible();
		await shot(page, 'mail-dialog');
	} finally {
		await turnMailOff(page);
	}
});

test('settings-recipe-editing', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings/users');
	// The card reads the setting after the page renders; the switch appears
	// once it has, so waiting for it keeps the skeleton off the picture.
	const toggle = page.getByRole('switch', {
		name: 'Only authors and admins edit recipes'
	});
	await expect(toggle).toBeVisible();
	// Centered, so neither the phone's sticky header nor its bottom navigation
	// covers the card.
	await toggle.evaluate((el) => el.closest('section')?.scrollIntoView({ block: 'center' }));
	await shot(page, 'settings-recipe-editing');
});

// Mila has an address, so "Send by mail" opens prefilled. The row action is
// in the "..." menu on a wide screen and in the person's sheet on a phone.
test('setup-link-send', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings/users');
	try {
		await configureMail(page);
		await page.reload();
		if (isMobile(testInfo)) {
			await page.getByRole('button', { name: /^Manage .*\(mila\)$/ }).click();
			await page.getByRole('button', { name: 'Setup link', exact: true }).click();
		} else {
			await page.getByRole('button', { name: 'More actions for mila' }).click();
			await page.getByRole('menuitem', { name: 'Setup link for mila' }).click();
		}
		const dialog = page.getByRole('dialog', { name: /^Setup link for/ });
		await expect(dialog).toBeVisible();
		await expect(dialog.getByLabel(/^Email/)).not.toHaveValue('');
		await shot(page, 'setup-link-send');
	} finally {
		await turnMailOff(page);
	}
});

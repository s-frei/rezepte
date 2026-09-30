import path from 'node:path';
import { expect, test } from '@playwright/test';
import { prepare, shot } from './helpers';

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
		headers: { Origin: new URL(page.url()).origin, 'Content-Type': 'application/json' },
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
	const button = page.getByRole('button', { name: 'Continue with single sign-on' });
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
	// The demo seeds its admin and two members, mila and jonas, so the list
	// needs nothing created here.
	await page.goto('/settings/users');
	await expect(page.getByRole('listitem').filter({ hasText: 'mila' })).toBeVisible();
	await shot(page, 'users');
});

test('user-create', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings/users');
	await page.getByRole('button', { name: 'Add account' }).click();
	await expect(page.getByRole('dialog')).toBeVisible();
	await shot(page, 'user-create');
});

// "Add account" defaults to "Send a setup link", so finishing it here is
// what opens the dialog the shot is of.
test('setup-link-dialog', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings/users');
	await page.getByRole('button', { name: 'Add account' }).click();
	const username = `invited-${testInfo.project.name}`;
	await page.getByLabel('Username').fill(username);
	await page.getByRole('button', { name: 'Add', exact: true }).click();
	// Named, not just `getByRole('dialog')`: the create dialog it replaces can
	// still be mid-close, and the two would otherwise both match.
	await expect(page.getByRole('dialog', { name: `Setup link for ${username}` })).toBeVisible();
	await shot(page, 'setup-link-dialog');
});

test('settings-recipe-editing', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.goto('/settings/users');
	// The card reads the setting after the page renders; the switch appears
	// once it has, so waiting for it keeps the skeleton off the picture.
	const toggle = page.getByRole('switch', { name: 'Only authors and admins edit recipes' });
	await expect(toggle).toBeVisible();
	// To the bottom, so the phone's bottom navigation does not cover the card.
	await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
	await shot(page, 'settings-recipe-editing');
});

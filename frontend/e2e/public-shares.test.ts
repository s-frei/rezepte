import { expect, test } from '@playwright/test';
import {
	createPublicShare,
	createRecipe,
	createUser,
	loadFixture,
	login,
	openRecipeMenu,
	type PublicShare,
	setCanSharePublicly,
	setPublicShareAttribution,
	setPublicShares,
	tinyPng,
	uniqueToken,
	uploadImage
} from './helpers';

// Public sharing is an instance-wide switch, like link previews: every test
// below that flips it runs in the desktop project only and turns it back
// off, whatever happens - see link-preview.test.ts for the pattern. They
// share one instance, so they run one after the other rather than in
// parallel.
test.describe.configure({ mode: 'serial' });

test('a stranger opens a public link', async ({ page, browser }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'public sharing is instance-wide');
	await login(page);
	await expect(page).toHaveURL('/');
	const title = `Public ${uniqueToken()}`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title });
	await uploadImage(page, recipe.id, tinyPng());

	try {
		await setPublicShares(page, true);
		const share = await createPublicShare(page, recipe.id);
		const token = share.path.replace('/s/', '');

		// A stranger: a fresh context with no session cookie at all.
		const strangerContext = await browser.newContext();
		try {
			const stranger = await strangerContext.newPage();
			await stranger.goto(`/s/${token}`);

			await expect(stranger.getByRole('heading', { level: 1, name: title })).toBeVisible();
			await expect(stranger.getByRole('heading', { name: 'Ingredients' })).toBeVisible();
			await expect(stranger.getByRole('heading', { name: 'Method' })).toBeVisible();

			const photo = stranger.getByRole('img', { name: title }).first();
			await expect(photo).toBeVisible();
			await expect(photo).toHaveJSProperty('complete', true);
			await expect
				.poll(() => photo.evaluate((img: HTMLImageElement) => img.naturalWidth))
				.toBeGreaterThan(0);

			await expect(stranger.getByText(recipe.createdBy.displayName)).toHaveCount(0);
			await expect(stranger.getByRole('link', { name: 'Edit' })).toHaveCount(0);
			await expect(stranger.getByRole('button', { name: 'Edit' })).toHaveCount(0);
			await expect(stranger.getByRole('button', { name: 'More actions' })).toHaveCount(0);
			const footer = stranger.getByRole('contentinfo');
			await expect(footer).toContainText('A cookbook like this one?');
			await expect(footer.getByRole('link', { name: /Get Rezepte/ })).toHaveAttribute(
				'href',
				'https://s-frei.github.io/rezepte/'
			);
			await expect(footer.getByRole('link', { name: /Source/ })).toHaveAttribute(
				'href',
				'https://github.com/s-frei/rezepte'
			);

			// A fresh context reports a light device; the button flips what is
			// on screen, and back.
			await stranger.getByRole('button', { name: 'Switch to the dark theme' }).click();
			await expect(stranger.locator('html')).toHaveAttribute('data-theme', 'dark');
			await stranger.getByRole('button', { name: 'Switch to the light theme' }).click();
			await expect(stranger.locator('html')).toHaveAttribute('data-theme', 'light');
		} finally {
			await strangerContext.close();
		}
	} finally {
		await setPublicShares(page, false);
	}
});

test('a signed-in member sees the same public page', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'public sharing is instance-wide');
	await login(page);
	await expect(page).toHaveURL('/');
	const title = `Member view ${uniqueToken()}`;
	const recipe = await createRecipe(page, { ...loadFixture(1), title });

	try {
		await setPublicShares(page, true);
		const share = await createPublicShare(page, recipe.id);
		const token = share.path.replace('/s/', '');

		await page.goto(`/s/${token}`);
		await expect(page).toHaveURL(`/s/${token}`);
		await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
		// No app shell: the bottom nav (a <nav> landmark) and the top bar's
		// user menu button are both gone.
		await expect(page.getByRole('navigation')).toHaveCount(0);
		await expect(page.getByRole('button', { name: 'Account menu' })).toHaveCount(0);
		await expect(
			page.getByRole('contentinfo').getByRole('link', { name: /Get Rezepte/ })
		).toBeVisible();
	} finally {
		await setPublicShares(page, false);
	}
});

test('a recipe without photos, description or times', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'public sharing is instance-wide');
	await login(page);
	await expect(page).toHaveURL('/');
	const title = `Bare ${uniqueToken()}`;
	const fixture = loadFixture(2);
	const recipe = await createRecipe(page, {
		...fixture,
		title,
		description: '',
		prepMinutes: null,
		cookMinutes: null,
		sourceUrl: null
	});

	try {
		await setPublicShares(page, true);
		const share = await createPublicShare(page, recipe.id);
		const token = share.path.replace('/s/', '');

		await page.goto(`/s/${token}`);
		await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
		await expect(page.getByRole('img', { name: title })).toHaveCount(0);
		await expect(page.getByText('Prep time')).toHaveCount(0);
		await expect(page.getByText('Cook time')).toHaveCount(0);
		await expect(page.getByRole('main').getByText('Source')).toHaveCount(0);
	} finally {
		await setPublicShares(page, false);
	}
});

test('a stranger sees where the recipe comes from', async ({ page, browser }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'public sharing is instance-wide');
	await login(page);
	await expect(page).toHaveURL('/');
	const recipe = await createRecipe(page, {
		...loadFixture(0),
		title: `Credited ${uniqueToken()}`,
		sourceName: 'Aunt Erika',
		sourceUrl: 'https://www.example.test/pie'
	});

	try {
		await setPublicShares(page, true);
		const share = await createPublicShare(page, recipe.id);
		const strangerContext = await browser.newContext();
		try {
			const stranger = await strangerContext.newPage();
			await stranger.goto(share.path);
			// The credit travels with a shared recipe: whoever it was adapted
			// from is part of the recipe, not of the household that keeps it.
			const credit = stranger.getByRole('main').getByText(/^Adapted from /);
			await expect(credit.getByRole('link', { name: 'Aunt Erika' })).toHaveAttribute(
				'href',
				'https://www.example.test/pie'
			);
		} finally {
			await strangerContext.close();
		}
	} finally {
		await setPublicShares(page, false);
	}
});

test('an unknown token', async ({ page }) => {
	await page.goto(`/s/${uniqueToken()}`);
	await expect(page.getByText('This link is no longer available.')).toBeVisible();
	// Nothing was shared, so there is nothing to credit.
	await expect(page.getByRole('contentinfo')).toHaveCount(0);
});

test('the owner stops public pages from naming Rezepte', async ({ page, browser }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'public sharing is instance-wide');
	await login(page);
	await expect(page).toHaveURL('/');
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Quiet ${uniqueToken()}` });

	try {
		await setPublicShares(page, true);
		const share = await createPublicShare(page, recipe.id);

		await page.goto('/settings/users');
		const mention = page.getByRole('switch', { name: 'Name Rezepte when sharing' });
		await expect(mention).toBeChecked();
		await mention.click();
		await expect(mention).not.toBeChecked();

		const strangerContext = await browser.newContext();
		try {
			const stranger = await strangerContext.newPage();
			await stranger.goto(share.path);
			await expect(stranger.getByRole('heading', { level: 1, name: recipe.title })).toBeVisible();
			await expect(stranger.getByRole('contentinfo')).toHaveCount(0);
		} finally {
			await strangerContext.close();
		}
	} finally {
		await setPublicShareAttribution(page, true);
		await setPublicShares(page, false);
	}
});

// The same switch names Rezepte at the foot of a recipe card, and the owner
// finds it whether public links are on or not (see share-image.test.ts).
test('the credit on a recipe card follows the owner setting', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'the setting is instance-wide');
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Credit ${uniqueToken()}` });
	const openCard = async () => {
		await page.getByRole('button', { name: 'Pass on' }).click();
		const sheet = page.getByRole('dialog', { name: 'Pass on recipe' });
		await expect(sheet.getByRole('img', { name: /^Recipe image:/ })).toBeVisible({
			timeout: 20_000
		});
	};

	try {
		await page.goto('/settings/users');
		await expect(page.getByRole('switch', { name: 'Name Rezepte when sharing' })).toBeVisible();

		await setPublicShareAttribution(page, false);
		await page.goto(`/recipes/${recipe.slug}`);
		await openCard();
		await expect(page.locator('[data-testid="share-card"] footer')).toHaveCount(0);

		await setPublicShareAttribution(page, true);
		await page.reload();
		await openCard();
		await expect(page.locator('[data-testid="share-card"] footer')).toHaveCount(1);
	} finally {
		await setPublicShareAttribution(page, true);
	}
});

// getPublicRecipe bypasses api() and must never let a network failure or a
// malformed body propagate into SvelteKit's generic error page: that would
// drop this route's own header along with the friendly message.
test('a public link that fails to load', async ({ page }) => {
	await page.route('**/api/v1/public/shares/**', (route) => route.abort());
	await page.goto(`/s/${uniqueToken()}`);
	await expect(page.getByRole('img', { name: 'Rezepte' })).toBeVisible();
	await expect(page.getByText('This link is no longer available.')).toBeVisible();
});

test('a public link that answers with a malformed body', async ({ page }) => {
	await page.route('**/api/v1/public/shares/**', (route) =>
		route.fulfill({ status: 200, body: 'not json' })
	);
	await page.goto(`/s/${uniqueToken()}`);
	await expect(page.getByRole('img', { name: 'Rezepte' })).toBeVisible();
	await expect(page.getByText('This link is no longer available.')).toBeVisible();
});

test('a member creates, copies and revokes a public link', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'public sharing is instance-wide');
	await login(page);
	await expect(page).toHaveURL('/');
	const title = `Shareable ${uniqueToken()}`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title });

	try {
		await setPublicShares(page, true, 7, null);
		await page.goto(`/recipes/${recipe.slug}`);

		await openRecipeMenu(page);
		await page.getByRole('menuitem', { name: 'Share publicly…' }).click();

		const dialog = page.getByRole('dialog', { name: 'Your public link' });
		await expect(
			dialog.getByText(
				'Anyone with this link sees the recipe without signing in, until you revoke it or it expires.'
			)
		).toBeVisible();
		// The household's default lifetime (7 days) comes preselected.
		await expect(dialog.getByRole('button', { name: 'Lifetime' })).toContainText('7 days');

		await dialog.getByRole('button', { name: 'Create public link' }).click();

		const linkLine = dialog.getByText(/\/s\//);
		await expect(linkLine).toBeVisible();
		const linkText = (await linkLine.textContent()) ?? '';
		const token = linkText.split('/s/')[1];
		expect(token?.length).toBeGreaterThan(0);

		await page.keyboard.press('Escape');
		await expect(dialog).toBeHidden();

		// The marker in the tag row and the colophon line both open the same
		// dialog; the colophon says how long the link runs.
		const marker = page.getByRole('button', { name: 'Shared publicly' });
		await expect(marker).toBeVisible();
		const colophon = page.locator('footer').filter({ hasText: 'Shared publicly by you until' });
		await expect(colophon).toBeVisible();
		await colophon.getByRole('button', { name: 'Manage link' }).click();
		await expect(dialog).toBeVisible();
		await page.keyboard.press('Escape');
		await expect(dialog).toBeHidden();

		await openRecipeMenu(page);
		await expect(page.getByRole('menuitem', { name: 'Public link…' })).toBeVisible();
		await expect(page.getByRole('menuitem', { name: 'Share publicly…' })).toHaveCount(0);
		await page.keyboard.press('Escape');

		await marker.click();
		await expect(dialog).toBeVisible();
		await dialog.getByRole('button', { name: 'Revoke' }).click();
		const confirmDialog = page.getByRole('dialog', { name: 'Revoke public link?' });
		await confirmDialog.getByRole('button', { name: 'Revoke' }).click();

		await expect(page.getByRole('button', { name: 'Shared publicly' })).toHaveCount(0);
		await expect(page.getByText('Shared publicly by you until')).toHaveCount(0);

		await page.goto(`/s/${token}`);
		await expect(page.getByText('This link is no longer available.')).toBeVisible();
	} finally {
		await setPublicShares(page, false);
	}
});

test('a refused link says whether sharing is off or the right withdrawn', async ({
	page,
	browser
}, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'public sharing is instance-wide');
	await login(page);
	await expect(page).toHaveURL('/');
	const token = uniqueToken();
	const username = `refused${token}`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Refused ${token}` });
	const member = await createUser(page, { username, role: 'user' });

	// The dialog is opened while sharing is allowed and the refusal happens
	// behind its back, the way a second tab or another admin would cause it.
	async function refuse(on: typeof page, change: () => Promise<unknown>, toast: string) {
		await on.goto(`/recipes/${recipe.slug}`);
		await openRecipeMenu(on);
		await on.getByRole('menuitem', { name: 'Share publicly…' }).click();
		const dialog = on.getByRole('dialog', { name: 'Your public link' });
		await expect(dialog).toBeVisible();
		await change();
		await dialog.getByRole('button', { name: 'Create public link' }).click();
		await expect(on.getByText(toast)).toBeVisible();
	}

	try {
		await setPublicShares(page, true);
		await refuse(
			page,
			() => setPublicShares(page, false),
			'Public sharing is off for this household.'
		);

		await setPublicShares(page, true);
		const memberContext = await browser.newContext();
		try {
			const memberPage = await memberContext.newPage();
			await login(memberPage, username);
			await expect(memberPage).toHaveURL('/');
			await refuse(
				memberPage,
				() => setCanSharePublicly(page, member.id, false),
				"You don't have permission to create public links."
			);
		} finally {
			await memberContext.close();
		}
	} finally {
		await setCanSharePublicly(page, member.id, true);
		await setPublicShares(page, false);
	}
});

test('a member without the right sees no menu item', async ({ page, browser }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'public sharing is instance-wide');
	await login(page);
	await expect(page).toHaveURL('/');
	const token = uniqueToken();
	const username = `withdrawn${token}`;
	const member = await createUser(page, { username, role: 'user' });
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Withdrawn ${token}` });

	try {
		await setPublicShares(page, true);
		await setCanSharePublicly(page, member.id, false);

		const memberContext = await browser.newContext();
		try {
			const memberPage = await memberContext.newPage();
			await login(memberPage, username);
			await expect(memberPage).toHaveURL('/');
			await memberPage.goto(`/recipes/${recipe.slug}`);
			await openRecipeMenu(memberPage);
			await expect(memberPage.getByRole('menuitem', { name: 'Share publicly…' })).toHaveCount(0);
			await expect(memberPage.getByRole('menuitem', { name: 'Public link…' })).toHaveCount(0);
		} finally {
			await memberContext.close();
		}
	} finally {
		await setCanSharePublicly(page, member.id, true);
		await setPublicShares(page, false);
	}
});

test('with public sharing off there is no menu item', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'public sharing is instance-wide');
	await login(page);
	await expect(page).toHaveURL('/');
	const title = `No sharing ${uniqueToken()}`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title });

	await setPublicShares(page, false);
	await page.goto(`/recipes/${recipe.slug}`);
	await openRecipeMenu(page);
	await expect(page.getByRole('menuitem', { name: 'Share publicly…' })).toHaveCount(0);
	await expect(page.getByRole('menuitem', { name: 'Public link…' })).toHaveCount(0);
});

test('the owner switches public sharing and its lifetimes', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'public sharing is instance-wide');
	await login(page);
	await expect(page).toHaveURL('/');
	await setPublicShares(page, false);

	try {
		await page.goto('/settings/users');
		await expect(page.getByRole('heading', { name: 'Public sharing' })).toBeVisible();

		// Turning it on asks first; cancelling leaves it off.
		const sharingSwitch = page.getByRole('switch', { name: 'Members may share recipes publicly' });
		const confirm = page.getByRole('dialog', { name: 'Turn on public sharing?' });
		await sharingSwitch.click();
		await expect(confirm).toBeVisible();
		await confirm.getByRole('button', { name: 'Cancel' }).click();
		await expect(confirm).toBeHidden();
		await expect(sharingSwitch).not.toBeChecked();
		await expect(page.getByRole('button', { name: 'Maximum lifetime' })).toHaveCount(0);

		await sharingSwitch.click();
		await confirm.getByRole('button', { name: 'Turn on' }).click();
		await expect(sharingSwitch).toBeChecked();

		const maxSelect = page.getByRole('button', { name: 'Maximum lifetime' });
		await maxSelect.click();
		await page.getByRole('option', { name: '30 days' }).click();
		await expect(maxSelect).toContainText('30 days');

		// The default is capped along with the maximum: neither "1 year" nor
		// "Permanent" is offered any more.
		const defaultSelect = page.getByRole('button', { name: 'Default lifetime' });
		await defaultSelect.click();
		await expect(page.getByRole('option', { name: '30 days' })).toBeVisible();
		await expect(page.getByRole('option', { name: '1 year' })).toHaveCount(0);
		await expect(page.getByRole('option', { name: 'Permanent' })).toHaveCount(0);
		await page.keyboard.press('Escape');

		await page.reload();
		await expect(page.getByRole('button', { name: 'Maximum lifetime' })).toContainText('30 days');
		await page.getByRole('button', { name: 'Default lifetime' }).click();
		await expect(page.getByRole('option', { name: '1 year' })).toHaveCount(0);
		await page.keyboard.press('Escape');
	} finally {
		await setPublicShares(page, false);
	}
});

test('shared links lists and revokes', async ({ page, browser }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'public sharing is instance-wide');
	await login(page);
	await expect(page).toHaveURL('/');
	const token = uniqueToken();
	const username = `sharelist${token}`;
	const title = `Shared list ${token}`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title });
	await createUser(page, { username, role: 'user' });

	try {
		await setPublicShares(page, true);

		// A fresh member, so their own list starts empty - the admin account
		// this suite otherwise logs in as accumulates shares from earlier
		// tests in this file that never revoke them.
		const memberContext = await browser.newContext();
		try {
			const memberPage = await memberContext.newPage();
			await login(memberPage, username);
			await expect(memberPage).toHaveURL('/');
			await createPublicShare(memberPage, recipe.id);

			await memberPage.goto('/settings/shares');
			const list = memberPage.getByRole('list', { name: 'Shared links' });
			const row = list.getByRole('listitem').filter({ hasText: title });
			await expect(row).toBeVisible();
			await expect(row.getByText('Active')).toBeVisible();
			await expect(row.getByRole('button', { name: 'Copy' })).toBeVisible();

			await row.getByRole('button', { name: 'Revoke' }).click();
			await memberPage
				.getByRole('dialog', { name: 'Revoke public link?' })
				.getByRole('button', { name: 'Revoke' })
				.click();

			await expect(list.getByRole('listitem')).toHaveCount(0);
			await expect(memberPage.getByText('No public links yet.')).toBeVisible();
		} finally {
			await memberContext.close();
		}
	} finally {
		await setPublicShares(page, false);
	}
});

test("admins see everyone's links and revoke all", async ({ page, browser }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'public sharing is instance-wide');
	await login(page);
	await expect(page).toHaveURL('/');
	const token = uniqueToken();
	const username = `sharer${token}`;
	const title = `Everyone ${token}`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title });
	await createUser(page, { username, role: 'user' });

	try {
		await setPublicShares(page, true);

		const memberContext = await browser.newContext();
		let share: PublicShare;
		try {
			const memberPage = await memberContext.newPage();
			await login(memberPage, username);
			await expect(memberPage).toHaveURL('/');
			share = await createPublicShare(memberPage, recipe.id);
		} finally {
			await memberContext.close();
		}
		const shareToken = share.path.replace('/s/', '');

		await page.goto('/settings/shares');
		await page.getByRole('switch', { name: 'Everyone in the household' }).click();

		const list = page.getByRole('list', { name: 'Shared links' });
		const row = list.getByRole('listitem').filter({ hasText: title });
		await expect(row).toBeVisible();
		await expect(row.getByText(`By ${username}`)).toBeVisible();

		await page.getByRole('button', { name: 'Revoke all' }).click();
		await page
			.getByRole('dialog', { name: 'Revoke every public link?' })
			.getByRole('button', { name: 'Revoke all' })
			.click();

		await expect(row).toHaveCount(0);
		await expect(page.getByText('No public links yet.')).toBeVisible();

		await page.goto(`/s/${shareToken}`);
		await expect(page.getByText('This link is no longer available.')).toBeVisible();
	} finally {
		await setPublicShares(page, false);
	}
});

test("an admin narrows everyone's links to one person", async ({ page, browser }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'public sharing is instance-wide');
	await login(page);
	await expect(page).toHaveURL('/');
	const token = uniqueToken();
	const username = `narrow${token}`;
	const theirs = `Theirs ${token}`;
	const mine = `Mine ${token}`;
	const theirRecipe = await createRecipe(page, { ...loadFixture(0), title: theirs });
	const myRecipe = await createRecipe(page, { ...loadFixture(1), title: mine });
	const member = await createUser(page, { username, role: 'user' });

	try {
		await setPublicShares(page, true);
		await createPublicShare(page, myRecipe.id);

		const memberContext = await browser.newContext();
		try {
			const memberPage = await memberContext.newPage();
			await login(memberPage, username);
			await expect(memberPage).toHaveURL('/');
			await createPublicShare(memberPage, theirRecipe.id);
		} finally {
			await memberContext.close();
		}

		await page.goto('/settings/shares');
		// The person filter belongs to the everyone view alone.
		await expect(page.getByRole('heading', { name: 'Created by' })).toHaveCount(0);
		await page.getByRole('switch', { name: 'Everyone in the household' }).click();
		await expect(page).toHaveURL('/settings/shares?all=1');

		const list = page.getByRole('list', { name: 'Shared links' });
		await expect(list.getByRole('listitem').filter({ hasText: mine })).toBeVisible();
		await expect(list.getByRole('listitem').filter({ hasText: theirs })).toBeVisible();

		await page.getByRole('button', { name: new RegExp(`^${username}`) }).click();
		await expect(page).toHaveURL(`/settings/shares?all=1&user=${member.id}`);
		await expect(list.getByRole('listitem').filter({ hasText: theirs })).toBeVisible();
		await expect(list.getByRole('listitem').filter({ hasText: mine })).toHaveCount(0);

		// Both filters live in the URL, so a reload keeps them.
		await page.reload();
		await expect(page.getByRole('switch', { name: 'Everyone in the household' })).toBeChecked();
		await expect(list.getByRole('listitem').filter({ hasText: theirs })).toBeVisible();
		await expect(list.getByRole('listitem').filter({ hasText: mine })).toHaveCount(0);

		// A second click on the chosen person shows everyone again.
		await page.getByRole('button', { name: new RegExp(`^${username}`) }).click();
		await expect(page).toHaveURL('/settings/shares?all=1');
		await expect(list.getByRole('listitem').filter({ hasText: mine })).toBeVisible();

		await page.getByRole('switch', { name: 'Everyone in the household' }).click();
		await expect(page).toHaveURL('/settings/shares');
		await expect(list.getByRole('listitem').filter({ hasText: theirs })).toHaveCount(0);
	} finally {
		await setPublicShares(page, false);
	}
});

test('shared links narrows by status and explains why links are off', async ({
	page,
	browser
}, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'public sharing is instance-wide');
	await login(page);
	await expect(page).toHaveURL('/');
	const token = uniqueToken();
	const username = `status${token}`;
	const active = `Status active ${token}`;
	const paused = `Status paused ${token}`;
	const activeRecipe = await createRecipe(page, { ...loadFixture(0), title: active });
	const pausedRecipe = await createRecipe(page, { ...loadFixture(1), title: paused });
	const member = await createUser(page, { username, role: 'user' });

	try {
		await setPublicShares(page, true);
		await createPublicShare(page, activeRecipe.id);
		const memberContext = await browser.newContext();
		try {
			const memberPage = await memberContext.newPage();
			await login(memberPage, username);
			await expect(memberPage).toHaveURL('/');
			await createPublicShare(memberPage, pausedRecipe.id);
		} finally {
			await memberContext.close();
		}
		// Withdrawing the member's right pauses their link; the admin's stays active.
		await setCanSharePublicly(page, member.id, false);

		await page.goto('/settings/shares?all=1');
		const list = page.getByRole('list', { name: 'Shared links' });
		await expect(list.getByRole('listitem').filter({ hasText: paused })).toBeVisible();
		await expect(list.getByRole('listitem').filter({ hasText: active })).toBeVisible();

		await page.getByRole('button', { name: /^Paused/ }).click();
		await expect(page).toHaveURL('/settings/shares?all=1&status=paused');
		await expect(list.getByRole('listitem').filter({ hasText: paused })).toBeVisible();
		await expect(list.getByRole('listitem').filter({ hasText: active })).toHaveCount(0);
		await page.reload();
		await expect(list.getByRole('listitem').filter({ hasText: paused })).toBeVisible();
		await expect(list.getByRole('listitem').filter({ hasText: active })).toHaveCount(0);

		// With the household switch off, the page says why nothing opens and
		// where the owner turns it back on.
		await setPublicShares(page, false);
		await page.reload();
		const notice = page.getByRole('status').filter({ hasText: 'Public sharing is off' });
		await expect(notice).toBeVisible();
		await expect(notice.getByRole('link', { name: 'Turn it on under People' })).toHaveAttribute(
			'href',
			'/settings/users'
		);
	} finally {
		await setCanSharePublicly(page, member.id, true);
		await setPublicShares(page, false);
	}
});

test('a member whose right is withdrawn sees why their link is paused', async ({
	page,
	browser
}, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'public sharing is instance-wide');
	await login(page);
	await expect(page).toHaveURL('/');
	const token = uniqueToken();
	const username = `withdrawn${token}`;
	const title = `Withdrawn ${token}`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title });
	const member = await createUser(page, { username, role: 'user' });

	try {
		await setPublicShares(page, true);
		const memberContext = await browser.newContext();
		try {
			const memberPage = await memberContext.newPage();
			await login(memberPage, username);
			await expect(memberPage).toHaveURL('/');
			await createPublicShare(memberPage, recipe.id);

			await setCanSharePublicly(page, member.id, false);

			// The recipe page keeps the marker and says in the colophon why the
			// link does not open.
			await memberPage.goto(`/recipes/${recipe.slug}`);
			await expect(memberPage.getByRole('button', { name: 'Shared publicly' })).toBeVisible();
			await expect(
				memberPage.locator('footer').filter({ hasText: 'Your public link is paused' })
			).toBeVisible();

			// Shared links says it too, above the list.
			await memberPage.goto('/settings/shares');
			await expect(
				memberPage
					.getByRole('status')
					.filter({ hasText: 'You may not create public links at the moment' })
			).toBeVisible();
			await expect(
				memberPage
					.getByRole('list', { name: 'Shared links' })
					.getByRole('listitem')
					.filter({ hasText: title })
					.getByText('Paused')
			).toBeVisible();
		} finally {
			await memberContext.close();
		}
	} finally {
		await setCanSharePublicly(page, member.id, true);
		await setPublicShares(page, false);
	}
});

test("only the owner switches an admin's right to share", async ({ page, browser }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'the members table is checked on desktop');
	await login(page);
	await expect(page).toHaveURL('/');
	const token = uniqueToken();
	const admin = `rankadmin${token}`;
	const otherAdmin = `rankpeer${token}`;
	const member = `rankmember${token}`;
	await createUser(page, { username: admin, role: 'admin' });
	await createUser(page, { username: otherAdmin, role: 'admin' });
	await createUser(page, { username: member, role: 'user' });

	// People are grouped by role, one list per group, so a row is found by
	// name across all of them.
	const rowOf = (p: typeof page, username: string) =>
		p.getByRole('listitem').filter({ hasText: username });

	// The owner reaches every row but their own, admins included.
	await page.goto('/settings/users');
	await expect(rowOf(page, otherAdmin).getByRole('button', { name: 'More actions' })).toBeVisible();
	await expect(rowOf(page, member).getByRole('button', { name: 'More actions' })).toBeVisible();

	// An admin reaches plain members only - not another admin, not themselves.
	const adminContext = await browser.newContext();
	try {
		const adminPage = await adminContext.newPage();
		await login(adminPage, admin);
		await expect(adminPage).toHaveURL('/');
		await adminPage.goto('/settings/users');
		await expect(
			rowOf(adminPage, member).getByRole('button', { name: 'More actions' })
		).toBeVisible();
		await expect(
			rowOf(adminPage, otherAdmin).getByRole('button', { name: 'More actions' })
		).toHaveCount(0);
		await expect(rowOf(adminPage, admin).getByRole('button', { name: 'More actions' })).toHaveCount(
			0
		);
	} finally {
		await adminContext.close();
	}
});

test("withdrawing sharing pauses a member's links", async ({ page, browser }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'public sharing is instance-wide');
	await login(page);
	await expect(page).toHaveURL('/');
	const token = uniqueToken();
	const username = `withdrawpause${token}`;
	const title = `Withdraw pause ${token}`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title });
	const member = await createUser(page, { username, role: 'user' });

	try {
		await setPublicShares(page, true);

		const memberContext = await browser.newContext();
		let share: PublicShare;
		try {
			const memberPage = await memberContext.newPage();
			await login(memberPage, username);
			await expect(memberPage).toHaveURL('/');
			share = await createPublicShare(memberPage, recipe.id);
		} finally {
			await memberContext.close();
		}
		const shareToken = share.path.replace('/s/', '');

		await page.goto(`/s/${shareToken}`);
		await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();

		await page.goto('/settings/users');
		const row = page
			.getByRole('list', { name: 'Members' })
			.getByRole('listitem')
			.filter({ hasText: username });
		await row.getByRole('button', { name: 'More actions' }).click();
		await page.getByRole('menuitemcheckbox', { name: 'May share publicly' }).click();
		await expect(page.getByText(`${username} can no longer share publicly`)).toBeVisible();

		await page.goto('/settings/shares');
		await page.getByRole('switch', { name: 'Everyone in the household' }).click();
		const shareRow = page
			.getByRole('list', { name: 'Shared links' })
			.getByRole('listitem')
			.filter({ hasText: title });
		await expect(shareRow.getByText('Paused')).toBeVisible();

		await page.goto(`/s/${shareToken}`);
		await expect(page.getByText('This link is no longer available.')).toBeVisible();

		await page.goto('/settings/users');
		await row.getByRole('button', { name: 'More actions' }).click();
		await page.getByRole('menuitemcheckbox', { name: 'May share publicly' }).click();
		await expect(page.getByText(`${username} can now share publicly`)).toBeVisible();

		await page.goto(`/s/${shareToken}`);
		await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
	} finally {
		await setCanSharePublicly(page, member.id, true);
		await setPublicShares(page, false);
	}
});

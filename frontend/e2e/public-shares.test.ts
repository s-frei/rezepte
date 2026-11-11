import { expect, test, type Page } from '@playwright/test';
import {
	addDiaryEntry,
	addDiaryPhoto,
	createPublicShare,
	createRecipe,
	createUser,
	loadFixture,
	login,
	openFromPeopleList,
	openLinkTab,
	openRecipeMenu,
	patchSettings,
	type PublicShare,
	setCanSharePublicly,
	setOwnAvatarViaApi,
	setPublicShares,
	tinyPng,
	uniqueToken,
	uploadImage
} from './helpers';

// Public sharing is an instance-wide switch, like link previews: this file
// runs in the desktop project only (the mobile project ignores it), and
// every test that flips it turns it back off, whatever happens. They share
// one instance, so they run one after the other rather than in parallel.
// The tests that never flip it, and so run on a phone too, live in
// public-shares-stateless.test.ts.
test.describe.configure({ mode: 'serial' });

test('a stranger opens a public link', async ({ page, browser }) => {
	await login(page);
	const title = `Public ${uniqueToken()}`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title });
	await uploadImage(page, recipe.id, tinyPng());
	await addDiaryEntry(page, recipe.id, 'Only for the household.');
	const commentImageId = await addDiaryPhoto(page, recipe.id, 'A private photo', tinyPng());

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
			// The comments stay in the household.
			await expect(stranger.getByRole('tab', { name: /^Comments/ })).toHaveCount(0);
			await expect(stranger.getByText('Only for the household.')).toHaveCount(0);
			// ... and so do their photos, whichever way they are asked for.
			await expect(stranger.getByText('A private photo')).toHaveCount(0);
			const leaked = await stranger.request.get(
				`/public-images/${token}/${commentImageId}/thumb.jpg`
			);
			expect(leaked.status()).toBe(404);
			await expect(stranger.getByRole('link', { name: 'Edit' })).toHaveCount(0);
			await expect(stranger.getByRole('button', { name: 'Edit' })).toHaveCount(0);
			await expect(stranger.getByRole('button', { name: 'More actions' })).toHaveCount(0);
			await expect(stranger.getByRole('button', { name: 'Add to shopping list' })).toHaveCount(0);
			const footer = stranger.getByRole('contentinfo');
			await expect(footer).toContainText('A cookbook like this one?');
			await expect(footer.getByRole('link', { name: /Get Rezepte/ })).toHaveAttribute(
				'href',
				'https://getrezepte.app/'
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

test('a signed-in member sees the same public page', async ({ page }) => {
	await login(page);
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
		await expect(page.getByRole('button', { name: 'Add to shopping list' })).toHaveCount(0);
		await expect(
			page.getByRole('contentinfo').getByRole('link', { name: /Get Rezepte/ })
		).toBeVisible();
	} finally {
		await setPublicShares(page, false);
	}
});

// With the sharer switch off (the default) a public page shows nobody by
// photo, and the browser's bar is the app's, not theirs. The one face a
// public page may show is the sharer's, under that switch (below).
test('a public page shows no one by photo, signed in or not', async ({ page, browser }) => {
	const name = `face${uniqueToken()}`.toLowerCase();
	await login(page);
	await createUser(page, { username: name, role: 'admin' });
	try {
		await setPublicShares(page, true);
		await login(page, name);
		await setOwnAvatarViaApi(page, tinyPng([30, 140, 90]));
		const title = `Faceless ${uniqueToken()}`;
		const recipe = await createRecipe(page, { ...loadFixture(1), title });
		await addDiaryEntry(page, recipe.id, 'With my face on it');
		const share = await createPublicShare(page, recipe.id);

		const stranger = await browser.newContext();
		try {
			for (const viewer of [page, await stranger.newPage()]) {
				await viewer.goto(share.path);
				await expect(viewer.getByRole('heading', { level: 1, name: title })).toBeVisible();
				await expect(viewer.locator('img[src*="/avatars/"]')).toHaveCount(0);
				await expect(
					viewer.locator('meta[name="theme-color"][data-scheme="light"]')
				).toHaveAttribute('content', '#f4ede2');
			}
		} finally {
			await stranger.close();
		}
	} finally {
		await login(page);
		await setPublicShares(page, false);
	}
});

test('a recipe without photos, description or times', async ({ page }) => {
	await login(page);
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

test('a stranger sees where the recipe comes from', async ({ page, browser }) => {
	await login(page);
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

test('a public link opens a step photo from its own image route', async ({ page, browser }) => {
	await login(page);
	const input = { ...loadFixture(0), title: `Step photo ${uniqueToken()}` };
	const recipe = await createRecipe(page, input);
	const image = await uploadImage(page, recipe.id, tinyPng());
	const [first, ...rest] = input.steps;
	const phrase = first.text.split(' ').slice(0, 2).join(' ');
	const put = await page.request.put(`/api/v1/recipes/${recipe.id}`, {
		data: { ...input, steps: [{ ...first, photos: [{ phrase, imageId: image.id }] }, ...rest] }
	});
	expect(put.ok(), `PUT recipe failed: ${put.status()} ${await put.text()}`).toBeTruthy();

	const strangerContext = await browser.newContext();
	try {
		await setPublicShares(page, true);
		const share = await createPublicShare(page, recipe.id);
		const stranger = await strangerContext.newPage();
		await stranger.goto(share.path);
		await stranger.locator('[data-step-photo]').click();
		await expect(stranger.getByRole('dialog').locator('img')).toHaveAttribute(
			'src',
			new RegExp(`/public-images/${share.path.slice(3)}/${image.id}/detail`)
		);
	} finally {
		await strangerContext.close();
		await setPublicShares(page, false);
	}
});

test('the owner stops public pages from naming Rezepte', async ({ page, browser }) => {
	await login(page);
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
		await patchSettings(page, { publicShareAttribution: true });
		await setPublicShares(page, false);
	}
});

test('the owner lets public pages name who shared, with their photo', async ({ page, browser }) => {
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Signed ${uniqueToken()}` });
	const me = (await (await page.request.get('/api/v1/auth/me')).json()) as { displayName: string };
	// A photo the stranger sees only while the switch is on, and only at
	// the share's own address.
	await setOwnAvatarViaApi(page, tinyPng([30, 140, 90]));

	try {
		await setPublicShares(page, true);
		const share = await createPublicShare(page, recipe.id);
		const strangerContext = await browser.newContext();
		try {
			const stranger = await strangerContext.newPage();
			await stranger.goto(share.path);
			await expect(stranger.getByRole('heading', { level: 1, name: recipe.title })).toBeVisible();
			// Off by default: nobody is named, no face shown.
			await expect(stranger.getByText(/^Shared by /)).toHaveCount(0);
			await expect(stranger.locator('img[src$="/sharer.jpg"], img[src*="/avatars/"]')).toHaveCount(
				0
			);

			await page.goto('/settings/users');
			const sharer = page.getByRole('switch', { name: 'Name who shared it' });
			await expect(sharer).not.toBeChecked();
			await sharer.click();
			await expect(sharer).toBeChecked();

			await stranger.reload();
			// The ribbon under the title block: the name in the line, the face
			// beside it from the share's own address.
			const ribbon = stranger
				.getByRole('main')
				.locator('p', { hasText: `Shared by ${me.displayName}` });
			await expect(ribbon).toBeVisible();
			await expect(ribbon.locator('b')).toHaveText(me.displayName);
			const photo = ribbon.locator(`img[src="/public-images/${share.path.slice(3)}/sharer.jpg"]`);
			await expect(photo).toBeVisible();
			await expect
				.poll(() => photo.evaluate((img: HTMLImageElement) => img.naturalWidth))
				.toBeGreaterThan(0);
			await expect(stranger.locator('img[src*="/avatars/"]')).toHaveCount(0);

			// Off again: name and photo go at once, the address with them.
			await patchSettings(page, { publicShareSharer: false });
			await stranger.reload();
			await expect(stranger.getByRole('heading', { level: 1, name: recipe.title })).toBeVisible();
			await expect(stranger.getByText(/^Shared by /)).toHaveCount(0);
			await expect(stranger.locator('img[src$="/sharer.jpg"], img[src*="/avatars/"]')).toHaveCount(
				0
			);
			expect(
				(await stranger.request.get(`/public-images/${share.path.slice(3)}/sharer.jpg`)).status()
			).toBe(404);
		} finally {
			await strangerContext.close();
		}
	} finally {
		await page.request.delete('/api/v1/auth/me/avatar');
		await patchSettings(page, { publicShareSharer: false });
		await setPublicShares(page, false);
	}
});

// The same switch names Rezepte at the foot of a recipe card, and the owner
// finds it whether public links are on or not (see share-image.test.ts).
test('the credit on a recipe card follows the owner setting', async ({ page }) => {
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

		await patchSettings(page, { publicShareAttribution: false });
		await page.goto(`/recipes/${recipe.slug}`);
		await openCard();
		await expect(page.locator('[data-testid="share-card"] footer')).toHaveCount(0);

		await patchSettings(page, { publicShareAttribution: true });
		await page.reload();
		await openCard();
		await expect(page.locator('[data-testid="share-card"] footer')).toHaveCount(1);
	} finally {
		await patchSettings(page, { publicShareAttribution: true });
	}
});

// Stands in for the share sheet, recording each address it was handed.
async function recordShares(page: Page) {
	await page.addInitScript(() => {
		const shared: string[] = [];
		(window as unknown as { shared: string[] }).shared = shared;
		Object.defineProperty(Navigator.prototype, 'share', {
			value: (data: ShareData) => {
				shared.push(data.url ?? '');
				return Promise.resolve();
			}
		});
	});
}

const sharedUrls = (page: Page) =>
	page.evaluate(() => (window as unknown as { shared: string[] }).shared);

// The date the chip and the row show: day and short month, the year only
// when it is not this one.
function shortDate(iso: string): string {
	const date = new Date(iso);
	const sameYear = date.getFullYear() === new Date().getFullYear();
	return date
		.toLocaleDateString('en', {
			day: 'numeric',
			month: 'short',
			year: sameYear ? undefined : 'numeric'
		})
		.replaceAll(' ', '\u00a0');
}

test('a member creates, shares, re-times and revokes a public link', async ({ page }) => {
	await recordShares(page);
	await login(page);
	const title = `Shareable ${uniqueToken()}`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title });

	try {
		await setPublicShares(page, true, 7, null);
		// The phone's sheet, where "Share link" hands the link to the share
		// sheet; tall enough that the preview keeps its chip.
		await page.setViewportSize({ width: 360, height: 900 });
		await page.goto(`/recipes/${recipe.slug}`);

		const sheet = await openLinkTab(page);
		await expect(sheet.getByRole('radio', { name: 'For the household' })).toBeChecked();
		// The household row unfolds its own detail: who opens it, and the way
		// to the people; picking the public row folds it away.
		const household = sheet.getByText('Anyone with an account here. Never expires.');
		await expect(household).toBeVisible();
		await expect(sheet.getByRole('link', { name: 'People' })).toHaveAttribute(
			'href',
			'/settings/users'
		);
		await sheet.getByRole('radio', { name: 'For everyone with the link' }).click();
		await expect(household).toHaveCount(0);
		// The arrow keys keep to the audience: the lifetime bar inside the
		// public row has radios of its own, which they once stepped into.
		await page.keyboard.press('ArrowDown');
		await expect(sheet.getByRole('radio', { name: 'For the household' })).toBeFocused();
		await sheet.getByRole('radio', { name: 'For everyone with the link' }).click();
		// The household's default lifetime (7 days) comes preselected.
		await expect(
			sheet.getByRole('radiogroup', { name: 'Lifetime' }).getByRole('radio', { name: '7 days' })
		).toBeChecked();

		// Creating is its own tap and hands nothing on yet.
		await sheet.getByRole('button', { name: 'Create public link' }).click();
		const shareLink = sheet.getByRole('button', { name: 'Share link', exact: true });
		await expect(shareLink).toBeEnabled();
		// Focus stays on the primary, which now hands the link on.
		await expect(shareLink).toBeFocused();
		expect(await sharedUrls(page)).toEqual([]);
		await shareLink.click();
		await expect.poll(() => sharedUrls(page)).toHaveLength(1);
		const [url] = await sharedUrls(page);
		expect(url).toMatch(/\/s\/[^/]+$/);
		const token = url.split('/s/')[1];

		// Every cell now restarts the link from today, at the same address.
		const patched = page.waitForResponse(
			(response) =>
				response.request().method() === 'PATCH' && response.url().includes('/api/v1/shares/')
		);
		await sheet.getByRole('button', { name: '30 days' }).click();
		const retimed = (await (await patched).json()) as PublicShare;
		expect(retimed.path).toBe(`/s/${token}`);
		await expect(sheet.getByRole('button', { name: '30 days' })).toHaveAttribute(
			'aria-pressed',
			'true'
		);
		const until = shortDate(retimed.expiresAt!);
		await expect(sheet.getByText(`Accessible to everyone · until ${until}`)).toBeVisible();
		await expect(sheet.getByText(`Opens without an account, until ${until}`)).toBeVisible();

		await page.keyboard.press('Escape');
		await expect(sheet).toBeHidden();
		await expect(page.getByRole('button', { name: 'Shared publicly' })).toBeVisible();

		// Revoking lives in the row, behind the existing confirmation.
		await page.getByRole('button', { name: 'Shared publicly' }).click();
		await expect(sheet.getByRole('radio', { name: 'For everyone with the link' })).toBeChecked();
		await expect(sheet.getByRole('link', { name: 'Shared links' })).toHaveAttribute(
			'href',
			'/settings/shares'
		);
		await sheet.getByRole('button', { name: 'Revoke' }).click();
		await page
			.getByRole('dialog', { name: 'Revoke public link?' })
			.getByRole('button', { name: 'Revoke' })
			.click();
		await expect(page.getByText('Public link revoked')).toBeVisible();
		await expect(sheet.getByRole('radio', { name: 'For the household' })).toBeChecked();
		// The Revoke button is gone: focus lands on the choice that is left.
		await expect(sheet.getByRole('radio', { name: 'For the household' })).toBeFocused();
		await expect(sheet.getByText('Accessible after signing in')).toBeVisible();

		await page.keyboard.press('Escape');
		await expect(page.getByRole('button', { name: 'Shared publicly' })).toHaveCount(0);
		await expect(page.getByText('Shared publicly by you until')).toHaveCount(0);

		await page.goto(`/s/${token}`);
		await expect(page.getByText('This link is no longer available.')).toBeVisible();

		// The dialog's primary takes over from Create the same way.
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto(`/recipes/${recipe.slug}`);
		const dialog = await openLinkTab(page);
		await dialog.getByRole('radio', { name: 'For everyone with the link' }).click();
		await dialog.getByRole('button', { name: 'Create public link' }).click();
		await expect(dialog.getByRole('button', { name: 'Copy link', exact: true })).toBeFocused();
	} finally {
		await setPublicShares(page, false);
	}
});

test('the marker and the colophon open the Link tab', async ({ page }) => {
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Marker ${uniqueToken()}` });

	try {
		await setPublicShares(page, true);
		await createPublicShare(page, recipe.id, 30);
		await page.goto(`/recipes/${recipe.slug}`);

		// The menu has no public-link item of its own any more.
		await openRecipeMenu(page);
		await expect(page.getByRole('menuitem', { name: /publicly|Public link/ })).toHaveCount(0);
		await page.keyboard.press('Escape');

		const sheet = page.getByRole('dialog', { name: 'Pass on recipe' });
		await page.getByRole('button', { name: 'Shared publicly' }).click();
		await expect(sheet.getByRole('radio', { name: 'Link', exact: true })).toBeChecked();
		await expect(sheet.getByRole('radio', { name: 'For everyone with the link' })).toBeChecked();
		await page.keyboard.press('Escape');
		await expect(sheet).toBeHidden();

		await page.locator('footer').getByRole('button', { name: 'Manage link' }).click();
		await expect(sheet.getByRole('radio', { name: 'Link', exact: true })).toBeChecked();
		await page.keyboard.press('Escape');
		await expect(sheet).toBeHidden();

		// "Pass on" opens on the tab this device picked last, which is none yet.
		await page.getByRole('button', { name: 'Pass on' }).click();
		await expect(sheet.getByRole('radio', { name: 'Image', exact: true })).toBeChecked();
	} finally {
		await setPublicShares(page, false);
	}
});

test('a refused link says whether sharing is off or the right withdrawn', async ({
	page,
	browser
}) => {
	await login(page);
	const token = uniqueToken();
	const username = `refused${token}`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Refused ${token}` });
	const member = await createUser(page, { username, role: 'user' });

	// The sheet is opened while sharing is allowed and the refusal happens
	// behind its back, the way a second tab or another admin would cause it.
	async function refuse(on: typeof page, change: () => Promise<unknown>, toast: string) {
		await on.goto(`/recipes/${recipe.slug}`);
		const sheet = await openLinkTab(on);
		await sheet.getByRole('radio', { name: 'For everyone with the link' }).click();
		await change();
		await sheet.getByRole('button', { name: 'Create public link' }).click();
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
			await refuse(
				memberPage,
				() => setCanSharePublicly(page, member.id, false),
				'You may not share recipes publicly.'
			);
		} finally {
			await memberContext.close();
		}
	} finally {
		await setCanSharePublicly(page, member.id, true);
		await setPublicShares(page, false);
	}
});

test('a member without the right is offered the household link only', async ({ page, browser }) => {
	await login(page);
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
			await memberPage.goto(`/recipes/${recipe.slug}`);
			const sheet = await openLinkTab(memberPage);
			await expect(sheet.getByText('Accessible after signing in')).toBeVisible();
			await expect(sheet.getByRole('radio', { name: /For everyone/ })).toHaveCount(0);
		} finally {
			await memberContext.close();
		}
	} finally {
		await setCanSharePublicly(page, member.id, true);
		await setPublicShares(page, false);
	}
});

test('with public sharing off the Link tab offers the household link only', async ({ page }) => {
	await login(page);
	const title = `No sharing ${uniqueToken()}`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title });

	await setPublicShares(page, false);
	await page.goto(`/recipes/${recipe.slug}`);
	const sheet = await openLinkTab(page);
	await expect(sheet.getByText('Accessible after signing in')).toBeVisible();
	await expect(sheet.getByRole('radio', { name: /For everyone/ })).toHaveCount(0);
});

test('a limited link starts again from the lifetime picked', async ({ page }) => {
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Limited ${uniqueToken()}` });

	try {
		await setPublicShares(page, true);
		await createPublicShare(page, recipe.id, 7);
		// A link is limited once the owner's maximum has run out since its
		// start, days from now: the page is told so instead.
		await page.route('**/api/v1/shares/by-recipe/*', async (route) => {
			const response = await route.fetch();
			const { share } = (await response.json()) as { share: PublicShare };
			await route.fulfill({ response, json: { share: { ...share, status: 'limited' } } });
		});
		await page.goto(`/recipes/${recipe.slug}`);

		await page.getByRole('button', { name: 'Shared publicly' }).click();
		const sheet = page.getByRole('dialog', { name: 'Pass on recipe' });
		await expect(sheet.getByRole('radio', { name: 'For the household' })).toBeChecked();
		await sheet.getByRole('radio', { name: 'For everyone with the link' }).click();
		await expect(sheet.getByText('Your public link is limited by the maximum')).toBeVisible();
		await expect(sheet.getByText('Pick a lifetime to start it again.')).toBeVisible();
		const copy = sheet.getByRole('button', { name: 'Copy link', exact: true });
		await expect(copy).toBeDisabled();
		await expect(sheet.getByRole('group', { name: 'Lifetime' })).not.toHaveAttribute(
			'aria-disabled',
			'true'
		);

		await sheet.getByRole('button', { name: '30 days' }).click();
		await expect(copy).toBeEnabled();
		await expect(sheet.getByText(/^Opens without an account, until /)).toBeVisible();
	} finally {
		await setPublicShares(page, false);
	}
});

test('a re-time the service refuses keeps the link, or drops a gone one', async ({ page }) => {
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Retime ${uniqueToken()}` });

	try {
		await setPublicShares(page, true);
		await createPublicShare(page, recipe.id, 7);
		let answer: { status: number; json: object } = {
			status: 422,
			json: { title: 'Unprocessable Entity', status: 422, errors: [{ location: 'body.days' }] }
		};
		await page.route('**/api/v1/shares/*', (route) =>
			route.request().method() === 'PATCH'
				? route.fulfill({ status: answer.status, json: answer.json })
				: route.fallback()
		);
		await page.goto(`/recipes/${recipe.slug}`);
		await page.getByRole('button', { name: 'Shared publicly' }).click();
		const sheet = page.getByRole('dialog', { name: 'Pass on recipe' });
		const everyone = sheet.getByRole('radio', { name: 'For everyone with the link' });
		await expect(everyone).toBeChecked();

		// Above a maximum lowered meanwhile: the link stays as it was.
		await sheet.getByRole('button', { name: '30 days' }).click();
		await expect(page.getByText('That lifetime is longer than the maximum allowed.')).toBeVisible();
		await expect(everyone).toBeChecked();
		await expect(sheet.getByRole('button', { name: '7 days' })).toHaveAttribute(
			'aria-pressed',
			'true'
		);

		// Revoked meanwhile, in another tab: the sheet falls back to the household.
		answer = { status: 404, json: { title: 'Not Found', status: 404 } };
		await sheet.getByRole('button', { name: '30 days' }).click();
		await expect(page.getByText('The lifetime could not be changed.')).toBeVisible();
		await expect(sheet.getByRole('radio', { name: 'For the household' })).toBeChecked();
		await expect(sheet.getByRole('radio', { name: 'For the household' })).toBeFocused();
		await expect(sheet.getByText('Opens without an account', { exact: true })).toBeVisible();

		// The right withdrawn meanwhile: the link is paused, nothing hands it on.
		answer = { status: 403, json: { title: 'Forbidden', status: 403, reason: 'not-allowed' } };
		await page.goto(`/recipes/${recipe.slug}`);
		await page.getByRole('button', { name: 'Shared publicly' }).click();
		await expect(everyone).toBeChecked();
		await sheet.getByRole('button', { name: '30 days' }).click();
		await expect(page.getByText('You may not share recipes publicly.')).toBeVisible();
		await expect(everyone).toBeChecked();
		await expect(sheet.getByText('Your public link is paused')).toBeVisible();
		await expect(sheet.getByRole('button', { name: 'Copy link', exact: true })).toBeDisabled();
		await expect(sheet.getByRole('group', { name: 'Lifetime' })).toHaveAttribute(
			'aria-disabled',
			'true'
		);
	} finally {
		await setPublicShares(page, false);
	}
});

test('revoking a link that is already gone lets go of it', async ({ page }) => {
	await recordShares(page);
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Gone ${uniqueToken()}` });

	try {
		await setPublicShares(page, true);
		await createPublicShare(page, recipe.id, 7);
		// Revoked meanwhile, in another tab: the service no longer knows it.
		await page.route('**/api/v1/shares/*', (route) =>
			route.request().method() === 'DELETE'
				? route.fulfill({ status: 404, json: { title: 'Not Found', status: 404 } })
				: route.fallback()
		);
		await page.setViewportSize({ width: 360, height: 900 });
		await page.goto(`/recipes/${recipe.slug}`);

		await page.getByRole('button', { name: 'Shared publicly' }).click();
		const sheet = page.getByRole('dialog', { name: 'Pass on recipe' });
		await expect(sheet.getByRole('radio', { name: 'For everyone with the link' })).toBeChecked();
		await sheet.getByRole('button', { name: 'Revoke' }).click();
		await page
			.getByRole('dialog', { name: 'Revoke public link?' })
			.getByRole('button', { name: 'Revoke' })
			.click();

		await expect(page.getByText('Public link revoked')).toBeVisible();
		await expect(sheet.getByRole('radio', { name: 'For the household' })).toBeChecked();
		await expect(sheet.getByText('Opens without an account', { exact: true })).toBeVisible();
		// Nothing in the sheet shows or hands out the dead address any more.
		await expect(sheet.getByText(/\/s\//)).toHaveCount(0);
		await sheet.getByRole('button', { name: 'Share link', exact: true }).click();
		await expect.poll(() => sharedUrls(page)).toHaveLength(1);
		const [url] = await sharedUrls(page);
		expect(url).toContain(`/recipes/${recipe.slug}`);
		expect(url).not.toContain('/s/');
	} finally {
		await setPublicShares(page, false);
	}
});

// The preview gives way on small phones: the buttons stay on screen, the
// sheet itself never scrolls, and the region above the buttons steps its
// preview down until it fits, shrinking the ticket rather than dropping it.
test('the sheet fits small screens', async ({ page }) => {
	await page.emulateMedia({ reducedMotion: 'reduce' });
	await recordShares(page);
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: 'Fit check' });
	await uploadImage(page, recipe.id, tinyPng([60, 140, 70], 320, 240));

	try {
		await setPublicShares(page, true);
		await createPublicShare(page, recipe.id, null);
		let last = -1;
		for (const viewport of [
			{ width: 360, height: 780 },
			{ width: 360, height: 640 },
			{ width: 320, height: 568 },
			// 150 % zoom on a 360px phone.
			{ width: 240, height: 520 }
		]) {
			await page.setViewportSize(viewport);
			await page.goto(`/recipes/${recipe.slug}`);
			await page.getByRole('button', { name: 'Shared publicly' }).click();
			const sheet = page.getByRole('dialog', { name: 'Pass on recipe' });
			const shareLink = sheet.getByRole('button', { name: 'Share link', exact: true });
			await expect(shareLink).toBeVisible();
			const box = (await shareLink.boundingBox())!;
			expect(box.y).toBeGreaterThanOrEqual(0);
			expect(box.y + box.height).toBeLessThanOrEqual(viewport.height);
			expect(box.x + box.width).toBeLessThanOrEqual(viewport.width);
			expect(await sheet.evaluate((dialog) => dialog.scrollHeight - dialog.clientHeight)).toBe(0);
			const region = sheet.locator('[data-fit]');
			// The ticket shrinks at the last step but never goes, nor runs past
			// the sheet's sides.
			const ticket = sheet.locator('[data-ticket]');
			await expect(ticket).toBeVisible();
			const within = async () => {
				const edges = (await ticket.boundingBox())!;
				expect(edges.x).toBeGreaterThanOrEqual(0);
				expect(edges.x + edges.width).toBeLessThanOrEqual(viewport.width);
			};
			await within();
			// The hidden ticket's recipe and address stay for screen readers.
			await expect(region.locator('p.sr-only', { hasText: 'Fit check' })).toHaveCount(1);
			// Nothing clipped under the buttons - unless the ticket is down to its
			// smallest, where the region scrolls as the last resort (320x568 in
			// English, whose public row wraps to three lines). Polled: the fit
			// runs a frame or two after the sheet is up.
			await expect
				.poll(() =>
					region.evaluate((r) => {
						const ticket = r.querySelector<HTMLElement>('[data-ticket]');
						const zoom = Number(ticket?.style.zoom || 1);
						return r.scrollHeight - r.clientHeight <= 1 || zoom <= 0.6;
					})
				)
				.toBe(true);
			// Smaller screens take the same step or a later one.
			const step = Number((await region.getAttribute('data-fit')) || 0);
			expect(step).toBeGreaterThanOrEqual(last);
			// The roomiest keeps its photo: a fit that over-steps fails here.
			if (last < 0) expect(step).toBeLessThan(3);
			last = step;
			// The household shows the public one's ticket, within the sheet too.
			await sheet.getByRole('radio', { name: 'For the household' }).click();
			await within();
		}
		// The smallest needs a step: the fit logic ran.
		expect(last).toBeGreaterThan(0);
	} finally {
		await setPublicShares(page, false);
	}
});

// The dialog keeps its size too when the audience changes: a centered box
// that grew would move both its edges.
test('the dialog keeps its size when the audience changes', async ({ page }) => {
	await page.emulateMedia({ reducedMotion: 'reduce' });
	await page.route('**/api/v1/settings', async (route) => {
		const response = await route.fetch();
		await route.fulfill({ response, json: { ...(await response.json()), publicShares: true } });
	});
	await page.route('**/api/v1/shares/by-recipe/*', (route) =>
		route.fulfill({ json: { share: null } })
	);
	await login(page);
	// A title over three lines makes the column taller than the preview, so a
	// detail taller than the other would grow the dialog.
	const recipe = await createRecipe(page, {
		...loadFixture(0),
		title: `Sized ${uniqueToken()}, a recipe whose title runs on over three lines of the dialog`
	});
	await page.goto(`/recipes/${recipe.slug}`);

	const sheet = await openLinkTab(page);
	const size = async () => {
		const box = (await sheet.boundingBox())!;
		return JSON.stringify([Math.round(box.y), Math.round(box.height)]);
	};
	await expect(sheet.getByRole('radio', { name: 'For the household' })).toBeChecked();
	// The size it settles on: read until two reads agree.
	let household = '';
	await expect
		.poll(async () => {
			const last = household;
			household = await size();
			return household === last;
		})
		.toBe(true);
	await sheet.getByRole('radio', { name: 'For everyone with the link' }).click();
	await expect(sheet.getByRole('button', { name: 'Create public link' })).toBeVisible();
	await expect.poll(size).toBe(household);
});

test('the owner switches public sharing and its lifetimes', async ({ page }) => {
	await login(page);
	await setPublicShares(page, false);

	try {
		await page.goto('/settings/users');
		await expect(page.getByRole('heading', { name: 'Public sharing' })).toBeVisible();

		// Turning it on asks first; canceling leaves it off.
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

test('shared links lists and revokes', async ({ page, browser }) => {
	await login(page);
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
			await createPublicShare(memberPage, recipe.id);

			await memberPage.goto('/settings/shares');
			const list = memberPage.getByRole('list', { name: 'Shared links' });
			const row = list.getByRole('listitem').filter({ hasText: title });
			await expect(row).toBeVisible();
			// An active link carries no stamp; only a paused or limited one does.
			await expect(row).toContainText('Active');
			// The card's edge is neutral, not its creator's color.
			expect(
				await row.evaluate((el) => {
					const cs = getComputedStyle(el);
					return cs.borderLeftColor === cs.borderTopColor;
				})
			).toBe(true);
			await expect(row.getByRole('button', { name: 'Copy link' })).toBeVisible();

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

test("admins see everyone's links and revoke all", async ({ page, browser }) => {
	await login(page);
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
		await expect(row).toContainText(username);

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

test("an admin narrows everyone's links to one person", async ({ page, browser }) => {
	await login(page);
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

test('shared links narrows by status and explains why links are off', async ({ page, browser }) => {
	await login(page);
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

test('a paused link opens on the household and cannot be shared', async ({ page, browser }) => {
	await login(page);
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
			await createPublicShare(memberPage, recipe.id);

			await setCanSharePublicly(page, member.id, false);

			// The recipe page keeps the marker and says in the colophon why the
			// link does not open.
			await memberPage.goto(`/recipes/${recipe.slug}`);
			await expect(memberPage.getByRole('button', { name: 'Shared publicly' })).toBeVisible();
			await expect(
				memberPage.locator('footer').filter({ hasText: 'Your public link is paused' })
			).toBeVisible();

			// The sheet opens on the household; the public row says why and
			// keeps Revoke and Shared links, while nothing hands the link on.
			await memberPage.getByRole('button', { name: 'Shared publicly' }).click();
			const sheet = memberPage.getByRole('dialog', { name: 'Pass on recipe' });
			await expect(sheet.getByRole('radio', { name: 'For the household' })).toBeChecked();
			await sheet.getByRole('radio', { name: 'For everyone with the link' }).click();
			await expect(sheet.getByText('Your public link is paused')).toBeVisible();
			await expect(sheet.getByRole('group', { name: 'Lifetime' })).toHaveAttribute(
				'aria-disabled',
				'true'
			);
			await expect(sheet.getByRole('button', { name: 'Copy link', exact: true })).toBeDisabled();
			await expect(sheet.getByRole('button', { name: 'Revoke' })).toBeVisible();
			await expect(sheet.getByRole('link', { name: 'Shared links' })).toBeVisible();
			await memberPage.keyboard.press('Escape');

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
					.getByText('This person may no longer share publicly.')
			).toBeVisible();
		} finally {
			await memberContext.close();
		}
	} finally {
		await setCanSharePublicly(page, member.id, true);
		await setPublicShares(page, false);
	}
});

test("only the owner switches an admin's right to share", async ({ page, browser }) => {
	await login(page);
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
	// A card the viewer may open is one link.
	const openRow = (p: typeof page, username: string) =>
		rowOf(p, username).getByRole('link').click();

	// The owner reaches every row but their own, admins included: each of
	// those cards leads to the person's page, where the switch is.
	await page.goto('/settings/users');
	await openRow(page, otherAdmin);
	await expect(page.getByRole('switch', { name: 'May share publicly' })).toBeVisible();
	await page.goBack();
	await openRow(page, member);
	await expect(page.getByRole('switch', { name: 'May share publicly' })).toBeVisible();

	// An admin reaches plain members only - not another admin, not themselves.
	const adminContext = await browser.newContext();
	try {
		const adminPage = await adminContext.newPage();
		await login(adminPage, admin);
		await adminPage.goto('/settings/users');
		await expect(rowOf(adminPage, otherAdmin).getByRole('link')).toHaveCount(0);
		await expect(rowOf(adminPage, admin).getByRole('link')).toHaveAttribute('href', '/settings');
		await openRow(adminPage, member);
		await expect(adminPage.getByRole('switch', { name: 'May share publicly' })).toBeVisible();
	} finally {
		await adminContext.close();
	}
});

test("withdrawing sharing pauses a member's links", async ({ page, browser }) => {
	await login(page);
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
			share = await createPublicShare(memberPage, recipe.id);
		} finally {
			await memberContext.close();
		}
		const shareToken = share.path.replace('/s/', '');

		await page.goto(`/s/${shareToken}`);
		await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();

		await page.goto('/settings/users');
		await openFromPeopleList(
			page
				.getByRole('list', { name: 'Members' })
				.getByRole('listitem')
				.filter({ hasText: username })
		);
		const may = page.getByRole('switch', { name: 'May share publicly' });
		await expect(may).toBeChecked();
		await may.click();
		await expect(page.getByText(`${username} can no longer share publicly`)).toBeVisible();
		await expect(may).not.toBeChecked();

		await page.goto('/settings/shares');
		await page.getByRole('switch', { name: 'Everyone in the household' }).click();
		const shareRow = page
			.getByRole('list', { name: 'Shared links' })
			.getByRole('listitem')
			.filter({ hasText: title });
		await expect(shareRow.getByText('This person may no longer share publicly.')).toBeVisible();

		await page.goto(`/s/${shareToken}`);
		await expect(page.getByText('This link is no longer available.')).toBeVisible();

		await page.goto(`/settings/users/${username}`);
		await page.getByRole('switch', { name: 'May share publicly' }).click();
		await expect(page.getByText(`${username} can now share publicly`)).toBeVisible();

		await page.goto(`/s/${shareToken}`);
		await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
	} finally {
		await setCanSharePublicly(page, member.id, true);
		await setPublicShares(page, false);
	}
});

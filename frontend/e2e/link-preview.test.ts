import { expect, test, type APIRequestContext, type Page } from '@playwright/test';
import {
	createRecipe,
	loadFixture,
	login,
	openRecipeMenu,
	setLinkPreviews,
	tinyPng,
	uniqueToken,
	uploadImage
} from './helpers';

// The crawler building a chat preview is not signed in, so every request
// below goes through the `request` fixture, which carries no session.
// Link previews are instance-wide and off by default; the tests that need
// them run in the desktop project only and switch them off again.
// Both tests flip the same instance-wide setting, so they run one after the
// other rather than in parallel.
test.describe.configure({ mode: 'serial' });

async function tags(request: APIRequestContext, path: string): Promise<Record<string, string>> {
	const html = await (await request.get(path)).text();
	const found: Record<string, string> = {};
	for (const [, key, value] of html.matchAll(
		/<meta\s+(?:property|name)="([^"]+)"\s+content="([^"]*)"/g
	)) {
		found[key] = value.replaceAll('&amp;', '&');
	}
	return found;
}

async function shareLink(
	page: Page,
	recipeId: string
): Promise<{ path: string; expiresAt: string | null }> {
	const origin = new URL(page.url()).origin;
	const response = await page.request.post(`/api/v1/recipes/${recipeId}/share-link`, {
		headers: { Origin: origin }
	});
	expect(response.ok()).toBeTruthy();
	return (await response.json()) as { path: string; expiresAt: string | null };
}

test('a share link previews the recipe, a plain link the app', async ({
	page,
	request
}, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'link previews are instance-wide');
	await login(page);
	await expect(page).toHaveURL('/');
	const title = `Preview ${uniqueToken()}`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title });
	const cover = await uploadImage(page, recipe.id, tinyPng([60, 140, 70]));
	const origin = new URL(page.url()).origin;

	try {
		await setLinkPreviews(page, true);
		const link = await shareLink(page, recipe.id);
		expect(link.path).toMatch(new RegExp(`^/recipes/${recipe.slug}\\?share=`));
		expect(link.expiresAt).not.toBeNull();

		const shared = await tags(request, link.path);
		expect(shared['og:title']).toBe(title);
		expect(shared['og:url']).toBe(new URL(link.path, origin).href);
		const image = new URL(shared['og:image']);
		expect(image.origin).toBe(origin);
		expect(image.pathname).toBe(`/link-preview/${recipe.id}/${cover.id}`);
		expect(shared['twitter:image']).toBe(shared['og:image']);
		const picture = await request.get(image.pathname + image.search);
		expect(picture.status()).toBe(200);
		expect(picture.headers()['content-type']).toBe('image/jpeg');

		// Without the token the page is the app's, and the cover stays private.
		const plain = await tags(request, `/recipes/${recipe.slug}`);
		expect(plain['og:title']).toBe('Rezepte');
		expect(new URL(plain['og:image']).pathname).toBe('/og.png');
		expect((await request.get(image.pathname)).status()).toBe(404);

		// The recipe itself still takes a login.
		expect((await request.get(`/api/v1/recipes/${recipe.id}`)).status()).toBe(401);

		// Switched off, a link carries no token and previews the app.
		await setLinkPreviews(page, false);
		const off = await shareLink(page, recipe.id);
		expect(off.path).toBe(`/recipes/${recipe.slug}`);
		expect(off.expiresAt).toBeNull();
		expect((await tags(request, link.path))['og:title']).toBe('Rezepte');
		expect((await request.get(image.pathname + image.search)).status()).toBe(404);
	} finally {
		await setLinkPreviews(page, false);
	}
});

test('Copy link hands out a previewing link and renews it when the tab comes back', async ({
	page,
	context,
	request
}, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'link previews are instance-wide');
	await context.grantPermissions(['clipboard-read', 'clipboard-write']);
	await login(page);
	await expect(page).toHaveURL('/');
	const title = `Copied ${uniqueToken()}`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title });
	const copied = async () => {
		await openRecipeMenu(page);
		await page.getByRole('menuitem', { name: 'Copy link' }).click();
		// The fake clock holds earlier toasts on screen; the newest one counts.
		await expect(page.getByText('Link copied').last()).toBeVisible();
		return page.evaluate(() => navigator.clipboard.readText());
	};

	try {
		await setLinkPreviews(page, true);
		// A fake clock, so the page can be moved past the link's lifetime
		// without its renewal timer firing first.
		await page.clock.install();
		const first = page.waitForResponse((r) => r.url().endsWith(`/${recipe.id}/share-link`));
		await page.goto(`/recipes/${recipe.slug}`);
		await first;

		const link = await copied();
		expect(link).toMatch(new RegExp(`/recipes/${recipe.slug}\\?share=`));
		expect((await tags(request, new URL(link).pathname + new URL(link).search))['og:title']).toBe(
			title
		);

		// Twenty minutes later the tab becomes visible again: the page renews
		// its stale 15-minute link before anyone copies it.
		await page.clock.setSystemTime(Date.now() + 20 * 60_000);
		const renewed = page.waitForResponse((r) => r.url().endsWith(`/${recipe.id}/share-link`));
		await page.evaluate(() => {
			Object.defineProperty(document, 'visibilityState', {
				configurable: true,
				get: () => 'visible'
			});
			document.dispatchEvent(new Event('visibilitychange'));
		});
		await renewed;
		const again = await copied();
		expect(again).toMatch(/\?share=/);
		expect((await tags(request, new URL(again).pathname + new URL(again).search))['og:title']).toBe(
			title
		);
	} finally {
		await setLinkPreviews(page, false);
	}
});

test('the owner switches link previews on and picks a lifetime', async ({ page }, testInfo) => {
	// One switch for the whole instance: only the desktop project flips it,
	// and it goes back off whatever happens.
	test.skip(testInfo.project.name.startsWith('mobile'), 'link previews are instance-wide');
	await login(page);
	await expect(page).toHaveURL('/');
	try {
		await page.goto('/settings/users');
		await expect(page.getByRole('heading', { name: 'Link previews' })).toBeVisible();
		const toggle = page.getByRole('switch', { name: 'Show recipes in link previews' });
		await expect(toggle).not.toBeChecked();
		await expect(
			page.getByRole('radiogroup', { name: 'How long a link shows the recipe' })
		).toHaveCount(0);

		await toggle.click();
		await expect(page.getByText('Link previews updated')).toBeVisible();
		await expect(page.getByRole('radio', { name: '15 min' })).toBeChecked();

		await page.getByRole('radio', { name: '1 hour' }).click();
		await expect(page.getByRole('radio', { name: '1 hour' })).toBeChecked();
		await page.reload();
		await expect(page.getByRole('radio', { name: '1 hour' })).toBeChecked();
	} finally {
		await setLinkPreviews(page, false);
	}
});

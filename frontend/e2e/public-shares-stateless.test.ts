import { expect, test } from '@playwright/test';
import { createRecipe, loadFixture, login, uniqueToken } from './helpers';

// The public-sharing tests that never flip the instance-wide switch: they
// read a page no share backs, or answer the settings themselves, so they run
// beside the rest and on a phone too. public-shares.test.ts holds the others.

test('an unknown token', async ({ page }) => {
	await page.goto(`/s/${uniqueToken()}`);
	await expect(page.getByText('This link is no longer available.')).toBeVisible();
	// Nothing was shared, so there is nothing to credit.
	await expect(page.getByRole('contentinfo')).toHaveCount(0);
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

// The tabs share one height on a phone: whatever the Link tab holds, the
// switch above and the buttons below never move on a switch, nor on a pick
// of the audience. Answered, not set: public sharing is instance-wide and
// this runs beside the rest.
test('the switch and the buttons stay put across the tabs', async ({ page }, testInfo) => {
	test.skip(!testInfo.project.use.isMobile, 'the phone sheet keeps one height');
	await page.emulateMedia({ reducedMotion: 'reduce' });
	// Short enough that the public rows cost the preview a fit step, and
	// narrow enough that a detail one line taller than the other shows.
	await page.setViewportSize({ width: 300, height: 640 });
	// A share sheet for files and links, so both tabs lead with Share.
	await page.addInitScript(() => {
		Object.defineProperty(Navigator.prototype, 'canShare', { value: () => true });
		Object.defineProperty(Navigator.prototype, 'share', { value: () => Promise.resolve() });
	});
	let publicShares = false;
	let share: object | null = null;
	await page.route('**/api/v1/settings', async (route) => {
		const response = await route.fetch();
		await route.fulfill({ response, json: { ...(await response.json()), publicShares } });
	});
	await page.route('**/api/v1/shares/by-recipe/*', (route) => route.fulfill({ json: { share } }));
	await login(page);
	const title = `Steady ${uniqueToken()}`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title });
	const now = Date.now();
	const active = {
		id: 'steady',
		recipe: { id: recipe.id, slug: recipe.slug, title },
		path: '/s/steadytoken',
		createdAt: new Date(now).toISOString(),
		startsAt: new Date(now).toISOString(),
		expiresAt: new Date(now + 7 * 86_400_000).toISOString(),
		status: 'active'
	};

	for (const [name, on, link] of [
		['household only', false, null],
		['with the choice', true, null],
		['with a public link', true, active]
	] as const) {
		publicShares = on;
		share = link;
		await page.goto(`/recipes/${recipe.slug}`);
		await page.getByRole('button', { name: 'Share', exact: true }).click();
		const sheet = page.getByRole('dialog', { name: 'Pass on recipe' });
		const where = async (button: string) => {
			await expect(sheet.getByRole('button', { name: button, exact: true })).toBeVisible();
			const tabs = await sheet.getByRole('radiogroup', { name: 'Pass on as' }).boundingBox();
			const primary = await sheet.getByRole('button', { name: button, exact: true }).boundingBox();
			return { tabs: Math.round(tabs!.y), buttons: Math.round(primary!.y) };
		};
		await sheet.getByRole('radio', { name: 'Image', exact: true }).click();
		const image = await where('Share image');
		await sheet.getByRole('radio', { name: 'Link', exact: true }).click();
		if (link) {
			await expect(sheet.getByRole('radio', { name: 'For everyone with the link' })).toBeChecked();
		}
		expect(await where('Share link'), name).toEqual(image);
		if (on) {
			// Each row's detail is as tall as the other's, so the sheet keeps its
			// height and both audiences show the ticket at one step.
			const ticket = sheet.locator('[data-ticket]');
			const size = async () => {
				const box = (await ticket.boundingBox())!;
				return `${Math.round(box.width)}x${Math.round(box.height)}`;
			};
			const before = await size();
			const other = link ? 'For the household' : 'For everyone with the link';
			await sheet.getByRole('radio', { name: other }).click();
			await expect(sheet.getByRole('radio', { name: other })).toBeChecked();
			const primary = !link ? 'Create public link' : 'Share link';
			await expect
				.poll(async () => JSON.stringify(await where(primary)), { message: `${name}, ${other}` })
				.toBe(JSON.stringify(image));
			await expect.poll(size, { message: `${name}, ${other}` }).toBe(before);
		}
		await page.keyboard.press('Escape');
		await expect(sheet).toBeHidden();
	}
});

import { expect, test, type Page } from '@playwright/test';
import { createRecipe, createUser, loadFixture, login, uniqueToken } from './helpers';

test.skip(({ isMobile }) => isMobile, 'the top bar is desktop-only');

const places = (page: Page) => page.getByRole('banner').getByRole('navigation');

test('the places mark where you are, and give way on a recipe', async ({ page }) => {
	await login(page);
	await expect(places(page).getByRole('link', { name: 'Recipes' })).toHaveAttribute(
		'aria-current',
		'page'
	);
	await places(page).getByRole('link', { name: 'Shopping lists' }).click();
	await expect(page).toHaveURL(/\/lists$/);
	await expect(places(page).getByRole('link', { name: 'Shopping lists' })).toHaveAttribute(
		'aria-current',
		'page'
	);
	await expect(places(page).getByRole('link', { name: 'Recipes' })).not.toHaveAttribute(
		'aria-current'
	);

	const recipe = await createRecipe(page, loadFixture(0));
	await page.goto(`/recipes/${recipe.slug}`);
	await expect(page.getByRole('banner').getByRole('link', { name: 'Edit' })).toBeVisible();
	await expect(places(page)).toHaveCount(0);
});

// The places sit in the exact middle, nothing in the bar overlaps, and
// "New recipe" or "New list" is a circle only where its label does not fit - in both
// languages, whose labels differ in length.
for (const locale of ['en', 'de'] as const) {
	test(`the places stay centered and "New recipe" or "New list" fits at every width (${locale})`, async ({
		page
	}) => {
		// An account of that language: the login sets the interface from it.
		const username = `bar${locale}${uniqueToken()}`;
		await login(page);
		await createUser(page, { username, role: 'user', locale });
		await page.context().clearCookies();
		await login(page, username);
		await expect(places(page)).toBeVisible();
		const seen = new Set<boolean>();
		const failures: string[] = [];
		// The recipes, then the lists reached through the bar, whose "New
		// list" has a label of its own to measure.
		for (const place of ['recipes', 'lists'] as const) {
			if (place === 'lists') {
				await page.setViewportSize({ width: 1440, height: 700 });
				await places(page)
					.getByRole('link', { name: locale === 'de' ? 'Einkaufslisten' : 'Shopping lists' })
					.click();
				await expect(page).toHaveURL(/\/lists$/);
			}
			for (let width = 768; width <= 1440; width += 24) {
				await page.setViewportSize({ width, height: 700 });
				// Two frames: the resize, then the measurement it triggers.
				await page.evaluate(
					() => new Promise((done) => requestAnimationFrame(() => requestAnimationFrame(done)))
				);
				const bar = await page.evaluate(() => {
					const header = document.querySelector('header')!.getBoundingClientRect();
					const nav = document.querySelector('header nav')!.getBoundingClientRect();
					const row = document.querySelector('header > :last-child > div')!.getBoundingClientRect();
					const newRecipe = document.querySelector(
						'header a[href="/recipes/new"], header [data-new]'
					)!;
					return {
						offCenter: Math.abs(nav.left + nav.width / 2 - (header.left + header.width / 2)),
						gap: row.left - nav.right,
						overflow: row.right - (header.right - 32),
						labeled: (newRecipe.textContent ?? '').trim().length > 0,
						tall: newRecipe.getBoundingClientRect().height > 40
					};
				});
				seen.add(bar.labeled);
				if (bar.offCenter > 1 || bar.gap < 16 || bar.overflow > 1 || bar.tall) {
					failures.push(`${place} ${width}px: ${JSON.stringify(bar)}`);
				}
			}
		}
		expect(failures).toEqual([]);
		// Both shapes occur across the range, so the check saw each.
		expect([...seen].sort()).toEqual([false, true]);
		await expect(
			page
				.getByRole('banner')
				.getByRole('button', { name: locale === 'de' ? 'Neue Liste' : 'New list' })
		).toBeVisible();
	});
}

test('the own avatar wears the signed-in person’s color, ringed', async ({ page }) => {
	const name = `bar${uniqueToken()}`.toLowerCase();
	await login(page);
	await createUser(page, { username: name, role: 'user' });
	await login(page, name);
	const me = (await (await page.request.get('/api/v1/auth/me')).json()) as { color: string };
	const mark = page.getByRole('button', { name: 'Account menu' }).locator('span').first();
	const look = await mark.evaluate((el, color) => {
		const resolve = (token: string) => {
			const probe = document.createElement('span');
			probe.style.color = `var(${token})`;
			document.body.append(probe);
			const value = getComputedStyle(probe).color;
			probe.remove();
			return value;
		};
		const cs = getComputedStyle(el);
		return {
			fill: cs.backgroundColor === resolve(`--color-user-${color}`),
			ring:
				cs.outlineStyle === 'solid' &&
				cs.outlineColor === resolve(`--color-user-${color}-foreground`),
			gap: cs.outlineOffset
		};
	}, me.color);
	expect(look).toEqual({ fill: true, ring: true, gap: '2px' });
});

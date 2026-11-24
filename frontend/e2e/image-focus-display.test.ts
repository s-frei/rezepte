import { expect, test, type Page } from '@playwright/test';
import {
	createRecipe,
	createUser,
	loadFixture,
	login,
	search,
	tinyPng,
	uniqueToken,
	uploadImage
} from './helpers';

/** The visible picture of image `id`, wherever the page shows it. */
const photo = (page: Page, id: string) => page.locator(`img[src*="/${id}/"]:visible`).first();

/** The `object-position` of the visible picture of image `id`. */
const position = (page: Page, id: string) =>
	photo(page, id).evaluate((el) => getComputedStyle(el).objectPosition);

/** The crop is pushed to the photo's bottom edge: the focus at y 0.9 of a tall photo. */
async function expectBottomCrop(page: Page, id: string) {
	const img = photo(page, id);
	await expect(img).toBeVisible();
	await expect
		.poll(() => img.evaluate((el) => getComputedStyle(el).objectPosition))
		.toMatch(/^50% 100%$/);
}

test('every place keeps the focus in view', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name !== 'desktop', 'sets both widths itself');
	const username = `focus${uniqueToken()}`.slice(0, 20);
	await login(page);
	await createUser(page, { username, role: 'user' });
	await page.context().clearCookies();
	await login(page, username);

	const title = `Tall ${uniqueToken()}`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title });
	const image = await uploadImage(page, recipe.id, tinyPng([90, 140, 60], 90, 160));
	const focused = await page.request.patch(`/api/v1/recipes/${recipe.id}/images/${image.id}`, {
		data: { focus: { x: 0.5, y: 0.9 } }
	});
	expect(focused.ok(), await focused.text()).toBe(true);

	for (const width of [1280, 360]) {
		await page.setViewportSize({ width, height: 800 });
		await page.goto('/');
		await search(page, title);
		await expectBottomCrop(page, image.id);

		// The desktop cover at 1280, the phone carousel at 360.
		await page.goto(`/recipes/${recipe.slug}`);
		await expectBottomCrop(page, image.id);
	}

	// A meal on today's plan.
	const plans = (await (await page.request.get('/api/v1/meal-plans')).json()) as {
		plans: { id: string; personal: boolean }[];
	};
	const planId = plans.plans.find((p) => p.personal)!.id;
	const now = new Date();
	const today = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`;
	const planned = await page.request.post(`/api/v1/meal-plans/${planId}/entries`, {
		data: { day: today, recipeId: recipe.id }
	});
	expect(planned.ok(), await planned.text()).toBe(true);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto(`/plans/${planId}`);
	await expectBottomCrop(page, image.id);

	// The recipe on a shopping list.
	const list = await page.request.post('/api/v1/shopping-lists', { data: { name: title } });
	expect(list.ok(), await list.text()).toBe(true);
	const listId = ((await list.json()) as { id: string }).id;
	const added = await page.request.post(`/api/v1/shopping-lists/${listId}/recipes`, {
		data: { recipeId: recipe.id, servings: 2, ingredients: [{ group: 0, index: 0 }] }
	});
	expect(added.ok(), await added.text()).toBe(true);
	await page.goto(`/lists/${listId}`);
	await expectBottomCrop(page, image.id);
});

test('each photo keeps its own focus, not the cover focus', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name !== 'desktop', 'sets both widths itself');
	await login(page);
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Two ${uniqueToken()}` });
	const cover = await uploadImage(page, recipe.id, tinyPng([200, 120, 40], 90, 160));
	const second = await uploadImage(page, recipe.id, tinyPng([40, 120, 200], 90, 160));
	const focused = await page.request.patch(`/api/v1/recipes/${recipe.id}/images/${second.id}`, {
		data: { focus: { x: 0.5, y: 0.9 } }
	});
	expect(focused.ok(), await focused.text()).toBe(true);

	// Desktop: the cover and its thumbnail stay centered, the second thumbnail follows its point.
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto(`/recipes/${recipe.slug}`);
	await expect.poll(() => position(page, second.id)).toBe('50% 100%');
	await expect.poll(() => position(page, cover.id)).toBe('50% 50%');

	// Phone: the carousel's second photo, off screen until swiped to, has its own point too.
	await page.setViewportSize({ width: 360, height: 800 });
	await page.goto(`/recipes/${recipe.slug}`);
	await expect.poll(() => position(page, second.id)).toBe('50% 100%');
	await expect.poll(() => position(page, cover.id)).toBe('50% 50%');
});

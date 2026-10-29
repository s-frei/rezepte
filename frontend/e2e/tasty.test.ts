import { expect, test, type Page, type TestInfo } from '@playwright/test';
import {
	createRecipe,
	createUser,
	loadFixture,
	login,
	signOut,
	tinyPng,
	uniqueToken,
	uploadImage
} from './helpers';

const cards = (page: Page) => page.getByRole('main').getByRole('heading', { level: 3 });

/**
 * Signs whoever is in out and `username` in, from the overview: the recipe
 * page hides the phone's bottom nav, which holds the sign-out.
 */
async function switchTo(page: Page, testInfo: TestInfo, username: string) {
	await page.goto('/');
	await signOut(page, testInfo);
	await login(page, username);
	await expect(page).toHaveURL('/');
}

/** Marks a recipe tasty through the API, as whoever `page` is signed in as. */
async function markTasty(page: Page, recipeId: string) {
	const origin = new URL(page.url()).origin;
	const response = await page.request.put(`/api/v1/recipes/${recipeId}/tasty`, {
		headers: { Origin: origin }
	});
	expect(response.status(), await response.text()).toBe(204);
}

test('a member marks somebody else’s recipe tasty and everyone sees it', async ({
	page,
	isMobile
}, testInfo) => {
	const token = uniqueToken();
	const mara = `mara${token}`.slice(0, 20);

	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username: mara, role: 'user' });
	const recipe = await createRecipe(page, { ...loadFixture(1), title: `Tasty ${token}` });

	// The author has nothing to press, on the card or on the recipe page.
	await page.goto(`/?q=${token}`);
	await expect(cards(page)).toHaveText([`Tasty ${token}`]);
	await expect(page.getByRole('button', { name: 'Tasty', exact: true })).toHaveCount(0);
	await page.goto(`/recipes/${recipe.slug}`);
	await expect(
		page.getByRole('button', { name: 'Favorite', exact: true, pressed: false })
	).toBeVisible();
	await expect(page.getByRole('button', { name: 'Tasty', exact: true })).toHaveCount(0);

	await switchTo(page, testInfo, mara);
	await page.goto(`/recipes/${recipe.slug}`);
	await page.getByRole('button', { name: 'Tasty', exact: true, pressed: false }).click();
	await expect(
		page.getByRole('button', { name: 'Tasty', exact: true, pressed: true })
	).toBeVisible();
	await expect(page.getByRole('button', { name: `Tasty for: ${mara}` })).toBeVisible();

	await page.reload();
	await expect(
		page.getByRole('button', { name: 'Tasty', exact: true, pressed: true })
	).toBeVisible();

	await page.goto(`/?q=${token}`);
	await expect(cards(page)).toHaveText([`Tasty ${token}`]);
	if (isMobile) {
		// A phone card shows the count and nothing to press.
		await expect(page.getByRole('button', { name: /tasty/i })).toHaveCount(0);
		await expect(
			page.getByRole('main').getByText('1 person finds this tasty').filter({ visible: true })
		).toHaveCount(1);
	} else {
		const heart = page.getByRole('button', { name: 'Tasty', exact: true, pressed: true });
		await heart.click();
		await expect(
			page.getByRole('button', { name: 'Tasty', exact: true, pressed: false })
		).toBeVisible();
		// The heart sits over the card's link and must not open the recipe.
		await expect(page).toHaveURL(`/?q=${token}`);
	}

	// The author sees the count too.
	await switchTo(page, testInfo, 'admin');
	await page.goto(`/recipes/${recipe.slug}`);
	await expect(page.getByRole('button', { name: 'Tasty', exact: true })).toHaveCount(0);
	if (isMobile) {
		await expect(page.getByRole('button', { name: `Tasty for: ${mara}` })).toBeVisible();
	} else {
		await expect(page.getByRole('button', { name: /^Tasty for/ })).toHaveCount(0);
	}
});

test('a tap on the count of a phone card opens the recipe', async ({ page, browser, isMobile }) => {
	test.skip(!isMobile, 'the desktop card has a button there');
	const token = uniqueToken();
	const jo = `jo${token}`.slice(0, 20);

	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username: jo, role: 'user' });
	const recipe = await createRecipe(page, { ...loadFixture(2), title: `Count ${token}` });

	// Marked through the API as jo, from a context of its own, so this page
	// stays signed in as the author.
	const other = await browser.newContext();
	const joPage = await other.newPage();
	await login(joPage, jo);
	await expect(joPage).toHaveURL('/');
	await markTasty(joPage, recipe.id);
	await other.close();

	await page.goto(`/?q=${token}`);
	await page
		.getByRole('main')
		.getByText('1 person finds this tasty')
		.filter({ visible: true })
		.click({ force: true });
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
});

test('sorts the overview by the most tasty marks', async ({ page, browser, isMobile }) => {
	const token = uniqueToken();
	const users = [`ann${token}`.slice(0, 20), `bo${token}`.slice(0, 20)];

	await login(page);
	await expect(page).toHaveURL('/');
	for (const username of users) {
		await createUser(page, { username, role: 'user' });
	}
	const once = await createRecipe(page, { ...loadFixture(1), title: `Once ${token}` });
	const twice = await createRecipe(page, { ...loadFixture(2), title: `Twice ${token}` });
	await createRecipe(page, { ...loadFixture(3), title: `Never ${token}` });

	for (const [index, username] of users.entries()) {
		const context = await browser.newContext();
		const other = await context.newPage();
		await login(other, username);
		await expect(other).toHaveURL('/');
		await markTasty(other, twice.id);
		if (index === 0) {
			await markTasty(other, once.id);
		}
		await context.close();
	}

	await page.goto(`/?q=${token}`);
	if (isMobile) {
		await page.getByRole('button', { name: 'Filters' }).click();
	}
	await page.getByRole('button', { name: 'Sort by' }).click();
	await page.getByRole('option', { name: 'Tastiest' }).click();

	await expect(page).toHaveURL(/sort=tasty/);
	await expect(cards(page)).toHaveText([`Twice ${token}`, `Once ${token}`, `Never ${token}`]);
});

// The count on a photo sits in the photo's bottom-left corner and shares its
// center (RecipeCard): 24px in the phone photo's 18px corner, 28px in the
// desktop photo's 20px one, each 6px in - at every width, also on both sides
// of md, where the corner and the count change together. A text card's count
// sits beside the initials and takes their size (see "a text card shows the
// tasty count …").
// The count shares a corner's center: on a phone the card's bottom-left
// one, across from the initials, from md up the photo's bottom-left one.
test('the count on a photo card shares the center of its corner', async ({
	page,
	browser
}, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'the sweep covers the phone widths itself');
	const token = uniqueToken();
	const lu = `lu${token}`.slice(0, 20);

	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username: lu, role: 'user' });
	const recipe = await createRecipe(page, { ...loadFixture(3), title: `Grow ${token}` });
	await uploadImage(page, recipe.id, tinyPng([200, 120, 40], 400, 300));
	const other = await browser.newContext();
	const luPage = await other.newPage();
	await login(luPage, lu);
	await expect(luPage).toHaveURL('/');
	await markTasty(luPage, recipe.id);
	await other.close();

	await page.goto(`/?q=${token}`);
	for (const width of [320, 360, 412, 480, 600, 700, 767, 768, 900, 1023, 1024, 1280]) {
		await page.setViewportSize({ width, height: 900 });
		const label = page
			.getByRole('main')
			.getByText('1 person finds this tasty')
			.filter({ visible: true });
		await expect(label).toHaveCount(1);
		const { pill, radius, x, y } = await label.evaluate((el, phone) => {
			const box = el.parentElement!.getBoundingClientRect();
			const card = el.closest('article')!;
			const corner = phone ? card : card.querySelector('img[src*="/images/"]')!;
			const frame = corner.getBoundingClientRect();
			return {
				pill: box.height,
				radius: parseFloat(getComputedStyle(corner).borderBottomLeftRadius),
				// The center of the pill's round left end, from the corner.
				x: box.left + box.height / 2 - frame.left,
				y: frame.bottom - (box.top + box.height / 2)
			};
		}, width < 768);
		expect(pill, `pill at ${width}px`).toBe(28);
		expect(Math.abs(x - radius), `x at ${width}px`).toBeLessThanOrEqual(0.5);
		expect(Math.abs(y - radius), `y at ${width}px`).toBeLessThanOrEqual(0.5);
	}
});

// On a phone the recipe page is where both marks are set, so the star and the
// heart are thumb-sized there: 44px, the phone floor for an action.
test('the star and the heart on the recipe page are thumb-sized on a phone', async ({
	page,
	isMobile
}, testInfo) => {
	test.skip(!isMobile, 'the desktop keeps its 36px pair');
	const token = uniqueToken();
	const kai = `kai${token}`.slice(0, 20);

	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username: kai, role: 'user' });
	const recipe = await createRecipe(page, { ...loadFixture(4), title: `Thumb ${token}` });
	await switchTo(page, testInfo, kai);
	await page.goto(`/recipes/${recipe.slug}`);

	for (const name of ['Favorite', 'Tasty']) {
		const box = await page.getByRole('button', { name, exact: true }).boundingBox();
		expect(box!.height, name).toBeGreaterThanOrEqual(44);
		expect(box!.width, name).toBeGreaterThanOrEqual(44);
	}
});

test('a text card shows the tasty count before the initials', async ({
	page,
	browser,
	isMobile
}) => {
	const token = uniqueToken();
	const lu = `lu${token}`.slice(0, 20);

	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username: lu, role: 'user' });
	const recipe = await createRecipe(page, { ...loadFixture(2), title: `Plain ${token}` });

	const other = await browser.newContext();
	const luPage = await other.newPage();
	await login(luPage, lu);
	await expect(luPage).toHaveURL('/');
	await markTasty(luPage, recipe.id);
	await other.close();

	await page.goto(`/?q=${token}`);
	const card = page.getByRole('article', { name: `Plain ${token}` });
	// The author's own recipe: the count alone, on both widths.
	const pill = card.getByTitle('1 person finds this tasty').filter({ visible: true });
	const initials = card.getByRole('button', { name: /^Added by admin/ });
	const [pillBox, initialsBox] = [await pill.boundingBox(), await initials.boundingBox()];
	expect(pillBox!.x + pillBox!.width).toBeLessThanOrEqual(initialsBox!.x);
	const center = (box: { y: number; height: number }) => box.y + box.height / 2;
	expect(Math.abs(center(pillBox!) - center(initialsBox!))).toBeLessThan(2);
	// As tall as the initials with their 2px ring: 24 + 4 on a phone, 28 + 4 from md.
	expect(pillBox!.height).toBe(isMobile ? 28 : 32);
});

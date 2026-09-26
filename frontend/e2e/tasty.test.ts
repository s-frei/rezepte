import { expect, test, type Page, type TestInfo } from '@playwright/test';
import { createRecipe, createUser, loadFixture, login, signOut, uniqueToken } from './helpers';

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
	await expect(page.getByRole('button', { name: 'Mark as tasty' })).toHaveCount(0);
	await page.goto(`/recipes/${recipe.slug}`);
	await expect(page.getByRole('button', { name: 'Add to favorites' })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Mark as tasty' })).toHaveCount(0);

	await switchTo(page, testInfo, mara);
	await page.goto(`/recipes/${recipe.slug}`);
	await page.getByRole('button', { name: 'Mark as tasty' }).click();
	await expect(page.getByRole('button', { name: 'Take back tasty' })).toBeVisible();
	await expect(page.getByRole('button', { name: `Tasty for: ${mara}` })).toBeVisible();

	await page.reload();
	await expect(page.getByRole('button', { name: 'Take back tasty' })).toBeVisible();

	await page.goto(`/?q=${token}`);
	await expect(cards(page)).toHaveText([`Tasty ${token}`]);
	if (isMobile) {
		// A phone card shows the count and nothing to press.
		await expect(page.getByRole('button', { name: /tasty/i })).toHaveCount(0);
		await expect(
			page.getByRole('main').getByText('1 person finds this tasty').filter({ visible: true })
		).toHaveCount(1);
	} else {
		const heart = page.getByRole('button', { name: 'Take back tasty' });
		await heart.click();
		await expect(page.getByRole('button', { name: 'Mark as tasty' })).toBeVisible();
		// The heart sits over the card's link and must not open the recipe.
		await expect(page).toHaveURL(`/?q=${token}`);
	}

	// The author sees the count too.
	await switchTo(page, testInfo, 'admin');
	await page.goto(`/recipes/${recipe.slug}`);
	await expect(page.getByRole('button', { name: 'Mark as tasty' })).toHaveCount(0);
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

// The count on a card is a display, so its size follows the card rather than
// the layout: it grows with the card up to the 36px of the desktop button and
// never shrinks while the card grows. Sized by the viewport instead, it went
// from 24px on a 357px phone card to 36px on the 220px card one pixel wider.
test('the count on a card grows with the card, never against it', async ({
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
	const other = await browser.newContext();
	const luPage = await other.newPage();
	await login(luPage, lu);
	await expect(luPage).toHaveURL('/');
	await markTasty(luPage, recipe.id);
	await other.close();

	await page.goto(`/?q=${token}`);
	const samples: { width: number; card: number; pill: number }[] = [];
	for (const width of [320, 360, 412, 480, 600, 700, 767, 768, 900, 1023, 1024, 1280]) {
		await page.setViewportSize({ width, height: 900 });
		const label = page
			.getByRole('main')
			.getByText('1 person finds this tasty')
			.filter({ visible: true });
		await expect(label).toHaveCount(1);
		samples.push({
			width,
			...(await label.evaluate((el) => ({
				card: el.closest('article')!.getBoundingClientRect().width,
				pill: el.parentElement!.getBoundingClientRect().height
			})))
		});
	}

	samples.sort((a, b) => a.card - b.card);
	for (const [index, sample] of samples.entries()) {
		expect(sample.pill, `pill at ${sample.width}px`).toBeGreaterThanOrEqual(24);
		expect(sample.pill, `pill at ${sample.width}px`).toBeLessThanOrEqual(36);
		const smaller = samples[index - 1];
		if (smaller) {
			expect(
				sample.pill,
				`pill on the ${Math.round(sample.card)}px card (${sample.width}px) against the ${Math.round(smaller.card)}px card (${smaller.width}px)`
			).toBeGreaterThanOrEqual(smaller.pill - 0.5);
		}
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

	for (const name of ['Add to favorites', 'Mark as tasty']) {
		const box = await page.getByRole('button', { name }).boundingBox();
		expect(box!.height, name).toBeGreaterThanOrEqual(44);
		expect(box!.width, name).toBeGreaterThanOrEqual(44);
	}
});

import { expect, test, type Page } from '@playwright/test';
import { createRecipe, loadFixture, login, uniqueToken } from './helpers';

// The phone's bottom nav: one bar of icons - Recipes, Search, You - with New
// at its end, a "You" sheet, and the command palette as its search. It
// floats at the top of a page and docks while reading. The desktop has none
// of it - the top bar carries the same ways out.
test.beforeEach(({ isMobile }) => {
	test.skip(!isMobile, 'the bottom nav is the phone layout only');
});

function bottomNav(page: Page) {
	return page.getByRole('navigation', { name: 'Main' });
}

/** The bar around the nav, which also holds New and carries `data-docked`. */
function bar(page: Page) {
	return bottomNav(page).locator('xpath=..');
}

/** A page of known height, whatever the shared database holds. */
async function makeTall(page: Page) {
	await page.evaluate(() => {
		const spacer = document.createElement('div');
		spacer.style.height = '4000px';
		document.querySelector('main')?.append(spacer);
	});
}

test('the bar shows icons only and marks the current place', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	const nav = bottomNav(page);
	// No word in the bar: nothing a longer language could squeeze.
	await expect(nav).toHaveText('');
	await expect(nav.getByRole('link', { name: 'Recipes' })).toHaveAttribute('aria-current', 'page');

	await page.goto('/settings');
	// You opens a sheet rather than being the page, so it is current, not the page.
	await expect(nav.getByRole('button', { name: 'You' })).toHaveAttribute('aria-current', 'true');
	await expect(nav.getByRole('link', { name: 'Recipes' })).not.toHaveAttribute('aria-current');
});

test('the bar stays whole while reading', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	// The address changes before the sign-in's transition swaps the page in,
	// so the login's <main> is still there for a moment.
	await expect(page.getByRole('heading', { name: 'What are we cooking today?' })).toBeVisible();
	await makeTall(page);
	await expect(bar(page)).not.toHaveAttribute('data-docked');

	await page.mouse.wheel(0, 800);
	await expect(bar(page)).toHaveAttribute('data-docked', '');
	const nav = bottomNav(page);
	await expect(nav.getByRole('link', { name: 'Recipes' })).toBeVisible();
	await expect(nav.getByRole('button', { name: 'Search' })).toBeVisible();
	await expect(nav.getByRole('button', { name: 'You' })).toBeVisible();
	await expect(page.getByRole('button', { name: 'New', exact: true })).toBeVisible();

	await page.mouse.wheel(0, -800);
	await expect(bar(page)).not.toHaveAttribute('data-docked');
});

test('the pill follows an open sheet and returns', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	const nav = bottomNav(page);
	await nav.getByRole('button', { name: 'Search' }).click();
	await expect(page.getByRole('dialog', { name: 'Command palette' })).toBeVisible();
	await expect(nav.getByRole('button', { name: 'Search' })).toHaveAttribute('data-current', '');

	await page.keyboard.press('Escape');
	await expect(page.getByRole('dialog', { name: 'Command palette' })).toHaveCount(0);
	await expect(nav.getByRole('link', { name: 'Recipes' })).toHaveAttribute('data-current', '');
	await expect(nav.getByRole('button', { name: 'Search' })).not.toHaveAttribute('data-current');
});

test('the bar steps aside while a field has focus', async ({ page }) => {
	await login(page);
	await page.goto('/settings');
	const field = page.locator('main input[type=password]').first();
	// The software keyboard comes up with the field; the bar would cover it.
	await field.focus();
	await expect(bottomNav(page)).toBeHidden();
	await field.blur();
	await expect(bottomNav(page)).toBeVisible();
});

test('a page restored below the fold starts docked', async ({ page }) => {
	await login(page);
	await page.goto('/settings');
	// The page has to be long enough to scroll before the scroll can land.
	await expect(page.locator('main input[type=password]').first()).toBeVisible();
	await page.evaluate(() => window.scrollTo(0, 900));
	await expect.poll(() => page.evaluate(() => window.scrollY)).toBeGreaterThan(120);
	await expect(bar(page)).toHaveAttribute('data-docked', '');

	await bottomNav(page).getByRole('button', { name: 'You' }).click();
	await page.getByRole('dialog').getByRole('link', { name: 'People' }).click();
	await expect(page).toHaveURL('/settings/users');
	// Every move of the bar on the way back is counted; checking afterwards
	// would miss a slide that is over within its 300ms.
	await bar(page).evaluate((el) => {
		(window as unknown as { slides: number }).slides = 0;
		el.addEventListener(
			'transitionstart',
			() => (window as unknown as { slides: number }).slides++
		);
	});
	await page.goBack();
	await expect(page).toHaveURL('/settings');
	await expect.poll(() => page.evaluate(() => window.scrollY)).toBeGreaterThan(120);
	await expect(bar(page)).toHaveAttribute('data-docked', '');
	// Docked from the first frame: no slide down from the floating spot.
	await page.waitForTimeout(400);
	expect(await page.evaluate(() => (window as unknown as { slides: number }).slides)).toBe(0);
});

test('Recipes scrolls to the top before it clears filters', async ({ page }) => {
	const token = uniqueToken();
	await login(page);
	await expect(page).toHaveURL('/');
	await createRecipe(page, { ...loadFixture(0), title: `Filter ${token}` });
	await page.goto(`/?q=${token}`);
	// The list first: a scroll before it arrives is undone when it does.
	await expect(page.getByRole('heading', { name: `Filter ${token}` })).toBeVisible();
	await makeTall(page);
	await page.mouse.wheel(0, 800);
	await expect.poll(() => page.evaluate(() => window.scrollY)).toBeGreaterThan(120);

	const recipes = bottomNav(page).getByRole('link', { name: 'Recipes' });
	// A tap while reading goes back up and keeps what was searched for.
	await recipes.click();
	await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0);
	await expect(page).toHaveURL(`/?q=${token}`);
	// At the top, the next tap starts over.
	await recipes.click();
	await expect(page).toHaveURL('/');
});

test('You shows who is signed in and reaches the settings pages', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');

	await bottomNav(page).getByRole('button', { name: 'You' }).click();
	// The `admin` the e2e task seeds is the owner, and its display name is
	// its login name.
	const sheet = page.getByRole('dialog', { name: 'admin' });
	await expect(sheet).toBeVisible();
	await expect(sheet.getByText('Owner, signed in as admin')).toBeVisible();
	await expect(sheet.getByRole('link')).toHaveText([
		'Profile',
		'Notifications',
		'Shared links',
		'API',
		'People',
		'Email',
		'Sign-in',
		'Import & export'
	]);
	// The household's pages stand apart from the account's own.
	await expect(sheet.getByRole('group', { name: 'Household' }).getByRole('link')).toHaveText([
		'People',
		'Email',
		'Sign-in',
		'Import & export'
	]);

	await sheet.getByRole('link', { name: 'People' }).click();
	await expect(page).toHaveURL('/settings/users');
	await expect(sheet).toHaveCount(0);
});

test('Search opens the command palette, which hands a query to the overview', async ({ page }) => {
	const token = uniqueToken();
	await login(page);
	await expect(page).toHaveURL('/');
	await createRecipe(page, { ...loadFixture(0), title: `Palette ${token}` });
	await page.goto('/settings');

	const search = bottomNav(page).getByRole('button', { name: 'Search' });
	await expect(search).toHaveAttribute('aria-expanded', 'false');
	await search.click();
	const palette = page.getByRole('dialog', { name: 'Command palette' });
	await expect(palette).toBeVisible();
	await expect(search).toHaveAttribute('aria-expanded', 'true');
	// The field takes focus, so the software keyboard comes up with it.
	await expect(palette.getByRole('combobox')).toBeFocused();
	// No "all results" without a query: there would be nothing to hand over.
	await expect(palette.getByRole('option', { name: /^All results/ })).toHaveCount(0);
	// Nor with a query nothing matches: the overview it leads to would be empty.
	await palette.getByRole('combobox').fill(`nothing${token}`);
	await expect(palette.getByText('No recipes found')).toBeVisible();
	await expect(palette.getByRole('option', { name: /^All results/ })).toHaveCount(0);

	await palette.getByRole('combobox').fill(token);
	await expect(palette.getByRole('option', { name: `Palette ${token}` })).toBeVisible();
	await palette.getByRole('option', { name: `All results for “${token}”` }).click();

	await expect(page).toHaveURL(`/?q=${token}`);
	await expect(palette).toHaveCount(0);
	await expect(page.getByRole('combobox', { name: 'Search recipes' })).toHaveValue(token);
});

test('the palette sheet ends on the keyboard, not above or below it', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await bottomNav(page).getByRole('button', { name: 'Search' }).click();
	const palette = page.getByRole('dialog', { name: 'Command palette' });
	await expect(palette).toBeVisible();

	// Headless Chromium has no software keyboard, so one is faked the way a
	// phone reports it: a visual viewport 300px shorter, and its resize event.
	const keyboardTop = await page.evaluate(() => {
		const viewport = window.visualViewport!;
		const top = viewport.height - 300;
		Object.defineProperty(viewport, 'height', { configurable: true, get: () => top });
		viewport.dispatchEvent(new Event('resize'));
		return viewport.offsetTop + top;
	});
	await expect
		.poll(async () => {
			const box = await palette.boundingBox();
			return box && Math.round(box.y + box.height);
		})
		.toBe(Math.round(keyboardTop));
});

test('You switches the theme without leaving the sheet', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');

	await bottomNav(page).getByRole('button', { name: 'You' }).click();
	const sheet = page.getByRole('dialog', { name: 'admin' });
	const theme = sheet.getByRole('radiogroup', { name: 'Color scheme' });
	await theme.getByRole('radio', { name: 'Dark' }).click();
	await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
	await expect(sheet).toBeVisible();

	await theme.getByRole('radio', { name: 'System' }).click();
	await expect(page.locator('html')).not.toHaveAttribute('data-theme');
});

test('no bar on a page that does not exist', async ({ page }) => {
	await login(page);
	await page.goto('/gibt-es-nicht');
	// Nothing would be current there, so the bar stays away and the page
	// offers its own way back.
	await expect(bottomNav(page)).toHaveCount(0);
	await page.getByRole('link', { name: 'Back to recipes' }).click();
	await expect(page).toHaveURL('/');
	await expect(bottomNav(page)).toBeVisible();
});

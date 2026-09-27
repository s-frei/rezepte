import { expect, test, type Page } from '@playwright/test';
import { createRecipe, loadFixture, login, uniqueToken } from './helpers';

// The phone's bottom nav: a compact bar with Recipes, Search and You, a
// separate New button, a "You" sheet, and the command palette as its search.
// The desktop has none of it - the top bar carries the same ways out.
test.beforeEach(({ isMobile }) => {
	test.skip(!isMobile, 'the bottom nav is the phone layout only');
});

function bottomNav(page: Page) {
	return page.getByRole('navigation', { name: 'Main' });
}

test('the current place carries its name, the others only their icon', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	const nav = bottomNav(page);
	// The name is printed beside the current icon only - where you are is
	// spelled out, and the other two keep the bar small.
	await expect(nav.getByText('Recipes', { exact: true })).toBeVisible();
	await expect(nav.getByText('Search', { exact: true })).toHaveCount(0);
	await expect(nav.getByText('You', { exact: true })).toHaveCount(0);

	await page.goto('/settings');
	await expect(nav.getByText('You', { exact: true })).toBeVisible();
	await expect(nav.getByText('Recipes', { exact: true })).toHaveCount(0);
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
	await expect(sheet.getByRole('link')).toHaveText(['Profile', 'People', 'API']);

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
	await expect(page.getByRole('textbox', { name: 'Search recipes' })).toHaveValue(token);
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

test('the bar shrinks while reading down and comes back on the way up', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	// A page of known height, whatever the shared database holds.
	await page.evaluate(() => {
		const spacer = document.createElement('div');
		spacer.style.height = '4000px';
		document.querySelector('main')?.append(spacer);
	});
	const nav = bottomNav(page);
	await expect(nav.getByRole('link', { name: 'Recipes' })).toBeVisible();

	await page.mouse.wheel(0, 800);
	const expand = nav.getByRole('button', { name: 'Show navigation' });
	await expect(expand).toBeVisible();
	await expect(nav.getByRole('button', { name: 'Search' })).toHaveCount(0);
	// New stays within reach in both shapes.
	await expect(page.getByRole('link', { name: 'New', exact: true })).toBeVisible();

	await page.mouse.wheel(0, -300);
	await expect(nav.getByRole('button', { name: 'Search' })).toBeVisible();

	// A tap on the shrunk bar brings it back as well, and reading on shrinks
	// it again: the focus the tap leaves in the bar is not keyboard focus.
	await page.mouse.wheel(0, 800);
	await expand.click();
	await expect(nav.getByRole('button', { name: 'Search' })).toBeVisible();
	await page.mouse.wheel(0, 800);
	await expect(expand).toBeVisible();

	// The same after a sheet closed by tap hands focus back to its trigger.
	await page.mouse.wheel(0, -300);
	await nav.getByRole('button', { name: 'Search' }).click();
	const palette = page.getByRole('dialog', { name: 'Command palette' });
	await palette.getByRole('button', { name: 'Close' }).click();
	await expect(palette).toHaveCount(0);
	await page.mouse.wheel(0, 800);
	await expect(expand).toBeVisible();
});

test('the bar keeps keyboard focus when it changes shape', async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
	await page.evaluate(() => {
		const spacer = document.createElement('div');
		spacer.style.height = '4000px';
		document.querySelector('main')?.append(spacer);
	});
	const nav = bottomNav(page);

	// Shrunk, the one button left says what it does; focusing and pressing it
	// brings the bar back with focus on the current place, not on the page.
	await page.mouse.wheel(0, 800);
	const expand = nav.getByRole('button', { name: 'Show navigation' });
	await expand.focus();
	await expect(nav.getByRole('link', { name: 'Recipes' })).toBeFocused();

	// While focus is in the bar, reading on does not pull it away.
	await page.mouse.wheel(0, 800);
	await expect(nav.getByRole('link', { name: 'Recipes' })).toBeFocused();
	await expect(nav.getByRole('button', { name: 'Search' })).toBeVisible();
});

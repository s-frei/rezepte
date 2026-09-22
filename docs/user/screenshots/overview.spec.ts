import { expect, test } from '@playwright/test';
import { prepare, shot } from './helpers';

test('overview', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	// Card titles are the only level-3 headings, and the demo seeds 12
	// recipes while the list page size is 24, so the whole grid is on screen.
	await expect(page.getByRole('heading', { level: 3 })).toHaveCount(12);
	await shot(page, 'overview');
});

test('command-palette', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	// Keyboard in both projects: the ⌘K button in SearchBar.svelte is
	// `hidden … md:inline-block`, so on the Pixel 7 viewport it is not
	// displayed and cannot be clicked. The root layout listens on the window
	// for Meta/Ctrl+K, which Chromium delivers in mobile emulation too.
	//
	// Not before the grid is there: the palette closes itself in
	// afterNavigate, and the navigation the login started can still finish
	// after the URL already reads `/` - which shut the palette under the
	// screenshot and left an empty overview on the picture.
	await expect(page.getByRole('heading', { level: 3 })).toHaveCount(12);
	await page.keyboard.press('ControlOrMeta+k');
	await expect(page.getByRole('dialog', { name: 'Command palette' })).toBeVisible();
	await shot(page, 'command-palette');
});

test('search', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.getByLabel('Search recipes').fill('Leek');
	// "Leek" only occurs in Leek and Potato Soup (title and ingredient).
	await expect(page.getByRole('heading', { level: 3 })).toHaveCount(1);
	await shot(page, 'search');
});

test('filters', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	// The bound comes from the address rather than from driving the slider:
	// a keyboard press would leave the thumb wearing its focus ring, which
	// belongs in a screenshot about as much as a mouse cursor does.
	await page.goto('/?maxMinutes=60');
	await page.getByRole('button', { name: 'Filters' }).click();
	await expect(page.getByRole('slider', { name: 'Maximum time' })).toHaveAttribute(
		'aria-valuetext',
		'up to 1 hr'
	);
	await shot(page, 'filters');
});

test('no-results', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.getByLabel('Search recipes').fill('zzz');
	await expect(page.getByRole('heading', { name: 'Nothing found' })).toBeVisible();
	await shot(page, 'no-results');
});

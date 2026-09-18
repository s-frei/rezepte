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
	await page.keyboard.press('ControlOrMeta+k');
	await expect(page.getByRole('dialog', { name: 'Befehlspalette' })).toBeVisible();
	await shot(page, 'command-palette');
});

test('search', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.getByLabel('Rezepte durchsuchen').fill('Linsen');
	// "Linsen" only occurs in Linseneintopf (title and ingredient).
	await expect(page.getByRole('heading', { level: 3 })).toHaveCount(1);
	await shot(page, 'search');
});

test('no-results', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true });
	await page.getByLabel('Rezepte durchsuchen').fill('zzz');
	await expect(page.getByRole('heading', { name: 'Nichts gefunden' })).toBeVisible();
	await shot(page, 'no-results');
});

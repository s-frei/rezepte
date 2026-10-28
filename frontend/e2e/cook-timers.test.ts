import { expect, test, type Page } from '@playwright/test';
import type { Recipe } from '../src/lib/api/recipes';
import { createRecipe, createUser, loadFixture, login, uniqueToken } from './helpers';

// Cook timers, read in German: a German account, and the German sample
// recipes whose steps carry stored times - Königsberger Klopse step 3
// "15 Minuten", Rinderrouladen step 5 "90 Minuten". The page clock is
// installed before the first navigation and paused once signed in, so the
// countdown moves only when a test says so; reduced motion takes Svelte's
// transitions (which run on the paused clock too) out of the way.

test.use({ contextOptions: { reducedMotion: 'reduce' } });

let username = '';

async function signInGerman(page: Page) {
	await page.clock.install();
	await login(page);
	await expect(page).toHaveURL('/');
	username = `timer${uniqueToken()}`;
	await createUser(page, { username, role: 'user', locale: 'de' });
	await page.context().clearCookies();
	await login(page, username);
	await expect(page.getByRole('heading', { name: 'Was kochen wir heute?' })).toBeVisible();
	await page.clock.pauseAt(Date.now() + 5_000);
}

async function germanRecipe(page: Page, index: number): Promise<Recipe> {
	const fixture = loadFixture(index, 'de');
	return createRecipe(page, { ...fixture, title: `${fixture.title} ${uniqueToken()}` });
}

/** Opens cook mode and pages forward to `step` (1-based). */
async function cookAt(page: Page, recipe: Recipe, step: number) {
	await page.goto(`/recipes/${recipe.slug}/cook`);
	for (let i = 1; i < step; i++) {
		await page.getByRole('button', { name: 'Weiter' }).click();
	}
	await expect(page.getByText(`Schritt ${step} von`)).toBeVisible();
}

const dock = (page: Page) => page.locator('[data-timer-dock]');

/** The dock's rows: on a phone behind the capsule, on a wide screen always open. */
async function expandDock(page: Page) {
	const expand = page.getByRole('button', { name: 'Alle Timer zeigen' });
	if (await expand.isVisible()) {
		await expand.click();
	}
}

test.beforeEach(async ({ page }) => {
	await signInGerman(page);
});

test('starts a timer from the step', async ({ page }) => {
	const recipe = await germanRecipe(page, 0);
	await cookAt(page, recipe, 3);
	await page.getByRole('button', { name: 'Timer starten: 15 Minuten' }).click();
	await expect(dock(page)).toContainText('15:00');
	await page.clock.runFor(60_000);
	await expect(dock(page)).toContainText('14:00');
	await expandDock(page);
	await expect(dock(page)).toContainText('Schritt 3 · 15 Minuten');
});

test('second tap starts nothing', async ({ page }) => {
	const recipe = await germanRecipe(page, 0);
	await cookAt(page, recipe, 3);
	const time = page.getByRole('button', { name: 'Timer starten: 15 Minuten' });
	await time.click();
	await time.click();
	await expandDock(page);
	await expect(dock(page).getByRole('button', { name: /stoppen$/ })).toHaveCount(1);
});

test('pause by tapping the ring', async ({ page }) => {
	const recipe = await germanRecipe(page, 0);
	await cookAt(page, recipe, 3);
	await page.getByRole('button', { name: 'Timer starten: 15 Minuten' }).click();
	await expandDock(page);
	await dock(page).getByRole('button', { name: 'Schritt 3 · 15 Minuten pausieren' }).click();
	await expect(dock(page)).toContainText('pausiert');
	const before = await dock(page).locator('[data-clock]').first().textContent();
	await page.clock.runFor(60_000);
	await expect(dock(page).locator('[data-clock]').first()).toHaveText(before!);
	await dock(page).getByRole('button', { name: 'Schritt 3 · 15 Minuten fortsetzen' }).click();
	await expect(dock(page)).not.toContainText('pausiert');
});

test('rings and turns off', async ({ page }) => {
	const recipe = await germanRecipe(page, 0);
	await cookAt(page, recipe, 3);
	await page.getByRole('button', { name: 'Timer starten: 15 Minuten' }).click();
	await page.clock.runFor(15 * 60_000);
	const banner = page.getByRole('alert').filter({ hasText: 'Fertig' });
	await expect(banner).toContainText('Schritt 3 · 15 Minuten');
	// A ringing timer leaves the dock: the banner is where it is.
	await expect(dock(page)).toHaveCount(0);
	await banner.getByRole('button', { name: 'Aus' }).click();
	await expect(banner).toHaveCount(0);
	await expect(dock(page)).toHaveCount(0);
});

test('survives a reload', async ({ page }) => {
	const recipe = await germanRecipe(page, 0);
	await cookAt(page, recipe, 3);
	await page.getByRole('button', { name: 'Timer starten: 15 Minuten' }).click();
	await page.clock.runFor(60_000);
	await expect(dock(page)).toContainText('14:00');
	await page.reload();
	await expect(dock(page)).toContainText(/1[34]:\d\d/);
	const before = await dock(page).locator('[data-clock]').first().textContent();
	await page.clock.runFor(60_000);
	await expect(dock(page).locator('[data-clock]').first()).not.toHaveText(before!);
});

test('phone collapses', async ({ page }) => {
	await page.setViewportSize({ width: 360, height: 780 });
	const recipe = await germanRecipe(page, 0);
	await cookAt(page, recipe, 3);
	await page.getByRole('button', { name: 'Timer starten: 15 Minuten' }).click();
	const expand = page.getByRole('button', { name: 'Alle Timer zeigen' });
	await expect(expand).toBeVisible();
	const stop = dock(page).getByRole('button', { name: /stoppen$/ });
	await expect(stop).toBeHidden();
	await expand.click();
	await expect(stop).toBeVisible();
	await page.getByRole('button', { name: 'Timer einklappen' }).click();
	await expect(stop).toBeHidden();
});

test('dock and footer leave the step readable at 320 px and the largest type', async ({ page }) => {
	await page.setViewportSize({ width: 320, height: 640 });
	// Rinderrouladen step 5 is long, and its time is the one to start.
	const recipe = await germanRecipe(page, 1);
	await cookAt(page, recipe, 5);
	await page.getByRole('button', { name: 'Timer starten: 90 Minuten' }).click();
	await page.getByRole('button', { name: 'Schriftgrad' }).click();
	await page.getByRole('radio', { name: 'Text', exact: true }).click();
	await page.keyboard.press('Escape');

	const section = page.locator('section').filter({ hasText: recipe.title });
	await section.evaluate((el) => el.scrollTo(0, el.scrollHeight));
	const text = await section.locator('p').boundingBox();
	const box = await dock(page).boundingBox();
	const footer = await page.locator('footer').boundingBox();
	expect(text && box && footer).toBeTruthy();
	expect(text!.y + text!.height).toBeLessThanOrEqual(box!.y);
	expect(box!.y + box!.height).toBeLessThanOrEqual(footer!.y);
});

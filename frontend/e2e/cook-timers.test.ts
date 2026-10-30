import { expect, test, type Locator, type Page } from '@playwright/test';
import type { Recipe } from '../src/lib/api/recipes';
import { createRecipe, createUser, loadFixture, login, signOut, uniqueToken } from './helpers';

// Cook timers, read in German: a German account, and the German sample
// recipes whose steps carry stored times - Königsberger Klopse step 3
// "15 Minuten", Rinderrouladen step 5 "90 Minuten". The page clock is
// installed before the first navigation and paused once signed in, so the
// countdown moves only when a test says so; reduced motion takes Svelte's
// transitions (which run on the paused clock too) out of the way.

// Notifications are granted, so a headed run opens no prompt on the first
// start; headless Chromium reads them as denied, so the dock shows
// `timer_push_unavailable` there.
test.use({ contextOptions: { reducedMotion: 'reduce' }, permissions: ['notifications'] });

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
	// `isVisible()` does not wait: the dock must be there before the capsule is looked for.
	await expect(dock(page)).toBeVisible();
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

	const section = page.locator('[role=group] section');
	await section.evaluate((el) => el.scrollTo(0, el.scrollHeight));
	const text = await section.locator('p').boundingBox();
	const whole = await page.getByRole('button', { name: 'Ganzer Schritt' }).boundingBox();
	const box = await dock(page).boundingBox();
	const footer = await page.locator('footer').boundingBox();
	expect(text && whole && box && footer).toBeTruthy();
	// Scrolled to the end, the last line clears the "Ganzer Schritt" button.
	expect(text!.y + text!.height).toBeLessThanOrEqual(whole!.y);
	expect(whole!.y + whole!.height).toBeLessThanOrEqual(box!.y);
	expect(box!.y + box!.height).toBeLessThanOrEqual(footer!.y);
});

test('a step that fits keeps its title and needs no whole view', async ({ page }) => {
	await page.setViewportSize({ width: 360, height: 780 });
	const recipe = await germanRecipe(page, 0);
	await cookAt(page, recipe, 3);
	await expect(page.getByRole('heading', { name: recipe.title })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Ganzer Schritt' })).toHaveCount(0);
});

test('a long step makes room and opens as a whole', async ({ page }) => {
	await page.setViewportSize({ width: 360, height: 780 });
	const recipe = await germanRecipe(page, 1);
	await cookAt(page, recipe, 5);
	await page.getByRole('button', { name: 'Schriftgrad' }).click();
	await page.getByRole('radio', { name: 'Text', exact: true }).click();
	await page.keyboard.press('Escape');

	// The title steps back, and the scrollbar sits at the screen edge.
	await expect(page.getByRole('heading', { name: recipe.title })).toHaveCount(0);
	const section = (await page.locator('[role=group] section').boundingBox())!;
	expect(Math.round(section.x + section.width)).toBe(360);

	const whole = page.getByRole('button', { name: 'Ganzer Schritt' });
	await whole.click();
	const close = page.getByRole('button', { name: 'Ganzen Schritt schließen' });
	await expect(close).toBeFocused();
	// Only the text, the type size and the timers stay.
	await expect(page.getByRole('button', { name: 'Weiter' })).toHaveCount(0);
	await expect(page.getByRole('button', { name: /^Zutaten/ })).toHaveCount(0);
	await expect(page.getByRole('progressbar')).toHaveCount(0);
	await expect(page.getByRole('button', { name: 'Schriftgrad' })).toBeVisible();
	await page.getByRole('button', { name: 'Timer starten: 90 Minuten' }).click();
	await expect(dock(page)).toBeVisible();

	// The reading view pages nothing; Escape returns to the step.
	await page.keyboard.press('ArrowRight');
	await expect(page).toHaveURL(/\?step=5$/);
	await page.keyboard.press('Escape');
	await expect(whole).toBeFocused();
	await expect(page.getByRole('button', { name: 'Weiter' })).toBeVisible();
	await expect(page).toHaveURL(/\?step=5$/);
});

test('one timer on a wide screen lines up with the footer', async ({ page }) => {
	await page.setViewportSize({ width: 1280, height: 800 });
	const recipe = await germanRecipe(page, 0);
	await cookAt(page, recipe, 3);
	await page.getByRole('button', { name: 'Timer starten: 15 Minuten' }).click();
	const row = (await dock(page).locator('.rounded-2xl').first().boundingBox())!;
	const back = (await page.getByRole('button', { name: 'Zurück', exact: true }).boundingBox())!;
	const next = (await page.getByRole('button', { name: 'Weiter' }).boundingBox())!;
	expect(Math.round(row.x)).toBe(Math.round(back.x));
	expect(Math.round(row.x + row.width)).toBe(Math.round(next.x + next.width));
});

// Outside cook mode: the player (phones) or the capsule (wide screens).
const indicator = (page: Page) => page.locator('[data-timer-indicator]');

async function startKlopse(page: Page): Promise<Recipe> {
	const recipe = await germanRecipe(page, 0);
	await cookAt(page, recipe, 3);
	await page.getByRole('button', { name: 'Timer starten: 15 Minuten' }).click();
	await page.getByRole('button', { name: 'Zurück zum Rezept' }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	return recipe;
}

test('shows on the overview', async ({ page }) => {
	await startKlopse(page);
	await page.goto('/');
	await page.setViewportSize({ width: 360, height: 780 });
	await expect(indicator(page)).toBeVisible();
	await expect(indicator(page)).toContainText('15:00');
	// At the bottom, above the nav.
	const player = await indicator(page).boundingBox();
	expect(player!.y + player!.height).toBeGreaterThan(780 - 120);
	await page.setViewportSize({ width: 1280, height: 800 });
	await expect(indicator(page)).toContainText('15:00');
	const capsule = await indicator(page).boundingBox();
	expect(capsule!.x + capsule!.width).toBeGreaterThan(1280 - 60);
	expect(capsule!.y + capsule!.height).toBeGreaterThan(800 - 60);
});

test('the player opens every timer and jumps to its step', async ({ page }) => {
	await page.setViewportSize({ width: 360, height: 780 });
	const recipe = await startKlopse(page);
	await page.goto('/');
	const toggle = indicator(page).locator('button[aria-expanded]');
	await expect(toggle).toHaveAttribute('aria-expanded', 'false');
	await toggle.click();
	await expect(toggle).toHaveAttribute('aria-expanded', 'true');
	const panel = page.locator('#timer-panel');
	await expect(panel).toBeVisible();
	// Above the player, as on a wide screen.
	const player = await toggle.boundingBox();
	const box = await panel.boundingBox();
	expect(box!.y + box!.height).toBeLessThan(player!.y);
	await page.keyboard.press('Escape');
	await expect(panel).toBeHidden();
	await expect(toggle).toBeFocused();
	await toggle.click();
	// A client-side link navigation stalls on the paused page clock.
	await page.clock.resume();
	await panel.getByRole('link', { name: /Schritt 3 · 15 Minuten/ }).click();
	await expect(page.getByText('Schritt 3 von 6')).toBeVisible();
	// The step stays in the URL while paging, so a reload keeps it.
	await expect(page).toHaveURL(`/recipes/${recipe.slug}/cook?step=3`);
	await page.getByRole('button', { name: 'Weiter' }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}/cook?step=4`);
	await page.reload();
	await expect(page.getByText('Schritt 4 von 6')).toBeVisible();
});

test('the desktop capsule opens upwards', async ({ page }) => {
	await page.setViewportSize({ width: 1280, height: 800 });
	await startKlopse(page);
	await page.goto('/');
	const toggle = indicator(page).locator('button[aria-expanded]');
	await toggle.click();
	const panel = page.locator('#timer-panel');
	await expect(panel).toBeVisible();
	const capsule = await toggle.boundingBox();
	const box = await panel.boundingBox();
	expect(box!.y + box!.height).toBeLessThan(capsule!.y);
	// A tap elsewhere closes it.
	await page.mouse.click(200, 200);
	await expect(panel).toBeHidden();
});

test('shows the timer with the least time left, paused or not', async ({ page }) => {
	await page.setViewportSize({ width: 360, height: 780 });
	const klopse = await germanRecipe(page, 0);
	const rouladen = await germanRecipe(page, 1);
	await cookAt(page, rouladen, 5);
	await page.getByRole('button', { name: 'Timer starten: 90 Minuten' }).click();
	// fastForward, not runFor: runFor renders every one of the 4 800 ticks
	// and took 25s; the dock reads the time left from the clock either way.
	await page.clock.fastForward(80 * 60_000);
	await expandDock(page);
	await dock(page).getByRole('button', { name: 'Schritt 5 · 90 Minuten pausieren' }).click();
	await cookAt(page, klopse, 3);
	await page.getByRole('button', { name: 'Timer starten: 15 Minuten' }).click();
	// Ten paused minutes left beat fifteen running ones.
	const capsule = page.getByRole('button', { name: 'Alle Timer zeigen' });
	await expect(capsule).toContainText('10:00');
	await expect(capsule).toContainText('Schritt 5 · 90 Minuten');
	await expect(capsule).toContainText('pausiert');
});

test("another recipe's timer opens its cook mode at the step", async ({ page }) => {
	const klopse = await startKlopse(page);
	const other = await germanRecipe(page, 1);
	await cookAt(page, other, 1);
	await expandDock(page);
	await page.clock.resume();
	await dock(page)
		.getByRole('button', { name: /Schritt 3 · 15 Minuten/ })
		.filter({ hasText: klopse.title })
		.click();
	await expect(page).toHaveURL(`/recipes/${klopse.slug}/cook?step=3`);
	await expect(page.getByText('Schritt 3 von 6')).toBeVisible();
});

test('jump clamps to the last step', async ({ page }) => {
	const recipe = await germanRecipe(page, 0);
	await page.goto(`/recipes/${recipe.slug}/cook?step=99`);
	await expect(page.getByText('Schritt 6 von 6')).toBeVisible();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}/cook?step=6`);
	await page.goto(`/recipes/${recipe.slug}/cook?step=abc`);
	await expect(page.getByText('Schritt 1 von 6')).toBeVisible();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}/cook?step=1`);
});

test('the player covers nothing', async ({ page }) => {
	await page.setViewportSize({ width: 360, height: 780 });
	const recipe = await startKlopse(page);
	const player = () => indicator(page).locator('button[aria-expanded]');
	const bottomNav = page.getByRole('navigation', { name: 'Hauptmenü' }).locator('..');

	// Above the nav, and the overview's last row ends above the player.
	await page.goto('/');
	// Scrolled again on every try: the overview may still be loading its
	// cards, and the nav docks on the scroll with the player riding along.
	await expect
		.poll(async () => {
			await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
			const box = (await player().boundingBox())!;
			const nav = (await bottomNav.boundingBox())!;
			const last = (await page.locator('main > *').last().boundingBox())!;
			return box.y + box.height <= nav.y && last.y + last.height <= box.y;
		})
		.toBe(true);
	// As wide as the nav, docked (scrolled past DOCK_AT) and floating (at
	// the top); a short viewport, so one recipe card scrolls far enough.
	await page.setViewportSize({ width: 360, height: 420 });
	await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
	const sameEdges = async () => {
		const p = (await indicator(page).locator('> div').first().boundingBox())!;
		const n = (await bottomNav.boundingBox())!;
		expect(Math.abs(p.x - n.x)).toBeLessThan(1);
		expect(Math.abs(p.x + p.width - (n.x + n.width))).toBeLessThan(1);
	};
	await expect.poll(() => bottomNav.getAttribute('data-docked')).toBe('');
	await expect.poll(async () => (await bottomNav.boundingBox())!.x).toBeLessThan(9);
	await sameEdges();
	await page.evaluate(() => window.scrollTo(0, 0));
	await expect.poll(() => bottomNav.getAttribute('data-docked')).toBeNull();
	// Both morph over 500 ms.
	await expect.poll(async () => (await bottomNav.boundingBox())!.x).toBeGreaterThan(15);
	await page.waitForTimeout(600);
	await sameEdges();

	// A page without the nav keeps its fixed top bar at the top.
	await page.goto(`/recipes/${recipe.slug}`);
	await expect(player()).toBeVisible();
	expect(
		(await page.getByRole('button', { name: 'Zurück' }).first().boundingBox())!.y
	).toBeLessThan(60);

	// The software keyboard is up: the player steps aside like the nav.
	await page.goto('/settings');
	await page.getByRole('textbox').first().focus();
	await expect(player()).toBeHidden();
});

/** The element shows its whole text: nothing cut off by an ellipsis. */
async function expectWhole(locator: Locator, text: string) {
	await expect(locator).toHaveText(text);
	const [scroll, client] = await locator.evaluate((el) => [el.scrollWidth, el.clientWidth]);
	expect(scroll).toBeLessThanOrEqual(client);
}

test('rings outside cook mode and turns off', async ({ page }) => {
	await page.setViewportSize({ width: 360, height: 780 });
	await startKlopse(page);
	await page.clock.runFor(15 * 60_000);
	await expect(indicator(page)).toContainText('Fertig');
	// The name, not the recipe title, is what tells two timers apart.
	const name = indicator(page).locator('button[aria-expanded] [data-timer-name]');
	await expectWhole(name, 'Schritt 3 · 15 Minuten');
	await page.setViewportSize({ width: 320, height: 640 });
	await expectWhole(name, 'Schritt 3 · 15 Minuten');
	await indicator(page).getByRole('button', { name: 'Aus' }).click();
	await expect(indicator(page)).toHaveCount(0);
});

test('sign-out clears timers', async ({ page }, testInfo) => {
	await startKlopse(page);
	await page.goto('/');
	await expect(indicator(page)).toBeVisible();
	await signOut(page, testInfo, 'de');
	await expect(page).toHaveURL(/\/login/);
	// The sign-in's transition waits on the page's timers; what follows only
	// asks whether a timer is left, which needs no stopped clock.
	await page.clock.resume();
	await login(page, username);
	await expect(page.getByRole('heading', { name: 'Was kochen wir heute?' })).toBeVisible();
	await expect(indicator(page)).toHaveCount(0);
});

test('a timer survives its recipe losing the step', async ({ page }) => {
	const recipe = await germanRecipe(page, 0);
	await cookAt(page, recipe, 3);
	await page.getByRole('button', { name: 'Timer starten: 15 Minuten' }).click();

	// The recipe is cut to two steps while the timer runs.
	const shorter = { ...loadFixture(0, 'de'), title: recipe.title };
	shorter.steps = shorter.steps.slice(0, 2);
	const response = await page.request.put(`/api/v1/recipes/${recipe.id}`, {
		headers: { Origin: new URL(page.url()).origin, 'Content-Type': 'application/json' },
		data: shorter
	});
	expect(response.ok()).toBeTruthy();

	await page.goto(`/recipes/${recipe.slug}/cook`);
	await expandDock(page);
	// The timer keeps its stored name; opening it lands on the last step left.
	await dock(page)
		.getByRole('button', { name: /fertig .* Schritt 3 · 15 Minuten$/ })
		.click();
	await expect(page.getByText('Schritt 2 von 2')).toBeVisible();
});

test("another recipe's banner keeps the timer's name whole", async ({ page }) => {
	await page.setViewportSize({ width: 360, height: 780 });
	await startKlopse(page);
	const other = await germanRecipe(page, 1);
	await cookAt(page, other, 1);
	await page.clock.runFor(15 * 60_000);
	const banner = page.getByRole('alert').filter({ hasText: 'Fertig' });
	await expect(banner).toContainText('Königsberger Klopse');
	await expectWhole(banner.locator('[data-timer-name]'), 'Schritt 3 · 15 Minuten');
});

test('serves the push worker uncached and registers it', async ({ page }) => {
	const res = await page.request.get('/service-worker.js');
	expect(res.ok()).toBe(true);
	expect(res.headers()['cache-control']).toBe('no-cache');
	await expect
		.poll(
			() =>
				page.evaluate(
					async () => (await navigator.serviceWorker.getRegistration())?.active?.scriptURL ?? ''
				),
			{ timeout: 15_000 }
		)
		.toContain('/service-worker.js');
});

test('schedules a push alarm in the cook language', async ({ page }) => {
	// Playwright's Chromium has no push service: a fake subscription stands
	// in, and the server's answers are stubbed, since it would refuse the
	// fake endpoint anyway. Headless Chromium also reports
	// `Notification.permission` as 'denied' even with the permission granted.
	await page.addInitScript(() => {
		Object.defineProperty(Notification, 'permission', { get: () => 'granted' });
		PushManager.prototype.getSubscription = async () => null;
		PushManager.prototype.subscribe = async () =>
			({
				endpoint: 'https://fcm.googleapis.com/fcm/send/e2e',
				toJSON: () => ({ keys: { p256dh: 'p', auth: 'a' } })
			}) as unknown as PushSubscription;
	});
	await page.route('**/api/v1/push/subscription', (route) => route.fulfill({ status: 204 }));
	await page.route('**/api/v1/push/alarms', (route) => route.fulfill({ json: { id: 'e2e' } }));
	const recipe = await germanRecipe(page, 0);
	await cookAt(page, recipe, 3);
	const alarm = page.waitForRequest(
		(r) => r.url().endsWith('/api/v1/push/alarms') && r.method() === 'POST'
	);
	await page.getByRole('button', { name: 'Timer starten: 15 Minuten' }).click();
	expect((await alarm).postDataJSON()).toMatchObject({
		endpoint: 'https://fcm.googleapis.com/fcm/send/e2e',
		title: `${recipe.title} · fertig`,
		body: 'Schritt 3 · 15 Minuten',
		url: `/recipes/${recipe.slug}/cook?step=3`
	});
	await expect(page.locator('[data-push-unavailable]')).toHaveCount(0);
});

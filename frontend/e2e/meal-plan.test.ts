import { expect, test, type Browser, type Locator, type Page } from '@playwright/test';
import {
	createRecipe,
	createUser,
	loadFixture,
	login,
	openDialog,
	switchTo,
	uniqueToken
} from './helpers';

const phone = (page: Page) => (page.viewportSize()?.width ?? 1280) < 768;

/** The place's link: the top bar's segment on a desktop, the bottom nav's basket on a phone. */
const placeLink = (page: Page) =>
	phone(page)
		? page.getByRole('navigation', { name: 'Main' }).getByRole('link', { name: 'Shopping & week' })
		: page
				.getByRole('banner')
				.getByRole('navigation')
				.getByRole('link', { name: 'Shopping & week' });

/** A person of the test's own: the projects run at once on one instance. */
async function loginFresh(page: Page) {
	const username = `plan${uniqueToken()}`.slice(0, 20);
	await login(page);
	await createUser(page, { username, role: 'user' });
	await page.context().clearCookies();
	await login(page, username);
}

const side = (page: Page, name: 'Lists' | 'Week') =>
	page.getByRole('radiogroup', { name: 'Lists or week' }).getByRole('radio', { name });

test('place switch remembers the side', async ({ page }) => {
	await loginFresh(page);
	// A first visit opens the lists.
	await expect(placeLink(page)).toHaveAttribute('href', '/lists');
	await placeLink(page).click();
	await expect(page).toHaveURL(/\/lists$/);
	await expect(page.getByRole('heading', { level: 1, name: 'Shopping & week' })).toBeVisible();
	await expect(side(page, 'Lists')).toBeChecked();

	await side(page, 'Week').click();
	await expect(page).toHaveURL(/\/plans$/);
	await expect(page.getByRole('heading', { level: 1, name: 'Shopping & week' })).toBeVisible();
	await expect(side(page, 'Week')).toBeChecked();
	// The week side opens on the plans overview.
	await expect(page.getByRole('heading', { level: 2, name: 'Your plans' })).toBeVisible();
	if (!phone(page)) {
		// The overview's own action replaces New list.
		const bar = page.getByRole('banner');
		await expect(bar.getByRole('button', { name: 'New plan' })).toBeVisible();
		await expect(bar.getByRole('button', { name: 'New list' })).toHaveCount(0);
	}

	// The place reopens on the week, also after a reload.
	await page.goto('/');
	await expect(placeLink(page)).toHaveAttribute('href', '/plans');
	await placeLink(page).click();
	await expect(page).toHaveURL(/\/plans$/);
	await expect(side(page, 'Week')).toBeChecked();

	await side(page, 'Lists').click();
	await expect(page).toHaveURL(/\/lists$/);
	await expect(placeLink(page)).toHaveAttribute('href', '/lists');
});

test('the switch swaps the sides in place, without a reload or a fade', async ({ page }) => {
	await loginFresh(page);
	const list = `Switch ${uniqueToken()}`;
	expect((await page.request.post('/api/v1/shopping-lists', { data: { name: list } })).ok()).toBe(
		true
	);
	await page.goto('/lists');
	await expect(page.getByRole('link', { name: list })).toBeVisible();
	const documents: string[] = [];
	page.on('request', (r) => {
		if (r.resourceType() === 'document') documents.push(r.url());
	});
	// Recorded on every change, not read once at the end: an expect that
	// retries passes over a skeleton or a bare page that came and went.
	// A reload would drop the marker; a page transition sets its attribute.
	await page.evaluate((list) => {
		const seen = { stayed: true, faded: false, skeleton: false, bare: false };
		Object.assign(window, { seen });
		const look = () => {
			const main = document.querySelector('main');
			if (document.documentElement.hasAttribute('data-page-transition')) seen.faded = true;
			if (main?.querySelector('.bg-skeleton')) seen.skeleton = true;
			// Some side is always whole on screen: its list card or its plans.
			const text = main?.textContent ?? '';
			if (!text.includes(list) && !text.includes('Your plans')) seen.bare = true;
		};
		new MutationObserver(look).observe(document.documentElement, {
			attributes: true,
			childList: true,
			subtree: true
		});
	}, list);
	for (const [to, url] of [
		['Week', /\/plans$/],
		['Lists', /\/lists$/],
		['Week', /\/plans$/],
		['Lists', /\/lists$/]
	] as const) {
		await side(page, to).click();
		await expect(page).toHaveURL(url);
		await expect(side(page, to)).toBeChecked();
	}
	await expect(page.getByRole('link', { name: list })).toBeVisible();
	expect(documents).toEqual([]);
	expect(await page.evaluate(() => (window as unknown as { seen: object }).seen)).toEqual({
		stayed: true,
		faded: false,
		skeleton: false,
		bare: false
	});
});

test('the pager moves by weeks, and Today comes back marked', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-20T10:00:00'));
	await loginFresh(page);
	await plan(page, { day: '2026-11-21', title: 'Pizza bestellen' });
	await page.goto('/lists');
	const id = await openOwn(page);
	const label = page.getByRole('heading', { level: 2 }).first();
	await expect(label).toContainText(/Week 47 · November 16–22|Nov 16–22/);
	const friday = day(page, 'Friday, November 20');
	await expect(friday).toHaveAttribute('aria-current', 'date');
	await expect(friday.getByText('Today')).toBeVisible();
	await page.getByRole('button', { name: 'Next week' }).click();
	await expect(page).toHaveURL(new RegExp(`/plans/${id}\\?from=2026-11-23$`));
	await expect(label).toContainText(/Week 48|Nov 23–29/);
	await expect(page.locator('[aria-current="date"]')).toHaveCount(0);
	await page.getByRole('button', { name: 'Today' }).click();
	await expect(page).toHaveURL(new RegExp(`/plans/${id}$`));
	await expect(friday).toHaveAttribute('aria-current', 'date');
	await expect(day(page, 'Saturday, November 21').getByText('Pizza bestellen')).toBeVisible();
	await page.getByRole('button', { name: 'Previous week' }).click();
	await expect(page).toHaveURL(new RegExp(`/plans/${id}\\?from=2026-11-09$`));
	await expect(
		page.getByRole('heading', { level: 2, name: 'Nothing planned this week' })
	).toBeVisible();
	// Paging replaced the plan's entry: one Back leaves the plan.
	await page.goBack();
	await expect(page).toHaveURL(/\/lists$/);
});

// The pick lands on another page; the keyboard stays in the switch.
test('an arrow key in the switch keeps the focus on the side it picked', async ({
	page,
	isMobile
}) => {
	test.skip(isMobile, 'arrow keys are a keyboard path');
	await loginFresh(page);
	await page.goto('/lists');
	await side(page, 'Lists').focus();
	await page.keyboard.press('ArrowRight');
	await expect(page).toHaveURL(/\/plans$/);
	await expect(side(page, 'Week')).toBeChecked();
	await expect(side(page, 'Week')).toBeFocused();
	await page.keyboard.press('ArrowLeft');
	await expect(page).toHaveURL(/\/lists$/);
	await expect(side(page, 'Lists')).toBeFocused();
});

test('the overview opens a plan, and the way back leads to it', async ({ page }) => {
	await loginFresh(page);
	const idle = `Idle ${uniqueToken()}`;
	const busy = `Busy ${uniqueToken()}`;
	await namedPlan(page, idle);
	const busyId = await namedPlan(page, busy);
	const added = await page.request.post(`/api/v1/meal-plans/${busyId}/entries`, {
		data: { day: today(), title: 'Pizza bestellen' }
	});
	expect(added.ok()).toBe(true);
	await page.goto('/plans');

	// My plan first, then the plan with meals this week; the idle one is a chip.
	await expect(page.getByRole('article').getByRole('heading', { level: 3 })).toHaveText([
		'My plan',
		busy
	]);
	const card = page.getByRole('article', { name: busy });
	await expect(card.getByText('1 meal', { exact: true })).toBeVisible();
	await expect(card.getByRole('list', { name: 'This week' }).getByRole('listitem')).toHaveCount(7);
	const chips = page.getByRole('list', { name: 'Empty this week' });
	await expect(chips.getByRole('link')).toHaveText([idle]);

	await card.getByRole('link', { name: busy }).click();
	await expect(page).toHaveURL(new RegExp(`/plans/${busyId}$`));
	await expect(page.getByRole('heading', { level: 1, name: busy })).toBeVisible();
	if (phone(page)) {
		await page.getByRole('main').getByRole('link', { name: 'Back' }).click();
	} else {
		await page.getByRole('banner').getByRole('link', { name: 'Meal plans' }).click();
	}
	await expect(page).toHaveURL(/\/plans$/);

	// "Today …" on the card opens the plan at today.
	await page
		.getByRole('article', { name: busy })
		.getByRole('link', { name: /^Today/ })
		.click();
	await expect(page).toHaveURL(new RegExp(`/plans/${busyId}#today$`));
	await expect(page.locator('#today')).toBeInViewport();

	// An empty today offers to plan it: the plan opens with the add sheet on today.
	await page.goto('/plans');
	await page
		.getByRole('article', { name: 'My plan' })
		.getByRole('link', { name: 'Plan today' })
		.click();
	await expect(page.getByRole('dialog', { name: 'Plan a meal' })).toBeVisible();
	await expect(page).toHaveURL(/\/plans\/[^?]+#today$/);
});

test('unknown and invited-only plan links land on the overview', async ({ page }) => {
	const { token, owner, guest, guestId } = await twoPeople(page);
	const name = `Invited ${token}`;
	const invited = await namedPlan(page, name, guestId);

	// A plan the reader cannot see (here deleted): the overview, saying so, never a 404.
	const gone = await namedPlan(page, `Gone ${token}`);
	expect((await page.request.delete(`/api/v1/meal-plans/${gone}`)).ok()).toBe(true);
	await page.goto(`/plans/${gone}`);
	await expect(page).toHaveURL(/\/plans$/);
	await expect(page.getByText('You no longer have access to this plan.')).toBeVisible();
	await expect(page.getByRole('heading', { level: 2, name: 'Your plans' })).toBeVisible();

	// A card gone stale under the open overview: one toast, and the card goes.
	const stale = `Stale ${token}`;
	const staleId = await namedPlan(page, stale);
	await page.request.post(`/api/v1/meal-plans/${staleId}/entries`, { data: { title: 'Suppe' } });
	await page.goto('/lists');
	await page.goto('/plans');
	const toast = page.getByText('You no longer have access to this plan.');
	await expect(toast).toHaveCount(0);
	expect((await page.request.delete(`/api/v1/meal-plans/${staleId}`)).ok()).toBe(true);
	await page.getByRole('article', { name: stale }).getByRole('link', { name: stale }).click();
	await expect(page).toHaveURL(/\/plans$/);
	await expect(toast).toHaveCount(1);
	await expect(page.getByRole('article', { name: stale })).toHaveCount(0);
	// The tap left no second overview entry: one Back leaves the overview.
	await page.goBack();
	await expect(page).toHaveURL(/\/lists$/);

	// Only invited: the overview under the invitation's card.
	await switchTo(page, guest);
	await page.goto(`/plans/${invited}`);
	await expect(page).toHaveURL(/\/plans$/);
	await expect(page.getByText(`${owner} invites you to the plan “${name}”`)).toBeVisible();
	await expect(page.getByText('You no longer have access to this plan.')).toHaveCount(0);
});

// The week view. Entries are arranged through the API; what is tested is the page.

type Plan = { id: string; entries: { id: string; title: string; day: string | null }[] };

/** `YYYY-MM-DD` of the browser's today. */
const today = () => new Date().toLocaleDateString('sv');

/** Opens the signed-in person's own plan, and gives its id. */
async function openOwn(page: Page): Promise<string> {
	const id = await ownPlan(page);
	await page.goto(`/plans/${id}`);
	return id;
}

async function ownPlan(page: Page): Promise<string> {
	const overview = (await (await page.request.get('/api/v1/meal-plans')).json()) as {
		plans: { id: string; personal: boolean }[];
	};
	return overview.plans.find((p) => p.personal)!.id;
}

async function plan(
	page: Page,
	entry: { day?: string; recipeId?: string; title?: string; servings?: number }
): Promise<Plan> {
	const id = await ownPlan(page);
	const made = await page.request.post(`/api/v1/meal-plans/${id}/entries`, { data: entry });
	expect(made.ok(), await made.text()).toBe(true);
	return (await made.json()) as Plan;
}

const day = (page: Page, name: string) => page.getByRole('region', { name });
const tray = (page: Page) => page.getByRole('region', { name: 'Someday' });

/** A finger's drag: held until the zone picks the card up, then moved over `to` in steps. */
async function touchDrag(page: Page, from: Locator, to: Locator) {
	const a = (await from.boundingBox())!;
	const b = (await to.boundingBox())!;
	const cdp = await page.context().newCDPSession(page);
	const point = (x: number, y: number) => ({ x: Math.round(x), y: Math.round(y) });
	const start = point(a.x + a.width / 2, a.y + a.height / 2);
	await cdp.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [start] });
	await page.waitForTimeout(500);
	const end = point(b.x + b.width / 2, b.y + Math.min(b.height / 2, 40));
	for (let i = 1; i <= 12; i++) {
		const at = point(
			start.x + ((end.x - start.x) * i) / 12,
			start.y + ((end.y - start.y) * i) / 12
		);
		await cdp.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: [at] });
		await page.waitForTimeout(30);
	}
	await page.waitForTimeout(200);
	await cdp.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
	await cdp.detach();
}

/** Whether a click on the page still reaches the document: a drop's click guard swallows it on the window. */
const clicksHeard = (page: Page) =>
	page.evaluate(() => {
		let heard = false;
		const hear = () => (heard = true);
		document.addEventListener('click', hear, { capture: true });
		document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }));
		document.removeEventListener('click', hear, { capture: true });
		return heard;
	});

test('plans a recipe and a note on a day', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-16T10:00:00'));
	await loginFresh(page);
	const title = `Curry ${uniqueToken()}`;
	await createRecipe(page, { ...loadFixture(1), title });
	await openOwn(page);

	// The empty week's button opens the sheet; the day is picked in it.
	const sheet = await openDialog(
		page,
		page.getByRole('main').getByRole('button', { name: 'Plan a meal' }),
		'Plan a meal'
	);
	await sheet.getByRole('radio', { name: 'Wednesday, November 18' }).click();
	await sheet.getByRole('combobox', { name: 'Recipe or note' }).fill(title.slice(0, 12));
	await sheet.getByRole('option', { name: title }).click();
	await expect(sheet).toBeHidden();
	const wednesday = day(page, 'Wednesday, November 18');
	await expect(wednesday.getByRole('link', { name: title })).toBeVisible();

	// The day's own "+" knows its day; a typed text that is no recipe is a note.
	const again = await openDialog(
		page,
		wednesday.getByRole('button', { name: 'Plan for Wednesday, November 18' }),
		'Plan a meal'
	);
	await expect(again.getByRole('radio', { name: 'Wednesday, November 18' })).toBeChecked();
	await again.getByRole('combobox', { name: 'Recipe or note' }).fill('Pizza bestellen');
	await again.getByRole('option', { name: 'Add “Pizza bestellen” as a note' }).click();
	await expect(wednesday.getByText('Pizza bestellen')).toBeVisible();
	await expect(day(page, 'Thursday, November 19').getByText('Pizza bestellen')).toHaveCount(0);
});

test('moves an entry to another day by menu', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-16T10:00:00'));
	await loginFresh(page);
	const title = `Klopse ${uniqueToken()}`;
	const recipe = await createRecipe(page, { ...loadFixture(1), title });
	await plan(page, { day: '2026-11-17', recipeId: recipe.id, servings: 7 });
	await openOwn(page);

	await day(page, 'Tuesday, November 17')
		.getByRole('button', { name: `Actions for ${title}` })
		.click();
	await page.getByRole('menuitem', { name: 'Move to a day…' }).click();
	const sheet = page.getByRole('dialog', { name: 'Move to a day' });
	await sheet.getByRole('button', { name: 'Thursday, November 19' }).click();
	await expect(sheet).toBeHidden();
	const thursday = day(page, 'Thursday, November 19');
	await expect(thursday.getByRole('link', { name: title })).toBeVisible();
	await expect(day(page, 'Tuesday, November 17').getByText(title)).toHaveCount(0);

	// A tap opens the recipe at the planned servings.
	const link = thursday.getByRole('link', { name: title });
	await expect(link).toHaveAttribute('href', `/recipes/${recipe.slug}?servings=7`);
	await link.click();
	const stepper = page.getByRole('group', { name: 'Servings' }).first();
	await expect(stepper.getByText('7', { exact: true })).toBeVisible();
	// Applied once, the ask leaves the address: the reader's own change
	// outlasts the recipe loading again.
	await expect(page).toHaveURL(new RegExp(`/recipes/${recipe.slug}$`));
	await stepper.getByRole('button', { name: 'Fewer servings' }).click();
	await expect(stepper.getByText('6', { exact: true })).toBeVisible();
	await page.reload();
	await expect(
		page.getByRole('group', { name: 'Servings' }).first().getByText('6', { exact: true })
	).toBeVisible();
});

test('a note typed fast and taken with Enter is the note, never a recipe', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-16T10:00:00'));
	await loginFresh(page);
	await createRecipe(page, { ...loadFixture(1), title: `Curry ${uniqueToken()}` });
	await openOwn(page);
	const sheet = await openDialog(
		page,
		page.getByRole('main').getByRole('button', { name: 'Plan a meal' }),
		'Plan a meal'
	);
	const field = sheet.getByRole('combobox', { name: 'Recipe or note' });
	// The recently changed recipes are listed, but Enter on the empty field plans nothing.
	await expect(sheet.getByRole('option').first()).toBeVisible();
	await field.press('Enter');
	await expect(sheet).toBeVisible();
	// Typed and taken before the search for it answers.
	await field.pressSequentially('Pizza bestellen', { delay: 5 });
	await field.press('Enter');
	await expect(sheet).toBeHidden();
	const monday = day(page, 'Monday, November 16');
	await expect(monday.getByText('Pizza bestellen')).toBeVisible();
	await expect(monday.getByRole('link')).toHaveCount(0);
});

/**
 * Takes `name` from the open entry menu by keyboard. Bits focuses the first
 * item twice: a tick after opening, and again from the focus scope's
 * `requestAnimationFrame`, which would undo a pick made in between. So: wait
 * for an item to hold the focus, then for one frame - a frame callback asked
 * for now runs after the scope's, which was asked for when the menu opened.
 */
async function pickByKey(page: Page, name: string) {
	const menu = page.getByRole('menu');
	await expect
		.poll(() => page.evaluate(() => document.activeElement?.getAttribute('role')))
		.toBe('menuitem');
	await page.evaluate(() => new Promise((done) => requestAnimationFrame(done)));
	await menu.getByRole('menuitem', { name }).focus();
	await page.keyboard.press('Enter');
}

test('the keyboard stays on the week after a move or a removal', async ({ page, isMobile }) => {
	test.skip(isMobile, 'a keyboard path');
	await page.clock.setFixedTime(new Date('2026-11-16T10:00:00'));
	await loginFresh(page);
	await plan(page, { day: '2026-11-17', title: 'Reste' });
	await plan(page, { day: '2026-11-18', title: 'Suppe' });
	await plan(page, { day: '2026-11-18', title: 'Brot' });
	await openOwn(page);

	// Moved: the focus is on the card's ⋯ in its new day.
	await day(page, 'Tuesday, November 17')
		.getByRole('button', { name: 'Actions for Reste' })
		.focus();
	await page.keyboard.press('Enter');
	await pickByKey(page, 'Move to a day…');
	const sheet = page.getByRole('dialog', { name: 'Move to a day' });
	// Bits UI focuses into the sheet a frame after it opens.
	await expect.poll(() => sheet.evaluate((el) => el.contains(document.activeElement))).toBe(true);
	await sheet.getByRole('button', { name: 'Thursday, November 19' }).focus();
	await page.keyboard.press('Enter');
	await expect(sheet).toBeHidden();
	const thursday = day(page, 'Thursday, November 19');
	await expect(thursday.getByRole('button', { name: 'Actions for Reste' })).toBeFocused();

	// Removed: the next card's ⋯, then, with the day empty, its "+".
	const wednesday = day(page, 'Wednesday, November 18');
	await wednesday.getByRole('button', { name: 'Actions for Suppe' }).focus();
	await page.keyboard.press('Enter');
	await pickByKey(page, 'Remove');
	await expect(wednesday.getByRole('button', { name: 'Actions for Brot' })).toBeFocused();
	await page.keyboard.press('Enter');
	await pickByKey(page, 'Remove');
	await expect(
		wednesday.getByRole('button', { name: 'Plan for Wednesday, November 18' }).first()
	).toBeFocused();
});

test('the phone’s "+" plans a meal on the week', async ({ page, isMobile }) => {
	test.skip(!isMobile, 'the bottom nav');
	await loginFresh(page);
	await openOwn(page);
	// The nav's "+", named by its label (the empty week's button has the words).
	const plus = page.locator('button[aria-label="Plan a meal"]');
	await expect(plus).toHaveAttribute('aria-expanded', 'false');
	await plus.click();
	await expect(page.getByRole('dialog', { name: 'Plan a meal' })).toBeVisible();
	await expect(plus).toHaveAttribute('aria-expanded', 'true');
});

test('drags an entry from the tray to a day', async ({ page, isMobile }) => {
	await page.clock.setFixedTime(new Date('2026-11-16T10:00:00'));
	await loginFresh(page);
	await plan(page, { title: 'Flammkuchen' });
	await plan(page, { day: '2026-11-20', title: 'Pizza bestellen' });
	await openOwn(page);

	const card = tray(page).getByText('Flammkuchen');
	const friday = day(page, 'Friday, November 20');
	if (isMobile) {
		await card.scrollIntoViewIfNeeded();
		await touchDrag(page, card, friday.getByText('Pizza bestellen'));
	} else {
		const from = (await card.boundingBox())!;
		const target = (await friday.getByText('Pizza bestellen').boundingBox())!;
		await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
		await page.mouse.down();
		// svelte-dnd-action places the card on an observation interval: the
		// pointer comes to rest over the target before it lets go.
		await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2 + 12, { steps: 4 });
		await page.waitForTimeout(200);
		await page.mouse.move(target.x + target.width / 2, target.y + target.height / 2, { steps: 12 });
		await page.waitForTimeout(400);
		await page.mouse.up();
	}
	await expect(friday.getByText('Flammkuchen')).toBeVisible();
	await expect(tray(page).getByText('Flammkuchen')).toHaveCount(0);
	// The server took it: a reload shows it on Friday.
	await page.reload();
	await expect(day(page, 'Friday, November 20').getByText('Flammkuchen')).toBeVisible();
});

test('a finger drops a card on an empty day by its name, over a dashed slot', async ({
	page,
	isMobile
}) => {
	test.skip(!isMobile, 'the day rows are the phone layout');
	await page.clock.setFixedTime(new Date('2026-11-16T10:00:00'));
	await loginFresh(page);
	await plan(page, { day: '2026-11-17', title: 'Linsensuppe' });
	await openOwn(page);
	const card = day(page, 'Tuesday, November 17').getByText('Linsensuppe');
	await card.scrollIntoViewIfNeeded();
	// Straight down onto the empty day's name, nowhere near its "+".
	const name = day(page, 'Wednesday, November 18').getByText('Wednesday', { exact: true });
	const a = (await card.boundingBox())!;
	const b = (await name.boundingBox())!;
	const cdp = await page.context().newCDPSession(page);
	const x = Math.round(a.x + a.width / 2);
	await cdp.send('Input.dispatchTouchEvent', {
		type: 'touchStart',
		touchPoints: [{ x, y: Math.round(a.y + a.height / 2) }]
	});
	await page.waitForTimeout(500);
	for (let i = 1; i <= 12; i++) {
		const y = a.y + a.height / 2 + ((b.y + b.height / 2 - a.y - a.height / 2) * i) / 12;
		await cdp.send('Input.dispatchTouchEvent', {
			type: 'touchMove',
			touchPoints: [{ x, y: Math.round(y) }]
		});
		await page.waitForTimeout(30);
	}
	await page.waitForTimeout(300);
	// The slot it would land in shows, dashed, not as a blank block.
	const slot = day(page, 'Wednesday, November 18').locator('li div[aria-hidden="true"]');
	await expect(slot).toBeVisible();
	await cdp.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
	await cdp.detach();
	await expect(day(page, 'Wednesday, November 18').getByText('Linsensuppe')).toBeVisible();
	await page.reload();
	await expect(day(page, 'Wednesday, November 18').getByText('Linsensuppe')).toBeVisible();
});

test('a held finger on a card gets no link menu, a tap opens the recipe', async ({
	page,
	isMobile
}) => {
	test.skip(!isMobile, 'a long press is a touch gesture');
	await page.clock.setFixedTime(new Date('2026-11-16T10:00:00'));
	await loginFresh(page);
	const title = `Klopse ${uniqueToken()}`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title });
	await plan(page, { day: '2026-11-18', recipeId: recipe.id });
	await openOwn(page);
	const link = day(page, 'Wednesday, November 18').getByRole('link', { name: title });
	await link.scrollIntoViewIfNeeded();
	// Two fingers: the guard holds until the last one lifts. (Chromium knows
	// no -webkit-touch-callout, so its style reads empty: ask a menu instead.)
	const blocked = () =>
		page.evaluate(() => {
			const menu = new MouseEvent('contextmenu', { bubbles: true, cancelable: true });
			return !document.body.dispatchEvent(menu);
		});
	const box = (await link.boundingBox())!;
	const at = { x: Math.round(box.x + box.width / 2), y: Math.round(box.y + box.height / 2) };
	const touch = await page.context().newCDPSession(page);
	const send = (type: 'touchStart' | 'touchEnd', touchPoints: { x: number; y: number }[]) =>
		touch.send('Input.dispatchTouchEvent', { type, touchPoints });
	await send('touchStart', [at]);
	await send('touchStart', [at, { x: 20, y: 20 }]);
	await send('touchEnd', [at]);
	expect(await blocked()).toBe(true);
	await send('touchEnd', []);
	expect(await blocked()).toBe(false);
	await touch.detach();

	// The long press reaches the copy the drag lifted into <body>, outside
	// the list: the window has to refuse the menu there.
	await page.evaluate(() => {
		addEventListener('contextmenu', (e) =>
			Object.assign(window, {
				menu: { prevented: e.defaultPrevented, inList: !!(e.target as Element).closest('ul') }
			})
		);
	});
	const cdp = await page.context().newCDPSession(page);
	await cdp.send('Input.synthesizeTapGesture', {
		x: Math.round(box.x + box.width / 2),
		y: Math.round(box.y + box.height / 2),
		duration: 1200,
		gestureSourceType: 'touch'
	});
	// The held press became a drag: the click at its lift is swallowed.
	expect(await clicksHeard(page)).toBe(false);
	await cdp.detach();
	const menu = await page.evaluate(() => (window as unknown as { menu?: object }).menu);
	expect(menu).toEqual({ prevented: true, inList: false });
	await expect(page).toHaveURL(/\/plans\//);

	// The held press was a drag too: the click after it is swallowed for a
	// moment, then a tap is a tap again.
	await expect.poll(() => clicksHeard(page)).toBe(true);
	await link.tap();
	await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
});

test('the click that ends a long drag never opens the recipe, a tap does', async ({
	page,
	isMobile
}) => {
	test.skip(!isMobile, 'a long press is a touch gesture');
	await page.clock.setFixedTime(new Date('2026-11-16T10:00:00'));
	await loginFresh(page);
	const title = `Klopse ${uniqueToken()}`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title });
	await plan(page, { day: '2026-11-17', recipeId: recipe.id });
	await plan(page, { day: '2026-11-18', title: 'Pizza bestellen' });
	await openOwn(page);
	const card = day(page, 'Tuesday, November 17').getByRole('link', { name: title });
	await card.scrollIntoViewIfNeeded();
	await touchDrag(page, card, day(page, 'Wednesday, November 18').getByText('Pizza bestellen'));
	// iOS lifts the finger from a long press as a tap: a trusted click on the
	// link under it, the dropped card's, right after the drop.
	const link = day(page, 'Wednesday, November 18').getByRole('link', { name: title });
	const box = (await link.boundingBox())!;
	await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
	await page.waitForTimeout(600);
	await expect(page).toHaveURL(/\/plans\//);

	// Past the moment after the drop, a tap is a tap again.
	await expect.poll(() => clicksHeard(page)).toBe(true);
	await link.tap();
	await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
});

test('a keyboard drag leaves the clicks alone', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-16T10:00:00'));
	await loginFresh(page);
	const title = `Klopse ${uniqueToken()}`;
	const recipe = await createRecipe(page, { ...loadFixture(0), title });
	await plan(page, { day: '2026-11-18', title: 'Pizza bestellen' });
	await plan(page, { day: '2026-11-18', recipeId: recipe.id });
	await openOwn(page);
	const wednesday = day(page, 'Wednesday, November 18');
	// The library still drags a focused card by Space, and its drop sends no finalize.
	await wednesday.locator('li', { hasText: 'Pizza bestellen' }).focus();
	await page.keyboard.press('Space');
	await page.keyboard.press('ArrowDown');
	await page.keyboard.press('Space');
	const order = () => wednesday.locator('li').allInnerTexts();
	await expect
		.poll(async () => (await order()).map((t) => t.includes(title)))
		.toEqual([true, false]);
	expect(await clicksHeard(page)).toBe(true);
	await wednesday.getByRole('link', { name: title }).click();
	await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
	// The move was a real one: the server has the new order.
	await openOwn(page);
	await expect
		.poll(async () => (await order()).map((t) => t.includes(title)))
		.toEqual([true, false]);
});

test('removes and undoes', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-16T10:00:00'));
	await loginFresh(page);
	await plan(page, { day: '2026-11-18', title: 'Reste vom Sonntag' });
	await openOwn(page);
	const wednesday = day(page, 'Wednesday, November 18');
	await wednesday.getByRole('button', { name: 'Actions for Reste vom Sonntag' }).click();
	await page.getByRole('menuitem', { name: 'Remove' }).click();
	await expect(wednesday.getByText('Reste vom Sonntag')).toHaveCount(0);
	await page.getByRole('button', { name: 'Undo' }).click();
	await expect(wednesday.getByText('Reste vom Sonntag')).toBeVisible();
	await page.reload();
	await expect(day(page, 'Wednesday, November 18').getByText('Reste vom Sonntag')).toBeVisible();
});

test('removing a listed recipe says the list lost it too', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-16T10:00:00'));
	await loginFresh(page);
	const t = uniqueToken();
	const pie = await createRecipe(page, { ...loadFixture(0), title: `Pie ${t}` });
	await plan(page, { day: '2026-11-18', recipeId: pie.id });
	await planOnList(page, await ownPlan(page), '2026-11-16', '2026-11-22', `Week ${t}`);
	await openOwn(page);
	const wednesday = day(page, 'Wednesday, November 18');
	await wednesday.getByRole('button', { name: `Actions for Pie ${t}` }).click();
	await page.getByRole('menuitem', { name: 'Remove' }).click();
	await expect(page.getByText(`Pie ${t} removed – from “Week ${t}” too`)).toBeVisible();
});

test('collapses past days on a phone', async ({ page, isMobile }) => {
	test.skip(!isMobile, 'the desktop shows every day');
	await page.clock.setFixedTime(new Date('2026-11-20T10:00:00'));
	await loginFresh(page);
	await plan(page, { day: '2026-11-16', title: 'Klopse' });
	await plan(page, { day: '2026-11-17', title: 'Rouladen' });
	await plan(page, { day: '2026-11-20', title: 'Spätzle' });
	await openOwn(page);

	const fold = page.getByRole('button', { name: /^Mon–Thu/ });
	await expect(fold).toHaveAccessibleName('Mon–Thu · 2 entries');
	await expect(fold).toHaveAttribute('aria-expanded', 'false');
	await expect(day(page, 'Monday, November 16')).toBeHidden();
	await expect(day(page, 'Friday, November 20').getByText('Spätzle')).toBeVisible();
	await fold.click();
	await expect(fold).toHaveAttribute('aria-expanded', 'true');
	await expect(day(page, 'Monday, November 16').getByText('Klopse')).toBeVisible();

	// Another week shows all its days.
	await page.getByRole('button', { name: 'Previous week' }).click();
	await expect(page.getByRole('button', { name: /^Mon–Thu/ })).toHaveCount(0);
});

// "Put on the list": the dialog over the shown week's rest.

type List = {
	name: string;
	entries: { id: string; title: string; servings: number }[];
	items: { entryId: string | null; name: string }[];
};

const toListButton = (page: Page) =>
	page.getByRole('main').getByRole('button', { name: /^Put on the list/ });

async function listFromToast(page: Page): Promise<List> {
	await page.getByRole('button', { name: 'Open list' }).click();
	await expect(page).toHaveURL(/\/lists\/[^/?]+$/);
	const id = new URL(page.url()).pathname.split('/').pop()!;
	return (await (await page.request.get(`/api/v1/shopping-lists/${id}`)).json()) as List;
}

test('puts the rest of the week on a new list without staples', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-18T10:00:00'));
	await loginFresh(page);
	const t = uniqueToken();
	const pie = await createRecipe(page, { ...loadFixture(0), title: `Pie ${t}` });
	const mash = await createRecipe(page, { ...loadFixture(2), title: `Mash ${t}` });
	const fish = await createRecipe(page, { ...loadFixture(3), title: `Fish ${t}` });
	await plan(page, { day: '2026-11-16', recipeId: fish.id });
	await plan(page, { day: '2026-11-18', recipeId: pie.id, servings: 6 });
	await plan(page, { day: '2026-11-21', recipeId: mash.id, servings: 3 });
	const staple = await page.request.put(`/api/v1/meal-plans/${await ownPlan(page)}/staples`, {
		data: { name: 'salt', staple: true }
	});
	expect(staple.ok()).toBe(true);
	await openOwn(page);

	const dialog = await openDialog(page, toListButton(page), 'Put on the list');
	// Today to Sunday; Monday is behind us.
	await expect(dialog.getByRole('button', { name: /^Days/ })).toContainText(
		'Wed Nov 18 – Sun Nov 22'
	);
	await expect(dialog.getByText(`Pie ${t}`)).toBeVisible();
	await expect(dialog.getByText(`Mash ${t}`)).toBeVisible();
	await expect(dialog.getByText(`Fish ${t}`)).toHaveCount(0);
	// No list yet: a new one, named after the week.
	await expect(dialog.getByRole('textbox', { name: 'Name' })).toHaveValue(
		'Shopping Nov\u00A018–22'
	);

	// The staple starts deselected, everything else picked.
	await dialog.getByRole('button', { name: `Ingredients of Pie ${t}` }).click();
	await expect(
		dialog.getByRole('checkbox', { name: 'salt', exact: true }).first()
	).not.toBeChecked();
	await expect(dialog.getByRole('checkbox', { name: 'onion', exact: true })).toBeChecked();
	// Marked as always at home here, it leaves this recipe and the other one
	// (Mash's "onions" is the same ingredient to the list).
	const mashOnions = dialog.getByRole('checkbox', { name: 'onions', exact: true });
	await dialog.getByRole('button', { name: `Ingredients of Mash ${t}` }).click();
	await expect(mashOnions).toBeChecked();
	await dialog.getByRole('button', { name: 'We always have onion', exact: true }).click();
	await expect(
		dialog.getByRole('button', { name: 'We always have onion', exact: true })
	).toHaveAttribute('aria-pressed', 'true');
	await expect(dialog.getByRole('checkbox', { name: 'onion', exact: true })).not.toBeChecked();
	await expect(mashOnions).not.toBeChecked();
	// The bulk dialog never asks for a second list: the server's skip rule holds.
	const sent = page.waitForRequest(
		(r) => r.method() === 'POST' && /\/meal-plans\/[^/]+\/shopping\?/.test(r.url())
	);
	await dialog.getByRole('button', { name: 'Add 2 recipes' }).click();
	expect((await sent).postDataJSON()).not.toHaveProperty('alsoListed');
	await expect(dialog).toBeHidden();

	const list = await listFromToast(page);
	expect(list.name).toBe('Shopping Nov\u00A018–22');
	expect(list.entries.map((e) => [e.title, e.servings]).sort()).toEqual([
		[`Mash ${t}`, 3],
		[`Pie ${t}`, 6]
	]);
	expect(list.items.some((i) => i.name === 'carrots')).toBe(true);
	expect(list.items.filter((i) => ['salt', 'onion', 'onions'].includes(i.name))).toEqual([]);
	await expect(page.getByRole('main').getByText(`Pie ${t}`).first()).toBeVisible();
});

test('the dialog takes the tray along when it holds recipes on no list', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-18T10:00:00'));
	await loginFresh(page);
	const t = uniqueToken();
	const pie = await createRecipe(page, { ...loadFixture(0), title: `Pie ${t}` });
	await plan(page, { recipeId: pie.id });
	await openOwn(page);
	await expect(toListButton(page)).toHaveAccessibleName('Put on the list, 1 recipe on no list');
	const dialog = await openDialog(page, toListButton(page), 'Put on the list');
	// What the badge counts is in from the start.
	await expect(dialog.getByRole('switch', { name: 'Include Someday' })).toBeChecked();
	await expect(dialog.getByText(`Pie ${t}`)).toBeVisible();
	await expect(dialog.getByRole('button', { name: 'Add 1 recipe' })).toBeEnabled();
});

test('the badge counts what the dialog picks: no past day', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-18T10:00:00'));
	await loginFresh(page);
	const t = uniqueToken();
	const pie = await createRecipe(page, { ...loadFixture(0), title: `Pie ${t}` });
	const mash = await createRecipe(page, { ...loadFixture(2), title: `Mash ${t}` });
	await plan(page, { day: '2026-11-17', recipeId: pie.id });
	await plan(page, { day: '2026-11-20', recipeId: mash.id });
	await page.goto('/plans');
	await expect(page.getByText('1 not on a list yet')).toBeVisible();
	await openOwn(page);
	await expect(toListButton(page)).toHaveAccessibleName('Put on the list, 1 recipe on no list');
	// Only the coming day's card says so; yesterday's is done.
	await expect(page.getByRole('main').getByText('On no list')).toHaveCount(1);
	const dialog = await openDialog(page, toListButton(page), 'Put on the list');
	await expect(dialog.getByText(`Mash ${t}`)).toBeVisible();
	await expect(dialog.getByText(`Pie ${t}`)).toHaveCount(0);
	await expect(dialog.getByRole('button', { name: 'Add 1 recipe' })).toBeEnabled();
});

test('a week gone by starts the dialog on the tray alone', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-25T10:00:00'));
	await loginFresh(page);
	const t = uniqueToken();
	const pie = await createRecipe(page, { ...loadFixture(0), title: `Pie ${t}` });
	const mash = await createRecipe(page, { ...loadFixture(2), title: `Mash ${t}` });
	await plan(page, { day: '2026-11-17', recipeId: pie.id });
	await plan(page, { recipeId: mash.id });
	await page.goto(`/plans/${await ownPlan(page)}?from=2026-11-16`);
	// The tray belongs to the plan, not to a week: it counts; last Tuesday does not.
	await expect(toListButton(page)).toHaveAccessibleName('Put on the list, 1 recipe on no list');
	const dialog = await openDialog(page, toListButton(page), 'Put on the list');
	await expect(dialog.getByRole('button', { name: /^Days/ })).toHaveAccessibleName('Days No days');
	await expect(dialog.getByRole('switch', { name: 'Include Someday' })).toBeChecked();
	// No days, no dates in the name.
	await expect(dialog.getByRole('textbox', { name: 'Name' })).toHaveValue('Shopping');
	await expect(dialog.getByText(`Mash ${t}`)).toBeVisible();
	await expect(dialog.getByText(`Pie ${t}`)).toHaveCount(0);
	await expect(dialog.getByRole('button', { name: 'Add 1 recipe' })).toBeEnabled();
	// Without the tray nothing is picked, and the dialog says what to do.
	await dialog.getByRole('switch', { name: 'Include Someday' }).click();
	await expect(
		dialog.getByText('This week is over. Pick days or take “Someday” along.')
	).toBeVisible();
	await expect(dialog.getByRole('button', { name: 'Put on the list', exact: true })).toBeDisabled();
});

test('a plan left open in view moves on at midnight', async ({ page }) => {
	await page.clock.install({ time: new Date('2026-11-18T23:58:00') });
	await loginFresh(page);
	const t = uniqueToken();
	const pie = await createRecipe(page, { ...loadFixture(0), title: `Pie ${t}` });
	await plan(page, { day: '2026-11-18', recipeId: pie.id });
	await openOwn(page);
	await expect(toListButton(page)).toHaveAccessibleName('Put on the list, 1 recipe on no list');
	await expect(page.getByRole('main').getByText('On no list')).toHaveCount(1);
	await page.clock.fastForward('05:00');
	await expect(toListButton(page)).toHaveAccessibleName('Put on the list');
	await expect(page.getByRole('main').getByText('On no list')).toHaveCount(0);
});

test('the plan moves on to a new day when the reader comes back', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-18T22:00:00'));
	await loginFresh(page);
	const t = uniqueToken();
	const pie = await createRecipe(page, { ...loadFixture(0), title: `Pie ${t}` });
	await plan(page, { day: '2026-11-18', recipeId: pie.id });
	await openOwn(page);
	await expect(toListButton(page)).toHaveAccessibleName('Put on the list, 1 recipe on no list');
	await expect(page.getByRole('main').getByText('On no list')).toHaveCount(1);
	// Past midnight, back on the tab: Wednesday is gone, its recipe needs no shopping.
	await page.clock.setFixedTime(new Date('2026-11-19T07:00:00'));
	await page.evaluate(() => window.dispatchEvent(new Event('focus')));
	await expect(toListButton(page)).toHaveAccessibleName('Put on the list');
	await expect(page.getByRole('main').getByText('On no list')).toHaveCount(0);
});

test('second time adds only new entries', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-16T10:00:00'));
	await loginFresh(page);
	const t = uniqueToken();
	const pie = await createRecipe(page, { ...loadFixture(0), title: `Pie ${t}` });
	const mash = await createRecipe(page, { ...loadFixture(2), title: `Mash ${t}` });
	await plan(page, { day: '2026-11-17', recipeId: pie.id });
	await openOwn(page);

	let dialog = await openDialog(page, toListButton(page), 'Put on the list');
	await dialog.getByRole('button', { name: 'Add 1 recipe' }).click();
	await expect(dialog).toBeHidden();
	await expect(toListButton(page)).toHaveAccessibleName('Put on the list');

	await plan(page, { day: '2026-11-19', recipeId: mash.id });
	await page.reload();
	await expect(toListButton(page)).toHaveAccessibleName('Put on the list, 1 recipe on no list');
	dialog = await openDialog(page, toListButton(page), 'Put on the list');
	// The list the plan went to last; the recipe on it is left out and said so.
	await expect(dialog.getByRole('button', { name: 'List', exact: true })).toHaveText(
		'Shopping Nov\u00A016–22'
	);
	await expect(dialog.getByText('1 recipe is already on “Shopping Nov\u00A016–22”')).toBeVisible();
	await expect(dialog.getByText(`Pie ${t}`)).toHaveCount(0);
	await dialog.getByRole('button', { name: 'Add 1 recipe' }).click();
	await expect(dialog).toBeHidden();

	const list = await listFromToast(page);
	expect(list.entries.map((e) => e.title).sort()).toEqual([`Mash ${t}`, `Pie ${t}`]);
});

test('a changed range waits for its recipes before it goes on a list', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-18T10:00:00'));
	await loginFresh(page);
	const t = uniqueToken();
	const pie = await createRecipe(page, { ...loadFixture(0), title: `Pie ${t}` });
	const mash = await createRecipe(page, { ...loadFixture(2), title: `Mash ${t}` });
	await plan(page, { day: '2026-11-18', recipeId: pie.id });
	await plan(page, { day: '2026-11-23', recipeId: mash.id, servings: 4 });
	await openOwn(page);

	const dialog = await openDialog(page, toListButton(page), 'Put on the list');
	const submit = dialog.getByRole('button', { name: 'Add 1 recipe' });
	await expect(submit).toBeEnabled();
	await expect(dialog.getByRole('textbox', { name: 'Name' })).toHaveValue(
		'Shopping Nov\u00A018–22'
	);

	// Escape in the calendar folds the calendar, not the dialog.
	const days = dialog.getByRole('button', { name: /^Days/ });
	await days.click();
	await expect(days).toHaveAttribute('aria-expanded', 'true');
	await dialog.locator('[data-bits-day][data-value="2026-11-20"]').focus();
	await page.keyboard.press('Escape');
	await expect(dialog).toBeVisible();
	await expect(days).toHaveAttribute('aria-expanded', 'false');
	await expect(days).toBeFocused();

	// Next week: until its recipes are in, the old ones cannot go on the list.
	let release = () => {};
	const held = new Promise<void>((resolve) => (release = resolve));
	await page.route('**/shopping-preview**', async (route) => {
		await held;
		await route.continue();
	});
	await days.click();
	await dialog.locator('[data-bits-day][data-value="2026-11-23"]').click();
	await dialog.locator('[data-bits-day][data-value="2026-11-29"]').click();
	await expect(days).toContainText('Mon Nov 23 – Sun Nov 29');
	await expect(submit).toBeDisabled();
	await expect(dialog.getByText(`Pie ${t}`)).toBeVisible();
	release();
	await expect(dialog.getByText(`Mash ${t}`)).toBeVisible();
	await expect(dialog.getByText(`Pie ${t}`)).toHaveCount(0);
	// Out of the shown week, a row says its date; the new list's name follows the days.
	await expect(dialog.getByText('Mon Nov 23 · 4 servings')).toBeVisible();
	await expect(dialog.getByRole('textbox', { name: 'Name' })).toHaveValue(
		'Shopping Nov\u00A023–29'
	);
	await expect(submit).toBeEnabled();

	// With one end picked, the calendar offers 62 days at most: the server's limit.
	await days.click();
	await dialog.locator('[data-bits-day][data-value="2026-11-18"]').click();
	const next = dialog.getByRole('button', { name: 'Next', exact: true });
	await next.click();
	await next.click();
	await expect(dialog.locator('[data-bits-day][data-value="2027-01-18"]')).not.toHaveAttribute(
		'data-disabled'
	);
	await expect(dialog.locator('[data-bits-day][data-value="2027-01-19"]')).toHaveAttribute(
		'data-disabled'
	);
});

test('plans a recipe from its page for Wednesday at 6 servings', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-16T10:00:00'));
	await loginFresh(page);
	const title = `Frühstück ${uniqueToken()}`;
	const recipe = await createRecipe(page, { ...loadFixture(1), title });
	await page.goto(`/recipes/${recipe.slug}`);
	const stepper = page.getByRole('group', { name: 'Servings' }).first();
	for (let i = 0; i < 4; i++) await stepper.getByRole('button', { name: 'More servings' }).click();
	await expect(stepper.getByText('6', { exact: true })).toBeVisible();

	const dialog = await openDialog(
		page,
		page.getByRole('button', { name: 'Add to plan' }),
		'Add to plan'
	);
	await expect(dialog.getByRole('radio', { name: 'Monday, November 16, Today' })).toBeChecked();
	await expect(dialog.getByRole('radio', { name: 'Sunday, November 29' })).toBeVisible();
	await expect(
		dialog.getByRole('group', { name: 'Servings' }).getByText('6', { exact: true })
	).toBeVisible();
	await dialog.getByRole('radio', { name: 'Wednesday, November 18' }).check();
	await dialog.getByRole('button', { name: 'Add to plan' }).click();
	await expect(dialog).toBeHidden();

	await page.getByRole('button', { name: 'Open plan' }).click();
	await expect(page).toHaveURL(/\/plans\/[^?]+$/);
	await expect(
		day(page, 'Wednesday, November 18').getByRole('link', { name: title })
	).toHaveAttribute('href', `/recipes/${recipe.slug}?servings=6`);
});

// Named plans: made, shared, renamed, deleted and left like lists.

/** The shown plan's name, the plan page's h1. */
const planTitle = (page: Page) => page.getByRole('main').getByRole('heading', { level: 1 });

/** A plan's card or chip on the overview, by the link that opens it. */
const planLink = (page: Page, name: string) =>
	page.getByRole('main').getByRole('link', { name: new RegExp(`^${name}`) });

/** The plans overview, where a plan gone for the reader leads. */
const overview = (page: Page) =>
	page.getByRole('main').getByRole('heading', { level: 2, name: 'Your plans' });

/** Two fresh people, `owner` signed in on `page`; the admin makes them. */
async function twoPeople(page: Page) {
	const token = uniqueToken();
	const owner = `pown${token}`.slice(0, 20);
	const guest = `pgst${token}`.slice(0, 20);
	await login(page);
	await createUser(page, { username: owner, role: 'user' });
	const made = await createUser(page, { username: guest, role: 'user' });
	await switchTo(page, owner);
	return { token, owner, guest, guestId: made.id };
}

/** A named plan of the signed-in person, made through the API. */
async function namedPlan(page: Page, name: string, invite?: string): Promise<string> {
	const made = await page.request.post('/api/v1/meal-plans', { data: { name } });
	expect(made.ok(), await made.text()).toBe(true);
	const { id } = (await made.json()) as { id: string };
	if (invite) {
		const sent = await page.request.post(`/api/v1/meal-plans/${id}/invitations`, {
			data: { userId: invite }
		});
		expect(sent.ok(), await sent.text()).toBe(true);
	}
	return id;
}

async function signedInAs(browser: Browser, name: string) {
	const context = await browser.newContext();
	const page = await context.newPage();
	await login(page, name);
	return { context, page };
}

test('creates a named plan and invites someone', async ({ page, browser }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'two browsers; runs once, on desktop');
	const { token, owner, guest } = await twoPeople(page);
	const name = `Household ${token}`;
	await page.goto('/plans');

	// "New plan" is the overview's action; the draft names it, Enter makes it.
	await page.getByRole('banner').getByRole('button', { name: 'New plan' }).click();
	await expect(page).toHaveURL(/\/plans\/new$/);
	const field = page.getByRole('textbox', { name: 'Name' });
	await expect(field).toBeFocused();
	await field.fill(name);
	await field.press('Enter');
	await expect(page).toHaveURL(/\/plans\/[^/?]+$/);
	await expect(planTitle(page)).toHaveText(name);
	// Created, the plan offers its sharing right away.
	await expect(page.getByRole('button', { name: 'Share', exact: true })).toBeFocused();

	// The guest has the week open before the invitation exists.
	const other = await signedInAs(browser, guest);
	try {
		const them = other.page;
		await them.goto('/plans');
		await expect(overview(them)).toBeVisible();
		await expect(them.getByText(/invites you to the plan/)).toHaveCount(0);

		await page.getByRole('button', { name: 'Share', exact: true }).click();
		const dialog = page.getByRole('dialog', { name: `Share “${name}”` });
		await dialog.getByRole('combobox', { name: 'Invite people' }).click();
		await dialog.getByRole('option', { name: `@${guest}` }).click();
		await dialog.getByRole('button', { name: 'Invite 1 person' }).click();
		await expect(dialog.getByText('Waiting for an answer')).toBeVisible();
		// The answer replaced the plan; the focus stays in the dialog.
		await expect(dialog.getByRole('button', { name: 'Share', exact: true })).toHaveCount(0);
		await expect(page.locator('[data-plan-share]')).not.toBeFocused();
		await dialog.getByRole('button', { name: 'Done' }).filter({ visible: true }).first().click();
		await expect(dialog).toBeHidden();

		// Back on the tab, the open overview shows the new invitation's card.
		await them.evaluate(() => document.dispatchEvent(new Event('visibilitychange')));
		await expect(side(them, 'Week')).toHaveAccessibleName(/1 invitation/);
		await expect(them.getByText(`${owner} invites you to the plan “${name}”`)).toBeVisible();
		await them.getByRole('button', { name: 'Accept' }).click();
		await expect(planTitle(them)).toHaveText(name);
		await expect(placeLink(them)).not.toHaveAccessibleName(/invitation/);
		await expect(them.getByRole('button', { name: 'Share', exact: true })).toHaveCount(0);

		// One plans, the other sees it without a reload.
		const sheet = await openDialog(
			page,
			page.getByRole('main').getByRole('button', { name: 'Plan a meal' }),
			'Plan a meal'
		);
		await sheet.getByRole('combobox', { name: 'Recipe or note' }).fill('Pizza bestellen');
		await sheet.getByRole('option', { name: 'Add “Pizza bestellen” as a note' }).click();
		await expect(sheet).toBeHidden();
		await expect(page.getByRole('main').getByText('Pizza bestellen')).toBeVisible();
		await expect(them.getByRole('main').getByText('Pizza bestellen')).toBeVisible();

		// The share button took the focus once, on arrival: a change arriving
		// later (here the guest's, over the stream) leaves the focus where it is.
		const menu = page.getByRole('main').getByRole('button', { name: 'More actions' });
		await menu.focus();
		const id = new URL(page.url()).pathname.split('/').pop();
		const added = await them.request.post(`/api/v1/meal-plans/${id}/entries`, {
			data: { title: 'Suppe' }
		});
		expect(added.ok()).toBe(true);
		await expect(page.getByRole('main').getByText('Suppe')).toBeVisible();
		await page.waitForTimeout(300);
		await expect(menu).toBeFocused();

		// The faces show who is on it.
		await them.getByRole('button', { name: '2 people on this plan' }).click();
		const people = them.getByRole('dialog', { name: 'Shared with' });
		await expect(people.getByText('You · can plan and change')).toBeVisible();
		await people.getByRole('button', { name: 'Done' }).last().click();

		// Removed by the owner, the open plan says so and goes to the overview.
		await page.getByRole('button', { name: 'Share', exact: true }).click();
		await page
			.getByRole('dialog')
			.getByRole('button', { name: `Remove ${guest}` })
			.click();
		await expect(page.getByRole('dialog').getByText(guest)).toHaveCount(0);
		await expect(them.getByText('You no longer have access to this plan.')).toBeVisible();
		await expect(them).toHaveURL(/\/plans$/);
		await expect(overview(them)).toBeVisible();
	} finally {
		await other.context.close();
	}
});

test('personal plan has no share or rename', async ({ page }) => {
	await loginFresh(page);
	await openOwn(page);
	await expect(planTitle(page)).toHaveText('My plan');
	const main = page.getByRole('main');
	await expect(main.getByRole('button', { name: 'Share', exact: true })).toHaveCount(0);
	// Its ⋯ holds only the calendar, the same feed every plan offers.
	await main.getByRole('button', { name: 'More actions' }).click();
	await expect(page.getByRole('menuitem')).toHaveText(['Subscribe in calendar']);
	await page.keyboard.press('Escape');
	// Nor does the API rename it.
	const own = await ownPlan(page);
	const renamed = await page.request.patch(`/api/v1/meal-plans/${own}`, { data: { name: 'X' } });
	expect(renamed.status()).toBe(422);
});

test('the owner renames and deletes a plan', async ({ page }) => {
	await loginFresh(page);
	const name = `Holiday ${uniqueToken()}`;
	const id = await namedPlan(page, name);
	// Opened from the overview, so Back after the delete has somewhere to go.
	await page.goto('/plans');
	await planLink(page, name).click();
	await expect(page).toHaveURL(new RegExp(`/plans/${id}$`));

	await page.getByRole('main').getByRole('button', { name: 'More actions' }).click();
	await expect(page.getByRole('menuitem')).toHaveText([
		'Rename',
		'Share',
		'Subscribe in calendar',
		'Delete plan'
	]);
	await page.getByRole('menuitem', { name: 'Rename' }).click();
	const rename = page.getByRole('dialog', { name: 'Rename' });
	await rename.getByLabel('Name').fill(`${name} 2`);
	await rename.getByRole('button', { name: 'Save' }).click();
	await expect(planTitle(page)).toHaveText(`${name} 2`);

	await page.getByRole('main').getByRole('button', { name: 'More actions' }).click();
	await page.getByRole('menuitem', { name: 'Delete plan' }).click();
	await page.getByRole('dialog').getByRole('button', { name: 'Delete' }).click();
	await expect(page.getByText(`“${name} 2” deleted`)).toBeVisible();
	// Opened from the overview, the plan goes back to it, focus on its heading.
	await expect(page).toHaveURL(/\/plans$/);
	await expect(overview(page)).toBeFocused();
	await expect(planLink(page, `${name} 2`)).toHaveCount(0);
	// The deleted plan left the history: Back never reopens it.
	await page.goBack();
	await expect(page).not.toHaveURL(new RegExp(id));
	await page.waitForTimeout(300);
	await expect(page.getByText('You no longer have access to this plan.')).toHaveCount(0);
});

test('a plan deleted when opened directly replaces itself with the overview', async ({ page }) => {
	await loginFresh(page);
	const name = `Direct ${uniqueToken()}`;
	const id = await namedPlan(page, name);
	await page.goto('/lists');
	await page.goto(`/plans/${id}`);
	await page.getByRole('main').getByRole('button', { name: 'More actions' }).click();
	await page.getByRole('menuitem', { name: 'Delete plan' }).click();
	await page.getByRole('dialog').getByRole('button', { name: 'Delete' }).click();
	await expect(page).toHaveURL(/\/plans$/);
	await expect(overview(page)).toBeFocused();
	await page.goBack();
	await expect(page).toHaveURL(/\/lists$/);
	await page.waitForTimeout(300);
	await expect(page.getByText('You no longer have access to this plan.')).toHaveCount(0);
});

test('member leaves a plan', async ({ page }) => {
	const { token, guest, guestId } = await twoPeople(page);
	const name = `Family ${token}`;
	const id = await namedPlan(page, name, guestId);
	await switchTo(page, guest);
	const accepted = await page.request.post(`/api/v1/meal-plans/${id}/membership`);
	expect(accepted.ok()).toBe(true);
	await page.goto('/plans');
	await planLink(page, name).click();
	await expect(planTitle(page)).toHaveText(name);

	// A member renames and leaves; sharing and deleting are the owner's.
	await expect(page.getByRole('main').getByRole('button', { name: 'Share' })).toHaveCount(0);
	await page.getByRole('main').getByRole('button', { name: 'More actions' }).click();
	await expect(page.getByRole('menuitem')).toHaveText([
		'Rename',
		'Subscribe in calendar',
		'Leave plan'
	]);
	await page.getByRole('menuitem', { name: 'Leave plan' }).click();
	const confirm = page.getByRole('dialog', { name: `Leave the plan “${name}”?` });
	await confirm.getByRole('button', { name: 'Leave plan' }).click();
	await expect(page.getByText(`You left “${name}”`)).toBeVisible();
	await expect(page).toHaveURL(/\/plans$/);
	await expect(planLink(page, name)).toHaveCount(0);
	await page.goBack();
	await expect(page).not.toHaveURL(new RegExp(id));
	await page.waitForTimeout(300);
	await expect(page.getByText('You no longer have access to this plan.')).toHaveCount(0);
});

test('invitation dot shows on the place and the Woche segment', async ({ page }) => {
	const { token, owner, guest, guestId } = await twoPeople(page);
	const name = `Weekend ${token}`;
	await namedPlan(page, name, guestId);
	await switchTo(page, guest);

	await page.goto('/lists');
	await expect(placeLink(page)).toHaveAccessibleName(/1 invitation/);
	await expect(side(page, 'Week')).toHaveAccessibleName(/1 invitation/);
	await expect(side(page, 'Lists')).not.toHaveAccessibleName(/invitation/);

	// The card waits on the week; declined, every dot is gone.
	await side(page, 'Week').click();
	await expect(page.getByText(`${owner} invites you to the plan “${name}”`)).toBeVisible();
	await page.getByRole('button', { name: 'Decline' }).click();
	await expect(page.getByText(`invites you to the plan “${name}”`)).toHaveCount(0);
	await expect(side(page, 'Week')).not.toHaveAccessibleName(/invitation/);
	await expect(placeLink(page)).not.toHaveAccessibleName(/invitation/);
});

// The list side: the planned day, the way back to the week, staples from the list.

/** Every recipe of the plan's `from`–`to` on a new list, all ingredients picked, through the API. */
async function planOnList(page: Page, planId: string, from: string, to: string, name: string) {
	const range = `from=${from}&to=${to}`;
	const preview = (await (
		await page.request.get(`/api/v1/meal-plans/${planId}/shopping-preview?${range}&tray=false`)
	).json()) as { entries: { entryId: string; ingredients: { group: number; index: number }[] }[] };
	const picks = preview.entries.map((e) => ({
		entryId: e.entryId,
		ingredients: e.ingredients.map(({ group, index }) => ({ group, index }))
	}));
	const made = await page.request.post(`/api/v1/meal-plans/${planId}/shopping?${range}`, {
		data: { newListName: name, picks }
	});
	expect(made.ok(), await made.text()).toBe(true);
	return ((await made.json()) as { listId: string }).listId;
}

test('a planned block wears its plan stamp, which jumps to the meal', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-18T10:00:00'));
	await loginFresh(page);
	const t = uniqueToken();
	const fish = await createRecipe(page, { ...loadFixture(3), title: `Fish ${t}` });
	const pie = await createRecipe(page, { ...loadFixture(0), title: `Pie ${t}` });
	const mash = await createRecipe(page, { ...loadFixture(2), title: `Mash ${t}` });
	await plan(page, { day: '2026-11-16', recipeId: fish.id });
	await plan(page, { day: '2026-11-18', recipeId: pie.id });
	await plan(page, { day: '2026-11-25', recipeId: mash.id });
	const planId = await ownPlan(page);
	const listId = await planOnList(page, planId, '2026-11-16', '2026-11-29', `Week ${t}`);

	await page.goto(`/lists/${listId}`);
	// By department, each source line names the day; beyond this week, the date too.
	await page.getByRole('radio', { name: 'Department' }).click();
	const main = page.getByRole('main');
	await expect(main.getByText(`Wed · Pie ${t}`).first()).toBeVisible();
	await expect(main.getByText(`Wed 25 · Mash ${t}`).first()).toBeVisible();
	// By recipe, the block's stamp names the plan and the day, kind first.
	await page.getByRole('radio', { name: 'Recipe' }).click();
	await expect(main.getByRole('heading', { level: 2, name: `Pie ${t}` })).toBeVisible();
	const head = main
		.getByRole('list', { name: 'From plans' })
		.getByRole('link', { name: 'My plan' });
	await expect(head).toHaveAttribute('href', `/plans/${planId}?day=2026-11-16`);
	await main.getByRole('link', { name: 'My plan, Wednesday, November 25' }).click();
	await expect(page).toHaveURL(`/plans/${planId}?from=2026-11-23`);
	await expect(page.locator('[data-entry-id]', { hasText: `Mash ${t}` })).toBeInViewport();

	// The head's stamp, by keyboard, lands on the plan's first day, which takes
	// the focus and shows it: `toBeFocused` alone passes with no ring drawn.
	await page.goBack();
	await head.focus();
	await page.keyboard.press('Enter');
	await expect(page).toHaveURL(`/plans/${planId}`);
	const monday = page.locator('[data-day="2026-11-16"]');
	await expect(monday).toBeFocused();
	expect(
		await monday.evaluate((el) => el.matches(':focus-visible') && getComputedStyle(el).outlineStyle)
	).not.toMatch(/^(none|false)$/);

	// Monday is gone by: a phone folds it, and the jump opens the fold.
	await page.goBack();
	await main.getByRole('link', { name: 'My plan, Monday, November 16' }).click();
	await expect(page).toHaveURL(`/plans/${planId}`);
	const card = page.locator('[data-entry-id]', { hasText: `Fish ${t}` });
	await expect(card).toBeInViewport();
	await expect(card.getByRole('link', { name: `Fish ${t}` })).toBeFocused();
});

test('a list member outside the plan sees who planned it, and no way to the plan', async ({
	page,
	browser
}, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'two browsers; runs once, on desktop');
	await page.clock.setFixedTime(new Date('2026-11-18T10:00:00'));
	const { token, owner, guest, guestId } = await twoPeople(page);
	const pie = await createRecipe(page, { ...loadFixture(0), title: `Pie ${token}` });
	await plan(page, { day: '2026-11-18', recipeId: pie.id });
	const listId = await planOnList(
		page,
		await ownPlan(page),
		'2026-11-16',
		'2026-11-22',
		`W ${token}`
	);
	const invited = await page.request.post(`/api/v1/shopping-lists/${listId}/invitations`, {
		data: { userId: guestId }
	});
	expect(invited.ok()).toBe(true);

	const other = await signedInAs(browser, guest);
	try {
		await other.page.clock.setFixedTime(new Date('2026-11-18T10:00:00'));
		expect(
			(await other.page.request.post(`/api/v1/shopping-lists/${listId}/membership`)).ok()
		).toBe(true);
		await other.page.goto(`/lists/${listId}`);
		await other.page.getByRole('radio', { name: 'Recipe' }).click();
		const main = other.page.getByRole('main');
		await expect(main.getByRole('heading', { level: 2, name: `Pie ${token}` })).toBeVisible();
		// Not the plan's name ("My plan" is the owner's), and no link that would be a 404.
		await expect(main.getByText(`Planned by ${owner}`).first()).toBeVisible();
		await expect(main.getByText('My plan')).toHaveCount(0);
		await expect(main.getByRole('link', { name: /^Plan / })).toHaveCount(0);
	} finally {
		await other.context.close();
	}
});

test("a card's list stamp lands on the meal's block, its title in view", async ({
	page,
	isMobile
}) => {
	if (isMobile) await page.setViewportSize({ width: 360, height: 780 });
	await page.clock.setFixedTime(new Date('2026-11-16T10:00:00'));
	await loginFresh(page);
	const t = uniqueToken();
	// A block taller than the screen, below another: centered, its title went out of view.
	const breakfast = await createRecipe(page, { ...loadFixture(1), title: `Breakfast ${t}` });
	const pie = await createRecipe(page, { ...loadFixture(0), title: `Pie ${t}` });
	await plan(page, { day: '2026-11-17', recipeId: breakfast.id });
	await plan(page, { day: '2026-11-18', recipeId: pie.id });
	const planId = await ownPlan(page);
	const name = `Jump ${t.slice(-4)}`;
	await planOnList(page, planId, '2026-11-16', '2026-11-22', name);
	await page.goto(`/plans/${planId}`);
	const card = page.locator('[data-entry-id]', { hasText: `Pie ${t}` });
	const stamp = card.getByRole('link', { name: `List ${name}` });
	await stamp.scrollIntoViewIfNeeded();
	if (isMobile) await stamp.tap();
	else await stamp.click();

	const title = page.getByRole('main').getByRole('heading', { level: 2, name: `Pie ${t}` });
	await expect(title).toBeFocused();
	// Lit once the scroll has settled; then the title is on screen, without a ring of its own.
	await expect(page.locator('[data-lit]')).toHaveCount(1);
	await expect(title).toBeInViewport({ ratio: 1 });
	expect(await title.evaluate((h) => getComputedStyle(h).outlineStyle)).toBe('none');
});

test('one meal goes on a second list from its menu', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-18T10:00:00'));
	await loginFresh(page);
	const t = uniqueToken();
	// Short list names: two fit a desktop column's card whole, so the card shows them as links.
	const a = `A ${t.slice(-3)}`;
	const b = `B ${t.slice(-3)}`;
	const pie = await createRecipe(page, { ...loadFixture(0), title: `Pie ${t}` });
	await plan(page, { day: '2026-11-19', recipeId: pie.id });
	const planId = await ownPlan(page);
	const first = await planOnList(page, planId, '2026-11-16', '2026-11-22', a);
	const made = await page.request.post('/api/v1/shopping-lists', { data: { name: b } });
	expect(made.ok(), await made.text()).toBe(true);
	const second = ((await made.json()) as { id: string }).id;
	await page.goto(`/plans/${planId}`);

	await page.getByRole('button', { name: `Actions for Pie ${t}` }).click();
	await page.getByRole('menuitem', { name: 'Put on another list…' }).click();
	const dialog = page.getByRole('dialog', { name: 'Put on another list' });
	// Only the list it is not on yet, and where it is already.
	await expect(dialog.getByRole('button', { name: 'List', exact: true })).toHaveText(b);
	await expect(dialog.getByText(`Already on “${a}”`)).toBeVisible();
	await expect(dialog.getByRole('checkbox', { name: `Pie ${t}`, exact: true })).toHaveCount(0);
	await dialog.getByRole('button', { name: `Put on “${b}”` }).click();
	await expect(dialog).toBeHidden();

	// The card and the week's head show both lists, each a link.
	const card = page.locator('[data-entry-id]', { hasText: `Pie ${t}` });
	const entryId = (await card.getAttribute('data-entry-id'))!;
	await expect(card.getByRole('link', { name: `List ${a}` })).toHaveAttribute(
		'href',
		`/lists/${first}?entry=${entryId}`
	);
	await expect(card.getByRole('link', { name: `List ${b}` })).toBeVisible();
	await expect(
		page.getByRole('list', { name: 'Lists of this plan' }).getByRole('link', { name: /^List B / })
	).toBeVisible();
	for (const id of [first, second]) {
		const list = (await (await page.request.get(`/api/v1/shopping-lists/${id}`)).json()) as List;
		expect(list.entries.map((e) => e.title)).toEqual([`Pie ${t}`]);
	}

	// Both lists name the plan the meal came from.
	await page.goto(`/lists/${second}`);
	await page.getByRole('radio', { name: 'Recipe' }).click();
	await expect(
		page.getByRole('main').getByRole('link', { name: 'My plan, Thursday, November 19' })
	).toBeVisible();
	// The reader's own view from here on is by department.
	await page.getByRole('radio', { name: 'Department' }).click();
	// The menu leads to each list.
	await page.goto(`/plans/${planId}`);
	await page.getByRole('button', { name: `Actions for Pie ${t}` }).click();
	await page.getByRole('menuitem', { name: `Open “${a}”` }).click();
	// The list opens at the meal's block, by recipe, its heading focused.
	await expect(page).toHaveURL(`/lists/${first}`);
	await expect(page.getByRole('radio', { name: 'Recipe' })).toBeChecked();
	await expect(
		page.getByRole('main').getByRole('heading', { level: 2, name: `Pie ${t}` })
	).toBeFocused();
	// The recipe view was for that visit: the next list is by department again.
	await page.goBack();
	await page
		.getByRole('list', { name: 'Lists of this plan' })
		.getByRole('link', { name: /^List B / })
		.click();
	await expect(page).toHaveURL(`/lists/${second}`);
	await expect(page.getByRole('radio', { name: 'Department' })).toBeChecked();
});

test('creates and replaces the calendar link', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-18T10:00:00'));
	await loginFresh(page);
	const t = uniqueToken();
	await plan(page, { day: '2026-11-19', title: `Curry ${t}` });
	await page
		.context()
		.grantPermissions(['clipboard-read', 'clipboard-write'])
		.catch(() => {});
	await openOwn(page);

	const open = async () => {
		await page.getByRole('main').getByRole('button', { name: 'More actions' }).click();
		await page.getByRole('menuitem', { name: 'Subscribe in calendar' }).click();
		const dialog = page.getByRole('dialog', { name: 'Subscribe in calendar' });
		await expect(dialog).toBeVisible();
		return dialog;
	};
	let dialog = await open();
	await dialog.getByRole('button', { name: 'Create link' }).click();
	const field = dialog.getByRole('textbox', { name: 'Your calendar link' });
	const first = await field.inputValue();
	expect(first).toMatch(/^http:\/\/[^/]+\/calendar\/[\w-]+\.ics$/);
	// Focus lands on Copy, and the copy says so.
	const copy = dialog.getByRole('button', { name: 'Copy' });
	await expect(copy).toBeFocused();
	await copy.click();
	await expect(page.getByText('Calendar link copied')).toBeVisible();
	const feed = await page.request.get(first);
	expect(feed.status()).toBe(200);
	expect(feed.headers()['content-type']).toContain('text/calendar');
	expect(await feed.text()).toContain(`SUMMARY:Curry ${t}`);
	await dialog.getByRole('button', { name: 'Done' }).last().click();
	await expect(dialog).toBeHidden();

	// Opened again: the link is not shown again, the subscription is.
	dialog = await open();
	await expect(dialog.getByText(/^Link active since /)).toBeVisible();
	await expect(dialog.getByRole('textbox')).toHaveCount(0);
	await dialog.getByRole('button', { name: 'Make a new link' }).click();
	const confirm = page.getByRole('dialog', { name: 'Make a new link?' });
	await confirm.getByRole('button', { name: 'Make a new link' }).click();
	const second = await dialog.getByRole('textbox', { name: 'Your calendar link' }).inputValue();
	expect(second).not.toBe(first);
	expect((await page.request.get(first)).status()).toBe(404);
	expect((await page.request.get(second)).status()).toBe(200);

	// Ended: the link is gone and the dialog offers a new one.
	await dialog.getByRole('button', { name: 'End subscription' }).click();
	await page
		.getByRole('dialog', { name: 'End the subscription?' })
		.getByRole('button', { name: 'End subscription' })
		.click();
	await expect(page.getByText('Subscription ended')).toBeVisible();
	await expect(dialog.getByRole('button', { name: 'Create link' })).toBeFocused();
	expect((await page.request.get(second)).status()).toBe(404);
});

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
	await expect(page).toHaveURL(/\/week$/);
	await expect(page.getByRole('heading', { level: 1, name: 'Shopping & week' })).toBeVisible();
	await expect(side(page, 'Week')).toBeChecked();
	await expect(page.getByRole('heading', { level: 2, name: /Week \d+ · |\d/ })).toBeVisible();
	if (!phone(page)) {
		// The week's own action replaces New list.
		const bar = page.getByRole('banner');
		await expect(bar.getByRole('button', { name: 'Plan a meal' })).toBeVisible();
		await expect(bar.getByRole('button', { name: 'New list' })).toHaveCount(0);
	}

	// The place reopens on the week, also after a reload.
	await page.goto('/');
	await expect(placeLink(page)).toHaveAttribute('href', '/week');
	await placeLink(page).click();
	await expect(page).toHaveURL(/\/week$/);
	await expect(side(page, 'Week')).toBeChecked();

	await side(page, 'Lists').click();
	await expect(page).toHaveURL(/\/lists$/);
	await expect(placeLink(page)).toHaveAttribute('href', '/lists');
});

test('the pager moves by weeks and Today comes back', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-20T10:00:00'));
	await loginFresh(page);
	await page.goto('/week');
	const label = page.getByRole('heading', { level: 2 }).first();
	await expect(label).toContainText(/November 16–22|Nov 16–22/);
	await page.getByRole('button', { name: 'Next week' }).click();
	await expect(page).toHaveURL(/\/week\?from=2026-11-23$/);
	await expect(label).toContainText(/November 23–29|Nov 23–29/);
	await page.getByRole('button', { name: 'Today' }).click();
	await expect(page).toHaveURL(/\/week$/);
	await page.getByRole('button', { name: 'Previous week' }).click();
	await expect(page).toHaveURL(/\/week\?from=2026-11-09$/);
	await expect(
		page.getByRole('heading', { level: 2, name: 'Nothing planned this week' })
	).toBeVisible();
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
	await expect(page).toHaveURL(/\/week$/);
	await expect(side(page, 'Week')).toBeChecked();
	await expect(side(page, 'Week')).toBeFocused();
	await page.keyboard.press('ArrowLeft');
	await expect(page).toHaveURL(/\/lists$/);
	await expect(side(page, 'Lists')).toBeFocused();
});

test('the picker switches to a shared plan and back to the own one', async ({ page }) => {
	await loginFresh(page);
	const name = `Familie ${uniqueToken()}`;
	const made = await page.request.post('/api/v1/meal-plans', { data: { name } });
	expect(made.ok()).toBe(true);
	const shared = (await made.json()) as { id: string };
	const overview = (await (await page.request.get('/api/v1/meal-plans')).json()) as {
		plans: { id: string; personal: boolean }[];
	};
	const own = overview.plans.find((p) => p.personal)!.id;
	await page.goto('/week');
	const picker = page.getByRole('button', { name: 'Plan', exact: true });
	await expect(picker).toHaveText('My plan');

	await picker.click();
	await page.getByRole('option', { name }).click();
	await expect(page).toHaveURL(new RegExp(`/week\\?plan=${shared.id}$`));
	await expect(picker).toHaveText(name);

	await picker.click();
	await page.getByRole('option', { name: 'My plan' }).click();
	await expect(page).toHaveURL(new RegExp(`/week\\?plan=${own}$`));
	await expect(picker).toHaveText('My plan');
	// And the week reopens on the plan picked last.
	await page.goto('/week');
	await expect(picker).toHaveText('My plan');
});

// The week view. Entries are arranged through the API; what is tested is the page.

type Plan = { id: string; entries: { id: string; title: string; day: string | null }[] };

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

test('plans a recipe and a note on a day', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-16T10:00:00'));
	await loginFresh(page);
	const title = `Curry ${uniqueToken()}`;
	await createRecipe(page, { ...loadFixture(1), title });
	await page.goto('/week');

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
	await page.goto('/week');

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
	await page.goto('/week');
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

/** Takes `name` from the open entry menu by keyboard, once the menu holds the focus. */
async function pickByKey(page: Page, name: string) {
	const menu = page.getByRole('menu');
	await expect.poll(() => menu.evaluate((el) => el.contains(document.activeElement))).toBe(true);
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
	await page.goto('/week');

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
	await page.goto('/week');
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
	await page.goto('/week');

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

test('removes and undoes', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-16T10:00:00'));
	await loginFresh(page);
	await plan(page, { day: '2026-11-18', title: 'Reste vom Sonntag' });
	await page.goto('/week');
	const wednesday = day(page, 'Wednesday, November 18');
	await wednesday.getByRole('button', { name: 'Actions for Reste vom Sonntag' }).click();
	await page.getByRole('menuitem', { name: 'Remove' }).click();
	await expect(wednesday.getByText('Reste vom Sonntag')).toHaveCount(0);
	await page.getByRole('button', { name: 'Undo' }).click();
	await expect(wednesday.getByText('Reste vom Sonntag')).toBeVisible();
	await page.reload();
	await expect(day(page, 'Wednesday, November 18').getByText('Reste vom Sonntag')).toBeVisible();
});

test('collapses past days on a phone', async ({ page, isMobile }) => {
	test.skip(!isMobile, 'the desktop shows every day');
	await page.clock.setFixedTime(new Date('2026-11-20T10:00:00'));
	await loginFresh(page);
	await plan(page, { day: '2026-11-16', title: 'Klopse' });
	await plan(page, { day: '2026-11-17', title: 'Rouladen' });
	await plan(page, { day: '2026-11-20', title: 'Spätzle' });
	await page.goto('/week');

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

test('pages weeks and keeps today marked', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-20T10:00:00'));
	await loginFresh(page);
	await plan(page, { day: '2026-11-21', title: 'Pizza bestellen' });
	await page.goto('/week');
	const friday = day(page, 'Friday, November 20');
	await expect(friday).toHaveAttribute('aria-current', 'date');
	await expect(friday.getByText('Today')).toBeVisible();

	await page.getByRole('button', { name: 'Next week' }).click();
	await expect(page).toHaveURL(/from=2026-11-23/);
	await expect(page.locator('[aria-current="date"]')).toHaveCount(0);
	await page.getByRole('button', { name: 'Today' }).click();
	await expect(day(page, 'Friday, November 20')).toHaveAttribute('aria-current', 'date');
	await expect(day(page, 'Saturday, November 21').getByText('Pizza bestellen')).toBeVisible();
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
	await page.goto('/week');

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
	await dialog.getByRole('button', { name: 'Add 2 recipes' }).click();
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

test('second time adds only new entries', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-16T10:00:00'));
	await loginFresh(page);
	const t = uniqueToken();
	const pie = await createRecipe(page, { ...loadFixture(0), title: `Pie ${t}` });
	const mash = await createRecipe(page, { ...loadFixture(2), title: `Mash ${t}` });
	await plan(page, { day: '2026-11-17', recipeId: pie.id });
	await page.goto('/week');

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
	await page.goto('/week');

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

	await page.getByRole('button', { name: 'Open week' }).click();
	await expect(page).toHaveURL(/\/week\?plan=[^&]+&from=2026-11-16$/);
	await expect(
		day(page, 'Wednesday, November 18').getByRole('link', { name: title })
	).toHaveAttribute('href', `/recipes/${recipe.slug}?servings=6`);
});

// Named plans: made, shared, renamed, deleted and left like lists.

const picker = (page: Page) => page.getByRole('button', { name: 'Plan', exact: true });

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
	await page.goto('/week');

	// "New plan" is the picker's last row; the draft names it, Enter makes it.
	await picker(page).click();
	await page.getByRole('option', { name: 'Create a new plan' }).click();
	await expect(page).toHaveURL(/\/week\/new$/);
	const field = page.getByRole('textbox', { name: 'Name' });
	await expect(field).toBeFocused();
	await field.fill(name);
	await field.press('Enter');
	await expect(page).toHaveURL(/\/week\?plan=[^&]+$/);
	await expect(picker(page)).toHaveText(name);
	// Created, the plan offers its sharing right away.
	await expect(page.getByRole('button', { name: 'Share', exact: true })).toBeFocused();

	// The guest has the week open before the invitation exists.
	const other = await signedInAs(browser, guest);
	try {
		const them = other.page;
		await them.goto('/week');
		await expect(picker(them)).toHaveText('My plan');
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

		// Back on the tab, the open week shows the new invitation's card.
		await them.evaluate(() => document.dispatchEvent(new Event('visibilitychange')));
		await expect(side(them, 'Week')).toHaveAccessibleName(/1 invitation/);
		await expect(them.getByText(`${owner} invites you to the plan “${name}”`)).toBeVisible();
		await them.getByRole('button', { name: 'Accept' }).click();
		await expect(picker(them)).toHaveText(name);
		await expect(side(them, 'Week')).not.toHaveAccessibleName(/invitation/);
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
		await picker(page).focus();
		const id = new URL(page.url()).searchParams.get('plan');
		const added = await them.request.post(`/api/v1/meal-plans/${id}/entries`, {
			data: { title: 'Suppe' }
		});
		expect(added.ok()).toBe(true);
		await expect(page.getByRole('main').getByText('Suppe')).toBeVisible();
		await page.waitForTimeout(300);
		await expect(picker(page)).toBeFocused();

		// The faces show who is on it.
		await them.getByRole('button', { name: '2 people on this plan' }).click();
		const people = them.getByRole('dialog', { name: 'Shared with' });
		await expect(people.getByText('You · can plan and change')).toBeVisible();
		await people.getByRole('button', { name: 'Done' }).last().click();

		// Removed by the owner, the open week says so and falls back to My plan.
		await page.getByRole('button', { name: 'Share', exact: true }).click();
		await page
			.getByRole('dialog')
			.getByRole('button', { name: `Remove ${guest}` })
			.click();
		await expect(page.getByRole('dialog').getByText(guest)).toHaveCount(0);
		await expect(them.getByText('This plan is no longer there for you')).toBeVisible();
		await expect(picker(them)).toHaveText('My plan');
	} finally {
		await other.context.close();
	}
});

test('personal plan has no share or rename', async ({ page }) => {
	await loginFresh(page);
	await page.goto('/week');
	await expect(picker(page)).toHaveText('My plan');
	const main = page.getByRole('main');
	await expect(main.getByRole('button', { name: 'Share', exact: true })).toHaveCount(0);
	await expect(main.getByRole('button', { name: 'More actions' })).toHaveCount(0);
	// Nor does the API rename it.
	const own = await ownPlan(page);
	const renamed = await page.request.patch(`/api/v1/meal-plans/${own}`, { data: { name: 'X' } });
	expect(renamed.status()).toBe(422);
});

test('the owner renames and deletes a plan', async ({ page }) => {
	await loginFresh(page);
	const name = `Holiday ${uniqueToken()}`;
	const id = await namedPlan(page, name);
	await page.goto(`/week?plan=${id}`);

	await page.getByRole('main').getByRole('button', { name: 'More actions' }).click();
	await expect(page.getByRole('menuitem')).toHaveText(['Rename', 'Share', 'Delete plan']);
	await page.getByRole('menuitem', { name: 'Rename' }).click();
	const rename = page.getByRole('dialog', { name: 'Rename' });
	await rename.getByLabel('Name').fill(`${name} 2`);
	await rename.getByRole('button', { name: 'Save' }).click();
	await expect(picker(page)).toHaveText(`${name} 2`);
	await picker(page).click();
	await expect(page.getByRole('option', { name: `${name} 2` })).toBeVisible();
	await page.keyboard.press('Escape');

	await page.getByRole('main').getByRole('button', { name: 'More actions' }).click();
	await page.getByRole('menuitem', { name: 'Delete plan' }).click();
	await page.getByRole('dialog').getByRole('button', { name: 'Delete' }).click();
	await expect(page.getByText(`“${name} 2” deleted`)).toBeVisible();
	await expect(picker(page)).toHaveText('My plan');
	await expect(picker(page)).toBeFocused();
	await picker(page).click();
	await expect(page.getByRole('option', { name: `${name} 2` })).toHaveCount(0);
});

test('member leaves a plan', async ({ page }) => {
	const { token, guest, guestId } = await twoPeople(page);
	const name = `Family ${token}`;
	const id = await namedPlan(page, name, guestId);
	await switchTo(page, guest);
	const accepted = await page.request.post(`/api/v1/meal-plans/${id}/membership`);
	expect(accepted.ok()).toBe(true);
	await page.goto(`/week?plan=${id}`);
	await expect(picker(page)).toHaveText(name);

	// A member renames and leaves; sharing and deleting are the owner's.
	await expect(page.getByRole('main').getByRole('button', { name: 'Share' })).toHaveCount(0);
	await page.getByRole('main').getByRole('button', { name: 'More actions' }).click();
	await expect(page.getByRole('menuitem')).toHaveText(['Rename', 'Leave plan']);
	await page.getByRole('menuitem', { name: 'Leave plan' }).click();
	const confirm = page.getByRole('dialog', { name: `Leave the plan “${name}”?` });
	await confirm.getByRole('button', { name: 'Leave plan' }).click();
	await expect(page.getByText(`You left “${name}”`)).toBeVisible();
	await expect(picker(page)).toHaveText('My plan');
	await expect(picker(page)).toBeFocused();
	await picker(page).click();
	await expect(page.getByRole('option', { name })).toHaveCount(0);
	await page.keyboard.press('Escape');
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

test('list shows the planned day and links back to the week', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-18T10:00:00'));
	await loginFresh(page);
	const t = uniqueToken();
	const pie = await createRecipe(page, { ...loadFixture(0), title: `Pie ${t}` });
	const mash = await createRecipe(page, { ...loadFixture(2), title: `Mash ${t}` });
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
	// By recipe, the block's head.
	await page.getByRole('radio', { name: 'Recipe' }).click();
	await expect(main.getByRole('heading', { level: 2, name: `Wed · Pie ${t}` })).toBeVisible();
	await expect(main.getByRole('heading', { level: 2, name: `Wed 25 · Mash ${t}` })).toBeVisible();

	await main.getByRole('link', { name: 'from the week' }).click();
	await expect(page).toHaveURL(`/week?plan=${planId}&from=2026-11-16`);
	await expect(page.getByText(`Pie ${t}`).first()).toBeVisible();
});

test('a list member outside the plan sees the day but no way to the week', async ({
	page,
	browser
}, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'two browsers; runs once, on desktop');
	await page.clock.setFixedTime(new Date('2026-11-18T10:00:00'));
	const { token, guest, guestId } = await twoPeople(page);
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
		await expect(main.getByRole('heading', { level: 2, name: `Wed · Pie ${token}` })).toBeVisible();
		await expect(main.getByRole('link', { name: 'from the week' })).toHaveCount(0);
	} finally {
		await other.context.close();
	}
});

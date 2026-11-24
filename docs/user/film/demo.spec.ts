import { expect, test, type Locator } from '@playwright/test';

import { guard, prepare } from '../screenshots/helpers';
import { film } from './film';

// The landing's film: public/film/demo-<scheme>.{mp4,jpg}.
// One recipe through its life - doubled, edited, cooked with a timer,
// discussed, passed on - then its week and the shopping list the week fills.
// Every hold starts once its change has landed and settled; network waits are
// squashed out of the film. Nothing here may change the demo: `guard`
// answers Save and the visit's own writes, and fails the take on any other;
// the suggestion accepted in the editor only ever reaches that guarded Save.

const RECIPE = '/recipes/shepherd-s-pie';

/** Just under the first `word` in `field`'s text, for pointing at one word of a paragraph without covering it. */
async function wordAt(field: Locator, word: string) {
	const at = await field.evaluate((el, word) => {
		const walk = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
		for (let n = walk.nextNode(); n; n = walk.nextNode()) {
			const i = n.textContent!.indexOf(word);
			if (i < 0) continue;
			const range = document.createRange();
			range.setStart(n, i);
			range.setEnd(n, i + word.length);
			const b = range.getBoundingClientRect();
			return { x: b.x + b.width / 2, y: b.y + b.height + 10 };
		}
		return null;
	}, word);
	if (!at) throw new Error(`no "${word}" in ${field}`);
	return at;
}

test('demo', async ({ page }, testInfo) => {
	await prepare(page, testInfo, { login: true, runningClock: true });
	await page.addInitScript(() => {
		// A device that allowed notifications, so the timer dock has no "only while open" line.
		Object.defineProperty(Notification, 'permission', { get: () => 'granted' });
		// Page changes cut instead of fading through an almost empty page, which reads as a flash in a film.
		delete (Document.prototype as { startViewTransition?: unknown }).startViewTransition;
		// "Shopping & week" opens on the week, as on a device that used it last.
		localStorage.setItem('rezepte:place-side', 'week');
	});
	const g = await guard(page);
	const f = await film(page, `demo-${testInfo.project.name}`);

	// Scene 0: the recipe at 4 servings, settled before the camera rolls.
	await page.goto(RECIPE);
	await expect(page.getByRole('heading', { level: 1, name: "Shepherd's Pie" })).toBeVisible();
	await expect(page.getByText('4 servings', { exact: true })).toBeVisible();
	await page.evaluate(() => document.fonts.ready);
	await page.waitForLoadState('networkidle');
	await page.getByRole('img', { name: /^Cover photo/ }).evaluate((img: HTMLImageElement) => img.decode());
	await f.glide({ x: 620, y: 300 }, 0);
	await f.settle();
	await f.roll();
	await f.hold(1200);

	// Scene 1: the servings, 4 to 8.
	f.mark('servings');
	await f.click(page.getByRole('button', { name: 'More servings' }), 550);
	for (let i = 0; i < 3; i++) {
		await f.hold(210);
		await f.click();
	}
	await f.land(() => expect(page.getByText('8 servings', { exact: true })).toBeVisible(), 1000, 'doubled');

	// Scene 2: the editor spots a time as it is typed - accept it, then Save.
	await f.click(page.getByRole('link', { name: 'Edit', exact: true }), 600, 'left');
	await f.land(() => expect(page.getByText(/suggestions? open/)).toBeVisible(), 0, 'editor');
	const step6 = page.getByRole('listitem', { name: 'Step 6' });
	await f.scrollTo(step6, 300, 600);
	const text6 = page.getByRole('textbox', { name: 'Step 6', exact: true });
	await f.click(text6, 350);
	await page.keyboard.press('End');
	await f.type(' Rest for 10 minutes.', 40);
	const summary = step6.getByRole('button', { name: '1 time, 1 suggestion' });
	await f.land(() => expect(summary).toBeVisible(), 400, 'typed');
	await f.click(summary, 350);
	const accept = step6.getByRole('button', { name: 'Accept time "10 minutes"' });
	await f.land(async () => {
		await expect(accept).toBeVisible();
		// Only the stored time and the new one: the demo's other steps carry no duplicate suggestions here.
		await expect(step6.getByRole('list').getByRole('listitem')).toHaveCount(2);
	}, 200, 'suggestion');
	await f.click(accept, 350);
	// The pointer leaves the list for the phrase in the step, now underlined solid.
	await f.glide(await wordAt(text6, '10 minutes'), 400);
	await f.land(() => expect(step6.getByRole('button', { name: '2 times' })).toBeVisible(), 500, 'accepted');
	await f.click(page.getByRole('button', { name: 'Save', exact: true }).filter({ visible: true }), 500);
	await f.land(() => expect(page).toHaveURL(RECIPE), 0);
	const toast = page.getByText('Recipe saved');
	await f.land(() => expect(toast).toBeVisible(), 600, 'saved');

	// Scene 3: cook mode, step 3's timer. The toast leaves before cook mode opens.
	const cook = page.getByRole('link', { name: 'Cook mode', exact: true });
	await f.glide(cook, 700, 'left');
	// Its fade-out left a gray ghost: the film cuts from the toast to the page without it.
	await f.skip(async () => {
		await expect(toast).toBeHidden({ timeout: 15_000 });
		await f.settle();
	});
	await f.click();
	// The recipe page names the same amounts, so the step counter says cook mode has arrived.
	await f.land(async () => {
		await expect(page.getByText('Step 1 of 6')).toBeVisible();
		await expect(page.getByText(/potatoes\s*\(2\s*kg\)/)).toBeVisible();
	}, 0, 'cook');
	// Straight on to Next: step 1 reads while the pointer travels. The steps cross-fade, which for a
	// moment lays two paragraphs over each other: the film cuts from one step to the next instead.
	await f.click(page.getByRole('button', { name: 'Next', exact: true }), 600);
	await f.skip(async () => {
		await expect(page.getByText('Step 2 of 6')).toBeVisible();
		await f.settle();
	});
	await f.hold(600);
	await f.click();
	await f.skip(async () => {
		await expect(page.getByText('Step 3 of 6')).toBeVisible();
		await f.settle();
	});
	const timer = page.getByRole('button', { name: 'Start timer: 15 minutes' });
	await f.land(() => expect(timer).toBeVisible(), 500, 'step-3');
	await f.click(timer, 500);
	await f.land(() => expect(page.locator('[data-timer-dock]')).toBeVisible(), 1100, 'timer');
	await f.click(page.getByRole('button', { name: 'Back to the recipe' }), 500);

	// Scene 4: back on the recipe, the timer still running; the comments.
	// The page comes back before its cover photo: the film cuts to it complete.
	await f.skip(async () => {
		await expect(page.getByRole('heading', { level: 1, name: "Shepherd's Pie" })).toBeVisible();
		await expect(page.getByRole('button').filter({ has: page.locator('[data-clock]') }).first()).toBeVisible();
		await page.getByRole('img', { name: /^Cover photo/ }).evaluate((img: HTMLImageElement) => img.decode());
		await f.settle();
	});
	f.mark('back');
	await f.hold(200);
	const tab = page.getByRole('tab', { name: /^Comments/ });
	await f.glide(tab, 500);
	await f.hold(80);
	await f.click();
	const comments = page.getByRole('tabpanel', { name: /^Comments/ });
	await f.land(() => expect(comments.getByText('Yesterday')).toBeVisible(), 0, 'comments');
	// Down to the Ingredients/Comments headings with air above them, the title gone entirely, Jonas's photo whole.
	const down = Math.round((await tab.boundingBox())!.y - 40);
	await f.scroll(down, 450);
	f.mark('photo-comment');
	await f.hold(1000);

	// Scene 5: Pass on, the recipe card. The top bar scrolled away with the page; the pointer sets off for it as the page comes back.
	const passOn = page.getByRole('button', { name: 'Pass on', exact: true });
	const p = (await passOn.boundingBox())!;
	await Promise.all([f.scroll(-down, 400), f.hold(250).then(() => f.glide({ x: p.x + Math.min(20, p.width / 4), y: p.y + down + p.height / 2 }, 600))]);
	await f.hold(80);
	await f.click();
	const sheet = page.getByRole('dialog', { name: 'Pass on recipe' });
	// The sheet draws its card a moment after it opens: the film cuts from the press to the finished card.
	await f.skip(async () => {
		await expect(sheet.getByRole('img', { name: /^Recipe image:/ })).toBeVisible({ timeout: 20_000 });
		await f.settle();
	});
	f.mark('card');
	await f.hold(1100);
	await f.glide(sheet.getByRole('button', { name: 'Close' }), 350);
	await f.hold(80);
	await f.click();
	await f.skip(async () => {
		await expect(sheet).toBeHidden();
		await f.settle();
	});

	// Scene 6: the week, and the shopping list it fills.
	// The recipe's own top bar has no navigation: home first, for a glimpse of the collection.
	await f.click(page.getByRole('banner').getByRole('link', { name: 'Rezepte' }), 450, 'left');
	// The grid's photos fade in as they load: the film cuts to the finished grid.
	await f.skip(async () => {
		await expect(page).toHaveURL('/');
		await page.evaluate(() => Promise.all([...document.images].filter((i) => !i.complete).map((i) => new Promise((r) => (i.onload = i.onerror = r)))));
		await f.settle();
	});
	f.mark('home');
	await f.click(page.getByRole('navigation', { name: 'Main' }).getByRole('link', { name: /^Shopping & week/ }), 450, 'left');
	const myPlan = page.getByRole('link', { name: /^My plan/ }).first();
	await f.land(() => expect(myPlan).toBeVisible(), 0, 'plans');
	await f.click(page.getByRole('heading', { name: 'My plan' }), 400, 'left');
	const title = page.getByRole('heading', { level: 1, name: 'My plan' });
	await f.land(async () => {
		await expect(title).toBeVisible();
		await expect(page.getByText('September 14–20', { exact: true })).toBeVisible();
	}, 0);
	// The week opens scrolled to today's row; up to its title and dates.
	const y = await page.evaluate(() => scrollY);
	if (y > 0) await f.scroll(-y, 500);
	f.mark('week');
	const pie = page.getByText("Shepherd's Pie", { exact: true }).first();
	await f.glide(pie, 400);
	await f.hold(200);
	// The card's list chip, under the dish in Tuesday's column.
	const pieBox = (await pie.boundingBox())!;
	const chips = await page.getByRole('link', { name: /Weekly shop/ }).all();
	let chip: Locator | undefined;
	for (const c of chips) {
		const b = await c.boundingBox();
		if (b && Math.abs(b.x - pieBox.x) < 40 && b.y > pieBox.y) chip = c;
	}
	if (!chip) throw new Error("no Weekly shop chip on Tuesday's Shepherd's Pie");
	await f.click(chip, 400, 'left');
	// The list scrolls to the dish's group and lights it for 1.8 s as it lands; the film cuts past all of that
	// to the list's top, its name in view.
	await f.skip(async () => {
		await expect(page.getByRole('heading', { level: 1, name: 'Weekly shop' })).toBeVisible();
		const lit = page.locator('[data-entry-block][data-lit]');
		await expect(lit).toHaveCount(1);
		await expect(lit).toHaveCount(0, { timeout: 5_000 });
		await f.settle();
		await page.evaluate(() => scrollTo({ top: 0, behavior: 'instant' }));
		await f.settle();
	});
	f.mark('list');
	// The name reads while the pointer steps aside onto the empty margin.
	await f.glide({ x: 1100, y: 420 }, 400);
	const group = page.getByRole('heading', { name: "Shepherd's Pie", exact: true }).first();
	await f.scrollTo(group, 90, 600);
	f.mark('group');
	// The last 400 ms become the crossfade into the first frame.
	await f.hold(1600 + 400);
	await f.cut();

	expect(g.state.saved, 'Save sent its PUT').toBe(true);
	g.done();
});

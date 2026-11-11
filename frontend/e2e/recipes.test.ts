import { expect, test, type Locator, type Page } from '@playwright/test';
import type { Editor } from '@tiptap/core';
import {
	createRecipe,
	createUser,
	loadFixture,
	login,
	openNewRecipe,
	openRecipeMenu,
	search,
	searchField,
	openWordCard,
	phoneLayout,
	selectInStep,
	signOut,
	tinyPng,
	uniqueToken,
	uploadImage
} from './helpers';

// Card titles are the only level-3 headings inside `main` - the filter
// panel (a portaled dialog, rendered outside `main`) has a level-3
// heading per section under its "Filters" level-2 title, so scoping to
// `main` is what keeps counting them equivalent to counting the grid once
// that panel can be open at the same time.
const cards = (page: Page) => page.getByRole('main').getByRole('heading', { level: 3 });

test.beforeEach(async ({ page }) => {
	await login(page);
});

test('shows the empty state while nothing is stored', async ({ page }) => {
	// The whole suite shares one binary and one database, and the desktop and
	// mobile projects run at the same time, so "no recipes at all" is a state
	// only the API can be pinned to - every other test here works against the
	// real one.
	await page.route(
		(url) => url.pathname === '/api/v1/recipes',
		(route) => route.fulfill({ json: { items: [], page: 1, limit: 24, total: 0 } })
	);
	await page.route(
		(url) => url.pathname === '/api/v1/tags',
		(route) => route.fulfill({ json: { items: [] } })
	);

	await page.goto('/');

	await expect(page.getByRole('heading', { name: 'No recipes yet' })).toBeVisible();
	const steps = page.getByRole('list').filter({ hasText: 'Or bring one in' });
	await expect(steps.getByRole('listitem')).toHaveText([
		/Write a recipe/,
		/Or bring one in/,
		/Cook together/
	]);
	// Scoped to the content area: on desktop the top bar carries the same call
	// to action.
	await expect(page.getByRole('main').getByRole('link', { name: 'New recipe' })).toBeVisible();
	await page.getByRole('main').getByRole('button', { name: 'Import' }).click();
	await expect(page.getByRole('dialog').getByLabel('Link or recipe text')).toBeVisible();
});

test('creates a recipe through the editor', async ({ page }) => {
	const token = uniqueToken();
	const title = `Lemon cake ${token}`;

	await openNewRecipe(page);

	// Role locators throughout: every list item in the editor repeats the
	// label of the field inside it ("Ingredient 1", "Step 1") so
	// svelte-dnd-action can announce a drag, which leaves `getByLabel`
	// matching two elements. "Unit" is a combobox rather than a textbox
	// because it offers a list of units beside free text.
	await page.getByRole('textbox', { name: 'Title', exact: true }).fill(title);
	await page.getByRole('textbox', { name: 'Amount', exact: true }).fill('2');
	await page.getByRole('combobox', { name: 'Unit', exact: true }).fill('pcs');
	await page.getByRole('textbox', { name: 'Ingredient', exact: true }).fill('Lemon');
	await page
		.getByRole('textbox', { name: 'Step 1', exact: true })
		.fill('Squeeze the lemons and stir in the juice.');

	const tags = page.getByRole('combobox', { name: 'Tags', exact: true });
	await tags.fill(token);
	await tags.press('Enter');
	// Taking the tag clears the field again.
	await expect(tags).toHaveValue('');

	await page.getByRole('button', { name: 'Save', exact: true }).click();

	// Landed on the new recipe's detail page.
	await expect(page).toHaveURL(/\/recipes\/lemon-cake-/);
	await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
	await expect(page.getByRole('checkbox', { name: 'Lemon', exact: true })).toBeVisible();
	await expect(page.getByText('Squeeze the lemons and stir in the juice.')).toBeVisible();

	// ... and the overview lists it, tag and all.
	await page.goto('/');
	await search(page, token);
	await expect(cards(page)).toHaveText([title]);
	// On the card itself: the tag row above the grid carries the same name.
	await expect(
		page.getByRole('article', { name: title }).getByText(token, { exact: true })
	).toBeVisible();
});

test('searches recipes and reaches the no-results state', async ({ page }) => {
	const token = uniqueToken();
	await createRecipe(page, { ...loadFixture(0), title: `Shepherd's Pie ${token}` });
	await createRecipe(page, { ...loadFixture(2), title: `Bangers and Mash ${token}` });

	await page.goto('/');

	// Both terms have to match (the API ANDs them), so the token keeps the
	// result to this test's own recipes while `Shep` does the actual work.
	await search(page, token);
	await expect(cards(page)).toHaveCount(2);

	await search(page, `Shep ${token}`);
	await expect(cards(page)).toHaveText([`Shepherd's Pie ${token}`]);

	await search(page, 'zzz');
	await expect(page.getByRole('heading', { name: 'Nothing found' })).toBeVisible();
	await expect(page.getByText('No recipes found for "zzz".')).toBeVisible();
});

// German on purpose: the search spells every term both ways, with umlauts
// and transliterated ("käse" and "kaese", see `variants` in
// service/internal/recipe/search.go), because a German keyboard is not
// always at hand and FTS5 alone folds "ö" to "o", not to "oe".
test('finds a German recipe with or without its umlauts', async ({ page }) => {
	const token = uniqueToken();
	await createRecipe(page, { ...loadFixture(0, 'de'), title: `Königsberger Klopse ${token}` });

	await page.goto('/');

	await search(page, `Königsberger ${token}`);
	await expect(cards(page)).toHaveText([`Königsberger Klopse ${token}`]);

	await search(page, `koenigsberger ${token}`);
	await expect(cards(page)).toHaveText([`Königsberger Klopse ${token}`]);
});

test('carries the search term into a new recipe', async ({ page }) => {
	const token = uniqueToken();

	await page.goto('/');
	await search(page, token);
	await expect(page.getByRole('heading', { name: 'Nothing found' })).toBeVisible();

	await page.getByRole('link', { name: 'Add as new recipe' }).click();

	// The hand-over is a route param the editor reads into its starting
	// values, so the term is in the field before anything is saved - and it
	// counts as the pristine state, which is why leaving again asks nothing.
	await expect(page.getByRole('textbox', { name: 'Title', exact: true })).toHaveValue(token);
});

test('names the filters, not an empty quote, when only they narrow the list', async ({ page }) => {
	// A bound no recipe can meet is easier to pin than to cook up: every
	// recipe in the shared database has some total time, and this asks for
	// one under a quarter of an hour that also carries a tag nothing has.
	await page.route(
		(url) => url.pathname === '/api/v1/recipes',
		(route) => route.fulfill({ json: { items: [], page: 1, limit: 24, total: 0 } })
	);

	await page.goto('/?maxMinutes=15');

	await expect(page.getByRole('heading', { name: 'Nothing found' })).toBeVisible();
	await expect(page.getByText('No recipes match the filters you have set.')).toBeVisible();
	// The bug this replaces: with nothing typed, the quoted sentence closed
	// around an empty pair of quotation marks.
	await expect(page.getByText('""')).toHaveCount(0);

	// Nothing to carry over, so the same button is the plain way on - it does
	// not vanish, because an empty result is still a reason to write one.
	await expect(page.getByRole('link', { name: 'Add as new recipe' })).toHaveCount(0);
	await page.getByRole('main').getByRole('link', { name: 'New recipe' }).click();
	await expect(page.getByRole('textbox', { name: 'Title', exact: true })).toHaveValue('');
});

test('narrows the grid with a tag chip', async ({ page, isMobile }) => {
	test.skip(isMobile, 'phones have no row of every tag; see the suggestion tests');
	const token = uniqueToken();
	const tag = `${token}a`;
	await createRecipe(page, {
		...loadFixture(2),
		title: `Tagged ${token}`,
		tags: [tag]
	});
	await createRecipe(page, {
		...loadFixture(3),
		title: `Untagged ${token}`,
		tags: [`${token}b`]
	});

	// The chip row only fits as many tags as its width allows before
	// overflowing into "+N more", ordered by how many recipes carry each
	// (`ORDER BY count DESC` server-side) - and the suite's tests share one
	// database, so by the time this one runs, tags from every other test
	// already outrank this fresh, single-use one. Pinning `/api/v1/tags` to
	// just the two this test cares about keeps its chip off that overflow
	// regardless of what else the shared database holds.
	await page.route(
		(url) => url.pathname === '/api/v1/tags',
		(route) =>
			route.fulfill({
				json: {
					items: [
						{ name: tag, count: 1 },
						{ name: `${token}b`, count: 1 }
					]
				}
			})
	);

	await page.goto('/');
	await search(page, token);
	await expect(cards(page)).toHaveCount(2);

	const chip = page.getByRole('button', { name: tag });
	await chip.click();

	await expect(chip).toHaveAttribute('aria-pressed', 'true');
	await expect(cards(page)).toHaveText([`Tagged ${token}`]);
});

test('narrows the grid with two tag chips at once', async ({ page, isMobile }) => {
	test.skip(isMobile, 'phones have no row of every tag; see the suggestion tests');
	const token = uniqueToken();
	const a = `${token}a`;
	const b = `${token}b`;

	await createRecipe(page, { ...loadFixture(1), title: `Both ${token}`, tags: [a, b] });
	await createRecipe(page, { ...loadFixture(2), title: `Only A ${token}`, tags: [a] });

	// Same reasoning as the test above: pin the chip row to this test's own
	// two tags so the second click doesn't depend on winning the shared
	// database's count-ordered chip row against every other test's tags.
	await page.route(
		(url) => url.pathname === '/api/v1/tags',
		(route) =>
			route.fulfill({
				json: {
					items: [
						{ name: a, count: 2 },
						{ name: b, count: 1 }
					]
				}
			})
	);

	// Newest first: "Only A" was created after "Both", so it sorts first
	// until the second chip narrows the grid down to "Both" alone.
	await page.goto(`/?tags=${a}`);
	await expect(cards(page)).toHaveText([`Only A ${token}`, `Both ${token}`]);

	await page.getByRole('button', { name: b }).click();
	await expect(page).toHaveURL(new RegExp(`tags=${a}%2C${b}|tags=${a},${b}`));
	await expect(cards(page)).toHaveText([`Both ${token}`]);
});

test('unfolds the chip row to reach a tag past the fold', async ({ page, isMobile }) => {
	test.skip(isMobile, 'phones have no row of every tag; see the suggestion tests');
	const token = uniqueToken();
	const tag = `${token}z`;
	await createRecipe(page, { ...loadFixture(1), title: `Past the fold ${token}`, tags: [tag] });

	// Twelve wide decoys ahead of it put `tag` past the fold on the widest
	// viewport the suite runs, so the chip is only reachable by unfolding -
	// which is the whole point of this test. Pinning `/api/v1/tags` also
	// keeps it independent of what the shared database holds by now.
	await page.route(
		(url) => url.pathname === '/api/v1/tags',
		(route) =>
			route.fulfill({
				json: {
					items: [
						...Array.from({ length: 12 }, (_, i) => ({
							name: `placeholder-tag-${i}`,
							count: 9
						})),
						{ name: tag, count: 1 }
					]
				}
			})
	);

	await page.goto('/');
	const chip = page.getByRole('button', { name: tag });
	// Not merely hidden: a folded row leaves the chips it has no room for
	// out of the DOM entirely (TagFilter.svelte). The off-screen measuring
	// twin does hold one, but it is `aria-hidden`, so no role locator sees it.
	await expect(chip).toHaveCount(0);

	const toggle = page.getByRole('button', { name: /^\+\d+ more$/ });
	await toggle.click();
	await expect(toggle).toHaveCount(0);

	await chip.click();
	await expect(page).toHaveURL(new RegExp(`tags=${tag}`));
	await expect(cards(page)).toHaveText([`Past the fold ${token}`]);

	// Folding back takes the row with it - except for the chip just picked.
	// A filter that is on stays on screen wherever it sits in the list, or
	// the row would go silent about the very thing narrowing the grid.
	await page.getByRole('button', { name: 'Less', exact: true }).click();
	await expect(page.getByRole('button', { name: /^\+\d+ more$/ })).toBeVisible();
	await expect(chip).toBeVisible();
	await expect(chip).toHaveAttribute('aria-pressed', 'true');
	// The decoys it was hiding behind are gone again, so this is the folded
	// row keeping one chip, not the unfolded row under another name.
	await expect(page.getByRole('button', { name: 'placeholder-tag-11' })).toHaveCount(0);
});

test('filters by maximum time from the panel', async ({ page }) => {
	const token = uniqueToken();
	await createRecipe(page, {
		...loadFixture(1),
		title: `Quick ${token}`,
		prepMinutes: 10,
		cookMinutes: 5
	});
	await createRecipe(page, {
		...loadFixture(2),
		title: `Slow ${token}`,
		prepMinutes: 60,
		cookMinutes: 120
	});

	await page.goto(`/?q=${token}`);
	await expect(cards(page)).toHaveCount(2);

	await page.getByRole('button', { name: 'Filters' }).click();

	// The scale is a list of stops, and the slider commits on every key
	// press, so Home is one step to the tightest of them: 15 minutes, which
	// is exactly the quick recipe's total and well under the slow one's.
	// Driving it by keyboard rather than by a synthesized drag is also the
	// path a reader without a pointer takes, so the test covers that too.
	const slider = page.getByRole('slider', { name: 'Maximum time' });
	await slider.press('Home');

	await expect(slider).toHaveAttribute('aria-valuetext', 'up to 15 min');
	await expect(page).toHaveURL(/maxMinutes=15/);
	await expect(cards(page)).toHaveText([`Quick ${token}`]);

	// The far stop is no bound at all, so the filter leaves the URL entirely
	// rather than naming its loosest value there.
	await slider.press('End');
	await expect(slider).toHaveAttribute('aria-valuetext', 'Any');
	await expect(page).not.toHaveURL(/maxMinutes/);
	await expect(cards(page)).toHaveCount(2);
});

// Bits UI positions the thumb and the ticks with an inline `left` and an
// inline `translate` and nothing else - no `top` - so their vertical place
// is whatever static position they land on, and a Tailwind `-translate-y-1/2`
// cannot correct it: in Tailwind v4 that utility writes the same `translate`
// property the library already set inline, and loses to it. Both ended up
// below the track, the thumb by most of its own height. Only properties the
// library leaves alone (`top`, `margin`) can center them, and only a
// measurement can tell whether they are centered.
test('centers the time slider thumb and stops on its track', async ({ page }) => {
	await page.goto('/');
	await page.getByRole('button', { name: 'Filters' }).click();

	await expect(page.getByRole('slider', { name: 'Maximum time' })).toBeVisible();

	// Every rect in one frame, rather than a `boundingBox()` per element: the
	// panel animates as it opens (it scales on desktop and slides on phones),
	// so two reads a round trip apart are two different moments of that
	// animation, and their difference is mostly the travel between them. That
	// read the thumb as 28px off a track that was merely still moving.
	const offsets = await page.evaluate(() => {
		const middle = (element: Element) => {
			const rect = element.getBoundingClientRect();
			return rect.y + rect.height / 2;
		};
		const track = document.querySelector('[data-slider-root] > span');
		const thumb = document.querySelector('[data-slider-thumb]');
		const ticks = [...document.querySelectorAll('[data-slider-tick]')];
		if (!track || !thumb) {
			throw new Error('slider not rendered');
		}
		const trackMiddle = middle(track);
		return {
			thumb: middle(thumb) - trackMiddle,
			ticks: ticks.map((tick) => middle(tick) - trackMiddle)
		};
	});

	expect(Math.abs(offsets.thumb)).toBeLessThanOrEqual(1);
	expect(offsets.ticks.length).toBeGreaterThan(0);
	for (const offset of offsets.ticks) {
		expect(Math.abs(offset)).toBeLessThanOrEqual(1);
	}
});

// The folded row reserves room for its own toggle by measuring a stand-in
// copy off-screen, and the copy has to report the width the real one will
// take. The narrowest width the row is shown at (md, 768px; phones find tags
// through the search instead) is where too small a reservation shows: the
// toggle is the last thing on the line, so whatever the measurement got
// wrong is clipped off its right edge by the row's `overflow-hidden` -
// silently, since a clipped element still reports itself visible.
test('keeps the tag row toggle inside the row at its narrowest', async ({ page }) => {
	await page.setViewportSize({ width: 768, height: 900 });
	await page.route(
		(url) => url.pathname === '/api/v1/tags',
		(route) =>
			route.fulfill({
				json: {
					items: [
						{ name: 'meat', count: 5 },
						{ name: 'classic', count: 4 },
						{ name: 'vegetarian', count: 3 },
						{ name: 'breakfast', count: 2 },
						{ name: 'for-a-crowd', count: 1 },
						...Array.from({ length: 8 }, (_, i) => ({ name: `weeknight-${i}`, count: 1 }))
					]
				}
			})
	);

	await page.goto('/');
	const toggle = page.getByRole('button', { name: /^\+\d+ more$/ });
	await expect(toggle).toBeVisible();

	await expect
		.poll(async () =>
			page.evaluate(() => {
				const button = [...document.querySelectorAll('button')].find((candidate) =>
					/^\+\d+ more$/.test(candidate.textContent?.trim() ?? '')
				);
				const row = button?.parentElement;
				if (!button || !row) {
					throw new Error('toggle not rendered');
				}
				return Math.round(button.getBoundingClientRect().right - row.getBoundingClientRect().right);
			})
		)
		.toBeLessThanOrEqual(0);
});

test('stars a recipe from the card and keeps it after a reload', async ({ page }) => {
	const token = uniqueToken();
	await createRecipe(page, { ...loadFixture(1), title: `Star ${token}` });

	await page.goto(`/?q=${token}`);
	const star = page.getByRole('button', { name: 'Favorite', exact: true, pressed: false });
	await star.click();
	await expect(
		page.getByRole('button', { name: 'Favorite', exact: true, pressed: true })
	).toBeVisible();

	await page.reload();
	await expect(
		page.getByRole('button', { name: 'Favorite', exact: true, pressed: true })
	).toBeVisible();
});

test('filters the overview down to favorites', async ({ page }) => {
	const token = uniqueToken();
	await createRecipe(page, { ...loadFixture(1), title: `Starred ${token}` });
	await createRecipe(page, { ...loadFixture(2), title: `Unstarred ${token}` });

	// Sorted by title (S before U) rather than relying on the default
	// "most recently changed first" order, which is nondeterministic here:
	// both recipes are created within the same request burst, so which one
	// counts as "most recent" isn't guaranteed - and `.first()` below needs
	// to land on "Starred" specifically.
	await page.goto(`/?q=${token}&sort=title`);
	await page.getByRole('button', { name: 'Favorite', exact: true, pressed: false }).first().click();
	await expect(
		page.getByRole('button', { name: 'Favorite', exact: true, pressed: true })
	).toBeVisible();
	await page.getByRole('button', { name: 'Filters' }).click();
	// `role="switch"`, not checkbox: the control is a Switch, and Playwright's
	// `check()` only drives a real checkbox or radio - clicking it and reading
	// `aria-checked` back is what tells a switch was actually flipped.
	const favorites = page.getByRole('switch', { name: 'Favorites only' });
	await favorites.click();
	await expect(favorites).toHaveAttribute('aria-checked', 'true');

	await expect(cards(page)).toHaveText([`Starred ${token}`]);
});

test('sorts the grid alphabetically', async ({ page, isMobile }) => {
	const token = uniqueToken();
	await createRecipe(page, { ...loadFixture(1), title: `Last ${token}` });
	await createRecipe(page, { ...loadFixture(2), title: `First ${token}` });

	await page.goto(`/?q=${token}`);

	// Select.svelte wraps Bits UI's Select, not a native <select>: this opens
	// the trigger and clicks the option instead of using selectOption(). On
	// phones the control lives inside the filter panel instead of row 1 (see
	// FilterPanel.svelte), so the panel has to be open first. `getByRole`
	// rather than `getByLabel`: the sort section's own aria-labelledby gives
	// it the same accessible name, so getByLabel('Sort by') matches both
	// it and the trigger button inside it.
	if (isMobile) {
		await page.getByRole('button', { name: 'Filters' }).click();
	}
	await page.getByRole('button', { name: 'Sort by' }).click();
	await page.getByRole('option', { name: 'A–Z' }).click();

	await expect(cards(page)).toHaveText([`First ${token}`, `Last ${token}`]);
});

test('sorting alone does not count as an active filter', async ({ page, isMobile }) => {
	const token = uniqueToken();
	await createRecipe(page, { ...loadFixture(1), title: `Last ${token}` });
	await createRecipe(page, { ...loadFixture(2), title: `First ${token}` });

	await page.goto(`/?q=${token}`);
	if (isMobile) {
		await page.getByRole('button', { name: 'Filters' }).click();
	}
	await page.getByRole('button', { name: 'Sort by' }).click();
	await page.getByRole('option', { name: 'A–Z' }).click();

	// Sorting reorders the grid rather than narrowing it, so with no tags
	// and no time filter active it must not read as an active filter: no
	// active-filter chip names it, and no reset is offered inside the panel.
	await expect(page.getByRole('button', { name: /^Remove filter/ })).toHaveCount(0);
	if (!isMobile) {
		await page.getByRole('button', { name: 'Filters', exact: true }).click();
	}
	await expect(page.getByRole('button', { name: 'Reset all filters' })).toHaveCount(0);
});

test('edits a recipe from the detail page', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, { ...loadFixture(4), title: `Before ${token}` });
	const renamed = `After ${token}`;

	await page.goto(`/recipes/${recipe.slug}`);
	await openRecipeMenu(page);
	await page.getByRole('menuitem', { name: 'Edit' }).click();

	await expect(page).toHaveURL(`/recipes/${recipe.slug}/edit`);
	await page.getByRole('textbox', { name: 'Title', exact: true }).fill(renamed);
	await page.getByRole('button', { name: 'Save', exact: true }).click();

	// The slug is a permalink, so only the heading changes.
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	await expect(page.getByRole('heading', { level: 1, name: renamed })).toBeVisible();
});

test('deletes a recipe through the menu and the confirmation', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, { ...loadFixture(5), title: `Out it goes ${token}` });

	await page.goto(`/recipes/${recipe.slug}`);
	await openRecipeMenu(page);
	await page.getByRole('menuitem', { name: 'Delete' }).click();

	const dialog = page.getByRole('dialog');
	await expect(dialog.getByText(`Really delete "Out it goes ${token}"?`)).toBeVisible();
	await dialog.getByRole('button', { name: 'Delete' }).click();

	await expect(page).toHaveURL('/');
	await search(page, token);
	await expect(page.getByRole('heading', { name: 'Nothing found' })).toBeVisible();
});

test('guards a dirty editor against navigating away', async ({ page }) => {
	await openNewRecipe(page);
	const title = page.getByRole('textbox', { name: 'Title', exact: true });
	await title.fill(`Half done ${uniqueToken()}`);

	await page.getByRole('button', { name: 'Cancel' }).click();

	const dialog = page.getByRole('dialog');
	await expect(dialog.getByText('Discard changes?')).toBeVisible();

	// Dismissing the dialog keeps the draft.
	await dialog.getByRole('button', { name: 'Cancel' }).click();
	await expect(dialog).toBeHidden();
	await expect(page).toHaveURL(/\/recipes\/new$/);
	await expect(title).not.toHaveValue('');

	await page.getByRole('button', { name: 'Cancel' }).click();
	await dialog.getByRole('button', { name: 'Discard' }).click();
	await expect(page).toHaveURL('/');
});

// A tag goes on only when it is chosen. Text typed into the field and left
// there - by Save, Tab or a tap elsewhere - creates nothing behind the
// author's back, even though the list highlights creating it.
test('saves no tag that was typed but never chosen', async ({ page }) => {
	const token = uniqueToken();
	// `beforeEach` already logs in - a second login here logs two HTTP 500s.
	await page.goto('/recipes/new');

	await page.getByRole('textbox', { name: 'Title', exact: true }).fill(`No Enter ${token}`);
	await page.getByRole('textbox', { name: 'Ingredient', exact: true }).first().fill('Flour');
	await page.getByRole('textbox', { name: 'Step 1', exact: true }).fill('Stir.');

	const tags = page.getByRole('combobox', { name: 'Tags', exact: true });
	await tags.fill(token);
	// Nothing matches, so the highlight is on creating it: Enter would.
	await expect(page.getByRole('option', { name: `Create the tag ${token}` })).toHaveAttribute(
		'aria-selected',
		'true'
	);
	// Tab leaves the field and adds nothing; the text stays, uncommitted.
	await page.keyboard.press('Tab');
	await expect(tags).not.toBeFocused();
	await expect(tags).toHaveValue(token);
	await expect(page.getByRole('button', { name: `Remove ${token}` })).toHaveCount(0);
	await page.getByRole('button', { name: 'Save', exact: true }).click();

	await expect(page.getByRole('heading', { level: 1, name: `No Enter ${token}` })).toBeVisible();
	await expect(page.getByText(token, { exact: true })).toHaveCount(0);
});

test('saves no typed tag when editing either', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, { ...loadFixture(4), title: `Edited ${token}` });

	await page.goto(`/recipes/${recipe.slug}/edit`);
	await page.getByRole('combobox', { name: 'Tags', exact: true }).fill(token);
	await page.getByRole('button', { name: 'Save', exact: true }).click();

	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	await expect(page.getByText(token, { exact: true })).toHaveCount(0);
});

// After a pick the list stays open for the next tag. A click on the field
// that already has the focus closes it and the next opens it again, the
// WAI-ARIA editable combobox's toggle: how a phone closes it in place.
test('a click on the focused tag field closes and opens its list', async ({ page }) => {
	const token = uniqueToken();
	const tag = `${token}cinnamon`;
	await createRecipe(page, { ...loadFixture(6), title: `Toggle ${token}`, tags: [tag] });

	await openNewRecipe(page);
	const tags = page.getByRole('combobox', { name: 'Tags', exact: true });
	const list = page.getByRole('listbox', { name: 'Tag suggestions' });
	await tags.fill(token);
	await page.getByRole('option', { name: `${tag} 1`, exact: true }).click();
	await expect(page.getByRole('button', { name: `Remove ${tag}` })).toHaveCount(1);
	await expect(tags).toBeFocused();
	await expect(list).toBeVisible();

	await tags.click();
	await expect(list).toHaveCount(0);
	await expect(tags).toBeFocused();
	await tags.click();
	await expect(list).toBeVisible();
});

test('adds a suggested tag by tapping it on touch', async ({ page, isMobile }) => {
	test.skip(!isMobile, 'tap() needs hasTouch, which only the mobile project enables');
	const token = uniqueToken();
	const tag = `${token}cinnamon`;
	// Seeding a recipe with this tag first is what makes it a real suggestion:
	// the editor's tag field only offers what `listTags()` already knows.
	await createRecipe(page, { ...loadFixture(6), title: `Suggested ${token}`, tags: [tag] });

	await openNewRecipe(page);
	await page.getByRole('combobox', { name: 'Tags', exact: true }).fill(token);
	// Playwright's click() also dispatches pointer events, so only tap()
	// actually exercises the touch path, where the list keeps the focus in
	// the field on `pointerdown` and the option is taken on the click.
	await page.getByRole('option', { name: `${tag} 1`, exact: true }).tap();

	await expect(page.getByText(tag, { exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: `Remove ${tag}`, exact: true })).toHaveCount(1);
});

test('adds a suggested tag by clicking it', async ({ page }) => {
	const token = uniqueToken();
	const tag = `${token}cinnamon`;
	await createRecipe(page, { ...loadFixture(6), title: `Suggested ${token}`, tags: [tag] });

	await openNewRecipe(page);
	await page.getByRole('combobox', { name: 'Tags', exact: true }).fill(token);
	await page.getByRole('option', { name: `${tag} 1`, exact: true }).click();

	await expect(page.getByText(tag, { exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: `Remove ${tag}`, exact: true })).toHaveCount(1);
});

test('leaves a suggested tag alone when a scroll starts on it', async ({ page }) => {
	const token = uniqueToken();
	const tag = `${token}cinnamon`;
	await createRecipe(page, { ...loadFixture(6), title: `Suggested ${token}`, tags: [tag] });

	await openNewRecipe(page);
	const tags = page.getByRole('combobox', { name: 'Tags', exact: true });
	await tags.fill(token);
	const option = page.getByRole('option', { name: `${tag} 1`, exact: true });

	// A finger that lands on an option and drags is a scroll through the
	// list: the browser cancels the pointer, and no click follows.
	const touch = { pointerType: 'touch', isPrimary: true, bubbles: true, cancelable: true };
	await option.dispatchEvent('pointerdown', touch);
	await option.dispatchEvent('pointercancel', touch);
	await expect(page.getByRole('button', { name: `Remove ${tag}`, exact: true })).toHaveCount(0);
	await expect(option).toBeVisible();
	await expect(tags).toHaveValue(token);
});

// The editor's save actions sit in the section rail, so that nothing on this
// page ever lies over the form: a bar floating above a scrolling form hides a
// band of exactly what is being read. Two things have to hold for that, and
// both are geometry rather than a matter of taste - the actions stay left of
// the form, and they stay on screen the whole way down.
test('the save actions never lie over the form', async ({ page }, testInfo) => {
	test.skip(
		testInfo.project.name.startsWith('mobile'),
		'the phone bar is pinned to the top of the screen instead'
	);

	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Rail ${uniqueToken()}` });
	await page.goto(`/recipes/${recipe.slug}/edit`);

	const save = page.getByRole('button', { name: 'Save', exact: true });
	const form = page.locator('form');
	await expect(save).toBeVisible();

	const column = (await form.boundingBox())!;
	const action = (await save.boundingBox())!;

	// Left of the form, with no overlap at all - this is the whole point.
	expect(action.x + action.width).toBeLessThanOrEqual(column.x);

	// The rail is sticky, so the action has to still be on screen once the
	// form has scrolled past. Without that it would only be reachable from
	// the top of a page that is several screens tall.
	await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
	await expect(save).toBeInViewport();
	const scrolled = (await save.boundingBox())!;
	expect(scrolled.x + scrolled.width).toBeLessThanOrEqual(column.x);
});

// A URL is the longest value this form takes. Sharing a four-column row with
// the servings and the two times left it a quarter of the card, so two
// recipes from the same site looked identical in the field.
test('the source field owns its line at every width', async ({ page }) => {
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Source ${uniqueToken()}` });
	await page.goto(`/recipes/${recipe.slug}/edit`);

	const description = page.locator('#editor-description');
	const card = page.locator('#editor-section-basics');
	const inner = await card.evaluate((el) => {
		const style = getComputedStyle(el);
		const box = el.getBoundingClientRect();
		return box.width - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight);
	});

	// Both source fields: the name with its prefix is as long a line as the
	// URL, and a cookbook's title with its page number needs the room.
	for (const id of ['#editor-sourceName', '#editor-sourceUrl']) {
		const source = page.locator(id);
		await expect(source).toBeVisible();

		// Under the description, not beside the numbers.
		const below = (await source.boundingBox())!.y > (await description.boundingBox())!.y;
		expect(below).toBeTruthy();

		// Full width: the field spans the card's content box, so it never has
		// to share its line with anything.
		expect(Math.round((await source.boundingBox())!.width)).toBe(Math.round(inner));
	}
});

// The detail page's hero is a text column beside the cover. A `1fr` track
// cannot shrink past its own min-content, and the title's longest word at
// 52px sets that floor at ~335px, so a rigid 420px cover beside it demanded
// more width than the card has between 768px and ~858px and hung over the
// edge of the page. That band lies between the Playwright projects' own
// viewports, which is why it survived a green suite - so sweep it here.
// German on purpose: "Königsberger" is the long single word that sets the
// floor, and English titles rarely carry one that long. The sweep reads the
// page as another member, since only they see the tasty heart beside the
// star, and the pair is what a phone has to fit beside that word.
test('the recipe page never scrolls sideways', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'the sweep covers the phone widths itself');

	const reader = `ida${uniqueToken()}`.slice(0, 20);
	await createUser(page, { username: reader, role: 'user' });
	const recipe = await createRecipe(page, {
		...loadFixture(0, 'de'),
		title: 'Königsberger Klopse'
	});
	await signOut(page, testInfo);
	await login(page, reader);
	await page.goto(`/recipes/${recipe.slug}`);
	await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
	await expect(
		page.getByRole('button', { name: 'Tasty', exact: true, pressed: false })
	).toBeVisible();

	// 768 is where the two columns appear, 858 where the cover stops giving
	// way; 320 is the narrowest phone the design system admits.
	for (const width of [320, 360, 412, 768, 790, 830, 858, 900, 1280]) {
		await page.setViewportSize({ width, height: 900 });
		await expect
			.poll(async () => page.evaluate(() => document.documentElement.scrollWidth), {
				message: `horizontal overflow at ${width}px`
			})
			.toBeLessThanOrEqual(width);
	}
});

// The star and the heart sit in the line under the title, beside the people
// who find it tasty, so the title keeps the whole width: beside it they left a
// phone about 180px, and "Königsberger Klopse" broke into three lines.
test('the recipe title keeps the whole width on a phone', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'sets the phone width itself');

	const reader = `ida${uniqueToken()}`.slice(0, 20);
	await createUser(page, { username: reader, role: 'user' });
	const recipe = await createRecipe(page, {
		...loadFixture(0, 'de'),
		title: 'Königsberger Klopse'
	});
	await signOut(page, testInfo);
	await login(page, reader);
	await page.setViewportSize({ width: 360, height: 800 });
	await page.goto(`/recipes/${recipe.slug}`);

	const title = page.getByRole('heading', { level: 1 });
	const heart = page.getByRole('button', { name: 'Tasty', exact: true, pressed: false });
	const star = page.getByRole('button', { name: 'Favorite', exact: true, pressed: false });
	await expect(heart).toBeVisible();

	const titleBox = (await title.boundingBox())!;
	const lineHeight = await title.evaluate((el) => parseFloat(getComputedStyle(el).lineHeight));
	expect(Math.round(titleBox.height / lineHeight), 'title lines').toBe(2);
	for (const button of [star, heart]) {
		expect((await button.boundingBox())!.y).toBeGreaterThanOrEqual(titleBox.y + titleBox.height);
	}
});

// From `md` up a card carries the star and the tasty heart over its photo,
// and the grid, not the card, owns the room they need: four fixed columns
// squeezed a card to 160px at 768px, where the two 36px controls hid the
// dish. So at every card size and every width of the desktop layout a
// column stays at 220px or more, which the widest content fits five of.
test('overview columns leave a card room for its controls', async ({ page }, testInfo) => {
	test.skip(
		testInfo.project.name.startsWith('mobile'),
		'the sweep covers the desktop widths itself'
	);

	const token = uniqueToken();
	await createRecipe(page, { ...loadFixture(2), title: `Columns ${token}` });
	await page.goto(`/?q=${token}`);
	await expect(cards(page)).toHaveText([`Columns ${token}`]);

	for (const size of ['Small cards', 'Medium cards', 'Large cards']) {
		await page.setViewportSize({ width: 1280, height: 900 });
		await page.getByRole('radio', { name: size }).click();
		for (const width of [768, 820, 900, 1000, 1023, 1024, 1100, 1280, 1440, 1920]) {
			await page.setViewportSize({ width, height: 900 });
			const tracks = await page.evaluate(() => {
				const grid = document.querySelector('main article')!.parentElement!;
				return getComputedStyle(grid).gridTemplateColumns.split(' ').map(parseFloat);
			});
			expect(tracks.length, `${size}: columns at ${width}px`).toBeLessThanOrEqual(5);
			expect(tracks.length, `${size}: columns at ${width}px`).toBeGreaterThanOrEqual(2);
			expect(Math.min(...tracks), `${size}: column width at ${width}px`).toBeGreaterThanOrEqual(
				220
			);
		}
	}
});

/** How many columns the overview's cards fill, once they stopped moving. */
async function columnCount(page: Page) {
	let last = '';
	await expect
		.poll(async () => {
			const now = JSON.stringify(
				await page
					.getByRole('main')
					.locator('article')
					.evaluateAll((cards) => cards.map((card) => card.getBoundingClientRect().left))
			);
			const same = now === last;
			last = now;
			return same;
		})
		.toBe(true);
	return new Set(JSON.parse(last) as number[]).size;
}

test('the card size sets how many cards share a row, and stays', async ({ page, isMobile }) => {
	test.skip(isMobile, 'a phone chooses between one and two per row');
	const token = uniqueToken();
	for (const i of [1, 2, 3, 4, 5, 6]) {
		await createRecipe(page, { ...loadFixture(i), title: `Size ${i} ${token}` });
	}
	await page.setViewportSize({ width: 1440, height: 900 });
	await page.goto(`/?q=${token}`);
	await expect(cards(page)).toHaveCount(6);

	// The default is today's grid: four at the page's widest.
	await expect(page.getByRole('radio', { name: 'Medium cards' })).toBeChecked();
	await expect.poll(() => columnCount(page)).toBe(4);
	// Icons only: under a mouse each names itself.
	await page.getByRole('radio', { name: 'Small cards' }).hover();
	await expect(
		page.getByText('Small cards', { exact: true }).filter({ visible: true })
	).toBeVisible();
	for (const [size, columns] of [
		['Small cards', 5],
		['Large cards', 3]
	] as const) {
		await page.getByRole('radio', { name: size }).click();
		await expect(page.getByRole('radio', { name: size })).toBeChecked();
		await expect.poll(() => columnCount(page), { message: size }).toBe(columns);
	}

	await page.reload();
	await expect(page.getByRole('radio', { name: 'Large cards' })).toBeChecked();
	await expect.poll(() => columnCount(page)).toBe(3);
});

// On a phone, one card per row makes the list about twice as tall, so a
// reader at the bottom has to be scrolled past where the page used to end:
// a transition scoped to the results held them at its old height, and the
// card landed far down the screen.
test('switching the card size keeps the card at the top in place', async ({ page, isMobile }) => {
	const token = uniqueToken();
	for (let i = 0; i < 24; i++) {
		await createRecipe(page, { ...loadFixture((i % 5) + 1), title: `Anchor ${i} ${token}` });
	}
	if (!isMobile) await page.setViewportSize({ width: 1280, height: 700 });
	await page.goto(`/?q=${token}`);
	await expect(cards(page)).toHaveCount(24);
	await columnCount(page);

	// The reader has scrolled into the list; the first card that starts on
	// screen is the one they are looking at.
	await page.evaluate(
		(bottom) => window.scrollTo(0, document.body.scrollHeight / (bottom ? 1 : 3)),
		isMobile
	);
	const first = () =>
		page
			.getByRole('main')
			.locator('article')
			.evaluateAll((all) => {
				const tops = all.map((card) => card.getBoundingClientRect().top);
				const top = Math.min(...tops.filter((y) => y >= 0));
				return { index: tops.indexOf(top), top };
			});
	const before = await first();
	const article = page.getByRole('main').locator('article').nth(before.index);
	// Clicked in place: Playwright's click would scroll the control into view first.
	await page
		.getByRole('radio', { name: isMobile ? 'One per row' : 'Small cards' })
		.evaluate((radio: HTMLElement) => radio.click());
	await columnCount(page);
	await expect
		.poll(async () => Math.abs((await article.boundingBox())!.y - before.top))
		.toBeLessThanOrEqual(1);
});

test('one card per row on a phone shows the desktop card, and stays', async ({
	page,
	browser,
	isMobile
}) => {
	test.skip(!isMobile, 'the desktop chooses between three card sizes');
	const token = uniqueToken();
	// Somebody else's recipe, so the card has a tasty button to press.
	const ola = `ola${token}`.slice(0, 20);
	await createUser(page, { username: ola, role: 'user' });
	const other = await browser.newContext();
	const olaPage = await other.newPage();
	await login(olaPage, ola);
	const recipe = await createRecipe(olaPage, { ...loadFixture(3), title: `Wide ${token}` });
	await uploadImage(olaPage, recipe.id, tinyPng([200, 120, 40], 400, 300));
	await other.close();
	await createRecipe(page, { ...loadFixture(2), title: `Wide plain ${token}` });
	await page.goto(`/?q=${token}`);
	await expect(cards(page)).toHaveCount(2);
	await expect.poll(() => columnCount(page)).toBe(2);

	await page.getByRole('radio', { name: 'One per row' }).click();
	await expect.poll(() => columnCount(page)).toBe(1);
	// The desktop card: the 28px star and 28px initials of the 28px corner.
	for (const title of [`Wide ${token}`, `Wide plain ${token}`]) {
		const article = card(page, title);
		const star = await article.getByRole('button', { name: 'Favorite' }).boundingBox();
		expect(star!.width, title).toBe(28);
		const initials = await article.getByRole('button', { name: /^Added by/ }).boundingBox();
		expect(initials!.height, title).toBe(28);
	}
	// Still a phone: the star and the heart are 28px discs, and their
	// `::after` makes each a 44px target.
	const photoCard = card(page, `Wide ${token}`);
	for (const button of [
		photoCard.getByRole('button', { name: 'Favorite' }),
		photoCard.getByRole('button', { name: 'Tasty', exact: true }),
		card(page, `Wide plain ${token}`).getByRole('button', { name: 'Favorite' })
	]) {
		const target = await button.evaluate((el: HTMLElement) => {
			const after = getComputedStyle(el, '::after');
			return after.display === 'none'
				? el.offsetWidth
				: el.offsetWidth - 2 * parseFloat(after.left);
		});
		expect(target).toBeGreaterThanOrEqual(44);
	}

	await page.reload();
	await expect(page.getByRole('radio', { name: 'One per row' })).toBeChecked();
	await expect.poll(() => columnCount(page)).toBe(1);
});

test('the card size answers to the arrow keys', async ({ page, isMobile }) => {
	test.skip(isMobile, 'the same control on a phone has two options');
	const token = uniqueToken();
	await createRecipe(page, { ...loadFixture(1), title: `Keys ${token}` });
	await page.goto(`/?q=${token}`);
	await expect(cards(page)).toHaveCount(1);

	// One tab stop, on the checked option; the arrows move and check.
	await page.getByRole('radio', { name: 'Medium cards' }).focus();
	await page.keyboard.press('ArrowRight');
	await expect(page.getByRole('radio', { name: 'Large cards' })).toBeChecked();
	await expect(page.getByRole('radio', { name: 'Large cards' })).toBeFocused();
	await page.keyboard.press('ArrowRight');
	await expect(page.getByRole('radio', { name: 'Small cards' })).toBeChecked();
	await expect(page.getByRole('radio', { name: 'Medium cards' })).toHaveAttribute('tabindex', '-1');
	// The keyboard still gets the label, which the icons alone do not say.
	await expect(page.getByText('Small cards', { exact: true })).toBeVisible();
});

// A tap focuses the button only after the finger has lifted, so a tooltip
// that opens on focus came up with every switch and stayed until the next
// tap somewhere else.
test('a tap on the card size shows no tooltip', async ({ page, isMobile }) => {
	test.skip(!isMobile, 'only a finger focuses after letting go');
	const token = uniqueToken();
	await createRecipe(page, { ...loadFixture(1), title: `Tap ${token}` });
	await page.goto(`/?q=${token}`);
	await expect(cards(page)).toHaveCount(1);

	await page.getByRole('radio', { name: 'One per row' }).tap();
	await expect(page.getByRole('radio', { name: 'One per row' })).toBeChecked();
	await page.waitForTimeout(600);
	await expect(page.getByText('One per row', { exact: true })).toHaveCount(0);
});

test('the colophon names the author, and the editor once somebody else edits', async ({
	page
}, testInfo) => {
	const token = uniqueToken();
	// Open on purpose: editing-rights.test.ts locks the household for a
	// while, and a recipe on Default would then turn the editor away.
	const recipe = await createRecipe(page, {
		...loadFixture(6),
		title: `Colophon ${token}`,
		editPolicy: 'open'
	});
	// One editor per project: desktop and mobile run against the same
	// database at the same time, and a shared username would collide.
	const editor = `kim${token}`.slice(0, 20);
	await createUser(page, { username: editor, role: 'user' });

	await page.goto(`/recipes/${recipe.slug}`);
	// Each person is their mark and name in one control (the mark is
	// decoration), which opens their card - a popover, on a phone a sheet.
	const colophon = page.locator('footer').filter({ hasText: 'Added by' });
	const author = colophon.getByRole('button', { name: 'admin', exact: true });
	await expect(author).toBeVisible();
	await expect(author.locator('.rounded-full[aria-hidden="true"]')).toHaveCount(1);
	// The caption line is 20px tall; the control answers on 44px around it.
	const reach = await author.evaluate((button) => {
		button.scrollIntoView({ block: 'center' });
		const box = button.getBoundingClientRect();
		const x = box.x + box.width / 2;
		const y = box.y + box.height / 2;
		return [-21, 21].map((dy) => button.contains(document.elementFromPoint(x, y + dy)));
	});
	expect(reach).toEqual([true, true]);
	// Nobody has edited it, so the second line would only repeat the first.
	await expect(page.getByText('Last edited by')).toBeHidden();
	await author.click();
	await expect(page.getByText('@admin', { exact: false })).toBeVisible();
	await expect(page.getByRole('link', { name: /Recipes by admin$/ })).toBeVisible();
	await page.keyboard.press('Escape');

	// The detail page replaces the top bar's actions and hides the phone's
	// bottom nav, so the sign-out menu is only reachable from the overview.
	await page.goto('/');
	await signOut(page, testInfo);
	await login(page, editor);
	// Wait out the sign-in before navigating, or the goto below races the
	// session cookie and the app bounces back to /login.
	await expect(page).toHaveURL('/');
	await page.goto(`/recipes/${recipe.slug}/edit`);
	await page.getByRole('textbox', { name: 'Title', exact: true }).fill(`Colophon ${token} new`);
	await page.getByRole('button', { name: 'Save', exact: true }).click();

	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	await expect(colophon.getByText('Last edited by')).toBeVisible();
	await expect(colophon.getByRole('button', { name: editor, exact: true })).toBeVisible();
	// The author line survives the edit - it is not "last touched by".
	await expect(colophon.getByRole('button', { name: 'admin', exact: true })).toBeVisible();
});

test('the initials on a card name their people without opening the recipe', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, { ...loadFixture(9), title: `Who was it ${token}` });

	await page.goto(`/?q=${token}`);
	const card = cards(page).first();
	await expect(card).toHaveText(`Who was it ${token}`);

	// Tapping the initials is what a phone can do - there is no hover there -
	// and it must resolve the letters rather than follow the card's link.
	// The letters open the author's card, which names them and links to
	// their recipes.
	await page.getByRole('button', { name: /^Added by admin/ }).click();
	await expect(page.getByText('@admin', { exact: false })).toBeVisible();
	await expect(page.getByRole('link', { name: /Recipes by admin$/ })).toBeVisible();
	await expect(page).toHaveURL(new RegExp(`\\?q=${token}$`));

	// The card itself still navigates, so the popover has not swallowed it.
	await page.keyboard.press('Escape');
	await card.click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
});

test('narrows the grid to one author and shows whose recipes they are', async ({
	page
}, testInfo) => {
	const token = uniqueToken();
	const writer = `kim${token}`.slice(0, 20);
	await createUser(page, { username: writer, role: 'user' });
	await createRecipe(page, { ...loadFixture(7), title: `By admin ${token}` });

	// The second recipe has to be written by the other person, so it is
	// created in their own session rather than handed a different author.
	await page.goto('/');
	await signOut(page, testInfo);
	await login(page, writer);
	await createRecipe(page, { ...loadFixture(8), title: `By kim ${token}` });

	await page.goto(`/?q=${token}`);
	await expect(cards(page)).toHaveCount(2);

	await page.getByRole('button', { name: 'Filters' }).click();
	// Inside the panel: once set, the same name also stands on the page as
	// an active-filter chip.
	const panel = page.getByRole('dialog');
	await panel.getByRole('button', { name: new RegExp(`^${writer}`) }).click();

	await expect(page).toHaveURL(new RegExp(`author=${writer}`));
	await expect(cards(page)).toHaveText([`By kim ${token}`]);
	// The card says whose it is, which is what makes the filter verifiable
	// rather than something you have to trust.
	await expect(page.getByRole('main').getByLabel(`Added by ${writer}`)).toBeVisible();

	// Tapping the same chip again clears the filter rather than re-applying it.
	await panel.getByRole('button', { name: new RegExp(`^${writer}`) }).click();
	await expect(page).not.toHaveURL(/author=/);
	await expect(cards(page)).toHaveCount(2);
});

// Narrowed to one person, the grid is headed by a band in their color: their
// face and name, "Your recipes" for the viewer. On a phone it stands in for
// the person's chip and carries its ×; desktop keeps the chip.
test("heads one author's recipes with a band of their own", async ({ page }, testInfo) => {
	const token = uniqueToken();
	const writer = `mia${token}`.slice(0, 20);
	const name = `Mia ${token}`;
	await createUser(page, { username: writer, role: 'user', displayName: name });
	await page.goto('/');
	await signOut(page, testInfo);
	await login(page, writer);
	await createRecipe(page, { ...loadFixture(8), title: `Band ${token}` });

	await page.goto(`/?author=${writer}`);
	await expect(page.getByRole('heading', { level: 2, name: 'Your recipes' })).toBeVisible();

	await page.goto('/');
	await signOut(page, testInfo);
	await login(page);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto(`/?author=${writer}`);
	const band = page.getByRole('heading', { level: 2, name: `Recipes by ${name}` });
	await expect(band).toBeVisible();
	await expect(cards(page)).toHaveText([`Band ${token}`]);
	// "1 found" in the running head already says it; the band does not repeat it.
	await expect(page.getByText('1 recipe', { exact: true })).toBeHidden();
	const remove = page.getByRole('button', { name: `Remove filter ${name}` });
	await expect(remove).toHaveCount(1);

	await page.setViewportSize({ width: 360, height: 780 });
	await expect(band).toBeVisible();
	await expect(page.locator('h2').getByText(name, { exact: true })).toBeVisible();
	await expect(page.getByText('1 recipe', { exact: true })).toBeVisible();
	// The band's × is the only one: the chip row has no person in it.
	await expect(remove).toHaveCount(1);

	// At 200% text the name wraps in the band rather than being cut, and
	// nothing leaves the page.
	await page.evaluate(() => (document.documentElement.style.fontSize = '200%'));
	const fit = await page.locator('h2', { hasText: name }).evaluate((heading) => {
		const box = heading.getBoundingClientRect();
		return {
			cut: heading.scrollWidth > heading.clientWidth,
			width: box.width,
			page: document.documentElement.scrollWidth - window.innerWidth
		};
	});
	expect(fit.cut).toBe(false);
	// The insets of the photo and the × left it 60px before.
	expect(fit.width).toBeGreaterThan(120);
	expect(fit.page).toBeLessThanOrEqual(0);
	await page.evaluate(() => (document.documentElement.style.fontSize = ''));

	await remove.click();
	await expect(page).not.toHaveURL(/author=/);
	await expect(band).toHaveCount(0);
	// The focus does not fall to <body> with the band, for a tap neither.
	await expect(page.getByRole('button', { name: 'Filters' })).toBeFocused();
});

// A link to someone who has not written a recipe yet (from the people list)
// still names them: the band, "0 recipes" and the empty grid; desktop's chip
// by their display name, not their login name.
test('heads an author without recipes with their band', async ({ page }) => {
	const token = uniqueToken();
	const writer = `new${token}`.slice(0, 20);
	const name = `Neu ${token}`;
	await createUser(page, { username: writer, role: 'user', displayName: name });

	await page.setViewportSize({ width: 360, height: 780 });
	await page.goto(`/?author=${writer}`);
	await expect(page.locator('h2').getByText(name, { exact: true })).toBeVisible();
	await expect(page.getByText('0 recipes', { exact: true })).toBeVisible();
	await expect(cards(page)).toHaveCount(0);

	await page.setViewportSize({ width: 1280, height: 800 });
	await expect(page.getByRole('heading', { level: 2, name: `Recipes by ${name}` })).toBeVisible();
	await expect(page.getByRole('button', { name: `Remove filter ${name}` })).toBeVisible();
	await expect(page.getByRole('button', { name: `Remove filter ${writer}` })).toHaveCount(0);
});

// A word in a step shows the quantity of the ingredient it names. These two
// cover the editor end of that: what the matcher proposes, and what the author
// links by hand when the matcher cannot decide.
/**
 * Opens every step's folded list of links. The list is where a link's remove
 * button lives now that the sentence's underline is the only thing shown by
 * default.
 */
// Waits for a toggle first: the links are derived after the editor renders,
// and a loop that counts before then opens nothing and passes.
async function showLinks(page: Page) {
	await expect(page.locator('button[aria-controls^="step-links-"]').first()).toBeVisible();
	const folded = page.locator('button[aria-controls^="step-links-"][aria-expanded="false"]');
	while ((await folded.count()) > 0) await folded.first().click();
}

const REFERENCE_RECIPE = {
	description: 'Nachtisch',
	servings: 4,
	prepMinutes: null,
	cookMinutes: null,
	sourceUrl: null,
	sourceName: null,
	tags: [],
	ingredientGroups: [
		{
			name: 'Grütze',
			ingredients: [
				{ quantity: 400, unit: 'ml', name: 'Saft', note: null },
				{ quantity: 100, unit: 'g', name: 'Zucker', note: null }
			]
		},
		{
			name: 'Vanillesoße',
			ingredients: [{ quantity: 50, unit: 'g', name: 'Zucker', note: null }]
		}
	]
};

// Deliberately full of characters an HTML round trip would mangle: opening the
// editor has to give the text back exactly, which is why the suggestions are
// ProseMirror decorations rather than anything written into the document.
const REFERENCE_STEP = 'Saft aufkochen.  Bei < 100 °C & ruhen lassen.';

test('accepts the matcher’s ingredient links and leaves the text as written', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Grütze ${token}`,
		steps: [{ text: REFERENCE_STEP, references: [], times: [] }]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	const step = page.getByRole('textbox', { name: 'Step 1', exact: true });
	await expect(step).toBeVisible();
	// Not `toHaveText`, which collapses whitespace: this has to be the very
	// string that was stored, character for character.
	expect(await step.innerText()).toBe(REFERENCE_STEP);

	// One proposal: "Saft". "Zucker" is not proposed, because it names a row in
	// two groups and only the author can say which one is meant. The row above
	// the steps counts what is open; a save takes none of it, and the save
	// button stays a plain "Save" (on a phone a check, named "Save").
	const banner = page.getByText('1 suggestion open');
	await expect(banner).toBeVisible();
	if (!(await phoneLayout(page))) {
		await expect(page.getByRole('button', { name: 'Save', exact: true })).toHaveText('Save');
	}

	// The proposal is a dotted underline over the word - a decoration, not
	// anything written into the text.
	const marked = page.locator('[data-ref-word="Saft"]');
	await expect(marked).toHaveClass('ref-suggestion');

	// Only the row's circle takes them all; its label is no button.
	await banner.click();
	await expect(marked).toHaveClass('ref-suggestion');
	await page.getByRole('button', { name: 'Accept all' }).click();
	// Accepting runs no keystroke through the editor, so the underline only
	// turns solid if the state change is pushed into ProseMirror. This is the
	// assertion that holds that bridge in place.
	await expect(marked).toHaveClass('ref-confirmed');
	// Nothing left to take over, so the banner goes with it.
	await expect(banner).toHaveCount(0);
	await showLinks(page);
	await expect(
		page.getByRole('button', { name: 'Remove link "Saft · 400 ml · Grütze"' })
	).toBeVisible();

	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	// The step reads as it was written, with the quantity beside the word. The
	// exact string was already checked in the editor above; here the word and
	// its annotation are separate elements, so this asserts the tail and the
	// quantity rather than one run of text.
	await expect(page.getByRole('main')).toContainText('Bei < 100 °C & ruhen lassen.');
	await expect(page.getByText('(400 ml)', { exact: true })).toBeVisible();
});

test('links the right one of two same-named ingredients with the @ picker', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Soße ${token}`,
		steps: [{ text: 'Alles verrühren.', references: [], times: [] }]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	const step = page.getByRole('textbox', { name: 'Step 1', exact: true });
	await step.click();
	await page.keyboard.press('End');

	// Typing an ingredient's name underlines it as a proposal.
	await page.keyboard.type(' Saft dazugeben.');
	await expect(page.locator('[data-ref-word="Saft"]')).toHaveClass('ref-suggestion');

	await page.keyboard.type(' @Zuck');

	// Both rows side by side, quantity and group and all - this is what makes
	// the ambiguity decidable at the moment of choosing.
	const picker = page.getByRole('listbox', { name: 'Link ingredient' });
	await expect(picker.getByRole('option')).toHaveCount(2);
	await expect(picker.getByRole('option').first()).toHaveAccessibleName('Zucker · 100 g · Grütze');
	await expect(picker.getByRole('option').last()).toHaveAccessibleName(
		'Zucker · 50 g · Vanillesoße'
	);

	// Take the second one, the 50 g from the custard.
	await page.keyboard.press('ArrowDown');
	await page.keyboard.press('Enter');
	// The plain word lands in the text; nothing else does.
	await expect(step).toHaveText('Alles verrühren. Saft dazugeben. Zucker', { useInnerText: true });
	await showLinks(page);
	await expect(
		page.getByRole('button', { name: 'Remove link "Zucker · 50 g · Vanillesoße"' })
	).toBeVisible();

	// The pick is one undo step: the word and its link go together, and come back together.
	await selectInStep(step, 'Saft', 'caret');
	await page.keyboard.press('ControlOrMeta+z');
	await expect(step).not.toContainText('Zucker', { useInnerText: true });
	await page.keyboard.press('ControlOrMeta+Shift+z');
	await expect(step).toHaveText('Alles verrühren. Saft dazugeben. Zucker', { useInnerText: true });
	await expect(
		page.getByRole('button', { name: 'Remove link "Zucker · 50 g · Vanillesoße"' })
	).toBeVisible();

	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	await expect(page.getByText('(50 g)', { exact: true })).toBeVisible();
});

test('keeps a picked name apart from the word the @ was typed before', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Soße ${token}`,
		steps: [{ text: 'Zum Schluss Sahne.', references: [], times: [] }]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	const step = page.getByRole('textbox', { name: 'Step 1', exact: true });
	// The caret goes right in front of "Sahne": where a click or a tap lands
	// differs per project.
	await selectInStep(step, 'Sahne', 'caret');
	await page.keyboard.type('@Zuck');

	const picker = page.getByRole('listbox', { name: 'Link ingredient' });
	await expect(picker.getByRole('option')).toHaveCount(2);
	await page.keyboard.press('Enter');
	// Glued on, `ZuckerSahne` would be a link the API refuses.
	await expect(step).toHaveText('Zum Schluss Zucker Sahne.', { useInnerText: true });

	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	await expect(page.getByText('(100 g)', { exact: true })).toBeVisible();
});

test('offers every ingredient in the @ picker and scrolls to the row the arrows reach', async ({
	page
}) => {
	const token = uniqueToken();
	const names = Array.from({ length: 12 }, (_, at) => `Zutat ${at + 1}`);
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Viele ${token}`,
		ingredientGroups: [
			{
				name: null,
				ingredients: names.map((name) => ({ quantity: null, unit: null, name, note: null }))
			}
		],
		steps: [{ text: 'Alles verrühren.', references: [], times: [] }]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	const step = page.getByRole('textbox', { name: 'Step 1', exact: true });
	await step.click();
	await page.keyboard.press('End');
	await page.keyboard.type(' @');

	// The list scrolls, so it holds every row rather than the first few.
	const picker = page.getByRole('listbox', { name: 'Link ingredient' });
	await expect(picker.getByRole('option')).toHaveText(names);
	// ArrowUp wraps to the last row, which starts below the fold.
	await page.keyboard.press('ArrowUp');
	await expect(picker.getByRole('option', { name: 'Zutat 12' })).toBeInViewport();
});

test('points out a repeated name once its field is left, and stops a save over two groups of one name', async ({
	page
}) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Kuchen ${token}`,
		ingredientGroups: [
			{
				name: 'Teig',
				ingredients: [
					{ quantity: 100, unit: 'g', name: 'Zucker', note: null },
					{ quantity: 200, unit: 'g', name: 'Mehl', note: null }
				]
			},
			{ name: 'Streusel', ingredients: [{ quantity: 80, unit: 'g', name: 'Butter', note: null }] }
		],
		steps: [{ text: 'Alles verrühren.', references: [], times: [] }]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	const groupName = page.getByRole('textbox', { name: 'Rename group' }).last();
	// The visible line; a live region beside it says the same to a screen reader.
	const groupError = page.locator('[id^="ingredient-group-error-"]', {
		hasText: 'Same name as the group “Teig” above. Give it a name of its own.'
	});

	// Nothing while the name is being typed, even when it passes a clash.
	await groupName.fill('');
	await groupName.pressSequentially('Teig');
	await expect(groupError).toHaveCount(0);
	await page.getByRole('textbox', { name: 'Title' }).click();
	await expect(groupError).toBeVisible();
	await expect(groupName).toHaveAttribute('aria-invalid', 'true');

	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(groupName).toBeFocused();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}/edit`);

	// Renaming the other group fixes the clash as well, and the error the
	// save left behind goes with it.
	const firstName = page.getByRole('textbox', { name: 'Rename group' }).first();
	await firstName.fill('Boden');
	await expect(groupError).toHaveCount(0);
	await firstName.fill('Teig');
	await page.getByRole('textbox', { name: 'Title' }).click();
	await expect(groupError).toBeVisible();

	// Typing the clash away clears it without leaving the field.
	await groupName.click();
	await groupName.press('End');
	await groupName.pressSequentially(' oben');
	await expect(groupError).toHaveCount(0);

	// The same name twice in one group is pointed out at both rows, and saves.
	const hint = page.getByText("Duplicate – can't be linked");
	await page.locator('input[id^="ingredient-name-"]').nth(1).fill('Zucker');
	await expect(hint).toHaveCount(0);
	await page.getByRole('textbox', { name: 'Title' }).click();
	await expect(hint).toHaveCount(2);

	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
});

test('closes the @ picker on a press outside it, and a fresh @ opens it again', async ({
	page
}) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Soße ${token}`,
		steps: [{ text: 'Alles verrühren.', references: [], times: [] }]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	const step = page.getByRole('textbox', { name: 'Step 1', exact: true });
	await step.click();
	await page.keyboard.press('End');
	await page.keyboard.type(' @Zuck');
	const picker = page.getByRole('listbox', { name: 'Link ingredient' });
	await expect(picker).toBeVisible();

	await page.getByRole('textbox', { name: 'Title' }).click();
	await expect(picker).toHaveCount(0);

	await step.click();
	await page.keyboard.press('End');
	await page.keyboard.type(' @Sa');
	await expect(picker.getByRole('option')).toHaveCount(1);
});

/** Selects `word` in a step field. */
function selectWord(step: Locator, word: string) {
	return selectInStep(step, word, 'word');
}

test('shows a link in a card at its word, and folds the list away', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Karte ${token}`,
		steps: [
			{
				text: 'Den Saft aufkochen.',
				references: [{ word: 'Saft', groupName: 'Grütze', ingredientName: 'Saft' }],
				times: []
			}
		]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	// The underline is what shows the link; under the step there is only the
	// count, folded.
	const summary = page.getByRole('button', { name: '1 linked' });
	await expect(summary).toHaveAttribute('aria-expanded', 'false');
	const remove = page.getByRole('button', {
		name: 'Remove link "Saft · 400 ml · Grütze"'
	});
	await expect(remove).toHaveCount(0);

	// The caret in the word brings up the card (a tap, the sheet, on a phone):
	// what the word points at, and without the group - "Saft" is in one group
	// only, so it would say nothing.
	const step = page.getByRole('textbox', { name: 'Step 1', exact: true });
	const card = await openWordCard(page, step, 'Saft', 'aft', 'Link for "Saft"');
	await expect(card).toContainText('Saft');
	await expect(card).toContainText('400 ml');
	await expect(card).not.toContainText('Grütze');

	// Taking the link off there makes the word a proposal again. At the caret
	// the card turns into the proposal's and the caret stays in the step; a
	// phone's sheet closes. A save takes no proposal, so it stays off.
	await card.getByRole('button', { name: 'Remove' }).click();
	await expect(page.locator('[data-ref-word="Saft"]')).toHaveClass('ref-suggestion');
	await expect(page.getByRole('button', { name: '1 suggestion', exact: true })).toBeVisible();
	if (await phoneLayout(page)) {
		await expect(card).toHaveCount(0);
	} else {
		await expect(card.getByRole('button', { name: 'Accept' })).toBeVisible();
		await expect(step).toBeFocused();
	}
	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	await expect(page.getByText('(400 ml)', { exact: true })).toHaveCount(0);
});

test('takes a proposal from the card at its word', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Vorschlag ${token}`,
		steps: [{ text: 'Den Saft aufkochen.', references: [], times: [] }]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	const marked = page.locator('[data-ref-word="Saft"]');
	await expect(marked).toHaveClass('ref-suggestion');
	await expect(page.getByRole('button', { name: '1 suggestion', exact: true })).toBeVisible();

	const step = page.getByRole('textbox', { name: 'Step 1', exact: true });
	const card = await openWordCard(page, step, 'Saft', 'aft', 'Link for "Saft"');
	// A phone's tap takes the keyboard away: the sheet needs the room.
	if (await phoneLayout(page)) await expect(step).not.toBeFocused();
	await card.getByRole('button', { name: 'Accept' }).click();

	// The card closes with the accept; back on the word, it shows the link.
	await expect(marked).toHaveClass('ref-confirmed');
	await expect(card).toHaveCount(0);
	await selectInStep(step, 'aufkochen', 'caret');
	const again = await openWordCard(page, step, 'Saft', 'aft', 'Link for "Saft"');
	await expect(again.getByRole('button', { name: 'Change' })).toBeVisible();

	const caretAt = () =>
		step.evaluate(
			(field) => (field as HTMLElement & { editor: Editor }).editor.state.selection.from
		);
	if (await phoneLayout(page)) {
		// The sheet's keyboard button goes back to writing, the caret at the
		// end of the word ("Den Saft" ends at 9, the paragraph opening at 0).
		await again.getByRole('button', { name: 'Edit the word' }).click();
		await expect(again).toHaveCount(0);
		await expect(step).toBeFocused();
		await expect.poll(caretAt).toBe(9);
		// A caret in the word opens nothing on a phone: only a tap does.
		await selectInStep(step, 'aft', 'caret');
		await expect(page.getByRole('dialog')).toHaveCount(0);
		await expect(page.getByRole('group', { name: 'Link for "Saft"' })).toHaveCount(0);
		// The step stays bright under the dim and takes a tap, which closes the
		// sheet and writes where it landed.
		const third = await openWordCard(page, step, 'Saft', 'aft', 'Link for "Saft"');
		// The sheet scrolls the word into the room above it: a tap aimed
		// before the scroll ends lands on the dim instead.
		await expect
			.poll(async () => {
				const before = (await step.boundingBox())!.y;
				await page.waitForTimeout(100);
				return (await step.boundingBox())!.y - before;
			})
			.toBe(0);
		await step.tap({ position: { x: 8, y: 8 } });
		await expect(third).toHaveCount(0);
		await expect(step).toBeFocused();
		await expect.poll(caretAt).toBe(1);
	} else {
		// Escape closes the card at the caret, until the caret moves.
		await page.keyboard.press('Escape');
		await expect(again).toHaveCount(0);
		await selectInStep(step, 'aufkochen', 'caret');
		await selectInStep(step, 'aft', 'caret');
		await expect(again).toBeVisible();
	}
	await expect(page.getByRole('button', { name: '1 linked' })).toBeVisible();
});

test('offers every group of a name that occurs in two, guessing from the sentence', async ({
	page
}) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Klopse ${token}`,
		ingredientGroups: [
			{
				name: 'Klopse',
				ingredients: [
					{ quantity: 500, unit: 'g', name: 'Hackfleisch', note: null },
					{ quantity: 1, unit: 'TL', name: 'Salz', note: null }
				]
			},
			{
				name: 'Kapernsauce',
				ingredients: [
					{ quantity: 200, unit: 'ml', name: 'Sahne', note: null },
					{ quantity: 1, unit: 'Prise', name: 'Salz', note: null }
				]
			}
		],
		steps: [
			{ text: 'Mit Sahne verrühren und mit Salz abschmecken.', references: [], times: [] },
			{ text: 'Mit Salz würzen.', references: [], times: [] }
		]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	const first = page.getByRole('textbox', { name: 'Step 1', exact: true });
	const second = page.getByRole('textbox', { name: 'Step 2', exact: true });
	await expect(first.locator('[data-ref-word="Salz"]')).toHaveClass('ref-suggestion');
	await expect(second.locator('[data-ref-word="Salz"]')).toHaveClass('ref-suggestion');

	// Step 1's other ingredient comes from the sauce, so that is the guess and
	// "Accept all" would take it; step 2 gives nothing away and waits.
	await expect(page.getByText('2 suggestions open')).toBeVisible();
	await expect(page.getByText('"Salz" in step 2 is waiting for your choice')).toBeVisible();

	const card = await openWordCard(page, first, 'Salz', 'al', 'Link for "Salz"');
	const options = card.getByRole('radio');
	await expect(options).toHaveText([/Kapernsauce.*goes with Sahne/, /Klopse/]);
	// The guess is picked already; a tap only picks, "Accept" links.
	await expect(card.getByRole('radio', { name: 'Salz · 1 Prise · Kapernsauce' })).toBeChecked();

	// The author knows better: the meatballs' salt.
	await card.getByRole('radio', { name: 'Salz · 1 TL · Klopse' }).click();
	await expect(first.locator('[data-ref-word="Salz"]')).toHaveClass(/\bref-suggestion\b/);
	// On a phone the word the sheet is about is framed in the step.
	if (await phoneLayout(page)) {
		await expect(first.locator('[data-ref-word="Salz"]')).toHaveClass(/\bref-active\b/);
		await expect(second.locator('[data-ref-word="Salz"]')).not.toHaveClass(/\bref-active\b/);
	}
	await card.getByRole('button', { name: 'Accept', exact: true }).click();
	await expect(first.locator('[data-ref-word="Salz"]')).toHaveClass('ref-confirmed');

	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	// Linked as chosen; the waiting word in step 2 stayed a plain word.
	await expect(page.getByText('(1 TL)', { exact: true })).toHaveCount(1);
	await expect(page.getByText('(1 Prise)', { exact: true })).toHaveCount(0);
});

test('names ingredient and group in the choice, and explains a repeated word', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Brot ${token}`,
		ingredientGroups: [
			{
				name: 'Teig',
				ingredients: [
					{ quantity: 1, unit: 'TL', name: 'feines Salz', note: null },
					{ quantity: 1, unit: 'EL', name: 'grobes Salz', note: null },
					{ quantity: 250, unit: 'g', name: 'Quark', note: null }
				]
			}
		],
		steps: [
			{ text: 'Mit Salz würzen.', references: [], times: [] },
			{ text: 'Quark verrühren. Den Quark kühlen.', references: [], times: [] }
		]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	// The head noun matched two rows of one group: the card names each.
	const card = await openWordCard(
		page,
		page.getByRole('textbox', { name: 'Step 1', exact: true }),
		'Salz',
		'al',
		'Link for "Salz"'
	);
	await expect(card.getByText('2 options')).toBeVisible();
	// They share the group, so the ingredient is what each row is titled by,
	// and with no guess "Accept" waits for a pick.
	await expect(card.getByRole('radio')).toHaveText([/feines Salz.*Teig/, /grobes Salz.*Teig/]);
	await expect(card.getByRole('button', { name: 'Accept', exact: true })).toBeDisabled();

	// Only the first "Quark" is linked; the second is drawn faintly and says why.
	const second = page.getByRole('textbox', { name: 'Step 2', exact: true });
	await expect(second.locator('[data-ref-word="Quark"]')).toHaveClass('ref-suggestion');
	await expect(second.locator('.ref-repeat')).toHaveText('Quark');
	const repeat = 'Only the first "Quark" in a step is linked; the quantity shows there.';
	if (await phoneLayout(page)) {
		// On a phone a tap on it opens the same note as a sheet, once the
		// choice's sheet is closed by its handle.
		await card.getByRole('button', { name: 'Close' }).click();
		await second.locator('.ref-repeat').tap();
		await expect(page.getByRole('dialog', { name: 'Link for "Quark"' })).toContainText(repeat);
		return;
	}
	await second.evaluate((field) => {
		const { editor } = field as HTMLElement & {
			editor: {
				chain: () => { focus: () => { setTextSelection: (at: number) => { run: () => void } } };
			};
		};
		const at = (field.textContent ?? '').lastIndexOf('Quark') + 2;
		editor
			.chain()
			.focus()
			.setTextSelection(at + 1)
			.run();
	});
	await expect(page.getByRole('group', { name: 'Link for "Quark"' })).toContainText(repeat);
	// "Link word" there would quietly re-link the first one, so it says so instead.
	await page.getByRole('button', { name: 'Link word' }).click();
	await expect(page.getByRole('listbox', { name: 'Link ingredient' })).toHaveCount(0);
	await expect(page.getByRole('status').filter({ hasText: repeat })).toHaveCount(1);
});

test('lists a choice with an accept per group, and says what a list button did', async ({
	page
}) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Liste ${token}`,
		ingredientGroups: [
			{ name: 'Teig', ingredients: [{ quantity: 1, unit: 'TL', name: 'Salz', note: null }] },
			{ name: 'Guss', ingredients: [{ quantity: 1, unit: 'Prise', name: 'Salz', note: null }] }
		],
		steps: [{ text: 'Mit Salz abschmecken.', references: [], times: [] }]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	// Nothing in the sentence points at a group: the line counts it as waiting.
	const summary = page.getByRole('button', { name: '1 waiting' });
	await expect(summary).toBeVisible();
	await summary.click();

	// The word once, then each group with its own "Accept"; nothing to dismiss.
	const list = page.getByRole('list', { name: 'Linked ingredients' });
	await expect(list.getByRole('button', { name: /^Dismiss/ })).toHaveCount(0);
	await expect(list.getByRole('button', { name: /^Accept/ })).toHaveCount(2);

	await list.getByRole('button', { name: 'Accept "Salz · 1 Prise · Guss"' }).click();
	await expect(
		page.getByRole('status').filter({ hasText: 'Accepted: Salz · 1 Prise · Guss' })
	).toHaveCount(1);
	// The row went; focus did not fall to the page but stayed in the list,
	// on the button now in its place - here the new link's.
	await expect(
		list.getByRole('button', { name: 'Remove link "Salz · 1 Prise · Guss"' })
	).toBeFocused();
});

test('brings a link back when the edit that removed its word is undone', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Rückgängig ${token}`,
		steps: [
			{
				text: 'Den Saft aufkochen.',
				references: [{ word: 'Saft', groupName: 'Grütze', ingredientName: 'Saft' }],
				times: []
			}
		]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	const step = page.getByRole('textbox', { name: 'Step 1', exact: true });
	const chip = page.getByRole('button', { name: 'Remove link "Saft · 400 ml · Grütze"' });
	await showLinks(page);
	await expect(chip).toBeVisible();

	// The word goes, and its link with it, in one undo step...
	await selectWord(step, 'Saft');
	await page.keyboard.press('Backspace');
	await expect(chip).toHaveCount(0);

	// ...so Cmd+Z brings back the word and the link it carried.
	await page.keyboard.press('ControlOrMeta+z');
	await expect(step).toHaveText('Den Saft aufkochen.', { useInnerText: true });
	await showLinks(page);
	await expect(chip).toBeVisible();
	await expect(page.locator('[data-ref-word="Saft"]')).toHaveClass('ref-confirmed');

	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	await expect(page.getByText('(400 ml)', { exact: true })).toBeVisible();
});

test('ends a link with its word, and undoes link actions', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Regeln ${token}`,
		steps: [
			{
				text: 'Den Saft aufkochen.',
				references: [{ word: 'Saft', groupName: 'Grütze', ingredientName: 'Saft' }],
				times: []
			}
		]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	const step = page.getByRole('textbox', { name: 'Step 1', exact: true });
	const saft = page.locator('[data-ref-word="Saft"]');
	await expect(saft).toHaveClass('ref-confirmed');

	// Deleted and typed again, the word is new: proposed, never linked by itself.
	await selectWord(step, 'Saft');
	await page.keyboard.press('Backspace');
	await page.keyboard.type('Saft');
	await expect(saft).toHaveClass('ref-suggestion');

	// Accepting is an undo step of its own. The caret is put in the word
	// rather than clicked there: clicks this quick on one spot would add up
	// to a triple click, which selects the whole step.
	const card = await openWordCard(page, step, 'Saft', 'aft', 'Link for "Saft"');
	await card.getByRole('button', { name: 'Accept', exact: true }).click();
	await expect(saft).toHaveClass('ref-confirmed');
	// Undo is the step's: back in it first, which a phone's sheet left.
	await selectInStep(step, 'aufkochen', 'caret');
	await page.keyboard.press('ControlOrMeta+z');
	await expect(saft).toHaveClass('ref-suggestion');
	await expect(step).toHaveText('Den Saft aufkochen.', { useInnerText: true });

	// "Dismiss" only closes the card: the word stays proposed, as if the
	// recipe were opened again, and the card is back once the caret returns.
	const dismissed = await openWordCard(page, step, 'Saft', 'aft', 'Link for "Saft"');
	await dismissed.getByRole('button', { name: 'Dismiss', exact: true }).click();
	await expect(dismissed).toHaveCount(0);
	await expect(saft).toHaveClass('ref-suggestion');
	await selectInStep(step, 'aufkochen', 'caret');
	const back = await openWordCard(page, step, 'Saft', 'aft', 'Link for "Saft"');
	await expect(back.getByRole('button', { name: 'Accept', exact: true })).toBeVisible();
});

test('saves a step whose linked word was deleted, without the link', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Ohne Wort ${token}`,
		steps: [
			{
				text: 'Den Saft aufkochen.',
				references: [{ word: 'Saft', groupName: 'Grütze', ingredientName: 'Saft' }],
				times: []
			}
		]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	const step = page.getByRole('textbox', { name: 'Step 1', exact: true });
	await selectWord(step, 'Saft');
	await page.keyboard.type('Sud');

	// Replacing the word ends its link; the save sends none.
	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	await expect(page.getByText('Den Sud aufkochen.')).toBeVisible();
	await expect(page.getByText('(400 ml)', { exact: true })).toHaveCount(0);
});

test('shows a link as broken once its ingredient is deleted', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Gelöscht ${token}`,
		steps: [
			{
				text: 'Saft aufkochen.',
				references: [{ word: 'Saft', groupName: 'Grütze', ingredientName: 'Saft' }],
				times: []
			}
		]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	const marked = page.locator('[data-ref-word="Saft"]');
	await expect(marked).toHaveClass('ref-confirmed');

	// Renaming the row is not a break: the link is anchored to the row, so it
	// follows the new name into the payload.
	const name = page.getByRole('textbox', { name: 'Ingredient', exact: true }).first();
	await name.fill('Traubensaft');
	await expect(marked).toHaveClass('ref-confirmed');
	await showLinks(page);
	await expect(
		page.getByRole('button', { name: 'Remove link "Traubensaft · 400 ml · Grütze"' })
	).toBeVisible();

	// Deleting it is: there is no row left to read a name off, and the API
	// would answer the next save with a 422. The editor has to say so before
	// the save, and must not quietly drop the link instead.
	await page.getByRole('button', { name: 'Remove ingredient' }).first().click();

	await expect(marked).toHaveClass('ref-broken');
	await expect(page.getByText('Ingredient missing')).toBeVisible();

	// And the save is refused rather than guessing. The names the link
	// remembers describe a row that is gone; sending them could resolve it onto
	// whatever carries those names now, and dropping it would take the author's
	// work away without a word. So the step says what is wrong and the author
	// decides.
	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page.getByText('This link points at a deleted ingredient')).toBeVisible();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}/edit`);
});

test('keeps the links when the unnamed group is finally given a name', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Benannt ${token}`,
		// One group, no name - what a recipe written in one go looks like - and
		// a link into it. Naming the group renames what the stored reference
		// points at, and the link has to move with it rather than break.
		ingredientGroups: [
			{
				name: null,
				ingredients: [
					{ quantity: 400, unit: 'ml', name: 'Saft', note: null },
					{ quantity: 100, unit: 'g', name: 'Zucker', note: null }
				]
			}
		],
		steps: [
			{
				text: 'Saft aufkochen.',
				references: [{ word: 'Saft', groupName: null, ingredientName: 'Saft' }],
				times: []
			}
		]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	const marked = page.locator('[data-ref-word="Saft"]');
	await expect(marked).toHaveClass('ref-confirmed');

	await page.getByRole('textbox', { name: 'Rename group' }).fill('Grütze');

	// Still confirmed, and the chip now names the group it was just given: the
	// link is anchored to the row, not to the names the row carried when the
	// link was made.
	await expect(marked).toHaveClass('ref-confirmed');
	await showLinks(page);
	await expect(
		page.getByRole('button', { name: 'Remove link "Saft · 400 ml · Grütze"' })
	).toBeVisible();

	// The save has to go through - it used to come back a 422 saying the
	// recipe has no unnamed group, with nothing stored.
	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	await expect(page.getByRole('main')).toContainText('Saft(400 ml) aufkochen.');

	// And the link is in the database, not just on the screen it was made on.
	await page.reload();
	await expect(page.getByRole('main')).toContainText('Saft(400 ml) aufkochen.');
	await page.goto(`/recipes/${recipe.slug}/edit`);
	await expect(page.locator('[data-ref-word="Saft"]')).toHaveClass('ref-confirmed');
	await showLinks(page);
	await expect(
		page.getByRole('button', { name: 'Remove link "Saft · 400 ml · Grütze"' })
	).toBeVisible();
});

test('links a word whose text differs from the ingredient’s name', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Flüssigkeit ${token}`,
		// Nothing here matches an ingredient by text, which is the point: the
		// sentence says "Flüssigkeit" and the list says "Saft". No matcher
		// bridges that, and `@` cannot either - it inserts the ingredient's
		// own name. Only a word the author selects can.
		steps: [{ text: 'Langsam aufkochen: die Flüssigkeit.', references: [], times: [] }]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	const step = page.getByRole('textbox', { name: 'Step 1', exact: true });
	// Three characters of the word, not the word: the editor grows a selection
	// out to whole words before it anchors anything to it, because the API
	// refuses a reference to half of one. Selected through the DOM, because
	// where a click lands in the field differs between the two projects.
	await selectWord(step, 'eit');

	await page.getByRole('button', { name: 'Link word' }).click();
	// The popup names the word it will link, so a selection that grew further
	// than the author meant is visible before anything happens.
	await expect(page.getByText('Ingredient for "Flüssigkeit"').first()).toBeVisible();

	// The query field has the focus, so the whole link is one typed word and
	// Enter - no pointer needed past opening it.
	await page.keyboard.type('Saft');
	const picker = page.getByRole('listbox', { name: 'Link ingredient' });
	await expect(picker.getByRole('option')).toHaveCount(1);
	await page.keyboard.press('Enter');

	await showLinks(page);
	await expect(
		page.getByRole('button', { name: 'Remove link "Saft · 400 ml · Grütze"' })
	).toBeVisible();
	// The word is underlined as a confirmed link although the text says
	// something else entirely, and the step's text is untouched.
	await expect(page.locator('[data-ref-word="Flüssigkeit"]')).toHaveClass('ref-confirmed');
	await expect(step).toHaveText('Langsam aufkochen: die Flüssigkeit.', { useInnerText: true });

	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	// The quantity stands after the word the sentence uses. There is no space
	// in the markup between the two - the gap is a CSS margin - so this is the
	// rendered text exactly as the DOM holds it.
	await expect(page.getByRole('main')).toContainText('Langsam aufkochen: die Flüssigkeit(400 ml).');
});

test('keeps both step editors and their links after a drag', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Sortiert ${token}`,
		steps: [
			{
				text: 'Saft aufkochen.',
				references: [{ word: 'Saft', groupName: 'Grütze', ingredientName: 'Saft' }],
				times: []
			},
			{
				text: 'Zucker einrühren.',
				references: [{ word: 'Zucker', groupName: 'Vanillesoße', ingredientName: 'Zucker' }],
				times: []
			}
		]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	const first = page.getByRole('listitem', { name: 'Step 1' });
	const second = page.getByRole('listitem', { name: 'Step 2' });
	await expect(first.getByRole('textbox')).toHaveText('Saft aufkochen.');
	await expect(second.getByRole('textbox')).toHaveText('Zucker einrühren.');

	// A drag is the one interaction that crosses a keyed `{#each}`, a
	// ProseMirror view's lifetime and svelte-dnd-action at once: the list is
	// keyed by step id so the editors move with their steps rather than being
	// rebuilt around new text.
	const handle = second.getByRole('button', { name: 'Move step', exact: true });
	// The mouse works in viewport coordinates, and the steps sit below the
	// fold on both viewports: without this the drag would be aimed at a point
	// nothing is under.
	await handle.scrollIntoViewIfNeeded();
	const from = await handle.boundingBox();
	const target = await first.boundingBox();
	expect(from && target).toBeTruthy();
	if (!from || !target) return;
	const grip = { x: from.x + from.width / 2, y: from.y + from.height / 2 };
	await page.mouse.move(grip.x, grip.y);
	await page.mouse.down();
	// svelte-dnd-action starts the drag after a few pixels and then decides
	// where the item belongs on an observation interval, so the pointer has to
	// come to rest over the target before the button is let go.
	await page.mouse.move(grip.x, grip.y - 12, { steps: 4 });
	await page.waitForTimeout(200);
	await page.mouse.move(grip.x, target.y + target.height / 4, { steps: 12 });
	await page.waitForTimeout(400);
	await page.mouse.up();

	// Both texts and both link lists, in the new order.
	await expect(first.getByRole('textbox')).toHaveText('Zucker einrühren.');
	await expect(second.getByRole('textbox')).toHaveText('Saft aufkochen.');
	await showLinks(page);
	await expect(
		first.getByRole('button', { name: 'Remove link "Zucker · 50 g · Vanillesoße"' })
	).toBeVisible();
	await expect(
		second.getByRole('button', { name: 'Remove link "Saft · 400 ml · Grütze"' })
	).toBeVisible();

	// Still a live editor rather than the corpse of one: typing has to reach
	// the step it moved with.
	await first.getByRole('textbox').click();
	await page.keyboard.press('End');
	await page.keyboard.type(' Fertig.');
	await expect(first.getByRole('textbox')).toHaveText('Zucker einrühren. Fertig.');

	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	await expect(page.getByRole('main')).toContainText('Zucker(50 g) einrühren. Fertig.');
	await expect(page.getByRole('main')).toContainText('Saft(400 ml) aufkochen.');
});

// Every zone in the editor is its own kind of list. Without a zone type
// svelte-dnd-action files them all under one, so a keyboard drag offered to
// carry an ingredient into the steps, and dropping it there broke the page.
// Groups share a type, so an ingredient still moves between them.
test('a dragged ingredient stays among the ingredients', async ({ page }) => {
	const errors: string[] = [];
	page.on('pageerror', (error) => errors.push(error.message));

	await openNewRecipe(page);
	await page.getByRole('textbox', { name: 'Ingredient', exact: true }).fill('Lemon');

	const ingredients = page.getByRole('list', { name: 'Ingredients' });
	const steps = page.getByRole('list', { name: 'Steps', exact: true });
	const alert = page.locator('#dnd-action-aria-alert');

	// Enter on the handle starts the drag.
	await ingredients.getByRole('button', { name: 'Move ingredient' }).first().focus();
	await page.keyboard.press('Enter');

	await expect(alert).toHaveText(
		'Moving "Lemon". Use the arrow keys to move it within the list Ingredients.'
	);
	// A zone the item may enter is made focusable for the drag.
	await expect(steps).toHaveAttribute('tabindex', '-1');
	await page.keyboard.press('Escape');

	await page.getByRole('button', { name: 'Add group' }).click();
	await ingredients.getByRole('button', { name: 'Move ingredient' }).first().focus();
	await page.keyboard.press('Enter');
	await expect(alert).toContainText('or Tab to switch to another list');
	await expect(steps).toHaveAttribute('tabindex', '-1');
	await page.keyboard.press('Escape');

	expect(errors).toEqual([]);
});

// The phone's running head and contents sheet. A short recipe cannot scroll
// "Steps" to the top, so the scroll a pick starts ends at the foot of the
// page - where the spy used to hand the highlight straight on to the last
// section. The picked entry has to hold until the reader scrolls again.
test('the contents sheet jumps to a section and the running head keeps it', async ({
	page
}, testInfo) => {
	test.skip(
		!testInfo.project.name.startsWith('mobile'),
		'the running head is the phone layout only'
	);

	const fixture = loadFixture(5);
	const title = `Contents ${uniqueToken()}`;
	const recipe = await createRecipe(page, {
		...fixture,
		title,
		ingredientGroups: [
			{
				...fixture.ingredientGroups[0],
				ingredients: fixture.ingredientGroups[0].ingredients.slice(0, 1)
			}
		],
		steps: fixture.steps.slice(0, 1).map((step) => ({ ...step, references: [], times: [] })),
		editPolicy: 'open'
	});
	await page.goto(`/recipes/${recipe.slug}/edit`);

	const head = page.getByRole('button', { name: 'Basics, open contents' });
	await expect(head).toBeVisible();
	await head.click();

	const sheet = page.getByRole('dialog', { name: 'Contents' });
	await expect(sheet.getByRole('button', { name: /^Basics/ })).toHaveAttribute(
		'aria-current',
		'true'
	);
	// The summaries read the live form.
	await expect(sheet.getByRole('button', { name: /^Basics/ })).toContainText(title);
	await expect(sheet.getByRole('button', { name: /^Ingredients/ })).toContainText('1');
	await expect(sheet.getByRole('button', { name: /^Steps/ })).toContainText('1');
	await expect(sheet.getByRole('button', { name: /^Editing/ })).toContainText('Everyone');

	await sheet.getByRole('button', { name: /^Steps/ }).click();
	await expect(sheet).toBeHidden();
	const steps = page.getByRole('button', { name: 'Steps, open contents' });
	await expect(steps).toBeVisible();
	// Long enough for the smooth scroll to settle at the foot of the page.
	await page.waitForTimeout(1000);
	await expect(steps).toBeVisible();
	// The heading the jump went to is not under the running head.
	const heading = page.getByRole('heading', { name: 'Steps', exact: true });
	expect((await heading.boundingBox())!.y).toBeGreaterThanOrEqual(
		(await steps.boundingBox())!.y + (await steps.boundingBox())!.height
	);
	// A report that arrives after the scroll has ended, without the window
	// moving - what a late observer callback amounts to - must not move it.
	await page.evaluate(() => window.dispatchEvent(new Event('scroll')));
	await expect(steps).toBeVisible();
	// The reader scrolling again hands the head back to the scroll position:
	// it lets go of the jump's section. Which one it then names depends on
	// how tall the sections above are, so the test does not pin it.
	await page.mouse.wheel(0, -400);
	await expect(steps).toBeHidden();
	await expect(page.getByRole('button', { name: /, open contents$/ })).toBeVisible();
});

test('opens the full unit list on every tap, even over a chosen unit', async ({ page }) => {
	await page.setViewportSize({ width: 360, height: 780 });
	await openNewRecipe(page);

	const unit = page.getByRole('combobox', { name: 'Unit', exact: true });
	const list = page.getByRole('listbox', { name: 'Unit suggestions' });
	const everyUnit = [
		'g',
		'kg',
		'ml',
		'l',
		'tsp',
		'tbsp',
		'cup',
		'fl oz',
		'oz',
		'lb',
		'pinch',
		'piece'
	];

	// Typing narrows the list; a tap on an option takes it.
	await unit.fill('tb');
	await expect(list.getByRole('option')).toHaveText(['tbsp']);
	await list.getByRole('option', { name: 'tbsp' }).click();
	await expect(unit).toHaveValue('tbsp');
	await expect(list).toBeHidden();

	// A second tap on the field that kept its focus opens it again - with
	// every unit, not only the ones that match the value, and the current one
	// marked. A native datalist showed nothing here.
	await unit.click();
	await expect(list.getByRole('option')).toHaveText(everyUnit);
	await expect(list.getByRole('option', { name: 'tbsp' })).toHaveAttribute('aria-selected', 'true');
	const box = await list.boundingBox();
	expect(box!.x + box!.width).toBeLessThanOrEqual(360);
	await list.getByRole('option', { name: 'g', exact: true }).click();
	await expect(unit).toHaveValue('g');

	// The chevron opens it too, and the keys walk it from the field.
	await page.getByRole('button', { name: 'Show units' }).click();
	await expect(list.getByRole('option')).toHaveText(everyUnit);
	await unit.press('ArrowDown');
	await unit.press('Enter');
	await expect(unit).toHaveValue('kg');

	// Free text stays free: a unit of its own is kept as typed.
	await unit.fill('bunch');
	await expect(list).toBeHidden();
	await unit.press('Tab');
	await expect(unit).toHaveValue('bunch');
});

// Enter - a phone keyboard's "next" key - walks an ingredient: name, amount,
// unit. Only from the unit (or the note) does it start the next ingredient,
// and Enter that picks a unit from the list only picks it.
test('Enter walks an ingredient row before it adds the next', async ({ page }) => {
	await openNewRecipe(page);
	const ingredients = page.getByRole('list', { name: 'Ingredients' });
	const names = ingredients.getByRole('textbox', { name: 'Ingredient', exact: true });
	await expect(names).toHaveCount(1);

	await names.first().fill('Flour');
	await names.first().press('Enter');
	const amount = ingredients.getByRole('textbox', { name: 'Amount', exact: true }).first();
	await expect(amount).toBeFocused();
	await expect(names).toHaveCount(1);
	await amount.fill('200');
	await amount.press('Enter');
	const unit = ingredients.getByRole('combobox', { name: 'Unit', exact: true }).first();
	await expect(unit).toBeFocused();

	await unit.press('ArrowDown');
	await unit.press('Enter');
	await expect(unit).not.toHaveValue('');
	await expect(names).toHaveCount(1);
	await unit.press('Enter');
	await expect(names).toHaveCount(2);
	await expect(names.nth(1)).toBeFocused();
});

test('leaves the unit list shut when a scroll starts on the chevron', async ({ page }) => {
	await openNewRecipe(page);

	const unit = page.getByRole('combobox', { name: 'Unit', exact: true });
	const list = page.getByRole('listbox', { name: 'Unit suggestions' });
	const chevron = page.getByRole('button', { name: 'Show units' });

	// A finger that lands on the chevron and drags is a scroll: the browser
	// takes it over and cancels the pointer, and no click follows.
	const touch = { pointerType: 'touch', isPrimary: true, bubbles: true, cancelable: true };
	await chevron.dispatchEvent('pointerdown', touch);
	await chevron.dispatchEvent('pointercancel', touch);
	await expect(list).toBeHidden();
	await expect(unit).not.toBeFocused();

	// A tap still opens it.
	await chevron.click();
	await expect(list).toBeVisible();
	await expect(unit).toBeFocused();
	await chevron.click();
	await expect(list).toBeHidden();
});

test('sets the unit list apart from the fields in dark mode', async ({ page }) => {
	// The shadow that sets the list apart in light does not read on dark
	// surfaces, so the popover tokens have to carry it: a list in the fields'
	// own color and border melts into the rows below it.
	await page.emulateMedia({ colorScheme: 'dark' });
	await openNewRecipe(page);

	const unit = page.getByRole('combobox', { name: 'Unit', exact: true });
	await unit.click();
	const list = page.getByRole('listbox', { name: 'Unit suggestions' });
	await expect(list).toBeVisible();

	// The row's field box and the list's frame carry the colors; the input
	// and the listbox inside them are transparent.
	const style = (el: Element, frame: string) => {
		const box = frame === 'fieldset' ? el.closest('fieldset')! : el.parentElement!;
		const cs = getComputedStyle(box);
		return { background: cs.backgroundColor, border: cs.borderTopColor };
	};
	const field = await unit.evaluate(style, 'fieldset');
	const popover = await list.evaluate(style, 'parent');
	expect(popover.background).not.toBe(field.background);
	expect(popover.border).not.toBe(field.border);
});

// The search field suggests tags while it has focus. Picking one turns it
// into a filter, clears what was typed to find it and keeps the focus in the
// field, so the next tag is one more word away - on both layouts, phones
// having no row of every tag at all.
test('finds a tag through the search and picks it', async ({ page, isMobile }) => {
	const token = uniqueToken();
	const tag = `${token}soup`;
	await createRecipe(page, { ...loadFixture(1), title: `Tagged ${token}`, tags: [tag] });
	await createRecipe(page, { ...loadFixture(2), title: `Other ${token}`, tags: [`${token}stew`] });

	await page.goto('/');
	const field = searchField(page);
	await field.fill(`${token}s`);
	const list = page.getByRole('listbox', { name: 'Suggestions' });
	await expect(list.getByRole('option')).toHaveText([
		`Search “${token}s” in the text`,
		/soup.*1$/,
		/stew.*1$/
	]);

	await list.getByRole('option', { name: new RegExp(`^${tag}`) }).click();
	await expect(page).toHaveURL(new RegExp(`tags=${tag}`));
	await expect(cards(page)).toHaveText([`Tagged ${token}`]);
	await expect(field).toHaveValue('');
	await expect(field).toBeFocused();
	await expect(list).toHaveCount(0);
	await expect(
		isMobile
			? page.getByRole('button', { name: `Remove filter ${tag}` })
			: page.getByRole('button', { name: new RegExp(`^${tag}`), pressed: true })
	).toBeVisible();
});

// Tags narrow with AND, so the suggestions only offer what still has a
// recipe beside the chosen ones, counted among those recipes.
test('suggests only tags that still match the chosen ones', async ({ page, isMobile }) => {
	const token = uniqueToken();
	const [a, b, c] = [`${token}a`, `${token}b`, `${token}c`];
	await createRecipe(page, { ...loadFixture(1), title: `Both ${token}`, tags: [a, b] });
	await createRecipe(page, { ...loadFixture(2), title: `Only A ${token}`, tags: [a] });
	await createRecipe(page, { ...loadFixture(3), title: `Only C ${token}`, tags: [c] });

	await page.goto(`/?tags=${a}`);
	await expect(cards(page)).toHaveCount(2);
	await searchField(page).fill(token);
	const list = page.getByRole('listbox', { name: 'Suggestions' });
	await expect(
		list.getByRole('option', { name: new RegExp(`^${a}`), selected: true })
	).toBeVisible();
	await expect(list.getByRole('option', { selected: true })).toHaveCount(1);
	await expect(list.getByRole('option', { name: new RegExp(`^${b}`) })).toHaveText(/1$/);
	await expect(list.getByRole('option', { name: new RegExp(`^${c}`) })).toHaveCount(0);
	// The chosen tag leads, right under the text row - and on desktop the row
	// of tags too, however it ranks by count.
	await expect(list.getByRole('option').nth(1)).toHaveAccessibleName(new RegExp(`^${a}`));
	if (!isMobile) {
		await page.keyboard.press('Escape');
		await expect(
			page.getByRole('main').getByRole('button', { pressed: true }).first()
		).toHaveAccessibleName(new RegExp(`^${a}`));
		await expect(
			page
				.getByRole('main')
				.getByRole('button', { pressed: false })
				.filter({ hasText: token })
				.first()
		).toHaveAccessibleName(new RegExp(`^${b}`));
	}
});

// The server only counts tags of recipes the filters still match, so a
// chosen tag that matches nothing - a link to a tag since removed - would
// drop out of the list it is drawn from. It has to stay on screen, or
// nothing would say what is emptying the grid, and stay one tap from gone.
test('keeps a chosen tag on screen when it matches nothing', async ({ page, isMobile }) => {
	const gone = `${uniqueToken()}gone`;
	await page.goto(`/?tags=${gone}`);
	const chip = isMobile
		? page.getByRole('button', { name: `Remove filter ${gone}` })
		: page.getByRole('button', { name: gone, pressed: true });
	await expect(chip).toBeVisible();
	await chip.click();
	await expect(page).not.toHaveURL(/tags=/);
});

// What the filter panel sets stands on the page as light chips: tapping one
// opens the panel, its × drops just that filter.
test("shows the panel's filters as chips", async ({ page }) => {
	// Wide enough that nothing folds, so this checks the chips themselves;
	// the folding has a test of its own below.
	await page.setViewportSize({ width: 1024, height: 800 });
	await page.goto('/?maxMinutes=30&favorites=true');

	await page.getByRole('button', { name: 'Remove filter Favorites' }).click();
	await expect(page).not.toHaveURL(/favorites=/);
	await expect(page).toHaveURL(/maxMinutes=30/);
	await expect(page.getByRole('button', { name: 'Remove filter Favorites' })).toHaveCount(0);

	await page.getByRole('button', { name: /^up to 30/ }).click();
	await expect(page.getByRole('dialog')).toBeVisible();
});

// On a phone the panel's chips and the chosen tags fold into one bar once
// they would need a second line - not merely because there are several.
test("folds the panel's filters only when the row runs out of room", async ({ page }) => {
	await page.setViewportSize({ width: 360, height: 780 });

	await page.goto('/?maxMinutes=180');
	await expect(page.getByRole('button', { name: 'Remove filter up to 3 hr' })).toBeVisible();
	await expect(page.getByRole('button', { name: /^\d active/ })).toHaveCount(0);

	const tag = uniqueToken();
	// No author: on a phone their band, not a chip, says who (see the band's test).
	await page.goto(`/?maxMinutes=90&favorites=true&tags=${tag},${uniqueToken()}`);
	// The tags fold in with the filters, and the bar counts them too.
	const folded = page.getByRole('button', { name: /^4 active/ });
	await expect(folded).toBeVisible();
	await expect(page.getByRole('button', { name: 'Remove filter Favorites' })).toHaveCount(0);
	await expect(page.getByRole('button', { name: `Remove filter ${tag}` })).toHaveCount(0);

	await folded.click();
	await expect(page.getByRole('button', { name: 'Remove filter Favorites' })).toBeVisible();
	await expect(page.getByRole('button', { name: `Remove filter ${tag}` })).toBeVisible();
	await page.getByRole('button', { name: 'Less', exact: true }).click();
	await expect(folded).toBeVisible();
});

// The folded bar's "All" clears the filters and the tags at once - and
// leaves the order, which is not a filter, and the search text, which the
// bar does not show.
test('clears every filter from the folded bar', async ({ page }) => {
	await page.setViewportSize({ width: 360, height: 780 });
	await page.goto(`/?q=zz&maxMinutes=90&favorites=true&tags=${uniqueToken()}&sort=title`);
	// Three filters - the order is not counted as one.
	await expect(page.getByRole('button', { name: /^3 active/ })).toBeVisible();
	await page.getByRole('button', { name: 'Clear filters and tags' }).click();
	await expect(page).not.toHaveURL(/maxMinutes=|favorites=|tags=/);
	await expect(page).toHaveURL(/sort=title/);
	await expect(page).toHaveURL(/q=zz/);
	await expect(page.getByRole('button', { name: /active/ })).toHaveCount(0);
});

// Tags alone fold too, once they no longer fit on one line.
test('folds the chosen tags alone when they run out of room', async ({ page }) => {
	await page.setViewportSize({ width: 360, height: 780 });
	const tags = [0, 1, 2].map((i) => `${uniqueToken()}-long-tag-${i}`);
	await page.goto(`/?tags=${tags.join(',')}`);
	await expect(page.getByRole('button', { name: /^3 active/ })).toBeVisible();
	await expect(page.getByRole('button', { name: `Remove filter ${tags[0]}` })).toHaveCount(0);
});

// On desktop the panel's filters open the tag row in a tray of their own;
// unfolded, the tags wrap under it rather than standing in a column beside it.
test('wraps the unfolded tag row under the filters', async ({ page, isMobile }) => {
	test.skip(isMobile, 'phones have no row of every tag');
	await page.setViewportSize({ width: 1024, height: 800 });
	await page.route(
		(url) => url.pathname === '/api/v1/tags',
		(route) =>
			route.fulfill({
				json: {
					items: Array.from({ length: 16 }, (_, i) => ({ name: `wrap-tag-${i}`, count: 9 }))
				}
			})
	);
	await page.goto('/?maxMinutes=180');
	await page.getByRole('button', { name: /^\+\d+ more$/ }).click();

	const filter = await page.getByRole('button', { name: /^up to 3 hr/ }).boundingBox();
	const last = await page.getByRole('button', { name: /^wrap-tag-15/ }).boundingBox();
	// A later line starts where the filters do, at the row's left edge.
	const lefts = await page
		.getByRole('button', { name: /^wrap-tag-/ })
		.evaluateAll((els) => els.map((el) => Math.round(el.getBoundingClientRect().left)));
	expect(last!.y).toBeGreaterThan(filter!.y + filter!.height);
	expect(Math.min(...lefts)).toBeLessThanOrEqual(Math.round(filter!.x));
});

// A chip removes itself when used. The keyboard's focus moves on to the
// control now in its place instead of falling back to the page.
test('keeps the keyboard in the chip row when a filter is removed', async ({ page, isMobile }) => {
	test.skip(isMobile, 'keyboard focus');
	await page.goto('/?maxMinutes=30&favorites=true');
	const remove = page.getByRole('button', { name: 'Remove filter Favorites' });
	await remove.focus();
	await page.keyboard.press('Enter');
	await expect(page).not.toHaveURL(/favorites=/);
	await expect(page.getByRole('button', { name: /^Remove filter up to 30/ })).toBeFocused();

	// Unfolding hands the focus to the first chip it reveals, folding back to
	// the folded chip.
	await page.setViewportSize({ width: 360, height: 780 });
	await page.goto(`/?maxMinutes=90&favorites=true&tags=${uniqueToken()},${uniqueToken()}`);
	const folded = page.getByRole('button', { name: /^4 active/ });
	await folded.focus();
	await page.keyboard.press('Enter');
	await expect(page.getByRole('button', { name: /^up to 1 hr 30/ })).toBeFocused();
	await page.getByRole('button', { name: 'Less', exact: true }).focus();
	await page.keyboard.press('Enter');
	await expect(folded).toBeFocused();

	// "All" leaves only the order's chip, and the focus on its label - not on
	// its ×, which a second Enter would take the order away with.
	await page.goto(`/?maxMinutes=90&favorites=true&tags=${uniqueToken()}&sort=title`);
	await page.getByRole('button', { name: 'Clear filters and tags' }).focus();
	await page.keyboard.press('Enter');
	await expect(page.getByRole('button', { name: 'A–Z', exact: true })).toBeFocused();
});

// The search field is a combobox: the arrow keys walk the suggestions while
// the focus stays in the field, Enter takes the highlighted tag, and Escape
// closes the list without touching what was typed.
test('walks the tag suggestions with the keyboard', async ({ page }) => {
	const token = uniqueToken();
	const tag = `${token}kb`;
	await createRecipe(page, { ...loadFixture(1), title: `Keyed ${token}`, tags: [tag] });

	await page.goto('/');
	const field = searchField(page);
	await field.fill(token);
	const list = page.getByRole('listbox', { name: 'Suggestions' });
	await expect(list.getByRole('option')).toHaveCount(2);

	await field.press('Escape');
	await expect(list).toHaveCount(0);
	await expect(field).toHaveValue(token);

	await field.press('ArrowDown');
	await field.press('ArrowDown');
	const option = list.getByRole('option', { name: new RegExp(`^${tag}`) });
	await expect(field).toHaveAttribute('aria-activedescendant', (await option.getAttribute('id'))!);
	await field.press('Enter');
	await expect(page).toHaveURL(new RegExp(`tags=${tag}`));
	await expect(field).toHaveValue('');
	await expect(field).toBeFocused();
});

// The suggestions' tags are counted without the search text, so typing must
// not refetch them on every keystroke; only a change of the other filters
// does. (The desktop row's own list follows the text - a separate request.)
test('does not refetch the suggested tags while typing', async ({ page }) => {
	const plain: string[] = [];
	page.on('request', (request) => {
		const url = new URL(request.url());
		if (url.pathname === '/api/v1/tags' && !url.searchParams.has('q')) {
			plain.push(url.search);
		}
	});
	await page.goto('/');
	await expect.poll(() => plain.length).toBe(1);
	await search(page, 'some');
	await search(page, 'something else');
	expect(plain).toHaveLength(1);
	await page.goto('/?maxMinutes=30');
	await expect.poll(() => plain.length).toBe(2);
});

// A pick from the desktop row keeps the search text, so the row counts its
// tags under that text too - the suggestions, whose pick clears it, do not.
test('counts the desktop row under the search text', async ({ page, isMobile }) => {
	test.skip(isMobile, 'phones have no row of every tag');
	const token = uniqueToken();
	const [a, b] = [`${token}a`, `${token}b`];
	await createRecipe(page, { ...loadFixture(1), title: `${token}one`, tags: [a, b] });
	await createRecipe(page, { ...loadFixture(2), title: `${token}two`, tags: [a] });

	await page.goto('/');
	await search(page, `${token}one`);
	await expect(cards(page)).toHaveText([`${token}one`]);
	await expect(page.getByRole('main').getByRole('button', { name: a, exact: false })).toHaveText(
		new RegExp(`^\\s*${a}\\s*1\\s*$`)
	);

	// The suggestions count without the text, since a pick there clears it.
	// Checked there rather than on the full row: in the suite's shared
	// database the unfiltered row folds, and this tag may sit past the fold.
	await searchField(page).fill(token);
	await expect(
		page
			.getByRole('listbox', { name: 'Suggestions' })
			.getByRole('option', { name: new RegExp(`^${a}`) })
	).toHaveText(new RegExp(`${a}\\s*2\\s*$`));
});

// The order a reader picks is remembered on the device and applied wherever
// the address names none; picking the default again forgets it.
test('remembers the chosen order', async ({ page, isMobile }) => {
	await page.goto('/');
	if (isMobile) {
		await page.getByRole('button', { name: 'Filters' }).click();
	}
	await page.getByRole('button', { name: 'Sort by' }).click();
	await page.getByRole('option', { name: 'A–Z' }).click();
	await expect(page).toHaveURL(/sort=title/);

	await page.goto('/');
	await expect(page).toHaveURL(/sort=title/);

	if (isMobile) {
		await page.getByRole('button', { name: 'Back to recently changed' }).click();
	} else {
		await page.getByRole('button', { name: 'Sort by' }).click();
		await page.getByRole('option', { name: 'Recently changed' }).click();
	}
	await expect(page).not.toHaveURL(/sort=/);
	await page.goto('/');
	await expect(page).not.toHaveURL(/sort=/);
});

// A link that names an order applies it, but does not make it the reader's.
test('does not remember an order a link brought', async ({ page }) => {
	await page.goto('/?sort=tasty');
	await expect(page).toHaveURL(/sort=tasty/);
	await page.goto('/');
	await expect(page).not.toHaveURL(/sort=/);
});

// A link naming the default order keeps it over a remembered one: the page
// writes the default back as a bare `/`, which must not read as a new visit.
test('keeps the default order a link names over a remembered one', async ({ page }) => {
	await page.goto('/');
	await page.evaluate(() => localStorage.setItem('rezepte-sort', 'title'));
	await page.goto('/?sort=updated');
	await page.waitForLoadState('networkidle');
	await expect(page).not.toHaveURL(/sort=title/);
});

// Letting go of a link's order on a phone drops it for this view only; the
// reader's own remembered order is still there on the next visit.
test("keeps the remembered order when a link's chip is cleared", async ({ page, isMobile }) => {
	test.skip(!isMobile, "the chip is the phone's");
	await page.goto('/');
	await page.evaluate(() => localStorage.setItem('rezepte-sort', 'title'));
	await page.goto('/?sort=tasty');
	await page.getByRole('button', { name: 'Back to recently changed' }).click();
	await expect(page).not.toHaveURL(/sort=/);
	await page.goto('/');
	await expect(page).toHaveURL(/sort=title/);
});

// On a phone the sort control lives in the panel, so an order other than the
// default stands as a chip ahead of the filters; desktop shows it beside the
// search already.
test('shows a changed order as a chip on phones', async ({ page, isMobile }) => {
	await page.goto('/?sort=tasty');
	const chip = page.getByRole('button', { name: 'Tastiest', exact: true });
	if (!isMobile) {
		await expect(chip).toHaveCount(0);
		return;
	}
	await expect(chip).toBeVisible();
	await chip.click();
	await expect(page.getByRole('dialog')).toBeVisible();
	await page.keyboard.press('Escape');
	await page.getByRole('button', { name: 'Back to recently changed' }).click();
	await expect(page).not.toHaveURL(/sort=/);
	await expect(chip).toHaveCount(0);
});

// The filter panel draws its own scrollbar and keeps it in view whenever
// there is more to scroll: iOS and macOS show the system's only while it moves.
test('keeps a scrollbar in view while the filter panel scrolls', async ({ page, isMobile }) => {
	test.skip(!isMobile, 'a phone-height sheet');
	// No author section: the shared database collects people, and the panel's
	// height must not depend on how many other tests made.
	await page.route(
		(url) => url.pathname === '/api/v1/authors',
		(route) => route.fulfill({ json: { items: [] } })
	);
	await page.setViewportSize({ width: 360, height: 900 });
	await page.goto('/');
	await page.getByRole('button', { name: 'Filters', exact: true }).click();
	const bar = page.getByRole('dialog').locator('[data-scrollbar]');
	// Everything fits: nothing to scroll, no bar.
	await expect(page.getByRole('dialog')).toBeVisible();
	await expect(bar).toHaveCount(0);

	await page.setViewportSize({ width: 360, height: 480 });
	await expect(bar).toBeVisible();
	// The thumb starts at the top of its track and follows the list down.
	const offset = () =>
		bar.evaluate((el) => {
			const thumb = el.firstElementChild as HTMLElement;
			return thumb.getBoundingClientRect().top - el.getBoundingClientRect().top;
		});
	expect(await offset()).toBe(0);
	await page
		.getByRole('dialog')
		.locator('.overflow-y-auto')
		.evaluate((el) => (el.scrollTop = el.scrollHeight));
	await expect.poll(offset).toBeGreaterThan(0);
});

// The overview is a masonry: cards as tall as their content, each placed in
// the shortest column. Boxes come from the layout (`offset*`), not from
// `boundingBox`, so a card tilted under the pointer does not count as moved,
// and are relative to the grid: the JS fallback positions its cards inside it,
// native `grid-lanes` leaves the offset parent further up.
type Box = { title: string; x: number; y: number; width: number; height: number };

function cardBoxes(page: Page): Promise<Box[]> {
	return page.getByRole('main').evaluate((main) =>
		[...main.querySelectorAll<HTMLElement>('article')].map((card) => {
			const grid = card.parentElement!;
			const own = card.offsetParent === grid;
			return {
				title: card.querySelector('h3')!.textContent!.trim(),
				x: card.offsetLeft - (own ? 0 : grid.offsetLeft),
				y: card.offsetTop - (own ? 0 : grid.offsetTop),
				width: card.offsetWidth,
				height: card.offsetHeight
			};
		})
	);
}

// One expect for all pairs: an expect per pair cost the 300-card test 40s.
function expectNoOverlap(boxes: Box[]) {
	const overlaps: string[] = [];
	for (const [i, a] of boxes.entries()) {
		for (const b of boxes.slice(i + 1)) {
			const apart =
				a.x + a.width <= b.x ||
				b.x + b.width <= a.x ||
				a.y + a.height <= b.y ||
				b.y + b.height <= a.y;
			if (!apart) overlaps.push(`${a.title} overlaps ${b.title}`);
		}
	}
	expect(overlaps).toEqual([]);
}

/** Waits until two reads of the layout agree: the masonry places a resize on the next frame. */
async function settledBoxes(page: Page, timeout?: number): Promise<Box[]> {
	let last = '';
	await expect
		.poll(
			async () => {
				const now = JSON.stringify(await cardBoxes(page));
				const same = now === last;
				last = now;
				return same;
			},
			{ timeout }
		)
		.toBe(true);
	return JSON.parse(last) as Box[];
}

function card(page: Page, title: string): Locator {
	return page.getByRole('article', { name: title, exact: true });
}

test('a recipe without a photo is a text card', async ({ page }) => {
	const token = uniqueToken();
	await createRecipe(page, { ...loadFixture(2), title: `Pesto ${token}`, tags: [`${token}t`] });
	const photo = await createRecipe(page, { ...loadFixture(3), title: `Photo ${token}` });
	await uploadImage(page, photo.id, tinyPng([200, 120, 40], 400, 300));
	const long = await createRecipe(page, {
		...loadFixture(4),
		title: `Schwarzwälderkirschtorte ${token}`
	});

	await page.goto(`/?q=${token}`);
	// A 24-letter compound wraps or hyphenates inside the card; the title's
	// clamp would crop it silently, so the title must not scroll sideways.
	const title = card(page, long.title).locator('h3');
	await expect(title).toBeVisible();
	expect(await title.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true);
	const text = card(page, `Pesto ${token}`);
	await expect(text).toBeVisible();
	await expect(text.locator('img[src*="/images/"]')).toHaveCount(0);
	await expect(text.locator('[aria-hidden="true"]', { hasText: /^P$/ })).toBeVisible();
	await expect(text.getByText(`${token}t`)).toBeVisible();

	const pictured = card(page, `Photo ${token}`);
	await expect(pictured.locator('img[src*="/images/"]')).toBeVisible();
	const [textBox, photoBox] = [await text.boundingBox(), await pictured.boundingBox()];
	expect(textBox!.height).toBeLessThan(photoBox!.height);
});

test('photo cards keep their photo’s shape', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, { ...loadFixture(2), title: `Tall ${token}` });
	await uploadImage(page, recipe.id, tinyPng([40, 120, 200], 1200, 1600));

	await page.goto(`/?q=${token}`);
	const img = card(page, `Tall ${token}`).locator('img[src*="/images/"]');
	await expect(img).toBeVisible();
	// 3:4 is taller than the 4:5 a card allows, so the box stops at 4:5.
	const { width, height } = await img.evaluate((el) => ({
		width: el.getBoundingClientRect().width,
		height: el.getBoundingClientRect().height
	}));
	expect(Math.abs(height - width * 1.25)).toBeLessThan(2);
});

test('cards never overlap and keep their place when more load', async ({ page }) => {
	test.setTimeout(90_000);
	const token = uniqueToken();
	for (let i = 0; i < 30; i++) {
		const recipe = await createRecipe(page, {
			...loadFixture(i % 10),
			title: `Masonry ${i} ${token}`,
			tags: i % 4 === 0 ? [] : [`${token}${i}`]
		});
		if (i % 3 === 0) {
			const sizes = [
				[400, 300],
				[300, 400],
				[640, 360]
			] as const;
			const [w, h] = sizes[(i / 3) % 3];
			await uploadImage(page, recipe.id, tinyPng([200, 120, 40], w, h));
		}
	}

	await page.goto(`/?q=${token}`);
	await expect(page.getByRole('main').locator('article')).toHaveCount(24);
	const before = await settledBoxes(page);
	expectNoOverlap(before);

	// Read at the very bottom, where a page that got shorter would have to
	// pull everything down: the last page takes the button away, but not its
	// row, so the reader stays exactly where they were.
	await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
	const scrolled = await page.evaluate(() => scrollY);
	await page.getByRole('button', { name: 'Load more' }).click();
	await expect(page.getByRole('main').locator('article')).toHaveCount(30);
	await expect(page.getByRole('button', { name: 'Load more' })).toHaveCount(0);
	expect(await page.evaluate(() => scrollY)).toBe(scrolled);
	const after = await settledBoxes(page);
	expect(after.slice(0, 24)).toEqual(before);
	expectNoOverlap(after);
	expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);

	// The URL now names page 2; a reload brings back both pages, not page 2 alone.
	await expect(page).toHaveURL(/[?&]page=2/);
	await page.reload();
	await expect(page.getByRole('main').locator('article')).toHaveCount(30);
});

// The Firefox fallback once used 1px grid rows, which Gecko caps at 10 000
// lines; the list is mocked tall enough to cross that, and nothing may stack.
test('cards never overlap in a list taller than 10 000px', async ({ page }) => {
	const token = uniqueToken();
	for (const i of [1, 2, 3]) {
		await createRecipe(page, { ...loadFixture(i), title: `Tall list ${i} ${token}` });
	}
	await page.route(
		(url) => url.pathname === '/api/v1/recipes',
		async (route) => {
			const body = await (await route.fetch()).json();
			const items = Array.from({ length: 300 }, (_, i) => {
				const source = body.items[i % body.items.length];
				return {
					...source,
					id: `${source.id.slice(0, -4)}${String(i).padStart(4, '0')}`,
					slug: `${source.slug}-${i}`,
					title: `${source.title} ${i}`
				};
			});
			await route.fulfill({ json: { ...body, items, total: 300, limit: 300 } });
		}
	);

	// 300 cards keep a phone WebKit busy for seconds on CI's four shared
	// cores: with the 5s default the locator got two answers before it gave
	// up. The test is about where the cards land, not how fast.
	test.setTimeout(60_000);
	await page.goto(`/?q=${token}`);
	await expect(page.getByRole('main').locator('article')).toHaveCount(300, { timeout: 20_000 });
	const boxes = await settledBoxes(page, 20_000);
	expect(Math.max(...boxes.map((box) => box.y + box.height))).toBeGreaterThan(10_000);
	expectNoOverlap(boxes);
});

test('a broken cover becomes a text card', async ({ page }) => {
	const token = uniqueToken();
	const broken = await createRecipe(page, { ...loadFixture(4), title: `Broken ${token}` });
	await uploadImage(page, broken.id, tinyPng([200, 120, 40], 400, 300));
	for (const i of [1, 2, 3]) {
		await createRecipe(page, { ...loadFixture(i), title: `Beside ${i} ${token}` });
	}
	await page.route(`**/images/${broken.id}/**`, (route) => route.fulfill({ status: 404 }));

	await page.goto(`/?q=${token}`);
	const text = card(page, `Broken ${token}`);
	await expect(text.locator('[aria-hidden="true"]', { hasText: /^B$/ })).toBeVisible();
	await expect(text.locator('img[src*="/images/"]')).toHaveCount(0);
	expectNoOverlap(await settledBoxes(page));
});

test('cards re-flow when the window crosses md', async ({ page, isMobile }) => {
	test.skip(isMobile, 'sets both widths itself');
	const token = uniqueToken();
	for (const i of [1, 2, 3, 4, 5]) {
		await createRecipe(page, { ...loadFixture(i), title: `Reflow ${i} ${token}` });
	}

	await page.setViewportSize({ width: 1280, height: 900 });
	await page.goto(`/?q=${token}`);
	await expect(page.getByRole('main').locator('article')).toHaveCount(5);
	expectNoOverlap(await settledBoxes(page));

	await page.setViewportSize({ width: 360, height: 780 });
	const boxes = await settledBoxes(page);
	expectNoOverlap(boxes);
	expect(new Set(boxes.map((box) => box.x)).size).toBe(2);
});

test('cards re-flow when a filter shortens the list', async ({ page, isMobile }) => {
	test.skip(isMobile, 'phones have no row of every tag; see the suggestion tests');
	const token = uniqueToken();
	const tag = `${token}a`;
	for (const i of [1, 2, 3, 4, 5, 6]) {
		await createRecipe(page, {
			...loadFixture(i),
			title: `Shorten ${i} ${token}`,
			// Every other one keeps the tag, so the survivors start low in
			// their columns and must move up.
			tags: i % 2 === 0 ? [tag] : [`${token}b`]
		});
	}
	// As in "narrows the grid with a tag chip": pinned, so the fresh tag's
	// chip is not pushed past the fold by the shared database's tags.
	await page.route(
		(url) => url.pathname === '/api/v1/tags',
		(route) =>
			route.fulfill({
				json: {
					items: [
						{ name: tag, count: 3 },
						{ name: `${token}b`, count: 3 }
					]
				}
			})
	);

	await page.goto('/');
	await search(page, token);
	await expect(cards(page)).toHaveCount(6);
	await settledBoxes(page);
	await page.getByRole('button', { name: tag }).click();
	await expect(cards(page)).toHaveCount(3);

	const boxes = await settledBoxes(page);
	expectNoOverlap(boxes);
	// The first row sits at the grid's top edge.
	expect(Math.min(...boxes.map((box) => box.y))).toBe(0);
});

// A round mark in a rounded corner shares the corner's center (RecipeCard):
// the initials' last circle is centered on the card's bottom-right corner,
// whatever the tags beside them do, so it sits as far from the right edge as
// from the bottom one, on the star's axis.
async function initialsCenter(card: Locator) {
	return card.evaluate((article) => {
		const box = article.getBoundingClientRect();
		const circles = [...article.querySelectorAll('button[aria-label^="Added by"] > *')].filter(
			(el) => el.getBoundingClientRect().width > 10
		);
		const mark = circles[circles.length - 1].getBoundingClientRect();
		return {
			radius: parseFloat(getComputedStyle(article).borderBottomRightRadius),
			size: mark.width,
			right: box.right - (mark.left + mark.width / 2),
			bottom: box.bottom - (mark.top + mark.height / 2)
		};
	});
}

test('the initials sit on the center of the card’s bottom-right corner', async ({
	page,
	isMobile
}) => {
	const token = uniqueToken();
	const tags = ['soup', 'weeknight', 'one pot', 'vegetarian', 'freezes well', 'for guests'].map(
		(tag) => `${tag} ${token}`
	);
	const photo = await createRecipe(page, { ...loadFixture(3), title: `Gap ${token}`, tags });
	await uploadImage(page, photo.id, tinyPng([200, 120, 40], 400, 300));
	await createRecipe(page, { ...loadFixture(2), title: `Gap plain ${token}`, tags });
	await page.goto(`/?q=${token}`);

	if (!isMobile) {
		// The tags wrap to two lines beside the initials, so the bottom
		// alignment is really exercised: a one-line row would pass by luck.
		const tagLines = await card(page, `Gap ${token}`)
			.locator('p', { hasText: tags[0] })
			.evaluate(
				(p) => p.getBoundingClientRect().height / parseFloat(getComputedStyle(p).lineHeight)
			);
		expect(tagLines, 'tag lines').toBeGreaterThanOrEqual(2);
	}
	for (const title of [`Gap ${token}`, `Gap plain ${token}`]) {
		const mark = await initialsCenter(card(page, title));
		expect(mark.radius, title).toBe(isMobile ? 24 : 28);
		expect(mark.size, title).toBe(isMobile ? 24 : 28);
		expect(Math.abs(mark.right - mark.radius), title).toBeLessThanOrEqual(0.5);
		expect(Math.abs(mark.bottom - mark.radius), title).toBeLessThanOrEqual(0.5);
		// The star above shares the axis.
		const star = await card(page, title).getByRole('button', { name: 'Favorite' }).boundingBox();
		const box = await card(page, title).boundingBox();
		expect(
			Math.abs(box!.x + box!.width - (star!.x + star!.width / 2) - mark.right),
			title
		).toBeLessThanOrEqual(0.5);
	}
});

test('the star is a disc on the photo corner’s center', async ({ page, isMobile }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, { ...loadFixture(3), title: `Corner ${token}` });
	await uploadImage(page, recipe.id, tinyPng([200, 120, 40], 400, 300));
	await page.goto(`/?q=${token}`);

	const article = card(page, `Corner ${token}`);
	const button = article.getByRole('button', { name: 'Favorite' });
	const star = await button.boundingBox();
	const img = await article.locator('img[src*="/images/"]').boundingBox();
	// 24px on a phone's 18px photo corner, 28px on the 20px one from md up.
	const [size, radius] = isMobile ? [24, 18] : [28, 20];
	expect(star!.width).toBe(size);
	expect(star!.height).toBe(size);
	expect(Math.abs(star!.x + size / 2 - (img!.x + img!.width - radius))).toBeLessThanOrEqual(0.5);
	expect(Math.abs(star!.y + size / 2 - (img!.y + radius))).toBeLessThanOrEqual(0.5);
	if (isMobile) {
		// The disc is small; its `::after` makes the target thumb-sized.
		const target = await button.evaluate(
			(el: HTMLElement) => el.offsetWidth - 2 * parseFloat(getComputedStyle(el, '::after').left)
		);
		expect(target).toBeGreaterThanOrEqual(44);
	}
});

test('back from a recipe returns to the same place in the list', async ({ page, isMobile }) => {
	const token = uniqueToken();
	// More than a page of 24, so the list needs "Load more" and the recipe
	// opened sits far down it.
	for (let i = 0; i < 30; i++) {
		await createRecipe(page, {
			...loadFixture(i % 3),
			title: `Return ${String(i).padStart(2, '0')} ${token}`,
			tags: [token]
		});
	}

	await page.goto(`/?tags=${token}`);
	await page.getByRole('button', { name: 'Load more' }).click();
	await expect(cards(page)).toHaveCount(30);
	await expect(page).toHaveURL(/page=2/);

	const target = cards(page).filter({ hasText: `Return 02 ${token}` });
	await target.scrollIntoViewIfNeeded();
	const before = await page.evaluate(() => window.scrollY);
	expect(before).toBeGreaterThan(0);
	await target.click();
	await expect(page).toHaveURL(/\/recipes\//);

	// Phones have the header's back button; the desktop has the browser's.
	if (isMobile) {
		await page.getByRole('button', { name: 'Back', exact: true }).click();
	} else {
		await page.goBack();
	}

	await expect(page).toHaveURL(new RegExp(`tags=${token}.*page=2|page=2.*tags=${token}`));
	await expect(cards(page)).toHaveCount(30);
	await expect.poll(() => page.evaluate(() => window.scrollY)).toBeCloseTo(before, -1);
	await expect(target).toBeInViewport();
});

test('back while the recipe still opens shows the list', async ({ page }) => {
	// The browser takes its picture of the old page before a view transition
	// updates it; under load that took long enough for a back to arrive in
	// between, and the recipe was drawn over the list the back had shown.
	// Holding every update for 300ms keeps that gap open on any machine.
	await page.addInitScript(() => {
		const start = document.startViewTransition?.bind(document);
		if (!start) return;
		document.startViewTransition = ((update: () => unknown) =>
			start(async () => {
				await new Promise((resolve) => setTimeout(resolve, 300));
				await update();
			})) as typeof document.startViewTransition;
	});
	const token = uniqueToken();
	for (let i = 0; i < 3; i++) {
		await createRecipe(page, { ...loadFixture(i), title: `Hasty ${i} ${token}`, tags: [token] });
	}
	await page.goto(`/?tags=${token}`);
	await expect(cards(page)).toHaveCount(3);

	await cards(page)
		.filter({ hasText: `Hasty 1 ${token}` })
		.click();
	await expect(page).toHaveURL(/\/recipes\//);
	await page.goBack();

	await expect(page).toHaveURL(new RegExp(`tags=${token}`));
	// The list is still on screen at once; the recipe came only once the held
	// update ran, so the check waits for the transition to be over.
	await expect(page.locator('html[data-page-transition]')).toHaveCount(0);
	await expect(cards(page)).toHaveCount(3);
	await expect(page.getByRole('heading', { level: 1, name: `Hasty 1 ${token}` })).toHaveCount(0);
});

test('the back button glides the cover home on a touch screen too', async ({ page, isMobile }) => {
	test.skip(!isMobile, 'the back button sits in the phone header; desktop has none');
	// A back on a touch screen gets no transition, as its swipe draws its own
	// slide; the app's back button draws none, so it keeps the transition.
	await page.addInitScript(() => {
		const start = document.startViewTransition?.bind(document);
		if (!start) return;
		const w = window as unknown as { transitions: number };
		w.transitions = 0;
		document.startViewTransition = ((update: () => unknown) => {
			w.transitions++;
			return start(update);
		}) as typeof document.startViewTransition;
	});
	const transitions = () =>
		page.evaluate(() => (window as unknown as { transitions: number }).transitions);
	const settled = () => expect(page.locator('html[data-page-transition]')).toHaveCount(0);
	const token = uniqueToken();
	await createRecipe(page, { ...loadFixture(0), title: `Glide ${token}`, tags: [token] });
	await page.goto(`/?tags=${token}`);
	await expect(cards(page)).toHaveCount(1);

	await cards(page).click();
	await expect(page).toHaveURL(/\/recipes\//);
	await settled();
	await page.getByRole('button', { name: 'Back', exact: true }).click();
	await expect(page).toHaveURL(new RegExp(`tags=${token}`));
	await settled();
	expect(await transitions()).toBe(2);

	await cards(page).click();
	await expect(page).toHaveURL(/\/recipes\//);
	await settled();
	await page.goBack();
	await expect(page).toHaveURL(new RegExp(`tags=${token}`));
	expect(await transitions()).toBe(3);
});

test('a refused save shows catalog copy, never the server’s own words', async ({ page }) => {
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Abgelehnt ${uniqueToken()}`,
		steps: [{ text: 'Saft aufkochen.', references: [], times: [] }]
	});
	// What the API answers when a link no longer matches its step: English
	// for an API client, not for the person at the editor.
	await page.route(`**/api/v1/recipes/${recipe.id}`, (route) =>
		route.request().method() === 'PUT'
			? route.fulfill({
					status: 422,
					contentType: 'application/problem+json',
					json: {
						title: 'Unprocessable Entity',
						status: 422,
						detail: 'validation failed',
						errors: [
							{
								location: 'body.steps[0].references[0].word',
								message: '"Saft" does not occur in the step\'s text'
							}
						]
					}
				})
			: route.fallback()
	);

	await page.goto(`/recipes/${recipe.slug}/edit`);
	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(
		page.getByText('A link, time or photo in this step no longer matches its text')
	).toBeVisible();
	await expect(page.getByText('does not occur')).toHaveCount(0);
	await expect(page.getByText('validation failed')).toHaveCount(0);
});

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
	signOut,
	uniqueToken
} from './helpers';

// Card titles are the only level-3 headings inside `main` - the filter
// panel (a portaled dialog, rendered outside `main`) has a level-3
// heading per section under its "Filters" level-2 title, so scoping to
// `main` is what keeps counting them equivalent to counting the grid once
// that panel can be open at the same time.
const cards = (page: Page) => page.getByRole('main').getByRole('heading', { level: 3 });

test.beforeEach(async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
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
	await expect(page.getByText('Get started and save your first recipe.')).toBeVisible();
	// Scoped to the content area: on desktop the top bar carries the same call
	// to action.
	await expect(page.getByRole('main').getByRole('link', { name: 'New recipe' })).toBeVisible();
});

test('creates a recipe through the editor', async ({ page }) => {
	const token = uniqueToken();
	const title = `Lemon cake ${token}`;

	await openNewRecipe(page);

	// Role locators throughout: every list item in the editor repeats the
	// label of the field inside it ("Ingredient 1", "Step 1") so
	// svelte-dnd-action can announce a drag, which leaves `getByLabel`
	// matching two elements. "Unit" is a combobox rather than a textbox
	// because its input points a `list` at the shared unit `<datalist>`.
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
	await expect(page.getByText(token, { exact: true })).toBeVisible();
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

test('narrows the grid with a tag chip', async ({ page }) => {
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

test('narrows the grid with two tag chips at once', async ({ page }) => {
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

test('unfolds the chip row to reach a tag past the fold', async ({ page }) => {
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
// take. 360px is where too small a reservation shows: the toggle is the last
// thing on the line, so whatever the measurement got wrong is clipped off its
// right edge by the row's `overflow-hidden` - silently, since a clipped
// element still reports itself visible.
test('keeps the tag row toggle inside the row at 360px', async ({ page }) => {
	await page.setViewportSize({ width: 360, height: 780 });
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
						{ name: 'for-a-crowd', count: 1 }
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
	const star = page.getByRole('button', { name: 'Add to favorites' });
	await star.click();
	await expect(page.getByRole('button', { name: 'Remove from favorites' })).toBeVisible();

	await page.reload();
	await expect(page.getByRole('button', { name: 'Remove from favorites' })).toBeVisible();
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
	await page.getByRole('button', { name: 'Add to favorites' }).first().click();
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
	// and no time filter active it must not read as an active filter:
	// no count badge on the trigger (an exact match on "Filters" fails the
	// moment a badge appends a digit to its accessible name), and no reset
	// offered inside the panel.
	await expect(page.getByRole('button', { name: 'Filters', exact: true })).toBeVisible();
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

test('keeps a typed tag when saving without pressing Enter', async ({ page }) => {
	const token = uniqueToken();
	// `beforeEach` already logs in - a second login here logs two HTTP 500s.
	await page.goto('/recipes/new');

	await page.getByRole('textbox', { name: 'Title', exact: true }).fill(`No Enter ${token}`);
	await page.getByRole('textbox', { name: 'Ingredient', exact: true }).first().fill('Flour');
	await page.getByRole('textbox', { name: 'Step 1', exact: true }).fill('Stir.');

	// Typed but NOT committed with Enter - clicking "Save" blurs the field.
	const tags = page.getByRole('combobox', { name: 'Tags', exact: true });
	await tags.fill(token);
	// The list is open on its "new:" row, but nothing is arrow-selected yet,
	// so no option may be announced as active.
	await expect(tags).not.toHaveAttribute('aria-activedescendant');
	await page.getByRole('button', { name: 'Save', exact: true }).click();

	await expect(page.getByRole('heading', { level: 1, name: `No Enter ${token}` })).toBeVisible();
	await expect(page.getByText(token, { exact: true })).toBeVisible();
});

test('keeps a typed tag when editing and saving without pressing Enter', async ({ page }) => {
	// Same mechanism as the test above, on an existing recipe: the tag is
	// typed but never committed with Enter, and "Save" blurs the field.
	const token = uniqueToken();
	const recipe = await createRecipe(page, { ...loadFixture(4), title: `Edited ${token}` });

	await page.goto(`/recipes/${recipe.slug}/edit`);
	await page.getByRole('combobox', { name: 'Tags', exact: true }).fill(token);
	await page.getByRole('button', { name: 'Save', exact: true }).click();

	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	await expect(page.getByText(token, { exact: true })).toBeVisible();
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
	// actually exercises the touch path `onpointerdown` is meant to cover.
	await page.getByRole('option', { name: tag, exact: true }).tap();

	await expect(page.getByText(tag, { exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Remove', exact: true })).toHaveCount(1);
});

test('adds a suggested tag by clicking it', async ({ page }) => {
	const token = uniqueToken();
	const tag = `${token}cinnamon`;
	await createRecipe(page, { ...loadFixture(6), title: `Suggested ${token}`, tags: [tag] });

	await openNewRecipe(page);
	await page.getByRole('combobox', { name: 'Tags', exact: true }).fill(token);
	await page.getByRole('option', { name: tag, exact: true }).click();

	await expect(page.getByText(tag, { exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Remove', exact: true })).toHaveCount(1);
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

	const source = page.locator('#editor-sourceUrl');
	const description = page.locator('#editor-description');
	const card = page.locator('#editor-section-basics');
	await expect(source).toBeVisible();

	// Under the description, not beside the numbers.
	const below = (await source.boundingBox())!.y > (await description.boundingBox())!.y;
	expect(below).toBeTruthy();

	// Full width: the field spans the card's content box, so it never has to
	// share its line with anything.
	const inner = await card.evaluate((el) => {
		const style = getComputedStyle(el);
		const box = el.getBoundingClientRect();
		return box.width - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight);
	});
	expect(Math.round((await source.boundingBox())!.width)).toBe(Math.round(inner));
});

// The detail page's hero is a text column beside the cover. A `1fr` track
// cannot shrink past its own min-content, and the title's longest word at
// 52px sets that floor at ~335px, so a rigid 420px cover beside it demanded
// more width than the card has between 768px and ~858px and hung over the
// edge of the page. That band lies between the Playwright projects' own
// viewports, which is why it survived a green suite - so sweep it here.
// German on purpose: "Königsberger" is the long single word that sets the
// floor, and English titles rarely carry one that long.
test('the recipe page never scrolls sideways', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'the sweep covers the phone widths itself');

	const recipe = await createRecipe(page, {
		...loadFixture(0, 'de'),
		title: 'Königsberger Klopse'
	});
	await page.goto(`/recipes/${recipe.slug}`);
	await expect(page.getByRole('heading', { level: 1 })).toBeVisible();

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
	await expect(page.getByText(`Added by admin on`)).toBeVisible();
	// Nobody has edited it, so the second line would only repeat the first.
	await expect(page.getByText('Last edited by')).toBeHidden();

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
	await expect(page.getByText(`Last edited by ${editor} on`)).toBeVisible();
	// The author line survives the edit - it is not "last touched by".
	await expect(page.getByText('Added by admin on')).toBeVisible();
});

test('the initials on a card name their people without opening the recipe', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, { ...loadFixture(9), title: `Who was it ${token}` });

	await page.goto(`/?q=${token}`);
	const card = cards(page).first();
	await expect(card).toHaveText(`Who was it ${token}`);

	// Tapping the initials is what a phone can do - there is no hover there -
	// and it must resolve the letters rather than follow the card's link.
	await page.getByRole('button', { name: /^Added by admin/ }).click();
	await expect(page.getByText('Added by admin', { exact: true })).toBeVisible();
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
	await expect(page).toHaveURL('/');
	await createRecipe(page, { ...loadFixture(8), title: `By kim ${token}` });

	await page.goto(`/?q=${token}`);
	await expect(cards(page)).toHaveCount(2);

	await page.getByRole('button', { name: 'Filters' }).click();
	await page.getByRole('button', { name: new RegExp(`^${writer}`) }).click();

	await expect(page).toHaveURL(new RegExp(`author=${writer}`));
	await expect(cards(page)).toHaveText([`By kim ${token}`]);
	// The card says whose it is, which is what makes the filter verifiable
	// rather than something you have to trust.
	await expect(page.getByRole('main').getByLabel(`Added by ${writer}`)).toBeVisible();

	// Tapping the same chip again clears the filter rather than re-applying it.
	await page.getByRole('button', { name: new RegExp(`^${writer}`) }).click();
	await expect(page).not.toHaveURL(/author=/);
	await expect(cards(page)).toHaveCount(2);
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
		steps: [{ text: REFERENCE_STEP, references: [] }]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	const step = page.getByRole('textbox', { name: 'Step 1', exact: true });
	await expect(step).toBeVisible();
	// Not `toHaveText`, which collapses whitespace: this has to be the very
	// string that was stored, character for character.
	expect(await step.innerText()).toBe(REFERENCE_STEP);

	// One proposal: "Saft". "Zucker" is not proposed, because it names a row in
	// two groups and only the author can say which one is meant. The banner
	// beside the proposals is what says a save will take them; the save button
	// itself stays a plain "Save".
	const banner = page.getByText('Saving accepts 1 suggestion');
	await expect(banner).toBeVisible();
	await expect(page.getByRole('button', { name: 'Save', exact: true })).toHaveText('Save');

	// The proposal is a dotted underline over the word - a decoration, not
	// anything written into the text.
	const marked = page.locator('[data-ref-word="Saft"]');
	await expect(marked).toHaveClass('ref-suggestion');

	await page.getByRole('button', { name: 'Accept now' }).click();
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
		steps: [{ text: 'Alles verrühren.', references: [] }]
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
	await expect(picker.getByRole('option').first()).toHaveText('Zucker · 100 g · Grütze');
	await expect(picker.getByRole('option').last()).toHaveText('Zucker · 50 g · Vanillesoße');

	// Take the second one, the 50 g from the custard.
	await page.keyboard.press('ArrowDown');
	await page.keyboard.press('Enter');
	// The plain word lands in the text; nothing else does.
	expect(await step.innerText()).toBe('Alles verrühren. Saft dazugeben. Zucker');
	await showLinks(page);
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
		steps: [{ text: 'Zum Schluss Sahne.', references: [] }]
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
	expect(await step.innerText()).toBe('Zum Schluss Zucker Sahne.');

	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	await expect(page.getByText('(100 g)', { exact: true })).toBeVisible();
});

/** Selects `word` in a step field. */
function selectWord(step: Locator, word: string) {
	return selectInStep(step, word, 'word');
}

/**
 * Selects `word` in a step field, or puts the caret in front of it.
 *
 * The selection goes through the editor, not through the DOM selection: the
 * editor reads a DOM selection only on the `selectionchange` event that follows
 * it, and a key pressed before that event is handled at the old selection -
 * where the click landed. Under a loaded full run that window is wide enough to
 * hit. The word's text node is searched for, because a linked word sits in its
 * own decoration span.
 */
async function selectInStep(step: Locator, word: string, what: 'word' | 'caret') {
	await step.click();
	await step.evaluate(
		(field, [wanted, what]) => {
			const walker = document.createTreeWalker(field, NodeFilter.SHOW_TEXT);
			let text: Node | null = walker.nextNode();
			while (text && !text.textContent?.includes(wanted)) text = walker.nextNode();
			const at = text?.textContent?.indexOf(wanted) ?? -1;
			if (!text || at < 0) throw new Error(`"${wanted}" is not in the step`);
			const { editor } = field as HTMLElement & { editor: Editor };
			const from = editor.view.posAtDOM(text, at);
			const to = what === 'word' ? from + wanted.length : from;
			editor.chain().focus().setTextSelection({ from, to }).run();
		},
		[word, what] as const
	);
}

test('shows a link in a card at its word, and folds the list away', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Karte ${token}`,
		steps: [
			{
				text: 'Den Saft aufkochen.',
				references: [{ word: 'Saft', groupName: 'Grütze', ingredientName: 'Saft' }]
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

	// The caret in the word brings up the card: what the word points at, and
	// without the group - "Saft" is in one group only, so it would say nothing.
	await page.locator('[data-ref-word="Saft"]').click();
	const card = page.getByRole('group', { name: 'Link for "Saft"' });
	await expect(card).toContainText('Saft');
	await expect(card).toContainText('400 ml');
	await expect(card).not.toContainText('Grütze');

	// Taking the link off there takes it off for good, and the caret stays in
	// the step.
	await card.getByRole('button', { name: 'Remove' }).click();
	await expect(card).toHaveCount(0);
	await expect(page.locator('[data-ref-word="Saft"]')).toHaveCount(0);
	await expect(summary).toHaveCount(0);
	await expect(page.getByRole('textbox', { name: 'Step 1', exact: true })).toBeFocused();
});

test('takes a proposal from the card at its word', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Vorschlag ${token}`,
		steps: [{ text: 'Den Saft aufkochen.', references: [] }]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	const marked = page.locator('[data-ref-word="Saft"]');
	await expect(marked).toHaveClass('ref-suggestion');
	await expect(page.getByRole('button', { name: '1 suggestion' })).toBeVisible();

	await marked.click();
	const card = page.getByRole('group', { name: 'Link for "Saft"' });
	await card.getByRole('button', { name: 'Accept' }).click();

	// The same card now shows the link it has become.
	await expect(marked).toHaveClass('ref-confirmed');
	await expect(card.getByRole('button', { name: 'Change' })).toBeVisible();
	await expect(page.getByRole('button', { name: '1 linked' })).toBeVisible();
});

test('brings a link back when the edit that removed its word is undone', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Rückgängig ${token}`,
		steps: [
			{
				text: 'Den Saft aufkochen.',
				references: [{ word: 'Saft', groupName: 'Grütze', ingredientName: 'Saft' }]
			}
		]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	const step = page.getByRole('textbox', { name: 'Step 1', exact: true });
	const chip = page.getByRole('button', { name: 'Remove link "Saft · 400 ml · Grütze"' });
	await showLinks(page);
	await expect(chip).toBeVisible();

	// The word goes, and the link is out of sight with it...
	await selectWord(step, 'Saft');
	await page.keyboard.press('Backspace');
	await expect(chip).toHaveCount(0);

	// ...but not gone: the editor's history holds only the text, and the link
	// comes back with the word it belongs to.
	await page.keyboard.press('ControlOrMeta+z');
	expect(await step.innerText()).toBe('Den Saft aufkochen.');
	await showLinks(page);
	await expect(chip).toBeVisible();
	await expect(page.locator('[data-ref-word="Saft"]')).toHaveClass('ref-confirmed');

	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	await expect(page.getByText('(400 ml)', { exact: true })).toBeVisible();
});

test('saves a step whose linked word was deleted, without the link', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, {
		...REFERENCE_RECIPE,
		title: `Ohne Wort ${token}`,
		steps: [
			{
				text: 'Den Saft aufkochen.',
				references: [{ word: 'Saft', groupName: 'Grütze', ingredientName: 'Saft' }]
			}
		]
	});

	await page.goto(`/recipes/${recipe.slug}/edit`);
	const step = page.getByRole('textbox', { name: 'Step 1', exact: true });
	await selectWord(step, 'Saft');
	await page.keyboard.type('Sud');

	// The link stayed on the step while the word was gone; the save leaves it
	// out instead of sending a word the API cannot find.
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
				references: [{ word: 'Saft', groupName: 'Grütze', ingredientName: 'Saft' }]
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
				references: [{ word: 'Saft', groupName: null, ingredientName: 'Saft' }]
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
		steps: [{ text: 'Langsam aufkochen: die Flüssigkeit.', references: [] }]
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
	expect(await step.innerText()).toBe('Langsam aufkochen: die Flüssigkeit.');

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
				references: [{ word: 'Saft', groupName: 'Grütze', ingredientName: 'Saft' }]
			},
			{
				text: 'Zucker einrühren.',
				references: [{ word: 'Zucker', groupName: 'Vanillesoße', ingredientName: 'Zucker' }]
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

	// The handle arms the drag and the item starts it, which is the order
	// svelte-dnd-action's handle zones take a keyboard drag in.
	await ingredients.getByRole('button', { name: 'Move ingredient' }).first().focus();
	await page.keyboard.press('Enter');
	await ingredients.getByRole('listitem').first().focus();
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
	await ingredients.getByRole('listitem').first().focus();
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
		steps: fixture.steps.slice(0, 1).map((step) => ({ ...step, references: [] })),
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
	// The reader scrolling again hands the head back to the scroll position.
	await page.mouse.wheel(0, -400);
	await expect(page.getByRole('button', { name: 'Ingredients, open contents' })).toBeVisible();
});

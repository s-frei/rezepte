import { expect, test, type Page } from '@playwright/test';
import {
	createRecipe,
	loadFixture,
	login,
	openNewRecipe,
	openRecipeMenu,
	search,
	uniqueToken
} from './helpers';

// Card titles are the only level-3 headings inside `main` - the filter
// panel (a portaled dialog, rendered outside `main`) has a level-3
// heading per section under its "Filter" level-2 title, so scoping to
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

	await expect(page.getByRole('heading', { name: 'Noch keine Rezepte' })).toBeVisible();
	await expect(page.getByText('Leg los und speichere dein erstes Rezept.')).toBeVisible();
	// Scoped to the content area: on desktop the top bar carries the same call
	// to action.
	await expect(page.getByRole('main').getByRole('link', { name: 'Neues Rezept' })).toBeVisible();
});

test('creates a recipe through the editor', async ({ page }) => {
	const token = uniqueToken();
	const title = `Zitronenkuchen ${token}`;

	await openNewRecipe(page);

	// Role locators throughout: every list item in the editor repeats the
	// label of the field inside it ("Zutat 1", "Schritt 1") so
	// svelte-dnd-action can announce a drag, which leaves `getByLabel`
	// matching two elements. "Einheit" is a combobox rather than a textbox
	// because its input points a `list` at the shared unit `<datalist>`.
	await page.getByRole('textbox', { name: 'Titel', exact: true }).fill(title);
	await page.getByRole('textbox', { name: 'Menge', exact: true }).fill('2');
	await page.getByRole('combobox', { name: 'Einheit', exact: true }).fill('Stück');
	await page.getByRole('textbox', { name: 'Zutat', exact: true }).fill('Zitrone');
	await page
		.getByRole('textbox', { name: 'Schritt 1', exact: true })
		.fill('Zitronen auspressen und den Saft unterrühren.');

	const tags = page.getByRole('combobox', { name: 'Tags', exact: true });
	await tags.fill(token);
	await tags.press('Enter');
	// Taking the tag clears the field again.
	await expect(tags).toHaveValue('');

	await page.getByRole('button', { name: 'Speichern', exact: true }).click();

	// Landed on the new recipe's detail page.
	await expect(page).toHaveURL(/\/recipes\/zitronenkuchen-/);
	await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
	await expect(page.getByRole('checkbox', { name: 'Zitrone', exact: true })).toBeVisible();
	await expect(page.getByText('Zitronen auspressen und den Saft unterrühren.')).toBeVisible();

	// ... and the overview lists it, tag and all.
	await page.goto('/');
	await search(page, token);
	await expect(cards(page)).toHaveText([title]);
	await expect(page.getByText(token, { exact: true })).toBeVisible();
});

test('searches recipes and reaches the no-results state', async ({ page }) => {
	const token = uniqueToken();
	await createRecipe(page, { ...loadFixture(0), title: `Königsberger Klopse ${token}` });
	await createRecipe(page, { ...loadFixture(1), title: `Rinderrouladen ${token}` });

	await page.goto('/');

	// Both terms have to match (the API ANDs them), so the token keeps the
	// result to this test's own recipes while `Klop` does the actual work.
	await search(page, token);
	await expect(cards(page)).toHaveCount(2);

	await search(page, `Klop ${token}`);
	await expect(cards(page)).toHaveText([`Königsberger Klopse ${token}`]);

	await search(page, 'zzz');
	await expect(page.getByRole('heading', { name: 'Nichts gefunden' })).toBeVisible();
	await expect(page.getByText('Keine Rezepte für „zzz“ gefunden.')).toBeVisible();
});

test('carries the search term into a new recipe', async ({ page }) => {
	const token = uniqueToken();

	await page.goto('/');
	await search(page, token);
	await expect(page.getByRole('heading', { name: 'Nichts gefunden' })).toBeVisible();

	await page.getByRole('link', { name: 'Als neues Rezept' }).click();

	// The hand-over is a route param the editor reads into its starting
	// values, so the term is in the field before anything is saved - and it
	// counts as the pristine state, which is why leaving again asks nothing.
	await expect(page.getByRole('textbox', { name: 'Titel', exact: true })).toHaveValue(token);
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

	await expect(page.getByRole('heading', { name: 'Nichts gefunden' })).toBeVisible();
	await expect(page.getByText('Keine Rezepte passen zu den gesetzten Filtern.')).toBeVisible();
	// The bug this replaces: with nothing typed, the quoted sentence closed
	// around an empty pair of quotation marks.
	await expect(page.getByText('„“')).toHaveCount(0);

	// Nothing to carry over, so the same button is the plain way on - it does
	// not vanish, because an empty result is still a reason to write one.
	await expect(page.getByRole('link', { name: 'Als neues Rezept' })).toHaveCount(0);
	await page.getByRole('main').getByRole('link', { name: 'Neues Rezept' }).click();
	await expect(page.getByRole('textbox', { name: 'Titel', exact: true })).toHaveValue('');
});

test('narrows the grid with a tag chip', async ({ page }) => {
	const token = uniqueToken();
	const tag = `${token}a`;
	await createRecipe(page, {
		...loadFixture(2),
		title: `Mit Tag ${token}`,
		tags: [tag]
	});
	await createRecipe(page, {
		...loadFixture(3),
		title: `Ohne Tag ${token}`,
		tags: [`${token}b`]
	});

	// The chip row only fits as many tags as its width allows before
	// overflowing into "+N weitere", ordered by how many recipes carry each
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
	await expect(cards(page)).toHaveText([`Mit Tag ${token}`]);
});

test('narrows the grid with two tag chips at once', async ({ page }) => {
	const token = uniqueToken();
	const a = `${token}a`;
	const b = `${token}b`;

	await createRecipe(page, { ...loadFixture(1), title: `Beide ${token}`, tags: [a, b] });
	await createRecipe(page, { ...loadFixture(2), title: `Nur A ${token}`, tags: [a] });

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

	// Newest first: "Nur A" was created after "Beide", so it sorts first
	// until the second chip narrows the grid down to "Beide" alone.
	await page.goto(`/?tags=${a}`);
	await expect(cards(page)).toHaveText([`Nur A ${token}`, `Beide ${token}`]);

	await page.getByRole('button', { name: b }).click();
	await expect(page).toHaveURL(new RegExp(`tags=${a}%2C${b}|tags=${a},${b}`));
	await expect(cards(page)).toHaveText([`Beide ${token}`]);
});

test('unfolds the chip row to reach a tag past the fold', async ({ page }) => {
	const token = uniqueToken();
	const tag = `${token}z`;
	await createRecipe(page, { ...loadFixture(1), title: `Hinter der Falte ${token}`, tags: [tag] });

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
							name: `platzhalter-tag-${i}`,
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

	const toggle = page.getByRole('button', { name: /^\+\d+ weitere$/ });
	await toggle.click();
	await expect(toggle).toHaveCount(0);

	await chip.click();
	await expect(page).toHaveURL(new RegExp(`tags=${tag}`));
	await expect(cards(page)).toHaveText([`Hinter der Falte ${token}`]);

	// Folding back takes the row with it - except for the chip just picked.
	// A filter that is on stays on screen wherever it sits in the list, or
	// the row would go silent about the very thing narrowing the grid.
	await page.getByRole('button', { name: 'Weniger' }).click();
	await expect(page.getByRole('button', { name: /^\+\d+ weitere$/ })).toBeVisible();
	await expect(chip).toBeVisible();
	await expect(chip).toHaveAttribute('aria-pressed', 'true');
	// The decoys it was hiding behind are gone again, so this is the folded
	// row keeping one chip, not the unfolded row under another name.
	await expect(page.getByRole('button', { name: 'platzhalter-tag-11' })).toHaveCount(0);
});

test('filters by maximum time from the panel', async ({ page }) => {
	const token = uniqueToken();
	await createRecipe(page, {
		...loadFixture(1),
		title: `Schnell ${token}`,
		prepMinutes: 10,
		cookMinutes: 5
	});
	await createRecipe(page, {
		...loadFixture(2),
		title: `Langsam ${token}`,
		prepMinutes: 60,
		cookMinutes: 120
	});

	await page.goto(`/?q=${token}`);
	await expect(cards(page)).toHaveCount(2);

	await page.getByRole('button', { name: 'Filter' }).click();

	// The scale is a list of stops, and the slider commits on every key
	// press, so Home is one step to the tightest of them: 15 minutes, which
	// is exactly the quick recipe's total and well under the slow one's.
	// Driving it by keyboard rather than by a synthesised drag is also the
	// path a reader without a pointer takes, so the test covers that too.
	const slider = page.getByRole('slider', { name: 'Maximale Zeit' });
	await slider.press('Home');

	await expect(slider).toHaveAttribute('aria-valuetext', 'bis 15 Min');
	await expect(page).toHaveURL(/maxMinutes=15/);
	await expect(cards(page)).toHaveText([`Schnell ${token}`]);

	// The far stop is no bound at all, so the filter leaves the URL entirely
	// rather than naming its loosest value there.
	await slider.press('End');
	await expect(slider).toHaveAttribute('aria-valuetext', 'Beliebig');
	await expect(page).not.toHaveURL(/maxMinutes/);
	await expect(cards(page)).toHaveCount(2);
});

// Bits UI positions the thumb and the ticks with an inline `left` and an
// inline `translate` and nothing else - no `top` - so their vertical place
// is whatever static position they land on, and a Tailwind `-translate-y-1/2`
// cannot correct it: in Tailwind v4 that utility writes the same `translate`
// property the library already set inline, and loses to it. Both ended up
// below the track, the thumb by most of its own height. Only properties the
// library leaves alone (`top`, `margin`) can centre them, and only a
// measurement can tell whether they are centred.
test('centres the time slider thumb and stops on its track', async ({ page }) => {
	await page.goto('/');
	await page.getByRole('button', { name: 'Filter' }).click();

	await expect(page.getByRole('slider', { name: 'Maximale Zeit' })).toBeVisible();

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
						{ name: 'fleisch', count: 5 },
						{ name: 'klassiker', count: 4 },
						{ name: 'vegetarisch', count: 3 },
						{ name: 'schwäbisch', count: 2 },
						{ name: 'für-viele', count: 1 }
					]
				}
			})
	);

	await page.goto('/');
	const toggle = page.getByRole('button', { name: /^\+\d+ weitere$/ });
	await expect(toggle).toBeVisible();

	await expect
		.poll(async () =>
			page.evaluate(() => {
				const button = [...document.querySelectorAll('button')].find((candidate) =>
					/^\+\d+ weitere$/.test(candidate.textContent?.trim() ?? '')
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
	await createRecipe(page, { ...loadFixture(1), title: `Stern ${token}` });

	await page.goto(`/?q=${token}`);
	const star = page.getByRole('button', { name: 'Zu Favoriten hinzufügen' });
	await star.click();
	await expect(page.getByRole('button', { name: 'Aus Favoriten entfernen' })).toBeVisible();

	await page.reload();
	await expect(page.getByRole('button', { name: 'Aus Favoriten entfernen' })).toBeVisible();
});

test('filters the overview down to favourites', async ({ page }) => {
	const token = uniqueToken();
	await createRecipe(page, { ...loadFixture(1), title: `Mit Stern ${token}` });
	await createRecipe(page, { ...loadFixture(2), title: `Ohne Stern ${token}` });

	// Sorted by title (M before O) rather than relying on the default
	// "most recently changed first" order, which is nondeterministic here:
	// both recipes are created within the same request burst, so which one
	// counts as "most recent" isn't guaranteed - and `.first()` below needs
	// to land on "Mit Stern" specifically.
	await page.goto(`/?q=${token}&sort=title`);
	await page.getByRole('button', { name: 'Zu Favoriten hinzufügen' }).first().click();
	await page.getByRole('button', { name: 'Filter' }).click();
	// `role="switch"`, not checkbox: the control is a Switch, and Playwright's
	// `check()` only drives a real checkbox or radio - clicking it and reading
	// `aria-checked` back is what tells a switch was actually flipped.
	const favourites = page.getByRole('switch', { name: 'Nur Favoriten' });
	await favourites.click();
	await expect(favourites).toHaveAttribute('aria-checked', 'true');

	await expect(cards(page)).toHaveText([`Mit Stern ${token}`]);
});

test('sorts the grid alphabetically', async ({ page, isMobile }) => {
	const token = uniqueToken();
	await createRecipe(page, { ...loadFixture(1), title: `Zuletzt ${token}` });
	await createRecipe(page, { ...loadFixture(2), title: `Anfang ${token}` });

	await page.goto(`/?q=${token}`);

	// Select.svelte wraps Bits UI's Select, not a native <select>: this opens
	// the trigger and clicks the option instead of using selectOption(). On
	// phones the control lives inside the filter panel instead of row 1 (see
	// FilterPanel.svelte), so the panel has to be open first. `getByRole`
	// rather than `getByLabel`: the sort section's own aria-labelledby gives
	// it the same accessible name, so getByLabel('Sortierung') matches both
	// it and the trigger button inside it.
	if (isMobile) {
		await page.getByRole('button', { name: 'Filter' }).click();
	}
	await page.getByRole('button', { name: 'Sortierung' }).click();
	await page.getByRole('option', { name: 'A–Z' }).click();

	await expect(cards(page)).toHaveText([`Anfang ${token}`, `Zuletzt ${token}`]);
});

test('sorting alone does not count as an active filter', async ({ page, isMobile }) => {
	const token = uniqueToken();
	await createRecipe(page, { ...loadFixture(1), title: `Zuletzt ${token}` });
	await createRecipe(page, { ...loadFixture(2), title: `Anfang ${token}` });

	await page.goto(`/?q=${token}`);
	if (isMobile) {
		await page.getByRole('button', { name: 'Filter' }).click();
	}
	await page.getByRole('button', { name: 'Sortierung' }).click();
	await page.getByRole('option', { name: 'A–Z' }).click();

	// Sorting reorders the grid rather than narrowing it, so with no tags
	// and no time filter active it must not read as an active filter:
	// no count badge on the trigger (an exact match on "Filter" fails the
	// moment a badge appends a digit to its accessible name), and no reset
	// offered inside the panel.
	await expect(page.getByRole('button', { name: 'Filter', exact: true })).toBeVisible();
	if (!isMobile) {
		await page.getByRole('button', { name: 'Filter', exact: true }).click();
	}
	await expect(page.getByRole('button', { name: 'Alle Filter zurücksetzen' })).toHaveCount(0);
});

test('edits a recipe from the detail page', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, { ...loadFixture(4), title: `Vorher ${token}` });
	const renamed = `Nachher ${token}`;

	await page.goto(`/recipes/${recipe.slug}`);
	await openRecipeMenu(page);
	await page.getByRole('menuitem', { name: 'Bearbeiten' }).click();

	await expect(page).toHaveURL(`/recipes/${recipe.slug}/edit`);
	await page.getByRole('textbox', { name: 'Titel', exact: true }).fill(renamed);
	await page.getByRole('button', { name: 'Speichern', exact: true }).click();

	// The slug is a permalink, so only the heading changes.
	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	await expect(page.getByRole('heading', { level: 1, name: renamed })).toBeVisible();
});

test('deletes a recipe through the menu and the confirmation', async ({ page }) => {
	const token = uniqueToken();
	const recipe = await createRecipe(page, { ...loadFixture(5), title: `Weg damit ${token}` });

	await page.goto(`/recipes/${recipe.slug}`);
	await openRecipeMenu(page);
	await page.getByRole('menuitem', { name: 'Löschen' }).click();

	const dialog = page.getByRole('dialog');
	await expect(
		dialog.getByText(`Möchtest du „Weg damit ${token}“ wirklich löschen?`)
	).toBeVisible();
	await dialog.getByRole('button', { name: 'Löschen' }).click();

	await expect(page).toHaveURL('/');
	await search(page, token);
	await expect(page.getByRole('heading', { name: 'Nichts gefunden' })).toBeVisible();
});

test('guards a dirty editor against navigating away', async ({ page }) => {
	await openNewRecipe(page);
	const title = page.getByRole('textbox', { name: 'Titel', exact: true });
	await title.fill(`Halbfertig ${uniqueToken()}`);

	await page.getByRole('button', { name: 'Abbrechen' }).click();

	const dialog = page.getByRole('dialog');
	await expect(dialog.getByText('Änderungen verwerfen?')).toBeVisible();

	// Dismissing the dialog keeps the draft.
	await dialog.getByRole('button', { name: 'Abbrechen' }).click();
	await expect(dialog).toBeHidden();
	await expect(page).toHaveURL(/\/recipes\/new$/);
	await expect(title).not.toHaveValue('');

	await page.getByRole('button', { name: 'Abbrechen' }).click();
	await dialog.getByRole('button', { name: 'Verwerfen' }).click();
	await expect(page).toHaveURL('/');
});

test('keeps a typed tag when saving without pressing Enter', async ({ page }) => {
	const token = uniqueToken();
	// `beforeEach` already logs in - a second login here logs two HTTP 500s.
	await page.goto('/recipes/new');

	await page.getByRole('textbox', { name: 'Titel', exact: true }).fill(`Ohne Enter ${token}`);
	await page.getByRole('textbox', { name: 'Zutat', exact: true }).first().fill('Mehl');
	await page.getByRole('textbox', { name: 'Schritt 1', exact: true }).fill('Verrühren.');

	// Typed but NOT committed with Enter - clicking "Speichern" blurs the field.
	const tags = page.getByRole('combobox', { name: 'Tags', exact: true });
	await tags.fill(token);
	// The list is open on its "neu:" row, but nothing is arrow-selected yet,
	// so no option may be announced as active.
	await expect(tags).not.toHaveAttribute('aria-activedescendant');
	await page.getByRole('button', { name: 'Speichern', exact: true }).click();

	await expect(page.getByRole('heading', { level: 1, name: `Ohne Enter ${token}` })).toBeVisible();
	await expect(page.getByText(token, { exact: true })).toBeVisible();
});

test('keeps a typed tag when editing and saving without pressing Enter', async ({ page }) => {
	// Same mechanism as the test above, on an existing recipe: the tag is
	// typed but never committed with Enter, and "Speichern" blurs the field.
	const token = uniqueToken();
	const recipe = await createRecipe(page, { ...loadFixture(4), title: `Bearbeitet ${token}` });

	await page.goto(`/recipes/${recipe.slug}/edit`);
	await page.getByRole('combobox', { name: 'Tags', exact: true }).fill(token);
	await page.getByRole('button', { name: 'Speichern', exact: true }).click();

	await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
	await expect(page.getByText(token, { exact: true })).toBeVisible();
});

test('adds a suggested tag by tapping it on touch', async ({ page, isMobile }) => {
	test.skip(!isMobile, 'tap() needs hasTouch, which only the mobile project enables');
	const token = uniqueToken();
	const tag = `${token}zimt`;
	// Seeding a recipe with this tag first is what makes it a real suggestion:
	// the editor's tag field only offers what `listTags()` already knows.
	await createRecipe(page, { ...loadFixture(6), title: `Mit Vorschlag ${token}`, tags: [tag] });

	await openNewRecipe(page);
	await page.getByRole('combobox', { name: 'Tags', exact: true }).fill(token);
	// Playwright's click() also dispatches pointer events, so only tap()
	// actually exercises the touch path `onpointerdown` is meant to cover.
	await page.getByRole('option', { name: tag, exact: true }).tap();

	await expect(page.getByText(tag, { exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Entfernen', exact: true })).toHaveCount(1);
});

test('adds a suggested tag by clicking it', async ({ page }) => {
	const token = uniqueToken();
	const tag = `${token}zimt`;
	await createRecipe(page, { ...loadFixture(6), title: `Mit Vorschlag ${token}`, tags: [tag] });

	await openNewRecipe(page);
	await page.getByRole('combobox', { name: 'Tags', exact: true }).fill(token);
	await page.getByRole('option', { name: tag, exact: true }).click();

	await expect(page.getByText(tag, { exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Entfernen', exact: true })).toHaveCount(1);
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

	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Leiste ${uniqueToken()}` });
	await page.goto(`/recipes/${recipe.slug}/edit`);

	const save = page.getByRole('button', { name: 'Speichern', exact: true });
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
	const recipe = await createRecipe(page, { ...loadFixture(0), title: `Quelle ${uniqueToken()}` });
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
test('the recipe page never scrolls sideways', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'the sweep covers the phone widths itself');

	const recipe = await createRecipe(page, { ...loadFixture(0), title: 'Königsberger Klopse' });
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

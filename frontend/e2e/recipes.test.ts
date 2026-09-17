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

// Card titles are the only level-3 headings on the overview, so counting
// them counts the grid.
const cards = (page: Page) => page.getByRole('heading', { level: 3 });

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

	await page.goto('/');
	await search(page, token);
	await expect(cards(page)).toHaveCount(2);

	const chip = page.getByRole('button', { name: tag });
	await chip.click();

	await expect(chip).toHaveAttribute('aria-pressed', 'true');
	await expect(cards(page)).toHaveText([`Mit Tag ${token}`]);
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

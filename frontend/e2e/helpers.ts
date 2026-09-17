import { readFileSync } from 'node:fs';
import { expect, type Page } from '@playwright/test';
// Type-only, so the relative hop into `src` is erased at runtime and the e2e
// suite still speaks the same contract as the app.
import type { Recipe, RecipeInput } from '../src/lib/api/recipes';

/** Logs in as the seeded admin (password from the e2e task). */
export async function login(page: Page, username = 'admin', password = 'e2e-password') {
	await page.goto('/login');
	await page.getByLabel('Benutzername').fill(username);
	await page.getByLabel('Passwort').fill(password);
	await page.getByRole('button', { name: 'Anmelden' }).click();
}

/**
 * A token unique to one test, used to keep recipes apart: the whole suite
 * runs against a single binary with a single database, and the desktop and
 * mobile projects run at the same time, so every test titles and tags its
 * own recipes with one of these instead of assuming it is alone.
 *
 * Starts with `e2e` so it never collides with the nonsense terms tests search
 * for to reach the no-results state.
 */
export function uniqueToken(): string {
	return `e2e${Math.random().toString(36).slice(2, 8)}${Date.now().toString(36).slice(-4)}`;
}

let fixtures: RecipeInput[] | null = null;

/**
 * Returns a copy of one of the German recipes the service tests are seeded
 * with (`service/internal/recipe/testdata/recipes.json`), so the e2e suite
 * exercises the same realistic data - umlauts, ingredient groups and all.
 */
export function loadFixture(index: number): RecipeInput {
	if (fixtures === null) {
		const file = new URL('../../service/internal/recipe/testdata/recipes.json', import.meta.url);
		fixtures = JSON.parse(readFileSync(file, 'utf8')) as RecipeInput[];
	}
	const fixture = fixtures[index];
	if (!fixture) {
		throw new Error(`no recipe fixture at index ${index}`);
	}
	return structuredClone(fixture);
}

/**
 * Creates a recipe straight through the API, reusing the session cookie the
 * page logged in with (`page.request` shares the browser context's cookie
 * jar). Sets `Origin` the way the browser would, since the server rejects
 * mutating API calls whose origin doesn't match its host.
 *
 * Use it to arrange state for a test; the editor flow itself is covered
 * through the UI.
 */
export async function createRecipe(page: Page, input: RecipeInput): Promise<Recipe> {
	const origin = new URL(page.url()).origin;
	const response = await page.request.post('/api/v1/recipes', {
		headers: { Origin: origin, 'Content-Type': 'application/json' },
		data: input
	});
	expect(
		response.ok(),
		`POST /api/v1/recipes failed: ${response.status()} ${await response.text()}`
	).toBeTruthy();
	return (await response.json()) as Recipe;
}

/** Opens the recipe editor through whichever "new recipe" entry point the viewport shows. */
export async function openNewRecipe(page: Page) {
	const topBar = page.getByRole('link', { name: 'Neues Rezept' });
	const bottomNav = page.getByRole('link', { name: 'Neu', exact: true });
	await topBar.or(bottomNav).first().click();
	await expect(page).toHaveURL(/\/recipes\/new$/);
}

/** Types into the overview's search field and waits out its 250ms debounce. */
export async function search(page: Page, term: string) {
	const field = page.getByRole('textbox', { name: 'Rezepte durchsuchen' });
	await field.fill(term);
	await expect(field).toHaveValue(term);
}

/** Opens the detail page's "..." menu - the only entry point both viewports share. */
export async function openRecipeMenu(page: Page) {
	await page.getByRole('button', { name: 'Weitere Aktionen' }).click();
	await expect(page.getByRole('menu')).toBeVisible();
}

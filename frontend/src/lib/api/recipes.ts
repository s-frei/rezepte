import { api } from './client';

// Contract types (binding for both the backend and the frontend, see
// docs/superpowers/plans/2026-09-17-phase-3-recipe-core.md, "API contract").

export type RecipeCard = {
	id: string;
	slug: string;
	title: string;
	tags: string[];
	totalMinutes: number | null;
	coverImageId: string | null;
	updatedAt: string;
};

export type Ingredient = {
	quantity: number | null;
	unit: string | null;
	name: string;
	note: string | null;
};

export type IngredientGroup = {
	name: string | null;
	ingredients: Ingredient[];
};

export type RecipeInput = {
	title: string;
	description: string;
	servings: number;
	prepMinutes: number | null;
	cookMinutes: number | null;
	sourceUrl: string | null;
	tags: string[];
	ingredientGroups: IngredientGroup[];
	steps: string[];
};

export type Recipe = RecipeInput & {
	id: string;
	slug: string;
	coverImageId: string | null;
	images: [];
	createdBy: string;
	createdAt: string;
	updatedAt: string;
};

export type RecipePage = {
	items: RecipeCard[];
	page: number;
	limit: number;
	total: number;
};

export type Tag = { name: string; count: number };

/** Unit suggestions offered in the ingredient row's unit combobox. */
export const UNIT_SUGGESTIONS = ['g', 'kg', 'ml', 'l', 'EL', 'TL', 'Stück', 'Prise'];

/**
 * Lists recipe cards, optionally filtered by search term and/or tag, paginated.
 *
 * `init` is forwarded to the underlying `fetch` call (e.g. `{ signal }` to
 * cancel a stale request when the overview page's filters change again
 * before this one resolves).
 */
export function listRecipes(
	params: { q?: string; tag?: string; page?: number; limit?: number } = {},
	init: RequestInit = {}
): Promise<RecipePage> {
	const query = new URLSearchParams();
	if (params.q) {
		query.set('q', params.q);
	}
	if (params.tag) {
		query.set('tag', params.tag);
	}
	if (params.page !== undefined) {
		query.set('page', String(params.page));
	}
	if (params.limit !== undefined) {
		query.set('limit', String(params.limit));
	}
	const qs = query.toString();
	return api<RecipePage>(`/recipes${qs ? `?${qs}` : ''}`, init);
}

/** Fetches a single recipe by its human-readable slug. */
export function getRecipeBySlug(slug: string): Promise<Recipe> {
	return api<Recipe>(`/recipes/by-slug/${encodeURIComponent(slug)}`);
}

/** Fetches a single recipe by id. */
export function getRecipe(id: string): Promise<Recipe> {
	return api<Recipe>(`/recipes/${encodeURIComponent(id)}`);
}

/** Creates a new recipe. */
export function createRecipe(input: RecipeInput): Promise<Recipe> {
	return api<Recipe>('/recipes', { method: 'POST', body: JSON.stringify(input) });
}

/** Replaces a recipe's content. */
export function updateRecipe(id: string, input: RecipeInput): Promise<Recipe> {
	return api<Recipe>(`/recipes/${encodeURIComponent(id)}`, {
		method: 'PUT',
		body: JSON.stringify(input)
	});
}

/** Deletes a recipe. */
export function deleteRecipe(id: string): Promise<void> {
	return api<void>(`/recipes/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

/** Lists all tags currently in use, with how many recipes carry each. */
export async function listTags(): Promise<Tag[]> {
	const page = await api<{ items: Tag[] }>('/tags');
	return page.items;
}

/** A blank `RecipeInput` for the "new recipe" form: one unnamed ingredient group with one empty row, 4 servings, one empty step. */
export function emptyInput(): RecipeInput {
	return {
		title: '',
		description: '',
		servings: 4,
		prepMinutes: null,
		cookMinutes: null,
		sourceUrl: null,
		tags: [],
		ingredientGroups: [
			{ name: null, ingredients: [{ quantity: null, unit: null, name: '', note: null }] }
		],
		steps: ['']
	};
}

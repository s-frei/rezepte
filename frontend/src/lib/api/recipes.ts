import type { UserColor } from '$lib/user/color';
import { api } from './client';

// Contract types (binding for both the backend and the frontend, see
// docs/superpowers/plans/2026-09-17-phase-3-recipe-core.md, "API contract").

/**
 * Somebody a recipe names - who wrote it, who last changed it. The service
 * sends the whole of it because user management is admin-only, so a member
 * could not resolve the id themselves.
 */
export type Person = {
	id: string;
	/** The login name, which `?author=` filters by. */
	username: string;
	displayName: string;
	/** The palette token of the person, for the author circle. */
	color: UserColor;
};

export type RecipeCard = {
	id: string;
	slug: string;
	title: string;
	tags: string[];
	totalMinutes: number | null;
	coverImageId: string | null;
	updatedAt: string;
	/** Whether the signed-in user has starred this recipe. */
	favorite: boolean;
	/** Who wrote the recipe and who last changed it, shown as initials. */
	createdBy: Person;
	updatedBy: Person;
};

export type Image = { id: string; width: number; height: number; position: number };

export type ImageVariant = 'thumb' | 'detail' | 'original';

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

/**
 * One word of a step's text, tied to an ingredient of the same recipe.
 * `word` and `ingredient` differ when the sentence says "Fleisch" and the list
 * says "Rinderbraten"; `group` breaks the tie between two entries of one name
 * and is null for the unnamed group.
 */
export type IngredientRef = { word: string; groupName: string | null; ingredientName: string };

export type Step = { text: string; references: IngredientRef[] };

/** Who besides the author and admins may edit; `default` follows the household setting. */
export type EditPolicy = 'default' | 'open' | 'locked';

export type RecipeInput = {
	title: string;
	description: string;
	servings: number;
	prepMinutes: number | null;
	cookMinutes: number | null;
	sourceUrl: string | null;
	tags: string[];
	ingredientGroups: IngredientGroup[];
	steps: Step[];
	/** Optional: an update without it keeps the stored policy. */
	editPolicy?: EditPolicy;
};

export type Recipe = RecipeInput & {
	id: string;
	slug: string;
	coverImageId: string | null;
	images: Image[];
	createdAt: string;
	createdBy: Person;
	updatedAt: string;
	updatedBy: Person;
	/** Whether the signed-in user has starred this recipe. */
	favorite: boolean;
	/** The effective state: the policy resolved against the household setting. */
	locked: boolean;
	/** What the signed-in user may do with this recipe, decided by the server. */
	canEdit: boolean;
	canDelete: boolean;
	canChangePolicy: boolean;
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

/** Most images the API accepts per recipe. */
export const MAX_IMAGES = 20;

/** Upload types the API decodes; also the file input's `accept` list. */
export const ACCEPTED_IMAGE_TYPES = ['image/jpeg', 'image/png', 'image/webp'];

/**
 * Largest upload the API accepts (10 MiB, `maxUploadBytes` in the service).
 * Checked before the request so an oversized pick is refused right away
 * instead of after a pointless upload.
 */
export const MAX_IMAGE_BYTES = 10 * 1024 * 1024;

/**
 * Lists recipe cards, optionally filtered by search term and/or a
 * combination of tags, paginated.
 *
 * `init` is forwarded to the underlying `fetch` call (e.g. `{ signal }` to
 * cancel a stale request when the overview page's filters change again
 * before this one resolves).
 */
export function listRecipes(
	params: {
		q?: string;
		tags?: string[];
		maxMinutes?: number;
		favorites?: boolean;
		author?: string;
		sort?: 'updated' | 'created' | 'title';
		page?: number;
		limit?: number;
	} = {},
	init: RequestInit = {}
): Promise<RecipePage> {
	const query = new URLSearchParams();
	if (params.q) {
		query.set('q', params.q);
	}
	if (params.tags && params.tags.length > 0) {
		query.set('tags', params.tags.join(','));
	}
	if (params.maxMinutes !== undefined && params.maxMinutes > 0) {
		query.set('maxMinutes', String(params.maxMinutes));
	}
	if (params.favorites) {
		query.set('favorites', 'true');
	}
	if (params.author) {
		query.set('author', params.author);
	}
	if (params.sort !== undefined && params.sort !== 'updated') {
		query.set('sort', params.sort);
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

/** Sets or clears the star on a recipe for the signed-in user. */
export async function setFavorite(id: string, on: boolean): Promise<void> {
	await api<void>(`/recipes/${encodeURIComponent(id)}/favorite`, {
		method: on ? 'PUT' : 'DELETE'
	});
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

/**
 * URL of one rendition of an image. Served outside `/api/v1` by a plain
 * file route (session cookie required) with a one-year immutable cache:
 * image ids are never reused, so the URL can be used as an `<img src>`
 * directly and cached freely.
 */
export function imageUrl(recipeId: string, imageId: string, variant: ImageVariant): string {
	return `/images/${encodeURIComponent(recipeId)}/${encodeURIComponent(imageId)}/${variant}.jpg`;
}

/** Uploads one image; the server appends it and makes it the cover if the recipe had none. */
export function uploadImage(recipeId: string, file: File): Promise<Image> {
	const body = new FormData();
	body.append('file', file, file.name);
	return api<Image>(`/recipes/${encodeURIComponent(recipeId)}/images`, { method: 'POST', body });
}

/** Deletes an image; the server promotes the next image to cover if needed. */
export function deleteImage(recipeId: string, imageId: string): Promise<void> {
	return api<void>(
		`/recipes/${encodeURIComponent(recipeId)}/images/${encodeURIComponent(imageId)}`,
		{ method: 'DELETE' }
	);
}

/** Sets the display order; `imageIds` must list every image exactly once. */
export async function reorderImages(recipeId: string, imageIds: string[]): Promise<Image[]> {
	const page = await api<{ items: Image[] }>(
		`/recipes/${encodeURIComponent(recipeId)}/images/order`,
		{ method: 'PUT', body: JSON.stringify({ imageIds }) }
	);
	return page.items;
}

/** Chooses the cover image. */
export function setCover(recipeId: string, imageId: string): Promise<void> {
	return api<void>(`/recipes/${encodeURIComponent(recipeId)}/cover`, {
		method: 'PUT',
		body: JSON.stringify({ imageId })
	});
}

/** Lists all tags currently in use, with how many recipes carry each. */
export async function listTags(): Promise<Tag[]> {
	const page = await api<{ items: Tag[] }>('/tags');
	return page.items;
}

/** `name` is the username the ?author= filter matches; the rest is what the facet shows. */
export type Author = { username: string; displayName: string; color: UserColor; count: number };

/**
 * Everyone who wrote at least one recipe, most recipes first, for the
 * overview's "Added by" filter. Readable by any signed-in member,
 * unlike the admin-only user management in `$lib/api/users`.
 */
export async function listAuthors(): Promise<Author[]> {
	const page = await api<{ items: Author[] }>('/authors');
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
		steps: [{ text: '', references: [] }],
		editPolicy: 'default'
	};
}

import { api } from './client';

/** An ingredient name in use; `also` lists other names its key covers ("Gurke": "Salatgurke"). */
export type IngredientSuggestion = { name: string; key: string; count: number; also: string[] };

/** Ingredient names in use with a word starting with `q`: first-word matches first, then the most used. */
export async function listIngredients(
	q: string,
	limit?: number,
	init: RequestInit = {}
): Promise<IngredientSuggestion[]> {
	const params = new URLSearchParams({ q });
	if (limit !== undefined) {
		params.set('limit', String(limit));
	}
	const page = await api<{ items: IngredientSuggestion[] }>(`/ingredients?${params}`, init);
	return page.items;
}

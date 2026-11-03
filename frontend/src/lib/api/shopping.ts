import { api } from './client';

export type Department =
	| 'produce'
	| 'bakery'
	| 'chilled'
	| 'meat'
	| 'pantry'
	| 'frozen'
	| 'drinks'
	| 'household'
	| 'other';

export type ShoppingEntry = {
	id: string;
	/** Null once the recipe is deleted; the entry keeps its title. */
	recipeId: string | null;
	recipeSlug: string | null;
	coverImageId: string | null;
	title: string;
	baseServings: number;
	servings: number;
};

export type ShoppingItem = {
	id: string;
	/** Null for a free item. */
	entryId: string | null;
	name: string;
	nameKey: string;
	/** Already scaled and rounded by the server. */
	quantity: number | null;
	unit: string | null;
	note: string | null;
	group: string | null;
	department: Department;
	/** Free items only. */
	addedBy: { id: string; displayName: string; color: string; avatarId: string | null } | null;
	checked: boolean;
};

export type ShoppingList = {
	id: string;
	name: string;
	createdBy: string;
	createdAt: string;
	updatedAt: string;
	openCount: number;
	doneCount: number;
	entries: ShoppingEntry[];
	items: ShoppingItem[];
};

export type ShoppingSummary = {
	id: string;
	name: string;
	openCount: number;
	doneCount: number;
	recipes: { title: string; servings: number }[];
	updatedAt: string;
};

const base = (id: string) => `/shopping-lists/${encodeURIComponent(id)}`;
const send = <T>(path: string, method: string, body?: unknown) =>
	api<T>(path, { method, body: body === undefined ? undefined : JSON.stringify(body) });

export async function listShoppingLists(): Promise<ShoppingSummary[]> {
	return (await api<{ items: ShoppingSummary[] }>('/shopping-lists')).items;
}

export const createShoppingList = (name: string) =>
	send<ShoppingList>('/shopping-lists', 'POST', { name });

export const getShoppingList = (id: string) => api<ShoppingList>(base(id));

export const renameShoppingList = (id: string, name: string) =>
	send<ShoppingList>(base(id), 'PATCH', { name });

export const deleteShoppingList = (id: string) => send<void>(base(id), 'DELETE');

export const addRecipeToList = (
	id: string,
	input: { recipeId: string; servings: number; ingredients: { group: number; index: number }[] }
) => send<ShoppingList>(`${base(id)}/recipes`, 'POST', input);

export const setEntryServings = (id: string, entryId: string, servings: number) =>
	send<ShoppingList>(`${base(id)}/recipes/${encodeURIComponent(entryId)}`, 'PATCH', { servings });

export const removeEntry = (id: string, entryId: string) =>
	send<ShoppingList>(`${base(id)}/recipes/${encodeURIComponent(entryId)}`, 'DELETE');

export const addFreeItem = (id: string, text: string) =>
	send<ShoppingList>(`${base(id)}/items`, 'POST', { text });

export const setItemsChecked = (id: string, itemIds: string[], checked: boolean) =>
	send<ShoppingList>(`${base(id)}/items/check`, 'POST', { itemIds, checked });

export const removeItems = (id: string, itemIds: string[]) =>
	send<ShoppingList>(`${base(id)}/items/remove`, 'POST', { itemIds });

export const clearChecked = (id: string) => send<ShoppingList>(`${base(id)}/clear-checked`, 'POST');

/** Overrides the department of an ingredient name for every list on the instance. */
export const setDepartment = (name: string, department: Department) =>
	send<void>('/shopping-departments', 'PUT', { name, department });

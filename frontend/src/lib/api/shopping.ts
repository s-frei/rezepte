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
	addedBy: ListPerson | null;
	checked: boolean;
	/** Who ticked it; null while open, or once that account is gone. */
	checkedBy: ListPerson | null;
	/** Client only (`ListState.shown`): ticked elsewhere a moment ago, still shown open. */
	lingering?: boolean;
};

export type ListPerson = {
	id: string;
	displayName: string;
	color: string;
	avatarId: string | null;
};

export type ShoppingList = {
	id: string;
	name: string;
	createdBy: string;
	createdAt: string;
	updatedAt: string;
	/** Counts the list's changes; the live stream sends it too. */
	revision: number;
	openCount: number;
	doneCount: number;
	entries: ShoppingEntry[];
	items: ShoppingItem[];
	owner: ListPerson;
	/** Accepted members, without the owner. */
	members: ListPerson[];
	/** Invited and not yet answered; only the creator sees them. */
	pending: ListPerson[];
};

export type ShoppingSummary = {
	id: string;
	name: string;
	openCount: number;
	doneCount: number;
	/** `recipeId` is null once the recipe is deleted. */
	recipes: { title: string; servings: number; recipeId: string | null }[];
	updatedAt: string;
	owner: ListPerson;
	members: ListPerson[];
	updatedBy: ListPerson | null;
};

export type ShoppingInvitation = {
	listId: string;
	listName: string;
	from: ListPerson;
	openCount: number;
	invitedAt: string;
};

const base = (id: string) => `/shopping-lists/${encodeURIComponent(id)}`;
const send = <T>(path: string, method: string, body?: unknown) =>
	api<T>(path, { method, body: body === undefined ? undefined : JSON.stringify(body) });

export const listShoppingLists = () =>
	api<{ items: ShoppingSummary[]; invitations: ShoppingInvitation[] }>('/shopping-lists');

export const createShoppingList = (name: string) =>
	send<ShoppingList>('/shopping-lists', 'POST', { name });

/** A new list from pasted text; `name` wins over the text's name line, `defaultName` is the fallback. */
export const importShoppingList = (text: string, name: string | undefined, defaultName: string) =>
	send<ShoppingList>('/shopping-lists/import', 'POST', { text, name, defaultName });

/** Appends the pasted text's items; the list keeps its name. */
export const importIntoShoppingList = (id: string, text: string) =>
	send<ShoppingList>(`${base(id)}/import`, 'POST', { text });

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

/** `quantity` and `unit`, when given, replace what the server parses from `text`. */
export const addFreeItem = (
	id: string,
	text: string,
	fields: { quantity?: number; unit?: string } = {}
) => send<ShoppingList>(`${base(id)}/items`, 'POST', { text, ...fields });

export const setItemsChecked = (id: string, itemIds: string[], checked: boolean) =>
	send<ShoppingList>(`${base(id)}/items/check`, 'POST', { itemIds, checked });

export const removeItems = (id: string, itemIds: string[]) =>
	send<ShoppingList>(`${base(id)}/items/remove`, 'POST', { itemIds });

export const clearChecked = (id: string) => send<ShoppingList>(`${base(id)}/clear-checked`, 'POST');

export const inviteToList = (id: string, userId: string) =>
	send<ShoppingList>(`${base(id)}/invitations`, 'POST', { userId });

export const removeListMember = (id: string, userId: string) =>
	send<ShoppingList>(`${base(id)}/members/${encodeURIComponent(userId)}`, 'DELETE');

export const acceptListInvitation = (id: string) =>
	send<ShoppingList>(`${base(id)}/membership`, 'POST');

/** Leaves a list, or declines its invitation. */
export const leaveList = (id: string) => send<void>(`${base(id)}/membership`, 'DELETE');

/** Overrides the department of an ingredient name for every list on the instance. */
export const setDepartment = (name: string, department: Department) =>
	send<void>('/shopping-departments', 'PUT', { name, department });

/** The server-sent events stream of one list: an event per change, see service/internal/shopping. */
export const listEventsUrl = (id: string) => `/api/v1${base(id)}/events`;

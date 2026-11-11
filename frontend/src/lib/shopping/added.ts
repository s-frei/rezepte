import type { ShoppingList } from '#lib/api/shopping.js';

/**
 * The items an import of mine added: in the answer, not in the list before
 * the call, and added by me - a member's item that landed meanwhile is not
 * the import's, so "Undo" must leave it.
 */
export const addedIds = (before: ShoppingList, after: ShoppingList, me: string | undefined) => {
	const had = new Set(before.items.map((i) => i.id));
	return after.items.filter((i) => !had.has(i.id) && i.addedBy?.id === me).map((i) => i.id);
};

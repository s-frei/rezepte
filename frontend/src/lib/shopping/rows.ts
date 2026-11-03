import type { Department, ShoppingEntry, ShoppingItem, ShoppingList } from '#lib/api/shopping.js';
import { formatQuantity } from '#lib/recipe/format.js';
import { DEPARTMENTS } from './departments';

export type Source = {
	itemId: string;
	/** Formatted quantity and unit; '' if none. */
	amount: string;
	/** Title of the entry the item came from; null for a free item. */
	recipe: string | null;
	addedBy: string | null;
	/** Account id behind `addedBy`, so the page can say "by you". */
	addedById: string | null;
	note: string | null;
	/** A free item whose adder's account is gone. */
	formerMember: boolean;
};

export type Row = {
	key: string;
	name: string;
	itemIds: string[];
	sources: Source[];
	/** All items checked. */
	checked: boolean;
};

function rowsOf(list: ShoppingList, items: ShoppingItem[]): Row[] {
	const titles = new Map(list.entries.map((e) => [e.id, e.title]));
	const rows = new Map<string, Row>();
	for (const it of items) {
		let row = rows.get(it.nameKey);
		if (!row) {
			row = { key: it.nameKey, name: it.name, itemIds: [], sources: [], checked: true };
			rows.set(it.nameKey, row);
		}
		row.itemIds.push(it.id);
		row.checked &&= it.checked;
		row.sources.push({
			itemId: it.id,
			amount: [it.quantity === null ? '' : formatQuantity(it.quantity), it.unit ?? '']
				.filter(Boolean)
				.join(' '),
			recipe: it.entryId ? (titles.get(it.entryId) ?? null) : null,
			addedBy: it.addedBy?.displayName ?? null,
			addedById: it.addedBy?.id ?? null,
			note: it.note,
			formerMember: it.entryId === null && it.addedBy === null
		});
	}
	return [...rows.values()];
}

/** Open rows by department, in aisle order; empty departments are omitted. */
export function groupByDepartment(list: ShoppingList): { department: Department; rows: Row[] }[] {
	return DEPARTMENTS.map((department) => ({
		department,
		rows: rowsOf(
			list,
			list.items.filter((i) => i.department === department)
		).filter((r) => !r.checked)
	})).filter((g) => g.rows.length > 0);
}

export function checkedRows(list: ShoppingList): Row[] {
	return rowsOf(list, list.items).filter((r) => r.checked);
}

/**
 * Every ticked item, merged by name - the recipe view's "Done", where each
 * item shows its own tick, so a half-ticked name still lists its ticked part.
 */
export function tickedItemRows(list: ShoppingList): Row[] {
	return rowsOf(
		list,
		list.items.filter((i) => i.checked)
	);
}

/** One group per entry in list order, free items last under `entry: null` (omitted when none). */
export function groupByRecipe(list: ShoppingList): { entry: ShoppingEntry | null; rows: Row[] }[] {
	const groups: { entry: ShoppingEntry | null; rows: Row[] }[] = list.entries.map((entry) => ({
		entry,
		rows: rowsOf(
			list,
			list.items.filter((i) => i.entryId === entry.id)
		)
	}));
	const free = rowsOf(
		list,
		list.items.filter((i) => i.entryId === null)
	);
	// Every entry stays, even without items, so it can still be rescaled or removed.
	if (free.length > 0) groups.push({ entry: null, rows: free });
	return groups;
}

/**
 * The open rows as plain text, one row per line in aisle order: amounts, then
 * the name ("4 Stück + 1 Stück Zwiebeln"), the shape `shoppingList` copies
 * from a recipe, so to-do apps turn each line into one item.
 */
export function listText(list: ShoppingList): string {
	return groupByDepartment(list)
		.flatMap((g) => g.rows)
		.map((row) =>
			[
				row.sources
					.map((s) => s.amount)
					.filter(Boolean)
					.join(' + '),
				row.name
			]
				.filter(Boolean)
				.join(' ')
		)
		.join('\n');
}

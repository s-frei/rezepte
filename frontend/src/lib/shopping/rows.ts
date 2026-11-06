import type {
	Department,
	ListPerson,
	ShoppingEntry,
	ShoppingItem,
	ShoppingList
} from '#lib/api/shopping.js';
import { formatQuantity } from '#lib/recipe/format.js';
import { dimensionOf, fromBase, toBase } from '#lib/recipe/measures.js';
import { holderNames } from '#lib/user/holders.js';
import { DEPARTMENTS } from './departments';
import type { ListView } from './view.svelte';

export type Source = {
	itemId: string;
	/** Formatted quantity and unit; '' if none. */
	amount: string;
	/** Title of the entry the item came from; null for a free item. */
	recipe: string | null;
	/** Who typed a free item in, so the row can show their mark and the page say "by you". */
	addedBy: ListPerson | null;
	/**
	 * `addedBy` the way the household calls them: the first name, the full
	 * name when two people on the list share it (`holderNames`).
	 */
	addedByName: string | null;
	note: string | null;
	/** A free item whose adder's account is gone. */
	formerMember: boolean;
	/** Its items are ticked though the row is not (ticked by recipe): shown struck through, left out of `total`. */
	checked: boolean;
};

export type Row = {
	key: string;
	name: string;
	itemIds: string[];
	/** One per recipe (or per person who typed it in) and unit, amounts added up. */
	sources: Source[];
	/**
	 * What is still to buy, added up per unit ("3 Stück", "2 Stück + 200 g");
	 * '' when nothing has an amount, or when an item without one would hide in the sum.
	 */
	total: string;
	/** All items checked. */
	checked: boolean;
	/** Another device just ticked all of it (`ListState.shown`): shown ticked in its place a moment. */
	lingering: boolean;
	/** Who ticked its items, each person once. */
	checkedBy: ListPerson[];
};

const amountText = (quantity: number | null, unit: string) =>
	[formatQuantity(quantity), unit].filter(Boolean).join(' ');

/**
 * The key amounts add up under: a weight with a weight and a volume with a
 * volume whatever their unit, pieces with pieces (a bare number counts
 * pieces), anything else when spelled alike, case ignored. An item without
 * an amount never joins one with ("Eier" + "2 Eier" stay apart).
 */
function unitKey(item: { quantity: number | null; unit: string | null }) {
	const key = dimensionOf(item.unit) ?? (item.unit ?? '').trim().toLowerCase();
	return `${item.quantity === null ? '?' : ''}${key}`;
}

/**
 * Adds up quantities per `unitKey`, in first-seen order. Weights and volumes
 * read in the unit that suits the sum ("100 ml" + "2 l" is "2.1 l"); pieces
 * round up, since nobody buys a quarter egg; other units stay as written.
 */
function sumByUnit(parts: { quantity: number | null; unit: string | null }[]) {
	const sums = new Map<string, { quantity: number | null; unit: string }>();
	for (const p of parts) {
		const unit = (p.unit ?? '').trim();
		const base = p.quantity === null ? null : toBase(p.quantity, unit);
		const sum = sums.get(unitKey(p));
		if (!sum) sums.set(unitKey(p), { quantity: base, unit });
		else {
			if (base !== null) sum.quantity = (sum.quantity ?? 0) + base;
			sum.unit ||= unit;
		}
	}
	return [...sums.values()].map(({ quantity, unit }) => {
		const dimension = dimensionOf(unit);
		if (quantity === null || !dimension) return { quantity, unit };
		if (dimension === 'count') return { quantity: Math.ceil(quantity - 1e-9), unit };
		return fromBase(quantity, dimension);
	});
}

/** Each person on the list (and each adder since gone from it) by `holderNames`, keyed by id. */
function namesOf(list: ShoppingList): Map<string, string> {
	const people = new Map<string, ListPerson>();
	for (const p of [list.owner, ...list.members, ...list.items.map((i) => i.addedBy)]) {
		if (p && !people.has(p.id)) people.set(p.id, p);
	}
	const all = [...people.values()];
	const names = holderNames(all);
	return new Map(all.map((p, i) => [p.id, names[i]]));
}

function rowsOf(list: ShoppingList, items: ShoppingItem[]): Row[] {
	const names = namesOf(list);
	const titles = new Map(list.entries.map((e) => [e.id, e.title]));
	// The same recipe added twice is one origin: its lines add up instead of repeating.
	const recipeOf = new Map(list.entries.map((e) => [e.id, e.recipeId ?? e.id]));
	const rows = new Map<string, { row: Row; items: ShoppingItem[] }>();
	for (const it of items) {
		let entry = rows.get(it.nameKey);
		if (!entry) {
			entry = {
				row: {
					key: it.nameKey,
					name: it.name,
					itemIds: [],
					sources: [],
					total: '',
					checked: true,
					lingering: false,
					checkedBy: []
				},
				items: []
			};
			rows.set(it.nameKey, entry);
		}
		const { row } = entry;
		row.itemIds.push(it.id);
		row.checked &&= it.checked;
		const by = it.checkedBy;
		if (by && !row.checkedBy.some((p) => p.id === by.id)) row.checkedBy.push(by);
		entry.items.push(it);
	}
	return [...rows.values()].map(({ row, items }) => {
		// One source per recipe (or adder) and unit: a recipe listing the same
		// ingredient in two groups, or added twice, reads as one amount.
		const origins = new Map<string, ShoppingItem[]>();
		for (const it of items) {
			const origin = it.entryId ? recipeOf.get(it.entryId) : `free:${it.addedBy?.id ?? ''}`;
			const key = `${origin}|${unitKey(it)}`;
			origins.set(key, [...(origins.get(key) ?? []), it]);
		}
		row.sources = [...origins.values()].map((group) => {
			const [first] = group;
			const [sum] = sumByUnit(group);
			const notes = [...new Set(group.map((i) => i.note).filter(Boolean))];
			return {
				itemId: first.id,
				amount: amountText(sum.quantity, sum.unit),
				recipe: first.entryId ? (titles.get(first.entryId) ?? null) : null,
				addedBy: first.addedBy,
				addedByName: first.addedBy ? (names.get(first.addedBy.id) ?? null) : null,
				note: notes.length > 0 ? notes.join(', ') : null,
				formerMember: first.entryId === null && first.addedBy === null,
				checked: !row.checked && group.every((i) => i.checked)
			};
		});
		// An open row adds up what is still open; a done row, what was bought.
		const counted = row.checked ? items : items.filter((i) => !i.checked);
		row.total = counted.some((i) => i.quantity === null)
			? ''
			: sumByUnit(counted)
					.map((sum) => amountText(sum.quantity, sum.unit))
					.filter(Boolean)
					.join(' + ');
		row.lingering = items.some((i) => i.lingering) && items.every((i) => i.checked || i.lingering);
		return row;
	});
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
 * the name ("5 Stück Zwiebeln", "2 Stück + 200 g Zwiebeln"), the shape `shoppingList` copies
 * from a recipe, so to-do apps turn each line into one item.
 */
export function listText(list: ShoppingList): string {
	return groupByDepartment(list)
		.flatMap((g) => g.rows)
		.map((row) => [row.total, row.name].filter(Boolean).join(' '))
		.join('\n');
}

/**
 * What both list pages show of a list: the department groups, the recipe
 * groups, "Done" for the current view (the recipe view shows each item's own
 * tick, so its "Done" holds every ticked item) and the counts of the
 * department view's rows, so "n open · m done" agrees with "Done · m" and the
 * overview card.
 */
export function listViews(list: ShoppingList, view: ListView) {
	const departments = groupByDepartment(list);
	const doneRows = checkedRows(list);
	return {
		departments,
		recipes: groupByRecipe(list),
		done: view === 'department' ? doneRows : tickedItemRows(list),
		openRows: departments.reduce((n, g) => n + g.rows.length, 0),
		doneRows: doneRows.length
	};
}

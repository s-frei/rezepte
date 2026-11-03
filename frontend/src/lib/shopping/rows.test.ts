import { describe, expect, it } from 'vitest';
import type { ShoppingItem, ShoppingList } from '#lib/api/shopping.js';
import { DEPARTMENTS } from './departments';
import { checkedRows, groupByDepartment, groupByRecipe, listText, tickedItemRows } from './rows';

const entry = (id: string, title: string) => ({
	id,
	recipeId: 'r' + id,
	recipeSlug: title.toLowerCase(),
	coverImageId: null,
	title,
	baseServings: 4,
	servings: 4
});

const item = (over: Partial<ShoppingItem> & { id: string }): ShoppingItem => ({
	entryId: null,
	name: 'Onion',
	nameKey: 'onion',
	quantity: null,
	unit: null,
	note: null,
	group: null,
	department: 'produce',
	addedBy: null,
	checked: false,
	...over
});

const list = (items: ShoppingItem[]): ShoppingList => ({
	id: 'l',
	name: 'Week',
	createdBy: 'u',
	createdAt: '',
	updatedAt: '',
	openCount: 0,
	doneCount: 0,
	entries: [entry('e1', 'Soup'), entry('e2', 'Stew')],
	items
});

describe('groupByDepartment', () => {
	it('merges same nameKey into one row with two sources', () => {
		const g = groupByDepartment(
			list([
				item({ id: 'a', entryId: 'e1', quantity: 1.5, unit: 'kg' }),
				item({ id: 'b', entryId: 'e2', quantity: 2, note: 'diced' })
			])
		);
		expect(g).toHaveLength(1);
		expect(g[0].department).toBe('produce');
		expect(g[0].rows).toHaveLength(1);
		expect(g[0].rows[0].itemIds).toEqual(['a', 'b']);
		expect(g[0].rows[0].sources).toEqual([
			{ itemId: 'a', amount: '1 ½ kg', recipe: 'Soup', addedBy: null, addedById: null, note: null },
			{ itemId: 'b', amount: '2', recipe: 'Stew', addedBy: null, addedById: null, note: 'diced' }
		]);
	});

	it('keeps a row open when only one of two items is checked', () => {
		const g = groupByDepartment(list([item({ id: 'a', checked: true }), item({ id: 'b' })]));
		expect(g[0].rows[0].checked).toBe(false);
		expect(checkedRows(list([item({ id: 'a', checked: true }), item({ id: 'b' })]))).toEqual([]);
	});

	it('carries addedBy on free items and orders departments', () => {
		const by = { id: 'u', displayName: 'Mara', color: 'x', avatarId: null };
		const g = groupByDepartment(
			list([
				item({ id: 'm', name: 'Milk', nameKey: 'milk', department: 'chilled' }),
				item({ id: 'a', addedBy: by })
			])
		);
		expect(g.map((d) => d.department)).toEqual(['produce', 'chilled']);
		const all = groupByDepartment(
			list(
				[...DEPARTMENTS]
					.reverse()
					.map((department) =>
						item({ id: department, nameKey: department, name: department, department })
					)
			)
		);
		expect(all.map((d) => d.department)).toEqual(DEPARTMENTS);
		expect(g[0].rows[0].sources[0]).toMatchObject({
			addedBy: 'Mara',
			addedById: 'u',
			recipe: null
		});
	});

	it('puts fully checked rows in checkedRows only', () => {
		const l = list([
			item({ id: 'a', checked: true }),
			item({ id: 'm', nameKey: 'milk', name: 'Milk' })
		]);
		expect(checkedRows(l).map((r) => r.key)).toEqual(['onion']);
		expect(groupByDepartment(l)[0].rows.map((r) => r.key)).toEqual(['milk']);
	});
});

describe('groupByRecipe', () => {
	it('lists entries in order and free items last under null', () => {
		const g = groupByRecipe(
			list([
				item({ id: 'f', nameKey: 'salt', name: 'Salt' }),
				item({ id: 'a', entryId: 'e2' }),
				item({ id: 'b', entryId: 'e1' })
			])
		);
		expect(g.map((x) => x.entry?.id ?? null)).toEqual(['e1', 'e2', null]);
		expect(g[2].rows[0].key).toBe('salt');
	});

	it('keeps an entry that has no items', () => {
		const g = groupByRecipe(list([item({ id: 'a', entryId: 'e2' })]));
		expect(g.map((x) => [x.entry?.id, x.rows.length])).toEqual([
			['e1', 0],
			['e2', 1]
		]);
	});
});

describe('listText', () => {
	it('writes open rows in aisle order, amounts joined, checked rows left out', () => {
		const l = list([
			item({
				id: 'm',
				name: 'Milk',
				nameKey: 'milk',
				department: 'chilled',
				quantity: 1,
				unit: 'l'
			}),
			item({ id: 'a', entryId: 'e1', quantity: 4, unit: 'pc' }),
			item({ id: 'b', entryId: 'e2', quantity: 1, unit: 'pc' }),
			item({ id: 's', name: 'Salt', nameKey: 'salt', department: 'pantry' }),
			item({ id: 'x', name: 'Soap', nameKey: 'soap', department: 'household', checked: true })
		]);
		expect(listText(l)).toBe('4 pc + 1 pc Onion\n1 l Milk\nSalt');
	});
});

describe('tickedItemRows', () => {
	it('merges ticked items by name and keeps the ticked half of a half-ticked name', () => {
		const l = list([
			item({ id: 'a', entryId: 'e1', checked: true }),
			item({ id: 'b', entryId: 'e2', checked: true }),
			item({ id: 's', name: 'Salt', nameKey: 'salt', entryId: 'e1', checked: true }),
			item({ id: 't', name: 'Salt', nameKey: 'salt', entryId: 'e2' })
		]);
		expect(tickedItemRows(l).map((r) => [r.key, r.itemIds, r.checked])).toEqual([
			['onion', ['a', 'b'], true],
			['salt', ['s'], true]
		]);
	});
});

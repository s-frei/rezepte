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
	checkedBy: null,
	...over
});

const list = (items: ShoppingItem[]): ShoppingList => ({
	id: 'l',
	name: 'Week',
	createdBy: 'u',
	createdAt: '',
	updatedAt: '',
	revision: 0,
	openCount: 0,
	doneCount: 0,
	entries: [entry('e1', 'Soup'), entry('e2', 'Stew')],
	items,
	owner: { id: 'u', displayName: 'U', color: 'terracotta', avatarId: null },
	members: [],
	pending: []
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
			{
				itemId: 'a',
				amount: '1 ½ kg',
				recipe: 'Soup',
				addedBy: null,
				addedByName: null,
				note: null,
				formerMember: false,
				checked: false
			},
			{
				itemId: 'b',
				amount: '2',
				recipe: 'Stew',
				addedBy: null,
				addedByName: null,
				note: 'diced',
				formerMember: false,
				checked: false
			}
		]);
	});

	it('keeps a row open when only one of two items is checked', () => {
		const g = groupByDepartment(list([item({ id: 'a', checked: true }), item({ id: 'b' })]));
		expect(g[0].rows[0].checked).toBe(false);
		expect(checkedRows(list([item({ id: 'a', checked: true }), item({ id: 'b' })]))).toEqual([]);
	});

	it('carries addedBy on free items and orders departments', () => {
		const by = { id: 'm', displayName: 'Mara', color: 'x', avatarId: null };
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
			addedBy: by,
			addedByName: 'Mara',
			recipe: null
		});
	});

	it('names an adder by first name, in full when two on the list share it', () => {
		const anna = (id: string, displayName: string) => ({
			id,
			displayName,
			color: 'sage',
			avatarId: null
		});
		const l = list([
			item({ id: 'a', addedBy: anna('a1', 'Anna Berg') }),
			item({ id: 'b', nameKey: 'milk', name: 'Milk', addedBy: anna('a2', 'Anna Tal') }),
			item({ id: 'c', nameKey: 'egg', name: 'Egg', addedBy: anna('j', 'Jonas Zimt') })
		]);
		const names = groupByDepartment(l)[0].rows.map((r) => r.sources[0].addedByName);
		expect(names).toEqual(['Anna Berg', 'Anna Tal', 'Jonas']);
	});

	it('marks free items of a gone account', () => {
		const by = { id: 'u', displayName: 'Mara', color: 'x', avatarId: null };
		const g = groupByDepartment(
			list([
				item({ id: 'a', entryId: null, addedBy: null }),
				item({ id: 'b', nameKey: 'milk', name: 'Milk', entryId: null, addedBy: by }),
				item({ id: 'c', nameKey: 'egg', name: 'Egg', entryId: 'e1' })
			])
		);
		const former = Object.fromEntries(g[0].rows.map((r) => [r.key, r.sources[0].formerMember]));
		expect(former).toEqual({ onion: true, milk: false, egg: false });
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
	it('writes open rows in aisle order, amounts added up, checked rows left out', () => {
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
			item({ id: 'g', entryId: 'e2', quantity: 200, unit: 'g' }),
			item({ id: 's', name: 'Salt', nameKey: 'salt', department: 'pantry' }),
			item({ id: 'x', name: 'Soap', nameKey: 'soap', department: 'household', checked: true })
		]);
		expect(listText(l)).toBe('5 pc + 200 g Onion\n1 l Milk\nSalt');
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

describe('checkedBy', () => {
	it('names each person who ticked a row once', () => {
		const mila = { id: 'm', displayName: 'Mila', color: 'sage', avatarId: null };
		const jonas = { id: 'j', displayName: 'Jonas', color: 'plum', avatarId: null };
		const rows = checkedRows(
			list([
				item({ id: 'a', checked: true, checkedBy: mila }),
				item({ id: 'b', checked: true, checkedBy: mila }),
				item({ id: 'c', checked: true, checkedBy: jonas })
			])
		);
		expect(rows[0].checkedBy.map((p) => p.displayName)).toEqual(['Mila', 'Jonas']);
	});
});

describe('amounts', () => {
	it('adds up a recipe listing a name twice, and the row across recipes', () => {
		const [row] = checkedRows(
			list([
				item({ id: 'a', entryId: 'e1', quantity: 1, unit: 'Stück', checked: true }),
				item({ id: 'b', entryId: 'e2', quantity: 1, unit: 'Stück', note: 'rot', checked: true }),
				item({ id: 'c', entryId: 'e2', quantity: 1, unit: 'stück', checked: true }),
				item({ id: 'd', entryId: 'e2', checked: true })
			])
		);
		// "d" has no amount: a line of its own, and no total that would hide it.
		expect(row.sources.map((s) => [s.amount, s.recipe, s.note])).toEqual([
			['1 Stück', 'Soup', null],
			['2 Stück', 'Stew', 'rot'],
			['', 'Stew', null]
		]);
		expect(row.total).toBe('');
		expect(row.itemIds).toEqual(['a', 'b', 'c', 'd']);
	});

	it('an open row adds up what is open and strikes a ticked recipe through', () => {
		const [group] = groupByDepartment(
			list([
				item({ id: 'a', entryId: 'e1', quantity: 1, unit: 'Stück', checked: true }),
				item({ id: 'b', entryId: 'e2', quantity: 2, unit: 'Stück' })
			])
		);
		const [row] = group.rows;
		expect(row.total).toBe('2 Stück');
		expect(row.sources.map((s) => s.checked)).toEqual([true, false]);
	});

	it('lingers only when every item behind the row is ticked', () => {
		const half = groupByDepartment(
			list([item({ id: 'a', entryId: 'e1', lingering: true }), item({ id: 'b', entryId: 'e2' })])
		)[0].rows[0];
		expect(half.lingering).toBe(false);
		const whole = groupByDepartment(
			list([
				item({ id: 'a', entryId: 'e1', lingering: true }),
				item({ id: 'b', entryId: 'e2', checked: true })
			])
		)[0].rows[0];
		expect(whole.lingering).toBe(true);
	});

	it('adds up the same recipe added twice into one line', () => {
		const l = list([
			item({ id: 'a', entryId: 'e1', quantity: 1, unit: 'Stück' }),
			item({ id: 'b', entryId: 'e3', quantity: 1, unit: 'Stück' })
		]);
		l.entries.push({ ...entry('e3', 'Soup'), recipeId: 're1' });
		const [row] = groupByDepartment(l)[0].rows;
		expect(row.sources.map((s) => [s.amount, s.recipe])).toEqual([['2 Stück', 'Soup']]);
	});

	it('adds weights and volumes across units and rounds pieces up', () => {
		const rows = groupByDepartment(
			list([
				item({ id: 'm1', name: 'Milk', nameKey: 'milk', entryId: 'e1', quantity: 100, unit: 'ml' }),
				item({ id: 'm2', name: 'Milk', nameKey: 'milk', entryId: 'e2', quantity: 2, unit: 'l' }),
				item({
					id: 'f1',
					name: 'Flour',
					nameKey: 'flour',
					entryId: 'e1',
					quantity: 1625,
					unit: 'g'
				}),
				item({ id: 'e', name: 'Egg', nameKey: 'egg', entryId: 'e1', quantity: 1.25, unit: 'Stück' })
			])
		)[0].rows;
		const by = (key: string) => rows.find((r) => r.key === key)!;
		expect(by('milk').total).toBe('2.1 l');
		expect(by('flour').sources[0].amount).toBe('1.63 kg');
		expect(by('egg').sources[0].amount).toBe('2 Stück');
	});

	it('counts a bare number as pieces', () => {
		const [row] = checkedRows(
			list([
				item({ id: 'a', entryId: 'e1', quantity: 1.5, unit: 'Stück', checked: true }),
				item({ id: 'b', quantity: 2, checked: true })
			])
		);
		expect(row.total).toBe('4 Stück');
	});
});

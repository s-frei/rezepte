import { describe, expect, it } from 'vitest';
import type { PlanEntry } from '#lib/api/mealplan.js';
import { addRows, byZone, countUnlisted, listsOf, placeEntry, shownEntries, TRAY } from './entries';

const entry = (id: string, day: string | null, position: number): PlanEntry => ({
	id,
	day,
	position,
	recipeId: null,
	recipeSlug: null,
	title: id,
	servings: null,
	photo: null,
	createdBy: null,
	lists: []
});

const days = ['2026-11-16', '2026-11-17'];
const ids = (zones: Record<string, PlanEntry[]>) =>
	Object.fromEntries(Object.entries(zones).map(([k, v]) => [k, v.map((e) => e.id)]));

describe('plan entries', () => {
	it('groups by day and tray in position order', () => {
		const zones = byZone(
			[entry('b', days[0], 1), entry('t', null, 0), entry('a', days[0], 0)],
			days
		);
		expect(ids(zones)).toEqual({ [TRAY]: ['t'], [days[0]]: ['a', 'b'], [days[1]]: [] });
	});

	it('moves from the tray into a day at an index and renumbers both', () => {
		const before = [
			entry('a', days[0], 0),
			entry('b', days[0], 1),
			entry('t', null, 0),
			entry('u', null, 1)
		];
		const after = placeEntry(before, 't', days[0], 1);
		expect(ids(byZone(after, days))).toEqual({
			[TRAY]: ['u'],
			[days[0]]: ['a', 't', 'b'],
			[days[1]]: []
		});
		expect(after.find((e) => e.id === 'u')!.position).toBe(0);
		expect(after.find((e) => e.id === 'b')!.position).toBe(2);
	});

	it('reorders within a day', () => {
		const before = [entry('a', days[0], 0), entry('b', days[0], 1), entry('c', days[0], 2)];
		expect(ids(byZone(placeEntry(before, 'a', days[0], 2), days))[days[0]]).toEqual([
			'b',
			'c',
			'a'
		]);
	});

	it('moves out of the shown week', () => {
		const after = placeEntry([entry('a', days[0], 0)], 'a', '2026-11-30', 0);
		expect(ids(byZone(after, days))[days[0]]).toEqual([]);
	});
});

describe('addRows', () => {
	const curry = { id: 'c' };
	it('lists the recipes found for the text, then the note', () => {
		expect(addRows('Cur', { query: 'Cur', items: [curry] })).toEqual([
			{ id: 'plan-add-c', recipe: curry },
			{ id: 'plan-add-note', note: 'Cur' }
		]);
	});

	it('leaves out an answer for an earlier text', () => {
		expect(addRows('Pizza bestellen', { query: '', items: [curry] })).toEqual([
			{ id: 'plan-add-note', note: 'Pizza bestellen' }
		]);
	});

	it('cuts a note at 120 runes', () => {
		const rows = addRows('ä'.repeat(130), null);
		expect([...rows[0].note!]).toHaveLength(120);
	});
});

describe('the days in view', () => {
	const recipe = (id: string, day: string | null, lists: string[] = []): PlanEntry => ({
		...entry(id, day, 0),
		recipeId: `r-${id}`,
		lists: lists.map((l) => ({ id: l, name: l, openCount: 1 }))
	});
	const all = [
		recipe('gone', '2026-11-20'),
		recipe('today', '2026-11-22'),
		recipe('listed', '2026-11-23', ['week']),
		recipe('later', '2026-11-29'),
		recipe('beyond', '2026-12-01', ['market']),
		recipe('tray', null),
		{ ...entry('note', '2026-11-24', 0) }
	];

	it('keeps the days in view and the tray', () => {
		expect(shownEntries(all, '2026-11-22', '2026-11-28').map((e) => e.id)).toEqual([
			'today',
			'listed',
			'tray',
			'note'
		]);
	});

	it('counts the recipes on no list from today on, and the tray, as the server', () => {
		const shown = shownEntries(all, '2026-11-19', '2026-11-25');
		// "gone" lies before today, "listed" is on a list, the note is no recipe.
		expect(countUnlisted(shown, '2026-11-22')).toBe(2);
		expect(countUnlisted(shownEntries(all, '2026-11-26', '2026-12-02'), '2026-11-22')).toBe(2);
	});

	it('keeps the lists the days in view went to, counting their entries', () => {
		const lists = [
			{ id: 'week', name: 'week', openCount: 3, entries: 9 },
			{ id: 'market', name: 'market', openCount: 1, entries: 4 }
		];
		expect(listsOf(lists, shownEntries(all, '2026-11-22', '2026-11-28'))).toEqual([
			{ id: 'week', name: 'week', openCount: 3, entries: 1 }
		]);
	});
});

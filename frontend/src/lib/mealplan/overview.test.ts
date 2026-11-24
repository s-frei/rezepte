import { describe, expect, it } from 'vitest';
import type { WeekEntry, WeekPlan } from '#lib/api/mealplan.js';
import { byUse, cellKind, isIdle, weekStrip } from './overview.svelte';

const person = { id: 'u', displayName: 'U', color: 'sage', avatarId: null };

function plan(id: string, meals: number, updatedAt: string, personal = false, tray = 0): WeekPlan {
	return {
		id,
		name: personal ? null : id,
		personal,
		owner: person,
		members: [],
		updatedAt,
		entries: Array.from({ length: meals }, (_, i) => entry(`${id}${i}`, '2026-11-16')),
		unlisted: 0,
		lists: [],
		tray,
		pending: []
	};
}

function entry(id: string, day: string, over: Partial<WeekEntry> = {}): WeekEntry {
	return {
		id,
		day,
		kind: 'recipe',
		recipeId: 'r',
		title: id,
		photo: null,
		photoWidth: null,
		photoHeight: null,
		photoFocus: null,
		...over
	};
}

describe('plans overview', () => {
	it('puts my plan first, then by meals this week, then by last change', () => {
		const plans = [
			plan('idle-old', 0, '2026-11-01T00:00:00Z'),
			plan('busy', 5, '2026-11-02T00:00:00Z'),
			plan('idle-new', 0, '2026-11-20T00:00:00Z'),
			plan('mine', 0, '2026-10-01T00:00:00Z', true),
			plan('some', 2, '2026-11-21T00:00:00Z')
		];
		expect(plans.sort(byUse).map((p) => p.id)).toEqual([
			'mine',
			'busy',
			'some',
			'idle-new',
			'idle-old'
		]);
	});

	it('makes a named plan with nothing this week, not even on the tray, a chip; never my plan', () => {
		expect(isIdle(plan('idle', 0, ''))).toBe(true);
		expect(isIdle(plan('busy', 1, ''))).toBe(false);
		expect(isIdle(plan('tray', 0, '', false, 2))).toBe(false);
		expect(isIdle(plan('mine', 0, '', true))).toBe(false);
	});

	it('lays the week out as seven days around today', () => {
		const strip = weekStrip(
			[
				entry('a', '2026-11-16'),
				entry('b', '2026-11-18'),
				entry('c', '2026-11-18'),
				entry('d', '2026-11-23')
			],
			'2026-11-16',
			'2026-11-18'
		);
		expect(strip.map((d) => d.day)).toEqual([
			'2026-11-16',
			'2026-11-17',
			'2026-11-18',
			'2026-11-19',
			'2026-11-20',
			'2026-11-21',
			'2026-11-22'
		]);
		expect(strip.map((d) => d.when)).toEqual([
			'past',
			'past',
			'today',
			'future',
			'future',
			'future',
			'future'
		]);
		expect(strip.map((d) => d.meals.map((e) => e.id))).toEqual([
			['a'],
			[],
			['b', 'c'],
			[],
			[],
			[],
			[]
		]);
	});

	it('shows a photo, a letter for a recipe without one or deleted, a pencil for a note', () => {
		expect(cellKind(entry('a', '', { photo: 'p' }))).toBe('photo');
		expect(cellKind(entry('a', ''))).toBe('letter');
		expect(cellKind(entry('a', '', { recipeId: null, photo: null }))).toBe('letter');
		expect(cellKind(entry('a', '', { kind: 'text', recipeId: null }))).toBe('note');
	});
});

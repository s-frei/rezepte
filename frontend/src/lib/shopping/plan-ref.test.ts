import { describe, expect, it } from 'vitest';
import type { ListPerson, PlanRef } from '#lib/api/shopping.js';
import {
	byListName,
	groupByPlan,
	planJumpHref,
	planStamp,
	uniquePlans,
	type Planned
} from './plan-ref';

const mila: ListPerson = { id: 'mila', displayName: 'Mila Majoran', color: 'sage', avatarId: null };
const me = 'me';
const thisMonday = '2026-11-16';

const byHand: PlanRef = {
	planId: null,
	planEntryId: null,
	planMember: false,
	planName: null,
	planPersonal: false,
	planAddedBy: null
};
const home = (day: string | null, entry = 'e1'): Planned => ({
	planId: 'home',
	planEntryId: entry,
	planMember: true,
	planName: 'Haushalt',
	planPersonal: false,
	planAddedBy: mila,
	planDay: day
});
const mine = (day: string): Planned => ({
	...home(day),
	planId: 'mine',
	planName: null,
	planPersonal: true
});
const foreign = (by: ListPerson | null, day: string | null = '2026-11-17'): Planned => ({
	...byHand,
	planId: `of-${by?.id ?? 'gone'}`,
	planEntryId: 'x',
	planAddedBy: by,
	planDay: day
});

describe('planJumpHref', () => {
	it('leaves this week out and keeps the day and meal', () => {
		expect(planJumpHref('p', '2026-11-18', 'e', thisMonday)).toBe(
			'/plans/p?day=2026-11-18&entry=e'
		);
		expect(planJumpHref('p', '2026-11-25', null, thisMonday)).toBe(
			'/plans/p?from=2026-11-23&day=2026-11-25'
		);
		// A tray entry: no week, only the meal.
		expect(planJumpHref('p', null, 'e', thisMonday)).toBe('/plans/p?entry=e');
	});
});

describe('planStamp', () => {
	it('names and links a plan the reader is in, at the meal', () => {
		expect(planStamp(home('2026-11-16'), me, thisMonday)).toEqual({
			key: 'home',
			label: 'Haushalt',
			href: '/plans/home?day=2026-11-16&entry=e1',
			member: true,
			personal: false
		});
		expect(planStamp(mine('2026-11-16'), me, thisMonday)?.label).toBe('My plan');
	});

	it('names only who put it on the list for anyone else, and never links', () => {
		const stamp = planStamp(foreign(mila), me, thisMonday);
		expect(stamp).toMatchObject({ label: 'Planned by Mila', href: null, member: false });
		expect(planStamp(foreign({ ...mila, id: me }), me, thisMonday)?.label).toBe('Planned by you');
		expect(planStamp(foreign({ ...mila, displayName: ' Jo Doe' }), me, thisMonday)?.label).toBe(
			'Planned by Jo'
		);
		expect(planStamp(foreign(null), me, thisMonday)?.label).toBe('Planned');
	});

	it('gives a recipe added by hand no stamp', () => {
		expect(planStamp(byHand, me, thisMonday)).toBeNull();
	});
});

describe('uniquePlans', () => {
	it('names each plan once, at its earliest day, in the stamps order', () => {
		const stamps = uniquePlans(
			[
				foreign(mila),
				home('2026-11-19', 'e2'),
				byHand,
				home('2026-11-17'),
				mine('2026-11-20'),
				{ ...foreign(mila), planId: 'another-of-mila' }
			],
			me,
			thisMonday
		);
		expect(stamps.map((s) => [s.label, s.href])).toEqual([
			['My plan', '/plans/mine?day=2026-11-20'],
			['Haushalt', '/plans/home?day=2026-11-17'],
			// Two plans of others put on by the same person read the same: one stamp.
			['Planned by Mila', null]
		]);
	});
});

describe('groupByPlan', () => {
	it('groups by plan and week, by day inside, the ones by hand last', () => {
		const items = [
			{ ...byHand, id: 'hand' },
			{ ...home('2026-11-25'), id: 'next' },
			{ ...home('2026-11-19'), id: 'thu' },
			{ ...home(null), id: 'tray' },
			{ ...home('2026-11-17'), id: 'tue' },
			{ ...foreign(mila), id: 'mila' },
			{ ...mine('2026-11-18'), id: 'mine' }
		];
		const groups = groupByPlan(items, me, thisMonday);
		expect(groups.map((g) => [g.plan?.label ?? null, g.monday, g.items.map((i) => i.id)])).toEqual([
			['My plan', '2026-11-16', ['mine']],
			['Haushalt', '2026-11-16', ['tue', 'thu']],
			['Haushalt', '2026-11-23', ['next']],
			// The tray has no week: after the plan's weeks.
			['Haushalt', '', ['tray']],
			['Planned by Mila', '2026-11-16', ['mila']],
			[null, '', ['hand']]
		]);
	});
});

describe('groupByPlan across the turn of the year', () => {
	it('puts January after December and links the head to the first day', () => {
		const groups = groupByPlan(
			[
				{ ...home('2027-01-05'), id: 'jan' },
				{ ...home('2026-12-29'), id: 'dec' }
			],
			me,
			thisMonday
		);
		expect(groups.map((g) => [g.monday, g.plan?.href])).toEqual([
			['2026-12-28', '/plans/home?from=2026-12-28&day=2026-12-29'],
			['2027-01-04', '/plans/home?from=2027-01-04&day=2027-01-05']
		]);
	});
});

describe('byListName', () => {
	it('keeps one order by name', () => {
		expect(
			byListName([{ name: 'Vorrat' }, { name: 'Markt' }, { name: 'Ähnlich' }]).map((l) => l.name)
		).toEqual(['Ähnlich', 'Markt', 'Vorrat']);
	});
});

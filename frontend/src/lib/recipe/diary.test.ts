import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Comment } from '$lib/api/comments';

let locale: 'en' | 'de' = 'de';
vi.mock('$lib/paraglide/runtime', () => ({
	getLocale: () => locale,
	experimentalStaticLocale: undefined
}));

const { entryTime, groupByDay, overLimit } = await import('./diary');

beforeEach(() => {
	locale = 'de';
});

const at = (iso: string, id = 1): Comment => ({
	id,
	body: 'x',
	createdAt: iso,
	editedAt: null,
	new: false,
	canEdit: false,
	canDelete: false
});

describe('groupByDay', () => {
	const now = new Date('2026-10-03T12:00:00');
	it('labels today and yesterday and keeps order', () => {
		const days = groupByDay(
			[
				at('2026-09-29T19:40:00', 1),
				at('2026-10-02T20:15:00', 2),
				at('2026-10-03T09:00:00', 3),
				at('2026-10-03T10:00:00', 4)
			],
			now
		);
		expect(days.map((d) => d.label)).toEqual(['Dienstag, 29. September', 'Gestern', 'Heute']);
		expect(days[2].entries.map((e) => e.id)).toEqual([3, 4]);
	});
	it('uses the local day, not the UTC day', () => {
		const late = new Date(2026, 9, 2, 23, 30).toISOString();
		expect(groupByDay([at(late)], now)[0].label).toBe('Gestern');
	});
	it('adds the year for another year', () => {
		locale = 'en';
		expect(groupByDay([at('2025-12-24T18:00:00')], now)[0].label).toBe(
			'Wednesday, December 24, 2025'
		);
	});
});

describe('entryTime', () => {
	it('formats hours and minutes', () => {
		expect(entryTime(new Date(2026, 9, 3, 7, 5).toISOString())).toBe('07:05');
	});
});

describe('overLimit', () => {
	it('counts code points after trimming, like the service', () => {
		expect(overLimit(`  ${'ä'.repeat(2000)}  `)).toBe(0);
		expect(overLimit('ä'.repeat(2001))).toBe(1);
		expect(overLimit('🍋'.repeat(2000))).toBe(0);
	});
});

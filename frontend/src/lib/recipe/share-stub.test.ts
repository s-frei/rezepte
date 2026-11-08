import { describe, expect, it } from 'vitest';
import { m } from '#lib/paraglide/messages.js';
import type { PublicShare } from '#lib/api/shares.js';
import { lifetimeCell, stubFace } from './share-stub';

const now = new Date(2026, 10, 8, 10, 0);

const share = (expires: Date | null, status: PublicShare['status'] = 'active'): PublicShare => ({
	id: 's',
	recipe: { id: 'r', slug: 'r', title: 'R' },
	path: '/s/x',
	createdAt: now.toISOString(),
	startsAt: now.toISOString(),
	expiresAt: expires === null ? null : expires.toISOString(),
	status
});

describe('stubFace', () => {
	it('reports paused and limited', () => {
		expect(stubFace(share(null, 'paused'), now)).toEqual({ kind: 'paused' });
		expect(stubFace(share(null, 'limited'), now)).toEqual({ kind: 'limited' });
	});

	it('is open without an expiry', () => {
		expect(stubFace(share(null), now)).toEqual({ kind: 'open' });
	});

	it('counts local calendar days', () => {
		expect(stubFace(share(new Date(2026, 10, 15, 10, 0)), now)).toEqual({ kind: 'days', value: 7 });
		expect(stubFace(share(new Date(2026, 10, 9, 9, 0)), now)).toEqual({ kind: 'days', value: 1 });
		expect(stubFace(share(new Date(2026, 11, 1)), now)).toEqual({ kind: 'days', value: 23 });
	});

	it('is today with the expiry time', () => {
		const until = new Date(2026, 10, 8, 18, 0);
		expect(stubFace(share(until), now)).toEqual({ kind: 'today', until });
	});

	it('rounds to months from 60 days', () => {
		expect(stubFace(share(new Date(2027, 10, 7)), now)).toEqual({ kind: 'months', value: 12 });
	});
});

describe('lifetimeCell', () => {
	it('splits number and unit', () => {
		expect(lifetimeCell(30)).toEqual({ value: '30', unit: m.share_unit_days() });
		expect(lifetimeCell(1)).toEqual({ value: '1', unit: m.share_unit_day() });
		expect(lifetimeCell(365)).toEqual({ value: '1', unit: m.share_unit_year() });
		expect(lifetimeCell(null)).toEqual({ value: '∞', unit: '' });
	});
});

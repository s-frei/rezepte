import { afterEach, describe, expect, it } from 'vitest';
import type { PersonEntry } from '#lib/api/users.js';
import { getLocale, overwriteGetLocale } from '#lib/paraglide/runtime.js';
import type { UserColor } from './color';
import { colorHolders, holderNames, holdersFullText, holdersText } from './holders';

function person(id: string, displayName: string, color: UserColor = 'amber'): PersonEntry {
	return { id, username: id, displayName, color, role: 'user', avatarId: null };
}

const english = getLocale;
afterEach(() => overwriteGetLocale(english));

describe('colorHolders', () => {
	it('lists the others in the color, in reading order', () => {
		const people = [
			person('me', 'Me'),
			person('s', 'Sofia'),
			person('x', 'Xaver', 'teal'),
			person('ae', 'Änna'),
			person('i', 'Ida')
		];
		expect(colorHolders(people, 'amber', 'me').map((p) => p.displayName)).toEqual([
			'Änna',
			'Ida',
			'Sofia'
		]);
	});
});

describe('holderNames', () => {
	it('names a holder by the first word', () => {
		expect(holderNames([person('s', 'Sofia Safran'), person('t', 'Tom')])).toEqual([
			'Sofia',
			'Tom'
		]);
	});

	it('names two holders who share a first word in full', () => {
		expect(
			holderNames([person('a', 'Anna Apfel'), person('b', 'Anna Birne'), person('i', 'Ida Ingwer')])
		).toEqual(['Anna Apfel', 'Anna Birne', 'Ida']);
	});
});

describe('holdersText', () => {
	const people = (...names: string[]) => names.map((n) => person(n, n));

	it('says when nobody else wears it', () => {
		overwriteGetLocale(() => 'de');
		expect(holdersText([])).toBe('trägt sonst niemand');
	});

	it('names up to three', () => {
		overwriteGetLocale(() => 'de');
		expect(holdersText(people('Sofia', 'Ida', 'Tom'))).toBe('trägt auch Sofia, Ida und Tom');
	});

	it('names two and counts the rest beyond three', () => {
		overwriteGetLocale(() => 'de');
		expect(holdersText(people('Sofia', 'Ida', 'Tom', 'Finn', 'Lea'))).toBe(
			'trägt auch Sofia, Ida und 3 weitere'
		);
	});
});

describe('holdersFullText', () => {
	it('names every holder in full', () => {
		const holders = ['Anna Apfel', 'Ida Ingwer', 'Sofia Safran', 'Tom Thymian'].map((name, i) =>
			person(String(i), name)
		);
		expect(holdersFullText(holders)).toBe(
			'is also worn by Anna Apfel, Ida Ingwer, Sofia Safran, and Tom Thymian'
		);
	});
});

import { describe, expect, it } from 'vitest';
import { buildListQuery, parseListQuery, type ListQuery } from './query';

function urlWith(qs: string): URL {
	return new URL(`https://rezepte.local/${qs ? `?${qs}` : ''}`);
}

describe('parseListQuery', () => {
	it('defaults to an empty query, no tags and page 1', () => {
		expect(parseListQuery(urlWith(''))).toEqual({ q: '', tags: [], page: 1 });
	});

	it('trims q', () => {
		expect(parseListQuery(urlWith('q=%20pasta%20'))).toEqual({ q: 'pasta', tags: [], page: 1 });
	});

	it('splits tags on commas and drops empty entries', () => {
		expect(parseListQuery(urlWith('tags=vegetarisch,schnell,'))).toEqual({
			q: '',
			tags: ['vegetarisch', 'schnell'],
			page: 1
		});
	});

	it('trims whitespace around each tag', () => {
		expect(parseListQuery(urlWith('tags=%20a%20,b'))).toEqual({
			q: '',
			tags: ['a', 'b'],
			page: 1
		});
	});

	it('lower-cases tags, since that is how they are stored', () => {
		expect(parseListQuery(urlWith('tags=Dessert,S%C3%9CSS'))).toEqual({
			q: '',
			tags: ['dessert', 'süss'],
			page: 1
		});
	});

	it('parses a valid page', () => {
		expect(parseListQuery(urlWith('page=3'))).toEqual({ q: '', tags: [], page: 3 });
	});

	it('falls back to page 1 for anything that is not a positive integer', () => {
		expect(parseListQuery(urlWith('page=0'))).toEqual({ q: '', tags: [], page: 1 });
		expect(parseListQuery(urlWith('page=-1'))).toEqual({ q: '', tags: [], page: 1 });
		expect(parseListQuery(urlWith('page=abc'))).toEqual({ q: '', tags: [], page: 1 });
		expect(parseListQuery(urlWith('page=2.5'))).toEqual({ q: '', tags: [], page: 1 });
	});
});

describe('buildListQuery', () => {
	it('omits every field at its default', () => {
		expect(buildListQuery({ q: '', tags: [], page: 1 })).toBe('');
	});

	it('includes q when set', () => {
		expect(buildListQuery({ q: 'pasta', tags: [], page: 1 })).toBe('q=pasta');
	});

	it('joins tags with commas when set', () => {
		expect(buildListQuery({ q: '', tags: ['a', 'b'], page: 1 })).toBe('tags=a%2Cb');
	});

	it('includes page when it is not 1', () => {
		expect(buildListQuery({ q: '', tags: [], page: 2 })).toBe('page=2');
	});

	it('orders q, tags, page stably when all are set', () => {
		expect(buildListQuery({ q: 'pasta', tags: ['a', 'b'], page: 3 })).toBe(
			'q=pasta&tags=a%2Cb&page=3'
		);
	});

	it('normalises the state it is handed, like parseListQuery does', () => {
		expect(buildListQuery({ q: '  pasta ', tags: [' Dessert', '', 'B'], page: 0 })).toBe(
			'q=pasta&tags=dessert%2Cb'
		);
	});
});

describe('round trip', () => {
	it('parseListQuery(url with buildListQuery(state)) recovers the state', () => {
		const states: ListQuery[] = [
			{ q: '', tags: [], page: 1 },
			{ q: 'pasta', tags: [], page: 1 },
			{ q: '', tags: ['a', 'b'], page: 1 },
			{ q: 'suppe', tags: ['vegetarisch'], page: 4 }
		];
		for (const state of states) {
			expect(parseListQuery(urlWith(buildListQuery(state)))).toEqual(state);
		}
	});

	// What the overview's "did the URL change under me?" check rests on: the
	// query string built from live state has to match the one built from the
	// URL that state was written into, however untidily the user typed it.
	it('builds the same query string before and after a trip through the URL', () => {
		const states: ListQuery[] = [
			{ q: 'pasta ', tags: [], page: 1 },
			{ q: '', tags: ['Dessert'], page: 1 },
			{ q: ' suppe', tags: [' Vegetarisch ', ''], page: 2 }
		];
		for (const state of states) {
			const qs = buildListQuery(state);
			expect(buildListQuery(parseListQuery(urlWith(qs)))).toBe(qs);
		}
	});
});

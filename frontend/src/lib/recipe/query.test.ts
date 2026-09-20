import { describe, expect, it } from 'vitest';
import { buildListQuery, parseListQuery, type ListQuery } from './query';

function urlWith(qs: string): URL {
	return new URL(`https://rezepte.local/${qs ? `?${qs}` : ''}`);
}

describe('parseListQuery', () => {
	it('defaults to an empty query, no tags and page 1', () => {
		expect(parseListQuery(urlWith(''))).toEqual({
			q: '',
			tags: [],
			maxMinutes: 0,
			favourites: false,
			sort: 'updated',
			page: 1
		});
	});

	it('trims q', () => {
		expect(parseListQuery(urlWith('q=%20pasta%20'))).toEqual({
			q: 'pasta',
			tags: [],
			maxMinutes: 0,
			favourites: false,
			sort: 'updated',
			page: 1
		});
	});

	it('splits tags on commas and drops empty entries', () => {
		expect(parseListQuery(urlWith('tags=vegetarisch,schnell,'))).toEqual({
			q: '',
			tags: ['vegetarisch', 'schnell'],
			maxMinutes: 0,
			favourites: false,
			sort: 'updated',
			page: 1
		});
	});

	it('trims whitespace around each tag', () => {
		expect(parseListQuery(urlWith('tags=%20a%20,b'))).toEqual({
			q: '',
			tags: ['a', 'b'],
			maxMinutes: 0,
			favourites: false,
			sort: 'updated',
			page: 1
		});
	});

	it('lower-cases tags, since that is how they are stored', () => {
		expect(parseListQuery(urlWith('tags=Dessert,S%C3%9CSS'))).toEqual({
			q: '',
			tags: ['dessert', 'süss'],
			maxMinutes: 0,
			favourites: false,
			sort: 'updated',
			page: 1
		});
	});

	it('parses a valid page', () => {
		expect(parseListQuery(urlWith('page=3'))).toEqual({
			q: '',
			tags: [],
			maxMinutes: 0,
			favourites: false,
			sort: 'updated',
			page: 3
		});
	});

	it('falls back to page 1 for anything that is not a positive integer', () => {
		expect(parseListQuery(urlWith('page=0'))).toEqual({
			q: '',
			tags: [],
			maxMinutes: 0,
			favourites: false,
			sort: 'updated',
			page: 1
		});
		expect(parseListQuery(urlWith('page=-1'))).toEqual({
			q: '',
			tags: [],
			maxMinutes: 0,
			favourites: false,
			sort: 'updated',
			page: 1
		});
		expect(parseListQuery(urlWith('page=abc'))).toEqual({
			q: '',
			tags: [],
			maxMinutes: 0,
			favourites: false,
			sort: 'updated',
			page: 1
		});
		expect(parseListQuery(urlWith('page=2.5'))).toEqual({
			q: '',
			tags: [],
			maxMinutes: 0,
			favourites: false,
			sort: 'updated',
			page: 1
		});
	});

	it('parses maxMinutes', () => {
		expect(parseListQuery(urlWith('maxMinutes=30'))).toEqual({
			q: '',
			tags: [],
			maxMinutes: 30,
			favourites: false,
			sort: 'updated',
			page: 1
		});
	});

	it('drops a non-numeric or negative maxMinutes', () => {
		expect(parseListQuery(urlWith('maxMinutes=abc')).maxMinutes).toBe(0);
		expect(parseListQuery(urlWith('maxMinutes=-5')).maxMinutes).toBe(0);
	});

	it('parses favourites', () => {
		expect(parseListQuery(urlWith('favourites=true')).favourites).toBe(true);
	});

	it('treats anything but the literal "true" as favourites being off', () => {
		expect(parseListQuery(urlWith('favourites=false')).favourites).toBe(false);
		expect(parseListQuery(urlWith('favourites=1')).favourites).toBe(false);
		expect(parseListQuery(urlWith('')).favourites).toBe(false);
	});

	it('parses sort', () => {
		expect(parseListQuery(urlWith('sort=created'))).toEqual({
			q: '',
			tags: [],
			maxMinutes: 0,
			favourites: false,
			sort: 'created',
			page: 1
		});
		expect(parseListQuery(urlWith('sort=title')).sort).toBe('title');
	});

	it('falls back to updated for an unknown sort', () => {
		expect(parseListQuery(urlWith('sort=bogus')).sort).toBe('updated');
		// Shaped like an injection attempt, the same as the server-side test:
		// it must degrade to the default order, not throw.
		expect(parseListQuery(urlWith('sort=%3B%20DROP%20TABLE%20recipes')).sort).toBe('updated');
	});
});

describe('buildListQuery', () => {
	it('omits every field at its default', () => {
		expect(
			buildListQuery({
				q: '',
				tags: [],
				maxMinutes: 0,
				favourites: false,
				sort: 'updated',
				page: 1
			})
		).toBe('');
	});

	it('includes q when set', () => {
		expect(
			buildListQuery({
				q: 'pasta',
				tags: [],
				maxMinutes: 0,
				favourites: false,
				sort: 'updated',
				page: 1
			})
		).toBe('q=pasta');
	});

	it('joins tags with commas when set', () => {
		expect(
			buildListQuery({
				q: '',
				tags: ['a', 'b'],
				maxMinutes: 0,
				favourites: false,
				sort: 'updated',
				page: 1
			})
		).toBe('tags=a%2Cb');
	});

	it('includes maxMinutes when set', () => {
		expect(
			buildListQuery({
				q: '',
				tags: [],
				maxMinutes: 30,
				favourites: false,
				sort: 'updated',
				page: 1
			})
		).toBe('maxMinutes=30');
	});

	it('omits maxMinutes at its default', () => {
		expect(
			buildListQuery({
				q: '',
				tags: [],
				maxMinutes: 0,
				favourites: false,
				sort: 'updated',
				page: 1
			})
		).toBe('');
	});

	it('includes favourites when set', () => {
		expect(
			buildListQuery({ q: '', tags: [], maxMinutes: 0, favourites: true, sort: 'updated', page: 1 })
		).toBe('favourites=true');
	});

	it('omits favourites at its default', () => {
		expect(
			buildListQuery({
				q: '',
				tags: [],
				maxMinutes: 0,
				favourites: false,
				sort: 'updated',
				page: 1
			})
		).toBe('');
	});

	it('includes sort when set', () => {
		expect(
			buildListQuery({ q: '', tags: [], maxMinutes: 0, favourites: false, sort: 'title', page: 1 })
		).toBe('sort=title');
	});

	it('omits sort at its default', () => {
		expect(
			buildListQuery({
				q: '',
				tags: [],
				maxMinutes: 0,
				favourites: false,
				sort: 'updated',
				page: 1
			})
		).toBe('');
	});

	it('includes page when it is not 1', () => {
		expect(
			buildListQuery({
				q: '',
				tags: [],
				maxMinutes: 0,
				favourites: false,
				sort: 'updated',
				page: 2
			})
		).toBe('page=2');
	});

	it('orders q, tags, maxMinutes, favourites, sort, page stably when all are set', () => {
		expect(
			buildListQuery({
				q: 'pasta',
				tags: ['a', 'b'],
				maxMinutes: 30,
				favourites: true,
				sort: 'created',
				page: 3
			})
		).toBe('q=pasta&tags=a%2Cb&maxMinutes=30&favourites=true&sort=created&page=3');
	});

	it('normalises the state it is handed, like parseListQuery does', () => {
		expect(
			buildListQuery({
				q: '  pasta ',
				tags: [' Dessert', '', 'B'],
				maxMinutes: -5,
				favourites: false,
				sort: 'updated',
				page: 0
			})
		).toBe('q=pasta&tags=dessert%2Cb');
	});
});

describe('round trip', () => {
	it('parseListQuery(url with buildListQuery(state)) recovers the state', () => {
		const states: ListQuery[] = [
			{ q: '', tags: [], maxMinutes: 0, favourites: false, sort: 'updated', page: 1 },
			{ q: 'pasta', tags: [], maxMinutes: 0, favourites: false, sort: 'updated', page: 1 },
			{ q: '', tags: ['a', 'b'], maxMinutes: 0, favourites: false, sort: 'updated', page: 1 },
			{
				q: 'suppe',
				tags: ['vegetarisch'],
				maxMinutes: 30,
				favourites: false,
				sort: 'updated',
				page: 4
			},
			{ q: '', tags: [], maxMinutes: 0, favourites: false, sort: 'title', page: 1 },
			{ q: '', tags: [], maxMinutes: 0, favourites: true, sort: 'updated', page: 1 }
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
			{ q: 'pasta ', tags: [], maxMinutes: 0, favourites: false, sort: 'updated', page: 1 },
			{ q: '', tags: ['Dessert'], maxMinutes: 0, favourites: false, sort: 'updated', page: 1 },
			{
				q: ' suppe',
				tags: [' Vegetarisch ', ''],
				maxMinutes: 45,
				favourites: false,
				sort: 'created',
				page: 2
			},
			{ q: '', tags: [], maxMinutes: 0, favourites: true, sort: 'updated', page: 1 }
		];
		for (const state of states) {
			const qs = buildListQuery(state);
			expect(buildListQuery(parseListQuery(urlWith(qs)))).toBe(qs);
		}
	});
});

import { describe, expect, it } from 'vitest';
import { buildListQuery, MAX_CHIPS, parseListQuery, type ListQuery } from './query';

function urlWith(qs: string): URL {
	return new URL(`https://rezepte.local/${qs ? `?${qs}` : ''}`);
}

describe('parseListQuery', () => {
	it('defaults to an empty query, no tags and page 1', () => {
		expect(parseListQuery(urlWith(''))).toEqual({
			q: '',
			tags: [],
			ingredients: [],
			have: [],
			maxMinutes: 0,
			favorites: false,
			author: '',
			sort: 'updated',
			page: 1
		});
	});

	it('trims q', () => {
		expect(parseListQuery(urlWith('q=%20pasta%20'))).toEqual({
			q: 'pasta',
			tags: [],
			ingredients: [],
			have: [],
			maxMinutes: 0,
			favorites: false,
			author: '',
			sort: 'updated',
			page: 1
		});
	});

	it('splits tags on commas and drops empty entries', () => {
		expect(parseListQuery(urlWith('tags=vegetarisch,schnell,'))).toEqual({
			q: '',
			tags: ['vegetarisch', 'schnell'],
			ingredients: [],
			have: [],
			maxMinutes: 0,
			favorites: false,
			author: '',
			sort: 'updated',
			page: 1
		});
	});

	it('trims whitespace around each tag', () => {
		expect(parseListQuery(urlWith('tags=%20a%20,b'))).toEqual({
			q: '',
			tags: ['a', 'b'],
			ingredients: [],
			have: [],
			maxMinutes: 0,
			favorites: false,
			author: '',
			sort: 'updated',
			page: 1
		});
	});

	it('lower-cases tags, since that is how they are stored', () => {
		expect(parseListQuery(urlWith('tags=Dessert,S%C3%9CSS'))).toEqual({
			q: '',
			tags: ['dessert', 'süss'],
			ingredients: [],
			have: [],
			maxMinutes: 0,
			favorites: false,
			author: '',
			sort: 'updated',
			page: 1
		});
	});

	it('parses a valid page', () => {
		expect(parseListQuery(urlWith('page=3'))).toEqual({
			q: '',
			tags: [],
			ingredients: [],
			have: [],
			maxMinutes: 0,
			favorites: false,
			author: '',
			sort: 'updated',
			page: 3
		});
	});

	it('falls back to page 1 for anything that is not a positive integer', () => {
		expect(parseListQuery(urlWith('page=0'))).toEqual({
			q: '',
			tags: [],
			ingredients: [],
			have: [],
			maxMinutes: 0,
			favorites: false,
			author: '',
			sort: 'updated',
			page: 1
		});
		expect(parseListQuery(urlWith('page=-1'))).toEqual({
			q: '',
			tags: [],
			ingredients: [],
			have: [],
			maxMinutes: 0,
			favorites: false,
			author: '',
			sort: 'updated',
			page: 1
		});
		expect(parseListQuery(urlWith('page=abc'))).toEqual({
			q: '',
			tags: [],
			ingredients: [],
			have: [],
			maxMinutes: 0,
			favorites: false,
			author: '',
			sort: 'updated',
			page: 1
		});
		expect(parseListQuery(urlWith('page=2.5'))).toEqual({
			q: '',
			tags: [],
			ingredients: [],
			have: [],
			maxMinutes: 0,
			favorites: false,
			author: '',
			sort: 'updated',
			page: 1
		});
	});

	it('parses maxMinutes', () => {
		expect(parseListQuery(urlWith('maxMinutes=30'))).toEqual({
			q: '',
			tags: [],
			ingredients: [],
			have: [],
			maxMinutes: 30,
			favorites: false,
			author: '',
			sort: 'updated',
			page: 1
		});
	});

	it('drops a non-numeric or negative maxMinutes', () => {
		expect(parseListQuery(urlWith('maxMinutes=abc')).maxMinutes).toBe(0);
		expect(parseListQuery(urlWith('maxMinutes=-5')).maxMinutes).toBe(0);
	});

	it('parses favorites', () => {
		expect(parseListQuery(urlWith('favorites=true')).favorites).toBe(true);
	});

	it('treats anything but the literal "true" as favorites being off', () => {
		expect(parseListQuery(urlWith('favorites=false')).favorites).toBe(false);
		expect(parseListQuery(urlWith('favorites=1')).favorites).toBe(false);
		expect(parseListQuery(urlWith('')).favorites).toBe(false);
	});

	it('parses sort', () => {
		expect(parseListQuery(urlWith('sort=created'))).toEqual({
			q: '',
			tags: [],
			ingredients: [],
			have: [],
			maxMinutes: 0,
			favorites: false,
			author: '',
			sort: 'created',
			page: 1
		});
		expect(parseListQuery(urlWith('sort=title')).sort).toBe('title');
		expect(parseListQuery(urlWith('sort=tasty')).sort).toBe('tasty');
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
				ingredients: [],
				have: [],
				maxMinutes: 0,
				favorites: false,
				author: '',
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
				ingredients: [],
				have: [],
				maxMinutes: 0,
				favorites: false,
				author: '',
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
				ingredients: [],
				have: [],
				maxMinutes: 0,
				favorites: false,
				author: '',
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
				ingredients: [],
				have: [],
				maxMinutes: 30,
				favorites: false,
				author: '',
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
				ingredients: [],
				have: [],
				maxMinutes: 0,
				favorites: false,
				author: '',
				sort: 'updated',
				page: 1
			})
		).toBe('');
	});

	it('includes favorites when set', () => {
		expect(
			buildListQuery({
				q: '',
				tags: [],
				ingredients: [],
				have: [],
				maxMinutes: 0,
				favorites: true,
				author: '',
				sort: 'updated',
				page: 1
			})
		).toBe('favorites=true');
	});

	it('omits favorites at its default', () => {
		expect(
			buildListQuery({
				q: '',
				tags: [],
				ingredients: [],
				have: [],
				maxMinutes: 0,
				favorites: false,
				author: '',
				sort: 'updated',
				page: 1
			})
		).toBe('');
	});

	it('includes sort when set', () => {
		expect(
			buildListQuery({
				q: '',
				tags: [],
				ingredients: [],
				have: [],
				maxMinutes: 0,
				favorites: false,
				author: '',
				sort: 'title',
				page: 1
			})
		).toBe('sort=title');
	});

	it('omits sort at its default', () => {
		expect(
			buildListQuery({
				q: '',
				tags: [],
				ingredients: [],
				have: [],
				maxMinutes: 0,
				favorites: false,
				author: '',
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
				ingredients: [],
				have: [],
				maxMinutes: 0,
				favorites: false,
				author: '',
				sort: 'updated',
				page: 2
			})
		).toBe('page=2');
	});

	it('orders q, tags, maxMinutes, favorites, author, sort, page stably when all are set', () => {
		expect(
			buildListQuery({
				q: 'pasta',
				tags: ['a', 'b'],
				ingredients: [],
				have: [],
				maxMinutes: 30,
				favorites: true,
				author: 'mara',
				sort: 'created',
				page: 3
			})
		).toBe('q=pasta&tags=a%2Cb&maxMinutes=30&favorites=true&author=mara&sort=created&page=3');
	});

	it('normalizes the state it is handed, like parseListQuery does', () => {
		expect(
			buildListQuery({
				q: '  pasta ',
				tags: [' Dessert', '', 'B'],
				ingredients: [],
				have: [],
				maxMinutes: -5,
				favorites: false,
				author: '',
				sort: 'updated',
				page: 0
			})
		).toBe('q=pasta&tags=dessert%2Cb');
	});
});

describe('round trip', () => {
	it('parseListQuery(url with buildListQuery(state)) recovers the state', () => {
		const states: ListQuery[] = [
			{
				q: '',
				tags: [],
				ingredients: [],
				have: [],
				maxMinutes: 0,
				favorites: false,
				author: '',
				sort: 'updated',
				page: 1
			},
			{
				q: 'pasta',
				tags: [],
				ingredients: [],
				have: [],
				maxMinutes: 0,
				favorites: false,
				author: '',
				sort: 'updated',
				page: 1
			},
			{
				q: '',
				tags: ['a', 'b'],
				ingredients: [],
				have: [],
				maxMinutes: 0,
				favorites: false,
				author: '',
				sort: 'updated',
				page: 1
			},
			{
				q: 'suppe',
				tags: ['vegetarisch'],
				ingredients: [],
				have: [],
				maxMinutes: 30,
				favorites: false,
				author: '',
				sort: 'updated',
				page: 4
			},
			{
				q: '',
				tags: [],
				ingredients: [],
				have: [],
				maxMinutes: 0,
				favorites: false,
				author: '',
				sort: 'title',
				page: 1
			},
			{
				q: '',
				tags: [],
				ingredients: [],
				have: [],
				maxMinutes: 0,
				favorites: true,
				author: '',
				sort: 'updated',
				page: 1
			}
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
			{
				q: 'pasta ',
				tags: [],
				ingredients: [],
				have: [],
				maxMinutes: 0,
				favorites: false,
				author: '',
				sort: 'updated',
				page: 1
			},
			{
				q: '',
				tags: ['Dessert'],
				ingredients: [],
				have: [],
				maxMinutes: 0,
				favorites: false,
				author: '',
				sort: 'updated',
				page: 1
			},
			{
				q: ' suppe',
				tags: [' Vegetarisch ', ''],
				ingredients: [],
				have: [],
				maxMinutes: 45,
				favorites: false,
				author: '',
				sort: 'created',
				page: 2
			},
			{
				q: '',
				tags: [],
				ingredients: [],
				have: [],
				maxMinutes: 0,
				favorites: true,
				author: '',
				sort: 'updated',
				page: 1
			}
		];
		for (const state of states) {
			const qs = buildListQuery(state);
			expect(buildListQuery(parseListQuery(urlWith(qs)))).toBe(qs);
		}
	});
});

describe('the author filter', () => {
	const base: ListQuery = {
		q: '',
		tags: [],
		ingredients: [],
		have: [],
		maxMinutes: 0,
		favorites: false,
		author: '',
		sort: 'updated',
		page: 1
	};

	it('round-trips through the URL', () => {
		expect(parseListQuery(urlWith('author=mara')).author).toBe('mara');
		expect(buildListQuery({ ...base, author: 'mara' })).toBe('author=mara');
	});

	it('is omitted from the query string when empty', () => {
		expect(buildListQuery(base)).toBe('');
	});

	// Usernames keep their case (the service only trims them), so unlike a
	// tag this must not be lower-cased - "Mara" would stop matching.
	it('trims but keeps the case', () => {
		expect(parseListQuery(urlWith('author=%20Mara%20')).author).toBe('Mara');
	});
});

describe('ingredients and have', () => {
	it('round-trip through the URL as one parameter per name, names as typed', () => {
		const url = urlWith('ingredients=Gurke&ingredients=Ei&have=Salz');
		expect(parseListQuery(url).ingredients).toEqual(['Gurke', 'Ei']);
		expect(parseListQuery(url).have).toEqual(['Salz']);
		expect(buildListQuery(parseListQuery(url))).toBe('ingredients=Gurke&ingredients=Ei&have=Salz');
	});

	it('keep a comma inside a name', () => {
		const state = parseListQuery(urlWith('ingredients=Gurke'));
		const qs = buildListQuery({ ...state, ingredients: ['Milch 1,5 %'], have: ['Salz, grob'] });
		const back = parseListQuery(urlWith(qs));
		expect(back.ingredients).toEqual(['Milch 1,5 %']);
		expect(back.have).toEqual(['Salz, grob']);
	});

	it('trim names and drop blanks', () => {
		const q = parseListQuery(
			urlWith('ingredients=%20Gurke%20&ingredients=&ingredients=%20&have=&have=Salz')
		);
		expect(q.ingredients).toEqual(['Gurke']);
		expect(q.have).toEqual(['Salz']);
	});

	it('drop case-insensitive duplicates', () => {
		const q = parseListQuery(urlWith('ingredients=Gurke&ingredients=gurke&have=Salz&have=SALZ'));
		expect(q.ingredients).toEqual(['Gurke']);
		expect(q.have).toEqual(['Salz']);
	});

	// The server keeps one chip per ingredient key; two here would read
	// "1 of 2" on a recipe that uses both.
	it('drop repeats by ingredient key, as the server does', () => {
		const q = parseListQuery(
			urlWith('ingredients=Zwiebel&ingredients=Zwiebeln&have=Kartoffeln&have=Zwiebeln')
		);
		expect(q.ingredients).toEqual(['Zwiebel']);
		expect(q.have).toEqual(['Kartoffeln']);
	});

	it("stop the chips at the server's cap", () => {
		const many = Array.from({ length: 25 }, (_, i) => `ingredients=Zutat${i}x`).join('&');
		expect(parseListQuery(urlWith(many)).ingredients).toHaveLength(MAX_CHIPS);
	});

	it('keep a name in both lists only in ingredients', () => {
		const q = parseListQuery(urlWith('ingredients=Gurke&have=gurke&have=Salz'));
		expect(q.ingredients).toEqual(['Gurke']);
		expect(q.have).toEqual(['Salz']);
	});
});

import { describe, expect, it } from 'vitest';
import { rankTags, tagSuggestions, tagToCommit } from './tag-suggestions';

const known = ['dessert', 'nudeln', 'nudelauflauf', 'schnell'];

describe('tagSuggestions', () => {
	it('lists the matches first and creating last, preferring the first match', () => {
		const { options, preferred } = tagSuggestions({ query: 'sch', known, selected: [] });
		expect(options).toEqual([
			{ name: 'schnell', isNew: false },
			{ name: 'sch', isNew: true }
		]);
		expect(preferred).toBe(0);
	});

	it('prefers creating only when no existing tag matches', () => {
		const { options, preferred } = tagSuggestions({ query: 'sommer', known, selected: [] });
		expect(options).toEqual([{ name: 'sommer', isNew: true }]);
		expect(preferred).toBe(0);
	});

	it('prefers nothing while the query is blank', () => {
		expect(tagSuggestions({ query: ' ', known, selected: [] }).preferred).toBe(-1);
	});

	it('offers no creation row for an exact match, which is preferred', () => {
		const { options, preferred } = tagSuggestions({
			query: 'nudel',
			known: ['nudelsalat', 'nudel'],
			selected: []
		});
		expect(options.map((option) => option.isNew)).toEqual([false, false]);
		expect(options[preferred].name).toBe('nudel');
	});

	it('offers every known tag while the query is empty', () => {
		expect(tagSuggestions({ query: '', known, selected: [] })).toMatchObject({
			matches: known,
			create: null
		});
	});

	it('filters by the query', () => {
		expect(tagSuggestions({ query: 'nud', known, selected: [] })).toMatchObject({
			matches: ['nudeln', 'nudelauflauf'],
			create: 'nud'
		});
	});

	it('lists tags containing the query after the ones starting with it', () => {
		expect(
			tagSuggestions({ query: 'auf', known: ['auflauf', ...known], selected: [] })
		).toMatchObject({
			matches: ['auflauf', 'nudelauflauf'],
			create: 'auf'
		});
	});

	it('puts the tag the query names exactly first', () => {
		expect(
			tagSuggestions({ query: 'nudel', known: ['nudelsalat', 'nudel'], selected: [] }).matches
		).toEqual(['nudel', 'nudelsalat']);
	});

	it('drops tags already on the recipe', () => {
		expect(tagSuggestions({ query: '', known, selected: ['dessert'] }).matches).toEqual([
			'nudeln',
			'nudelauflauf',
			'schnell'
		]);
	});

	it('suppresses the creation row when the query matches a known tag exactly', () => {
		expect(tagSuggestions({ query: 'nudeln', known, selected: [] })).toMatchObject({
			matches: ['nudeln'],
			create: null
		});
	});

	it('offers creation for a query that is only a prefix of a known tag', () => {
		expect(tagSuggestions({ query: 'nudel', known, selected: [] }).create).toBe('nudel');
	});

	it('normalizes the query before comparing', () => {
		expect(tagSuggestions({ query: '  NUDELN ', known, selected: [] })).toMatchObject({
			matches: ['nudeln'],
			create: null
		});
	});

	it('suppresses the creation row when the tag is already selected', () => {
		expect(tagSuggestions({ query: 'dessert', known, selected: ['dessert'] })).toMatchObject({
			matches: [],
			create: null
		});
	});

	it('offers nothing to create for a blank query', () => {
		expect(tagSuggestions({ query: '   ', known, selected: [] }).create).toBeNull();
	});

	it('treats a selected tag that is not in the known list as existing', () => {
		// A tag created earlier in this editing session sits on the recipe
		// but is absent from the list fetched when the editor mounted.
		expect(tagSuggestions({ query: 'eigenbau', known, selected: [' Eigenbau '] })).toMatchObject({
			matches: [],
			create: null
		});
	});

	it('normalizes known and selected entries instead of trusting them', () => {
		expect(
			tagSuggestions({ query: 'des', known: ['Dessert', 'SÜSS'], selected: [' süss '] })
		).toMatchObject({ matches: ['dessert'], create: 'des' });
	});
});

describe('tagToCommit', () => {
	const { options } = tagSuggestions({ query: 'sch', known, selected: [] });

	it('takes the highlighted row on Enter and on a comma', () => {
		expect(tagToCommit('enter', options, 0)).toBe('schnell');
		expect(tagToCommit('comma', options, 1)).toBe('sch');
	});

	it('creates only when the creation row is the one highlighted', () => {
		expect(options[1].isNew).toBe(true);
		expect(tagToCommit('enter', options, 0)).not.toBe('sch');
	});

	it('commits nothing on Tab or blur, whatever is highlighted', () => {
		expect(tagToCommit('tab', options, 0)).toBeNull();
		expect(tagToCommit('blur', options, 1)).toBeNull();
	});

	it('commits nothing with no row highlighted', () => {
		expect(tagToCommit('enter', options, -1)).toBeNull();
		expect(tagToCommit('enter', [], 0)).toBeNull();
	});
});

describe('rankTags', () => {
	// Most used first, the order listTags() returns.
	const tags = [
		{ name: 'fleisch' },
		{ name: 'schnell' },
		{ name: 'dessert' },
		{ name: 'schmoren' }
	];

	it('keeps every tag in the given order while the query is blank', () => {
		expect(rankTags(tags, '  ')).toEqual(tags);
	});

	it('puts tags starting with the query before tags containing it', () => {
		expect(rankTags(tags, 'SCH').map((tag) => tag.name)).toEqual([
			'schnell',
			'schmoren',
			'fleisch'
		]);
	});
});

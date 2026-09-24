import { describe, expect, it } from 'vitest';
import { tagSuggestions } from './tag-suggestions';

const known = ['dessert', 'nudeln', 'nudelauflauf', 'schnell'];

describe('tagSuggestions', () => {
	it('offers every known tag while the query is empty', () => {
		expect(tagSuggestions({ query: '', known, selected: [] })).toEqual({
			matches: known,
			create: null
		});
	});

	it('filters by prefix', () => {
		expect(tagSuggestions({ query: 'nud', known, selected: [] })).toEqual({
			matches: ['nudeln', 'nudelauflauf'],
			create: 'nud'
		});
	});

	it('drops tags already on the recipe', () => {
		expect(tagSuggestions({ query: '', known, selected: ['dessert'] }).matches).toEqual([
			'nudeln',
			'nudelauflauf',
			'schnell'
		]);
	});

	it('suppresses the creation row when the query matches a known tag exactly', () => {
		expect(tagSuggestions({ query: 'nudeln', known, selected: [] })).toEqual({
			matches: ['nudeln'],
			create: null
		});
	});

	it('offers creation for a query that is only a prefix of a known tag', () => {
		expect(tagSuggestions({ query: 'nudel', known, selected: [] }).create).toBe('nudel');
	});

	it('normalizes the query before comparing', () => {
		expect(tagSuggestions({ query: '  NUDELN ', known, selected: [] })).toEqual({
			matches: ['nudeln'],
			create: null
		});
	});

	it('suppresses the creation row when the tag is already selected', () => {
		expect(tagSuggestions({ query: 'dessert', known, selected: ['dessert'] })).toEqual({
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
		expect(tagSuggestions({ query: 'eigenbau', known, selected: [' Eigenbau '] })).toEqual({
			matches: [],
			create: null
		});
	});

	it('normalizes known and selected entries instead of trusting them', () => {
		expect(
			tagSuggestions({ query: 'des', known: ['Dessert', 'SÜSS'], selected: [' süss '] })
		).toEqual({ matches: ['dessert'], create: 'des' });
	});
});

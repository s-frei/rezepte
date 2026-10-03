import { expect, it } from 'vitest';
import { acceptSuggestion } from './tag-suggestions-import';

it('accepting moves a suggestion into the tags', () => {
	expect(acceptSuggestion(['a'], ['B', 'c'], 'B', 20)).toEqual({
		tags: ['a', 'B'],
		suggestions: ['c']
	});
});

it('accepting at the limit changes nothing', () => {
	const full = Array.from({ length: 20 }, (_, i) => `t${i}`);
	expect(acceptSuggestion(full, ['x'], 'x', 20)).toEqual({ tags: full, suggestions: ['x'] });
});

it('accepting a tag that is already set only removes the suggestion', () => {
	expect(acceptSuggestion(['käse'], ['Käse'], 'Käse', 20)).toEqual({
		tags: ['käse'],
		suggestions: []
	});
});

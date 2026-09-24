import { describe, expect, it } from 'vitest';
import { belongsToRecipe, ingredientKey, parseCheckedKeys, serializeCheckedKeys } from './checked';

describe('ingredientKey', () => {
	it('joins recipe id, group index and ingredient index with colons', () => {
		expect(ingredientKey('recipe-1', 0, 2)).toBe('recipe-1:0:2');
	});
});

describe('belongsToRecipe', () => {
	it('matches a key built for the same recipe', () => {
		expect(belongsToRecipe(ingredientKey('recipe-1', 1, 0), 'recipe-1')).toBe(true);
	});

	it('does not match a key from a different recipe', () => {
		expect(belongsToRecipe(ingredientKey('recipe-2', 1, 0), 'recipe-1')).toBe(false);
	});

	it('does not false-match when one recipe id is a prefix of another', () => {
		// "abcd:0:0".startsWith("abc") is true, but this key does not belong
		// to recipe "abc" - the colon boundary must be respected.
		expect(belongsToRecipe(ingredientKey('abcd', 0, 0), 'abc')).toBe(false);
	});
});

describe('parseCheckedKeys', () => {
	it('returns an empty array for null', () => {
		expect(parseCheckedKeys(null)).toEqual([]);
	});

	it('returns an empty array for an empty string', () => {
		expect(parseCheckedKeys('')).toEqual([]);
	});

	it('returns an empty array for invalid JSON', () => {
		expect(parseCheckedKeys('not json')).toEqual([]);
	});

	it('returns an empty array for JSON that is not an array', () => {
		expect(parseCheckedKeys('{"a":1}')).toEqual([]);
	});

	it('drops non-string entries from an otherwise valid array', () => {
		expect(parseCheckedKeys('["a", 1, null, "b"]')).toEqual(['a', 'b']);
	});

	it('parses a valid array of keys', () => {
		expect(parseCheckedKeys('["recipe-1:0:0","recipe-1:0:1"]')).toEqual([
			'recipe-1:0:0',
			'recipe-1:0:1'
		]);
	});
});

describe('serializeCheckedKeys', () => {
	it('serializes an empty iterable to an empty JSON array', () => {
		expect(serializeCheckedKeys([])).toBe('[]');
	});

	it('serializes a Set in iteration order', () => {
		expect(serializeCheckedKeys(new Set(['a', 'b']))).toBe('["a","b"]');
	});
});

describe('round trip', () => {
	it('parseCheckedKeys(serializeCheckedKeys(keys)) recovers the keys', () => {
		const keys = [ingredientKey('recipe-1', 0, 0), ingredientKey('recipe-1', 1, 2)];
		expect(parseCheckedKeys(serializeCheckedKeys(keys))).toEqual(keys);
	});
});

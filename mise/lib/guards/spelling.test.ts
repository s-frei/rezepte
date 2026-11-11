import { expect, test } from 'bun:test';
import { excluded, hits } from './spelling';

test('finds British spellings as whole words, any case', () => {
	expect(hits('a Colour\nthe user cancelled it, greyed out')).toEqual([
		{ line: 1, word: 'Colour' },
		{ line: 2, word: 'cancelled' },
		{ line: 2, word: 'greyed' }
	]);
	expect(hits('cancelled_at, 250 millilitres, Centring').map((h) => h.word)).toEqual(['cancelled', 'millilitres', 'Centring']);
});

test('leaves American spellings and other words alone', () => {
	expect(hits('color favorite center normalize behavior gray catalog canceled labeled liter meter analyze')).toEqual([]);
	expect(hits('greyhound, decolourize, analyses, centered')).toEqual([]);
});

test('excludes German data, the language page and lockfiles', () => {
	expect(excluded('frontend/messages/de.json')).toBe(true);
	expect(excluded('service/internal/recipe/testdata/recipes.de.json')).toBe(true);
	expect(excluded('docs/memory/content/conventions/language.mdx')).toBe(true);
	expect(excluded('docs/user/bun.lock')).toBe(true);
	expect(excluded('frontend/messages/en.json')).toBe(false);
	expect(excluded('assets/brand/mascot/prompts.md')).toBe(false);
});

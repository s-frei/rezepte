import { describe, expect, it } from 'vitest';
import { tintFor } from './placeholder';

describe('tintFor', () => {
	it('is stable for the same id', () => {
		expect(tintFor('abc')).toBe(tintFor('abc'));
		expect(tintFor('11111111-1111-1111-1111-111111111111')).toBe(
			tintFor('11111111-1111-1111-1111-111111111111')
		);
	});

	it('always returns one of the three tint tokens', () => {
		const ids = ['', 'a', 'recipe-1', '11111111-1111-1111-1111-111111111111'];
		for (const id of ids) {
			expect(['tint-1', 'tint-2', 'tint-3']).toContain(tintFor(id));
		}
	});

	it('reaches all three tints across a spread of ids', () => {
		const seen = new Set(Array.from({ length: 30 }, (_, i) => tintFor(`recipe-${i}`)));
		expect(seen).toEqual(new Set(['tint-1', 'tint-2', 'tint-3']));
	});
});

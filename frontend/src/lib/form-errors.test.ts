import { describe, expect, it } from 'vitest';
import { withoutErrors } from './form-errors';

describe('withoutErrors', () => {
	it('drops the named keys and keeps the rest', () => {
		const errors = { title: 'a', servings: 'b', sourceUrl: 'c' };
		expect(withoutErrors(errors, ['title', 'servings'])).toEqual({ sourceUrl: 'c' });
	});

	it('hands back the same object when none of the keys has an error', () => {
		const errors: { title?: string; servings?: string } = { title: 'a' };
		expect(withoutErrors(errors, ['servings'])).toBe(errors);
	});
});

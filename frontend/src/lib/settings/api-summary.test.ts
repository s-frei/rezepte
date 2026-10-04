import { describe, expect, it } from 'vitest';
import { countOperations } from './api-summary';

describe('countOperations', () => {
	it('counts one operation per method across paths', () => {
		expect(
			countOperations({
				paths: {
					'/recipes': { get: { tags: ['recipes'] }, post: { tags: ['recipes'] } },
					'/recipes/{id}': { get: { tags: ['recipes'] }, delete: { tags: ['recipes'] } }
				}
			})
		).toBe(4);
	});

	// A path item may carry keys that are not operations. OpenAPI puts shared
	// parameters and a summary right next to the methods, and counting those
	// would inflate the figure the page shows.
	it('ignores path-item keys that are not HTTP methods', () => {
		expect(
			countOperations({
				paths: {
					'/recipes': {
						summary: 'Recipes',
						parameters: [{ name: 'page', in: 'query' }],
						get: { tags: ['recipes'] }
					}
				}
			})
		).toBe(1);
	});

	it('reads an empty document as zero', () => {
		expect(countOperations({})).toBe(0);
	});
});

import { describe, expect, it } from 'vitest';
import { summarizeSpec } from './api-summary';

describe('summarizeSpec', () => {
	it('counts one operation per method across paths', () => {
		const summary = summarizeSpec({
			paths: {
				'/recipes': { get: { tags: ['recipes'] }, post: { tags: ['recipes'] } },
				'/recipes/{id}': { get: { tags: ['recipes'] }, delete: { tags: ['recipes'] } }
			}
		});
		expect(summary.operations).toBe(4);
	});

	// A path item may carry keys that are not operations. OpenAPI puts shared
	// parameters and a summary right next to the methods, and counting those
	// would inflate the figure the card shows.
	it('ignores path-item keys that are not HTTP methods', () => {
		const summary = summarizeSpec({
			paths: {
				'/recipes': {
					summary: 'Recipes',
					parameters: [{ name: 'page', in: 'query' }],
					get: { tags: ['recipes'] }
				}
			}
		});
		expect(summary.operations).toBe(1);
	});

	it('counts each tag once, however many operations carry it', () => {
		const summary = summarizeSpec({
			paths: {
				'/recipes': { get: { tags: ['recipes'] }, post: { tags: ['recipes'] } },
				'/tokens': { get: { tags: ['tokens'] } }
			}
		});
		expect(summary.areas).toBe(2);
	});

	it('counts both tags of an operation that carries two', () => {
		const summary = summarizeSpec({
			paths: { '/auth/me': { get: { tags: ['auth', 'users'] } } }
		});
		expect(summary.areas).toBe(2);
	});

	// An untagged operation still exists; it just belongs to no area. Counting
	// it as an area would invent one, dropping it from the operations would
	// undercount the API.
	it('counts an untagged operation without inventing an area', () => {
		const summary = summarizeSpec({ paths: { '/healthz': { get: {} } } });
		expect(summary).toEqual({ operations: 1, areas: 0 });
	});

	it('reads an empty document as zero of both', () => {
		expect(summarizeSpec({})).toEqual({ operations: 0, areas: 0 });
	});
});

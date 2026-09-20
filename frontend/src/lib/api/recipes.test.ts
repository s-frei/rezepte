import { afterEach, describe, expect, it, vi } from 'vitest';
import { listRecipes } from './recipes';

function mockFetch(body: unknown) {
	const fn = vi.fn(
		async () =>
			new Response(JSON.stringify(body), {
				status: 200,
				headers: { 'Content-Type': 'application/json' }
			})
	);
	vi.stubGlobal('fetch', fn);
	return fn;
}

afterEach(() => vi.unstubAllGlobals());

describe('listRecipes', () => {
	it('builds the query string from the given params', async () => {
		const fn = mockFetch({ items: [], page: 1, limit: 24, total: 0 });
		await listRecipes({ q: 'soup', tags: ['vegan', 'quick'], page: 2, limit: 10 });
		const [url] = fn.mock.calls[0] as unknown as [string];
		expect(url).toBe('/api/v1/recipes?q=soup&tags=vegan%2Cquick&page=2&limit=10');
	});

	it('omits unset params', async () => {
		const fn = mockFetch({ items: [], page: 1, limit: 24, total: 0 });
		await listRecipes();
		const [url] = fn.mock.calls[0] as unknown as [string];
		expect(url).toBe('/api/v1/recipes');
	});

	it('forwards init (e.g. an AbortSignal) to fetch', async () => {
		const fn = mockFetch({ items: [], page: 1, limit: 24, total: 0 });
		const controller = new AbortController();
		await listRecipes({ q: 'soup' }, { signal: controller.signal });
		const [, init] = fn.mock.calls[0] as unknown as [string, RequestInit];
		expect(init.signal).toBe(controller.signal);
	});
});

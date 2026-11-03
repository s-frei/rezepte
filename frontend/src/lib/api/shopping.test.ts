import { afterEach, describe, expect, it, vi } from 'vitest';
import { addRecipeToList, listShoppingLists, setDepartment, setItemsChecked } from './shopping';

function mockFetch(body: unknown, status = 200) {
	const fn = vi.fn(
		async () =>
			new Response(status === 204 ? null : JSON.stringify(body), {
				status,
				headers: { 'Content-Type': 'application/json' }
			})
	);
	vi.stubGlobal('fetch', fn);
	return fn;
}

afterEach(() => vi.unstubAllGlobals());

describe('shopping api', () => {
	it('returns the lists and the invitations', async () => {
		mockFetch({ items: [{ id: 'a' }], invitations: [] });
		expect(await listShoppingLists()).toEqual({ items: [{ id: 'a' }], invitations: [] });
	});

	it('posts recipe ingredients', async () => {
		const fn = mockFetch({ id: 'l' });
		await addRecipeToList('l', {
			recipeId: 'r',
			servings: 2,
			ingredients: [{ group: 0, index: 0 }]
		});
		const [url, init] = fn.mock.calls[0] as unknown as [string, RequestInit];
		expect(url).toBe('/api/v1/shopping-lists/l/recipes');
		expect(init.method).toBe('POST');
		expect(JSON.parse(init.body as string)).toEqual({
			recipeId: 'r',
			servings: 2,
			ingredients: [{ group: 0, index: 0 }]
		});
	});

	it('checks items', async () => {
		const fn = mockFetch({ id: 'l' });
		await setItemsChecked('l', ['i'], true);
		const [url, init] = fn.mock.calls[0] as unknown as [string, RequestInit];
		expect(url).toBe('/api/v1/shopping-lists/l/items/check');
		expect(JSON.parse(init.body as string)).toEqual({ itemIds: ['i'], checked: true });
	});

	it('puts a department override', async () => {
		const fn = mockFetch(null, 204);
		await setDepartment('Zwiebel', 'produce');
		const [url, init] = fn.mock.calls[0] as unknown as [string, RequestInit];
		expect(url).toBe('/api/v1/shopping-departments');
		expect(init.method).toBe('PUT');
	});
});

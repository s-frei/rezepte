import { afterEach, describe, expect, it, vi } from 'vitest';
import { createToken, deleteToken, listTokens } from './tokens';

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

describe('listTokens', () => {
	it('unwraps the items array', async () => {
		mockFetch({ items: [{ id: 'a', name: 'mcp' }] });
		const tokens = await listTokens();
		expect(tokens).toHaveLength(1);
		expect(tokens[0].name).toBe('mcp');
	});
});

describe('createToken', () => {
	it('sends the scopes and the expiry in days', async () => {
		const fn = mockFetch({ id: 'a', token: 'rzp_x' }, 201);
		await createToken({ name: 'mcp', scopes: ['recipes:read'], expiresInDays: 90 });
		const [url, init] = fn.mock.calls[0] as unknown as [string, RequestInit];
		expect(url).toBe('/api/v1/tokens');
		expect(init.method).toBe('POST');
		expect(JSON.parse(init.body as string)).toEqual({
			name: 'mcp',
			scopes: ['recipes:read'],
			expiresInDays: 90
		});
	});

	it('omits expiresInDays entirely when the token never expires', async () => {
		const fn = mockFetch({ id: 'a', token: 'rzp_x' }, 201);
		await createToken({ name: 'forever', scopes: ['recipes:read'], expiresInDays: null });
		const [, init] = fn.mock.calls[0] as unknown as [string, RequestInit];
		expect(JSON.parse(init.body as string)).toEqual({
			name: 'forever',
			scopes: ['recipes:read']
		});
	});
});

describe('deleteToken', () => {
	it('encodes the id into the path', async () => {
		const fn = mockFetch(null, 204);
		await deleteToken('a/b');
		const [url, init] = fn.mock.calls[0] as unknown as [string, RequestInit];
		expect(url).toBe('/api/v1/tokens/a%2Fb');
		expect(init.method).toBe('DELETE');
	});
});

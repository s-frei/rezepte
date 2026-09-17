import { afterEach, describe, expect, it, vi } from 'vitest';
import { api, ApiError } from './client';

function mockFetch(status: number, body: unknown, contentType = 'application/json') {
	const fn = vi.fn(
		async () =>
			new Response(body === undefined ? null : JSON.stringify(body), {
				status,
				headers: { 'Content-Type': contentType }
			})
	);
	vi.stubGlobal('fetch', fn);
	return fn;
}

afterEach(() => vi.unstubAllGlobals());

describe('api', () => {
	it('returns parsed JSON on success', async () => {
		mockFetch(200, { id: '1' });
		await expect(api<{ id: string }>('/recipes/1')).resolves.toEqual({ id: '1' });
	});

	it('prefixes /api/v1 and sends JSON headers', async () => {
		const fn = mockFetch(200, {});
		await api('/recipes', { method: 'POST', body: JSON.stringify({ title: 'x' }) });
		const [url, init] = fn.mock.calls[0] as unknown as [string, RequestInit];
		expect(url).toBe('/api/v1/recipes');
		expect(new Headers(init.headers).get('Content-Type')).toBe('application/json');
		expect(init.credentials).toBe('same-origin');
	});

	it('throws ApiError with problem details', async () => {
		mockFetch(
			422,
			{
				title: 'Unprocessable Entity',
				detail: 'validation failed',
				errors: [{ location: 'body.title', message: 'required' }]
			},
			'application/problem+json'
		);
		const err = (await api('/recipes').catch((e) => e)) as ApiError;
		expect(err).toBeInstanceOf(ApiError);
		expect(err.status).toBe(422);
		expect(err.detail).toBe('validation failed');
		expect(err.errors).toEqual([{ location: 'body.title', message: 'required' }]);
	});

	it('handles 204 without body', async () => {
		mockFetch(204, undefined);
		await expect(api<void>('/recipes/1', { method: 'DELETE' })).resolves.toBeUndefined();
	});

	it('handles an empty 200 body', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => new Response('', { status: 200 }))
		);
		await expect(api('/x')).resolves.toBeUndefined();
	});
});

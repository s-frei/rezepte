import { afterEach, describe, expect, it, vi } from 'vitest';
import { goto } from '$app/navigation';
import { session } from '$lib/auth.svelte';
import { api, ApiError, isSignedOut } from './client';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

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

	it('leaves the Content-Type to the browser for FormData bodies', async () => {
		const fn = mockFetch(200, {});
		const body = new FormData();
		body.append('file', new Blob(['x'], { type: 'image/png' }), 'x.png');
		await api('/recipes/r1/images', { method: 'POST', body });
		const [, init] = fn.mock.calls[0] as unknown as [string, RequestInit];
		// fetch derives "multipart/form-data; boundary=..." itself; a manual
		// value would lack the boundary and the server could not parse the body.
		expect(new Headers(init.headers).has('Content-Type')).toBe(false);
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

describe('isSignedOut', () => {
	it('is true for a 401 ApiError', () => {
		expect(isSignedOut(new ApiError(401, { title: 'Unauthorized' }))).toBe(true);
	});

	it('is false for any other status', () => {
		expect(isSignedOut(new ApiError(403, { title: 'Forbidden' }))).toBe(false);
		expect(isSignedOut(new ApiError(422, { title: 'Unprocessable Entity' }))).toBe(false);
	});

	it('is false for a non-ApiError', () => {
		expect(isSignedOut(new Error('boom'))).toBe(false);
		expect(isSignedOut(null)).toBe(false);
		expect(isSignedOut(undefined)).toBe(false);
	});
});

describe('api central 401 handling', () => {
	const mockedGoto = vi.mocked(goto);

	function stubBrowser(pathname: string, search = '') {
		vi.stubGlobal('window', { location: { pathname, search } });
	}

	afterEach(() => {
		mockedGoto.mockClear();
		session.user = null;
	});

	it('clears the session and redirects to /login with a next param', async () => {
		stubBrowser('/recipes', '?tag=soup');
		session.user = {
			id: '1',
			username: 'sam',
			displayName: 'Sam',
			role: 'user',
			color: 'amber',
			locale: 'en'
		};
		mockFetch(401, { title: 'Unauthorized' }, 'application/problem+json');

		await expect(api('/recipes')).rejects.toBeInstanceOf(ApiError);

		expect(session.user).toBeNull();
		expect(mockedGoto).toHaveBeenCalledWith(
			'/login?next=' + encodeURIComponent('/recipes?tag=soup')
		);
	});

	it('does not redirect for the login call itself', async () => {
		stubBrowser('/some-page');
		mockFetch(401, { title: 'Unauthorized' }, 'application/problem+json');

		await expect(api('/auth/login', { method: 'POST' })).rejects.toBeInstanceOf(ApiError);

		expect(mockedGoto).not.toHaveBeenCalled();
	});

	it('does not redirect for the GET me call (the root layout load already redirects via SvelteKit redirect())', async () => {
		stubBrowser('/some-page');
		mockFetch(401, { title: 'Unauthorized' }, 'application/problem+json');

		await expect(api('/auth/me')).rejects.toBeInstanceOf(ApiError);

		expect(mockedGoto).not.toHaveBeenCalled();
	});

	it('redirects for a mutating me call, which runs from a page and not from load', async () => {
		stubBrowser('/settings');
		session.user = {
			id: '1',
			username: 'sam',
			displayName: 'Sam',
			role: 'user',
			color: 'amber',
			locale: 'en'
		};
		mockFetch(401, { title: 'Unauthorized' }, 'application/problem+json');

		await expect(
			api('/auth/me', { method: 'PATCH', body: JSON.stringify({ password: 'x' }) })
		).rejects.toBeInstanceOf(ApiError);

		expect(session.user).toBeNull();
		expect(mockedGoto).toHaveBeenCalledWith('/login?next=' + encodeURIComponent('/settings'));
	});

	it('does not redirect when already on the login page', async () => {
		stubBrowser('/login');
		mockFetch(401, { title: 'Unauthorized' }, 'application/problem+json');

		await expect(api('/recipes')).rejects.toBeInstanceOf(ApiError);

		expect(mockedGoto).not.toHaveBeenCalled();
	});

	it('does not redirect outside a browser context', async () => {
		// No stubBrowser(): window stays undefined, as in this file's other tests.
		mockFetch(401, { title: 'Unauthorized' }, 'application/problem+json');

		await expect(api('/recipes')).rejects.toBeInstanceOf(ApiError);

		expect(mockedGoto).not.toHaveBeenCalled();
	});
});

import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from './client';
import { createDraft, draftFailure, fetchDraftPhoto } from './drafts';

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('createDraft', () => {
	it('posts the source as JSON to /api/v1/recipe-drafts', async () => {
		const draft = { recipe: { title: 'Käsespätzle' }, review: [], suggestedTags: [] };
		const fetchMock = vi.fn(
			async () =>
				new Response(JSON.stringify(draft), {
					status: 200,
					headers: { 'Content-Type': 'application/json' }
				})
		);
		vi.stubGlobal('fetch', fetchMock);
		await expect(createDraft({ url: 'https://a.example/x' })).resolves.toEqual(draft);
		const [path, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
		expect(path).toBe('/api/v1/recipe-drafts');
		expect(init.method).toBe('POST');
		expect(JSON.parse(init.body as string)).toEqual({ url: 'https://a.example/x' });
	});
	it('passes the abort signal on', async () => {
		const fetchMock = vi.fn(async () => new Response('{}', { status: 200 }));
		vi.stubGlobal('fetch', fetchMock);
		const controller = new AbortController();
		await createDraft({ text: 'x' }, controller.signal);
		await fetchDraftPhoto('/p', controller.signal).catch(() => {});
		const calls = fetchMock.mock.calls as unknown as [string, RequestInit][];
		expect(calls.map(([, init]) => init.signal)).toEqual([controller.signal, controller.signal]);
	});
});

describe('draftFailure', () => {
	it("names the import's reason for a 422", () => {
		const error = new ApiError(422, {
			title: 'Unprocessable Entity',
			errors: [{ location: 'body.url', message: 'no-recipe' }]
		});
		expect(draftFailure(error)).toBe('no-recipe');
	});
	it('is null for an unknown message and for other errors', () => {
		const error = new ApiError(422, { errors: [{ location: 'body.url', message: 'required' }] });
		expect(draftFailure(error)).toBeNull();
		expect(draftFailure(new ApiError(500, { title: 'Internal Server Error' }))).toBeNull();
		expect(draftFailure(new TypeError('offline'))).toBeNull();
	});
});

describe('fetchDraftPhoto', () => {
	it.each([
		['image/jpeg', 'import.jpeg'],
		['image/png', 'import.png'],
		['image/webp', 'import.webp']
	])('returns a %s file named %s', async (type, name) => {
		vi.stubGlobal(
			'fetch',
			vi.fn(
				async () =>
					new Response(new Uint8Array(3), { status: 200, headers: { 'Content-Type': type } })
			)
		);
		const file = await fetchDraftPhoto('/api/v1/recipe-drafts/photo?u=x');
		expect(file.type).toBe(type);
		expect(file.name).toBe(name);
		expect(file.size).toBe(3);
	});
	it('rejects on a 404', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => new Response(null, { status: 404 }))
		);
		await expect(fetchDraftPhoto('/api/v1/recipe-drafts/photo?u=x')).rejects.toThrow();
	});
});

import { afterEach, expect, it, vi } from 'vitest';
import { listComments, markCommentsSeen } from './comments';

function mockFetch(status: number, body?: unknown) {
	const fn = vi.fn(
		async () =>
			new Response(body === undefined ? null : JSON.stringify(body), {
				status,
				headers: { 'Content-Type': 'application/json' }
			})
	);
	vi.stubGlobal('fetch', fn);
	return fn;
}

afterEach(() => vi.unstubAllGlobals());

it('unwraps the list', async () => {
	mockFetch(200, { comments: [{ id: 1 }] });
	expect(await listComments('r1')).toEqual([{ id: 1 }]);
});

it('marks seen up to the given id', async () => {
	const fetch = mockFetch(204);
	await markCommentsSeen('r1', 7);
	const [, init] = fetch.mock.calls[0] as unknown as [string, RequestInit];
	expect(init?.body).toBe('{"upTo":7}');
});

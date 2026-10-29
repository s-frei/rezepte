import { afterEach, expect, it, vi } from 'vitest';
import {
	addComment,
	addCommentImageToRecipe,
	commentImageUrl,
	editComment,
	listComments,
	markCommentsSeen,
	uploadCommentImage
} from './comments';

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

const sent = (fetch: ReturnType<typeof mockFetch>) =>
	fetch.mock.calls[0] as unknown as [string, RequestInit];

it('sends imageIds only when given', async () => {
	let fetch = mockFetch(200, {});
	await addComment('r1', 'hi');
	expect(sent(fetch)[1].body).toBe('{"body":"hi"}');
	fetch = mockFetch(200, {});
	await addComment('r1', 'hi', ['a']);
	expect(sent(fetch)[1].body).toBe('{"body":"hi","imageIds":["a"]}');
	fetch = mockFetch(200, {});
	await editComment(3, 'hi', []);
	expect(sent(fetch)[1].body).toBe('{"body":"hi","imageIds":[]}');
	fetch = mockFetch(200, {});
	await editComment(3, 'hi');
	expect(sent(fetch)[1].body).toBe('{"body":"hi"}');
});

it('uploads a photo as multipart without a Content-Type', async () => {
	const fetch = mockFetch(201, { id: 'i', width: 1, height: 1, inRecipe: false });
	await uploadCommentImage('r/1', new File(['x'], 'a.png', { type: 'image/png' }));
	const [url, init] = sent(fetch);
	expect(url).toContain('/recipes/r%2F1/comment-images');
	expect(init.body).toBeInstanceOf(FormData);
	expect(new Headers(init.headers).has('Content-Type')).toBe(false);
});

it('builds the photo URL', () => {
	expect(commentImageUrl('r/1', 'i d', 'thumb')).toBe('/images/r%2F1/comments/i%20d/thumb.jpg');
});

it('adds a comment photo to the recipe', async () => {
	const fetch = mockFetch(201, { id: 'x' });
	await addCommentImageToRecipe('r1', 'c1');
	const [url, init] = sent(fetch);
	expect(url).toContain('/recipes/r1/images/from-comment');
	expect(init.body).toBe('{"commentImageId":"c1"}');
});

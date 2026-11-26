import { afterEach, expect, it, vi } from 'vitest';
import { ApiError } from './client';
import { requestPdf } from './pdf';

afterEach(() => vi.unstubAllGlobals());

const body = {} as never;

it('parses filename* before filename and reports progress', async () => {
	vi.stubGlobal(
		'fetch',
		vi.fn(
			async () =>
				new Response(new Uint8Array(4), {
					headers: {
						'Content-Length': '4',
						'Content-Disposition': `attachment; filename="Kasespatzle.pdf"; filename*=UTF-8''K%C3%A4sesp%C3%A4tzle.pdf`
					}
				})
		)
	);
	const seen: [number, number | null][] = [];
	const out = await requestPdf(body, (l, t) => seen.push([l, t]));
	expect(out.filename).toBe('Käsespätzle.pdf');
	expect(out.blob.size).toBe(4);
	expect(seen.at(-1)).toEqual([4, 4]);
});

it('throws ApiError on a problem response', async () => {
	vi.stubGlobal(
		'fetch',
		vi.fn(async () => new Response(JSON.stringify({ title: 'Nope' }), { status: 403 }))
	);
	await expect(requestPdf(body)).rejects.toBeInstanceOf(ApiError);
});

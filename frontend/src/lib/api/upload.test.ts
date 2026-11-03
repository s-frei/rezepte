import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from './client';
import { uploadWithProgress } from './upload';

class FakeXhr {
	static last: FakeXhr;
	upload: { onprogress?: (e: ProgressEvent) => void; onload?: () => void } = {};
	status = 0;
	responseText = '';
	onload?: () => void;
	onerror?: () => void;
	constructor() {
		FakeXhr.last = this;
	}
	open() {}
	setRequestHeader() {}
	send() {}
}

afterEach(() => vi.unstubAllGlobals());

describe('uploadWithProgress', () => {
	it('resolves the parsed body on the expected status and reports progress', async () => {
		vi.stubGlobal('XMLHttpRequest', FakeXhr);
		const seen: number[] = [];
		const p = uploadWithProgress<{ id: string }>('/x', new FormData(), {
			expect: 201,
			onProgress: (f) => seen.push(f)
		});
		FakeXhr.last.upload.onprogress!({
			lengthComputable: true,
			loaded: 5,
			total: 10
		} as ProgressEvent);
		FakeXhr.last.upload.onload!();
		FakeXhr.last.status = 201;
		FakeXhr.last.responseText = '{"id":"a"}';
		FakeXhr.last.onload!();
		await expect(p).resolves.toEqual({ id: 'a' });
		expect(seen).toEqual([0.5, 1]);
	});

	it('rejects an ApiError carrying status and problem', async () => {
		vi.stubGlobal('XMLHttpRequest', FakeXhr);
		const p = uploadWithProgress('/x', new FormData(), { expect: 201 });
		FakeXhr.last.status = 413;
		FakeXhr.last.responseText = '{"title":"Too large","detail":"over limit"}';
		FakeXhr.last.onload!();
		const err = (await p.catch((e) => e)) as ApiError;
		expect(err).toBeInstanceOf(ApiError);
		expect(err.status).toBe(413);
		expect(err.title).toBe('Too large');
	});

	it('maps a network error to ApiError status 0', async () => {
		vi.stubGlobal('XMLHttpRequest', FakeXhr);
		const p = uploadWithProgress('/x', new FormData(), { expect: 201 });
		FakeXhr.last.onerror!();
		await expect(p).rejects.toMatchObject({ status: 0 });
	});
});

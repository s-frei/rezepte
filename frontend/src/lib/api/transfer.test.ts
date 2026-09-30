import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from './client';
import { exportRecipes, importZip, saveBlob } from './transfer';

afterEach(() => {
	vi.unstubAllGlobals();
	vi.useRealTimers();
});

describe('exportRecipes', () => {
	it('reports progress against Content-Length and returns the named file', async () => {
		const body = new Uint8Array(10);
		vi.stubGlobal(
			'fetch',
			vi.fn(
				async () =>
					new Response(body, {
						status: 200,
						headers: {
							'Content-Type': 'application/zip',
							'Content-Length': '10',
							'Content-Disposition': 'attachment; filename="rezepte-2026-09-30.zip"'
						}
					})
			)
		);
		const seen: [number, number | null][] = [];
		const out = await exportRecipes(['a'], (loaded, total) => seen.push([loaded, total]));
		expect(out.filename).toBe('rezepte-2026-09-30.zip');
		expect(out.blob.size).toBe(10);
		expect(seen.at(-1)).toEqual([10, 10]);
	});

	it('throws an ApiError with the problem on failure', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(
				async () =>
					new Response(JSON.stringify({ title: 'Not Found', status: 404 }), { status: 404 })
			)
		);
		await expect(exportRecipes(['a'], () => {})).rejects.toBeInstanceOf(ApiError);
	});
});

// A stand-in XMLHttpRequest: the test fires its handlers by hand.
class FakeXhr {
	static last: FakeXhr;
	upload: { onprogress?: (e: ProgressEvent) => void; onload?: () => void } = {};
	status = 0;
	responseText = '';
	onload?: () => void;
	onerror?: () => void;
	onabort?: () => void;
	ontimeout?: () => void;
	constructor() {
		FakeXhr.last = this;
	}
	open() {}
	setRequestHeader() {}
	send() {}
}

describe('importZip', () => {
	it('reports the upload as complete when it ends without progress events', async () => {
		vi.stubGlobal('XMLHttpRequest', FakeXhr);
		const seen: number[] = [];
		const done = importZip(new Uint8Array(1), (f) => seen.push(f));
		FakeXhr.last.upload.onload!();
		Object.assign(FakeXhr.last, { status: 201, responseText: '{"created":[]}' });
		FakeXhr.last.onload!();
		await expect(done).resolves.toEqual({ created: [] });
		expect(seen).toEqual([1]);
	});

	it.each(['onabort', 'ontimeout'] as const)('rejects with an ApiError on %s', async (event) => {
		vi.stubGlobal('XMLHttpRequest', FakeXhr);
		const done = importZip(new Uint8Array(1), () => {});
		FakeXhr.last[event]!();
		await expect(done).rejects.toBeInstanceOf(ApiError);
	});
});

describe('saveBlob', () => {
	it('clicks an attached anchor and revokes the URL later', () => {
		vi.useFakeTimers();
		const calls: string[] = [];
		const anchor = {
			href: '',
			download: '',
			click: () => calls.push(`click:${anchor.download}`),
			remove: () => calls.push('remove')
		};
		vi.stubGlobal('document', {
			createElement: () => anchor,
			body: { append: () => calls.push('append') }
		});
		const revoke = vi.fn();
		vi.stubGlobal('URL', { createObjectURL: () => 'blob:x', revokeObjectURL: revoke });

		saveBlob(new Blob(), 'r.zip');
		expect(calls).toEqual(['append', 'click:r.zip', 'remove']);
		expect(revoke).not.toHaveBeenCalled();
		vi.runAllTimers();
		expect(revoke).toHaveBeenCalledWith('blob:x');
	});
});

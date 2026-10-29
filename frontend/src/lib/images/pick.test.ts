import { beforeEach, expect, it, vi } from 'vitest';
import { toast } from 'svelte-sonner';
import { ApiError } from '#lib/api/client.js';
import { m } from '#lib/paraglide/messages.js';
import { acceptFiles, uploadErrorMessage } from './pick';

vi.mock('svelte-sonner', () => ({ toast: { error: vi.fn() } }));
const toasted = vi.mocked(toast.error);
beforeEach(() => toasted.mockReset());

const file = (type: string, size = 10) => {
	const f = new File(['x'], 'a', { type });
	Object.defineProperty(f, 'size', { value: size });
	return f;
};

it('rejects a GIF by type', () => {
	expect(acceptFiles([file('image/gif')], 4, 'full')).toEqual([]);
	expect(toasted.mock.calls).toEqual([[m.images_rejected_type()]]);
});

it('rejects 11 MiB by size', () => {
	expect(acceptFiles([file('image/jpeg', 11 * 1024 * 1024)], 4, 'full')).toEqual([]);
	expect(toasted.mock.calls).toEqual([[m.images_rejected_size()]]);
});

it('keeps what fits and toasts each reason once, in order', () => {
	const files = Array.from({ length: 5 }, () => file('image/png'));
	const all = [file('image/gif'), file('image/gif'), file('image/png', 11 * 1024 * 1024), ...files];
	expect(acceptFiles(all, 4, 'full')).toEqual(files.slice(0, 4));
	expect(toasted.mock.calls).toEqual([
		[m.images_rejected_type()],
		[m.images_rejected_size()],
		['full']
	]);
});

it('maps 413 and 415 to the limits', () => {
	expect(uploadErrorMessage(new ApiError(413, {}))).toBe(m.images_rejected_size());
	expect(uploadErrorMessage(new ApiError(415, {}))).toBe(m.images_rejected_type());
});

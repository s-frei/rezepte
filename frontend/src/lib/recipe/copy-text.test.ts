import { afterEach, describe, expect, it, vi } from 'vitest';
import { toast } from 'svelte-sonner';
import { copyText } from './share.svelte';

vi.mock('svelte-sonner', () => ({ toast: Object.assign(vi.fn(), { success: vi.fn() }) }));
vi.mock('$lib/paraglide/messages', () => ({ m: { copy_unavailable: () => 'unavailable' } }));

// The hidden field fallback, reduced to what copyThroughField touches.
function stubDocument(copies: boolean) {
	const field = { style: {}, focus: vi.fn(), setSelectionRange: vi.fn(), remove: vi.fn() };
	vi.stubGlobal('document', {
		createElement: () => field,
		activeElement: null,
		body: { append: vi.fn() },
		execCommand: () => copies
	});
}

describe('copyText', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
		vi.clearAllMocks();
	});

	it('reports a copy through the clipboard API', async () => {
		const writeText = vi.fn().mockResolvedValue(undefined);
		vi.stubGlobal('navigator', { clipboard: { writeText } });

		expect(await copyText('Zwiebeln', 'copied')).toBe(true);
		expect(writeText).toHaveBeenCalledWith('Zwiebeln');
		expect(toast.success).toHaveBeenCalledWith('copied');
	});

	it('reports a copy through the hidden field where the clipboard API is missing', async () => {
		vi.stubGlobal('navigator', {});
		stubDocument(true);

		expect(await copyText('Zwiebeln', 'copied')).toBe(true);
	});

	// The caller's check must not appear when the text only sits in a toast.
	it('reports no copy when every way fails', async () => {
		vi.stubGlobal('navigator', {
			clipboard: { writeText: vi.fn().mockRejectedValue(new Error('denied')) }
		});
		stubDocument(false);

		expect(await copyText('Zwiebeln', 'copied')).toBe(false);
		expect(toast).toHaveBeenCalledWith('unavailable', { description: 'Zwiebeln' });
	});
});

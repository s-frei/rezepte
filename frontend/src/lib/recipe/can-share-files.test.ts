import { afterEach, expect, it, vi } from 'vitest';
import { canShareFiles } from './share.svelte';

afterEach(() => vi.unstubAllGlobals());

it.each([
	[undefined, 'card.png', 'image/png'],
	['application/pdf', 'card.pdf', 'application/pdf']
])('probes with a matching file (%s)', (arg, name, type) => {
	const canShare = vi.fn(() => true);
	vi.stubGlobal('navigator', { canShare });
	expect(canShareFiles(arg)).toBe(true);
	const file = (canShare.mock.calls[0] as unknown as [{ files: File[] }])[0].files[0];
	expect([file.name, file.type]).toEqual([name, type]);
});

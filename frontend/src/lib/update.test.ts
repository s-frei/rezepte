import { expect, test } from 'vitest';
import type { UpdateStatus } from './api/settings';
import { noteVisible } from './update';

const available: UpdateStatus = {
	current: '1.5.0',
	state: 'available',
	latest: { version: '1.6.0', publishedAt: '2026-11-12T09:30:00Z', changelogUrl: 'https://x/' }
};

test('noteVisible shows an undismissed available update', () => {
	expect(noteVisible(available, null)).toBe(true);
	expect(noteVisible(available, '1.5.0')).toBe(true);
	expect(noteVisible(available, '1.6.0')).toBe(false);
});

test('noteVisible hides every other state', () => {
	for (const state of ['current', 'unknown', 'off'] as const) {
		expect(noteVisible({ current: '1.5.0', state }, null)).toBe(false);
	}
	expect(noteVisible(undefined, null)).toBe(false);
});

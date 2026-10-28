import { describe, expect, it } from 'vitest';
import { notificationOptions, parsePayload, type PushPayload } from './push-worker';

const payload: PushPayload = {
	title: 'Soup · done',
	body: 'Step 3 · 10 min',
	url: '/recipes/soup/cook?step=3',
	tag: 'a1'
};

describe('parsePayload', () => {
	it('accepts a complete payload', () => {
		expect(parsePayload(payload)).toEqual(payload);
	});

	it('rejects missing or wrong fields', () => {
		expect(parsePayload(null)).toBeNull();
		expect(parsePayload('text')).toBeNull();
		for (const key of ['title', 'body', 'url', 'tag'] as const) {
			const rest: Record<string, unknown> = { ...payload };
			delete rest[key];
			expect(parsePayload(rest)).toBeNull();
			expect(parsePayload({ ...payload, [key]: 1 })).toBeNull();
		}
	});

	it('rejects a url that leaves the app', () => {
		expect(parsePayload({ ...payload, url: 'https://evil.example/' })).toBeNull();
		expect(parsePayload({ ...payload, url: '//evil.example/' })).toBeNull();
	});
});

describe('notificationOptions', () => {
	it('is silent and closed at once while a window is visible', () => {
		const o = notificationOptions(payload, true);
		expect(o.silent).toBe(true);
		expect(o.closeAtOnce).toBe(true);
	});

	it('rings when no window is visible', () => {
		const o = notificationOptions(payload, false);
		expect(o).toMatchObject({
			body: payload.body,
			tag: payload.tag,
			data: { url: payload.url },
			silent: false,
			closeAtOnce: false
		});
	});
});

import { describe, expect, it } from 'vitest';
import {
	notificationOptions,
	parsePayload,
	parseToastMessage,
	toastMessage,
	type PushPayload
} from './push-worker';

const timer: PushPayload = {
	kind: 'timer',
	title: 'Soup · done',
	body: 'Step 3 · 10 min',
	url: '/recipes/soup/cook?step=3',
	tag: 'a1'
};

const comment: PushPayload = {
	kind: 'comment',
	title: 'Anna on Soup',
	body: 'Lovely!',
	url: '/recipes/soup?tab=comments',
	tag: 'comment-r1',
	collapse: { title: 'Soup', bodyOne: '1 new comment', bodyMany: '{count} new comments' }
};

const hidden = { anyClientVisible: false, shownCount: 0 };

describe('parsePayload', () => {
	it('accepts a complete payload', () => {
		expect(parsePayload(timer)).toEqual(timer);
		expect(parsePayload(comment)).toEqual(comment);
	});

	it('rejects missing or wrong fields', () => {
		expect(parsePayload(null)).toBeNull();
		expect(parsePayload('text')).toBeNull();
		for (const key of ['kind', 'title', 'body', 'url', 'tag'] as const) {
			const rest: Record<string, unknown> = { ...timer };
			delete rest[key];
			expect(parsePayload(rest)).toBeNull();
			expect(parsePayload({ ...timer, [key]: 1 })).toBeNull();
		}
	});

	it('rejects a malformed collapse', () => {
		expect(parsePayload({ ...comment, collapse: 'x' })).toBeNull();
		expect(parsePayload({ ...comment, collapse: null })).toBeNull();
		expect(
			parsePayload({ ...comment, collapse: { title: 'Soup', bodyOne: '1', bodyMany: 2 } })
		).toBeNull();
	});

	it('rejects a url that leaves the app', () => {
		expect(parsePayload({ ...timer, url: 'https://evil.example/' })).toBeNull();
		expect(parsePayload({ ...timer, url: '//evil.example/' })).toBeNull();
	});
});

describe('notificationOptions', () => {
	it('keeps a timer silent and closed at once while a window is visible', () => {
		const o = notificationOptions(timer, { ...hidden, anyClientVisible: true });
		expect(o.silent).toBe(true);
		expect(o.closeAtOnce).toBe(true);
	});

	it('rings a timer when no window is visible', () => {
		const o = notificationOptions(timer, hidden);
		expect(o).toMatchObject({
			title: timer.title,
			body: timer.body,
			tag: timer.tag,
			data: { url: timer.url },
			silent: false,
			closeAtOnce: false
		});
	});

	it('shows a comment with its own copy, also while a window is visible', () => {
		const o = notificationOptions(comment, { ...hidden, anyClientVisible: true });
		expect(o).toMatchObject({
			title: comment.title,
			body: comment.body,
			data: { url: comment.url, count: 1 },
			silent: false,
			closeAtOnce: false
		});
	});

	it('rings any kind other than a timer, such as the test push', () => {
		const test: PushPayload = {
			kind: 'test',
			title: 'Test',
			body: 'Works',
			url: '/settings/notifications',
			tag: 'test'
		};
		expect(parsePayload(test)).toEqual(test);
		const o = notificationOptions(test, { ...hidden, anyClientVisible: true });
		expect(o).toMatchObject({ title: 'Test', silent: false, closeAtOnce: false });
	});

	it('collapses a second comment into a count', () => {
		const o = notificationOptions(comment, { ...hidden, shownCount: 1 });
		expect(o.title).toBe('Soup');
		expect(o.body).toBe('2 new comments');
		expect(o.data).toEqual({ url: comment.url, count: 2 });
		expect(o.renotify).toBe(true);
	});
});

describe('toastMessage', () => {
	it('carries what the notification shows, with its url and tag', () => {
		const o = notificationOptions(comment, { anyClientVisible: true, shownCount: 2 });
		expect(toastMessage(comment, o)).toEqual({
			type: 'rezepte-push',
			title: 'Soup',
			body: '3 new comments',
			url: '/recipes/soup?tab=comments',
			tag: 'comment-r1'
		});
	});

	it('is null for a timer, whose page rings and shows its own banner', () => {
		expect(toastMessage(timer, notificationOptions(timer, hidden))).toBeNull();
	});
});

describe('parseToastMessage', () => {
	const msg = { type: 'rezepte-push', title: 'T', body: 'B', url: '/recipes/x', tag: 't' };

	it('accepts what the worker posts', () => {
		expect(parseToastMessage(msg)).toEqual(msg);
	});

	it('rejects other messages, missing fields and urls leaving the app', () => {
		expect(parseToastMessage(null)).toBeNull();
		expect(parseToastMessage({ ...msg, type: 'other' })).toBeNull();
		expect(parseToastMessage({ ...msg, body: 1 })).toBeNull();
		expect(parseToastMessage({ ...msg, url: 'https://evil.example/' })).toBeNull();
		expect(parseToastMessage({ ...msg, url: '//evil.example/' })).toBeNull();
	});
});

// The service worker's pure parts, tested without a worker.

/**
 * What the Go service pushes: a timer alarm (`kind: 'timer'`, `tag` is the
 * alarm's id) or a notification. `collapse` folds a second push with the
 * same tag into a count; `bodyMany` carries a literal `{count}`.
 */
export type PushPayload = {
	kind: string;
	title: string;
	body: string;
	url: string;
	tag: string;
	collapse?: { title: string; bodyOne: string; bodyMany: string };
};

const isString = (v: unknown): v is string => typeof v === 'string';

export function parsePayload(data: unknown): PushPayload | null {
	if (typeof data !== 'object' || data === null) {
		return null;
	}
	const { kind, title, body, url, tag, collapse } = data as Record<string, unknown>;
	if (![kind, title, body, url, tag].every(isString)) {
		return null;
	}
	const p = { kind, title, body, url, tag } as PushPayload;
	// A click opens this; it stays inside the app.
	if (!p.url.startsWith('/') || p.url.startsWith('//')) {
		return null;
	}
	if (collapse !== undefined) {
		if (typeof collapse !== 'object' || collapse === null) {
			return null;
		}
		const c = collapse as Record<string, unknown>;
		if (![c.title, c.bodyOne, c.bodyMany].every(isString)) {
			return null;
		}
		p.collapse = {
			title: c.title as string,
			bodyOne: c.bodyOne as string,
			bodyMany: c.bodyMany as string
		};
	}
	return p;
}

export type ShowContext = {
	anyClientVisible: boolean;
	/** How many pushes the notification already shown under this tag stands for. */
	shownCount: number;
};

/**
 * A visible window already rings and shows the timer's banner, so a timer's
 * notification there is silent and closed at once: Chromium wants one shown
 * per push. Every other kind shows normally: a page open on it does not
 * refresh, so the push is the only news.
 */
export function notificationOptions(
	p: PushPayload,
	ctx: ShowContext
): NotificationOptions & {
	closeAtOnce: boolean;
	title: string;
	renotify?: boolean;
} {
	const base = { title: p.title, body: p.body, tag: p.tag, icon: '/icon-192.png' };
	if (p.kind === 'timer') {
		return {
			...base,
			data: { url: p.url },
			silent: ctx.anyClientVisible,
			closeAtOnce: ctx.anyClientVisible
		};
	}
	if (!p.collapse) {
		return { ...base, data: { url: p.url }, silent: false, closeAtOnce: false };
	}
	const count = ctx.shownCount + 1;
	return {
		...base,
		...(ctx.shownCount >= 1 && {
			title: p.collapse.title,
			body: p.collapse.bodyMany.replace('{count}', String(count)),
			renotify: true
		}),
		data: { url: p.url, count },
		silent: false,
		closeAtOnce: false
	};
}

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

/**
 * A click opens it, so it is a path on this origin. Resolved, not matched:
 * browsers read `/\evil.example` as `//evil.example`, another origin.
 */
const insideApp = (url: string, origin: string) =>
	url.startsWith('/') && new URL(url, origin).origin === origin;

/** `origin` is the worker's own, `self.location.origin`. */
export function parsePayload(origin: string, data: unknown): PushPayload | null {
	if (typeof data !== 'object' || data === null) {
		return null;
	}
	const { kind, title, body, url, tag, collapse } = data as Record<string, unknown>;
	if (![kind, title, body, url, tag].every(isString)) {
		return null;
	}
	const p = { kind, title, body, url, tag } as PushPayload;
	if (!insideApp(p.url, origin)) {
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

/** What the worker posts to a visible window, which shows it as a toast. */
export type ToastMessage = {
	type: 'rezepte-push';
	title: string;
	body: string;
	url: string;
	tag: string;
};

/**
 * The toast beside a notification: the same title and body as shown, so a
 * collapsed comment says "3 new comments" in both. None for a timer, whose
 * page rings and shows its own banner.
 */
export function toastMessage(
	p: PushPayload,
	o: { title: string; body?: string }
): ToastMessage | null {
	if (p.kind === 'timer') {
		return null;
	}
	return { type: 'rezepte-push', title: o.title, body: o.body ?? '', url: p.url, tag: p.tag };
}

/**
 * The page's side: only the worker's own message, and only a url inside the
 * app. `origin` is the page's, `location.origin`.
 */
export function parseToastMessage(origin: string, data: unknown): ToastMessage | null {
	if (typeof data !== 'object' || data === null) {
		return null;
	}
	const { type, title, body, url, tag } = data as Record<string, unknown>;
	if (type !== 'rezepte-push' || ![title, body, url, tag].every(isString)) {
		return null;
	}
	if (!insideApp(url as string, origin)) {
		return null;
	}
	return { type, title, body, url, tag } as ToastMessage;
}

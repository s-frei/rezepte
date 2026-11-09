import { goto } from '$app/navigation';
import type { Path } from '$app/types';
import { session } from '#lib/auth.svelte.js';
import { signedOutTarget } from '#lib/device-accounts.js';
import { resolvePath } from '#lib/paths.js';

const BASE = '/api/v1';
const LOGIN_PATH = '/api/v1/auth/login';
const ME_PATH = '/api/v1/auth/me';
const LOGOUT_PATH = '/api/v1/auth/logout';

export type FieldError = { location: string; message: string };

// & Record<string, unknown> rather than a closed shape: a handler can attach
// an extra field to its problem+json body (create-public-share's 409 carries
// the caller's existing share as `share`), and ApiError.body below is how a
// caller reaches it without every route needing its own error subclass.
export type Problem = {
	title?: string;
	detail?: string;
	status?: number;
	errors?: Array<{ location?: string; message?: string }>;
} & Record<string, unknown>;

export class ApiError extends Error {
	readonly status: number;
	readonly title: string;
	readonly detail?: string;
	readonly errors: FieldError[];
	/** The raw problem+json body, for a caller that needs a field beyond the ones above. */
	readonly body: Problem;

	constructor(status: number, problem: Problem) {
		super(problem.detail ?? problem.title ?? `HTTP ${status}`);
		this.name = 'ApiError';
		this.status = status;
		this.title = problem.title ?? `HTTP ${status}`;
		this.detail = problem.detail;
		this.errors = (problem.errors ?? []).map((e) => ({
			location: e.location ?? '',
			message: e.message ?? ''
		}));
		this.body = problem;
	}
}

/**
 * True when a call failed with the 401 that `api()` already turned into a
 * redirect to "Who's cooking?" or the login page. Callers use it to skip their own error toast:
 * the browser is leaving the page anyway.
 */
export function isSignedOut(error: unknown): boolean {
	return error instanceof ApiError && error.status === 401;
}

/**
 * Clears the session and sends the browser where a signed-out moment goes
 * (signedOutTarget: "Who's cooking?" or the login page), with a `next` param
 * pointing back at the page that got the 401. Not from those two pages,
 * which handle a 401 themselves. Several calls failing together leave once:
 * the first one asks for the target and navigates, the rest wait for it.
 */
export function redirectSignedOut(): Promise<void> {
	const { pathname, search } = window.location;
	if (pathname === '/login' || pathname === '/switch') return Promise.resolve();
	leaving ??= (async () => {
		session.user = null;
		const target = await signedOutTarget(pathname + search);
		await goto(resolvePath(target as Path));
	})().finally(() => (leaving = null));
	return leaving;
}

let leaving: Promise<void> | null = null;

/** Calls the JSON API. Resolves with the parsed body, or undefined for empty responses. */
export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
	const headers = new Headers(init.headers);
	headers.set('Accept', 'application/json');
	// A FormData body must reach fetch without a Content-Type: fetch sets
	// "multipart/form-data; boundary=..." itself, and the boundary is the
	// one thing a hand-written header cannot know.
	if (init.body !== undefined && !(init.body instanceof FormData) && !headers.has('Content-Type')) {
		headers.set('Content-Type', 'application/json');
	}
	const url = BASE + path;
	const method = (init.method ?? 'GET').toUpperCase();
	const res = await fetch(url, { ...init, headers, credentials: 'same-origin' });
	if (!res.ok) {
		const problem = await res.json().catch(() => ({}) as Problem);
		// Central 401 handling: any call other than login itself signing the
		// user out server-side (session expired, cookie cleared, ...) sends
		// the client where a signed-out moment goes instead of leaving every caller
		// to check res.status === 401 individually.
		//
		// /auth/login is excluded because a 401 there just means "wrong
		// credentials", not "you got signed out". GET /auth/me is excluded
		// because its only caller is the root +layout.ts `load`, which
		// already redirects via SvelteKit's `redirect()` - the SvelteKit-
		// blessed way to navigate from inside `load`. Calling `goto()` here
		// as well would race that in-flight navigation (SvelteKit explicitly
		// warns against calling `goto` from `load`) and can leave the app on
		// the wrong URL.
		//
		// POST /auth/logout is excluded because signOut() navigates itself,
		// exactly once; a 401 from an already ended session must not race it.
		//
		// The method is part of that exclusion: PATCH /auth/me (the password
		// change) runs from a page, not from `load`, so an expired session
		// there has to redirect like every other 401 - otherwise the form
		// would fail silently.
		if (
			res.status === 401 &&
			url !== LOGIN_PATH &&
			url !== LOGOUT_PATH &&
			!(url === ME_PATH && method === 'GET') &&
			typeof window !== 'undefined'
		) {
			void redirectSignedOut();
		}
		throw new ApiError(res.status, problem);
	}
	if (res.status === 204) {
		return undefined as T;
	}
	const text = await res.text();
	if (text.length === 0) {
		return undefined as T;
	}
	return JSON.parse(text) as T;
}

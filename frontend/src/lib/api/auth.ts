import type { Locale } from '$lib/paraglide/runtime';
import type { UserColor } from '$lib/user/color';
import { api } from './client';

/**
 * Re-exported rather than spelled out: Paraglide generates it from
 * `project.inlang/settings.json`, so a language added there reaches the API
 * types without anyone editing a union. The service constrains the same set
 * from `user.Locales`, and a test holds the two lists to each other.
 */
export type { Locale };

export type User = {
	id: string;
	username: string;
	displayName: string;
	role: 'superadmin' | 'admin' | 'user';
	color: UserColor;
	locale: Locale;
};

/** One palette colour and how many accounts hold it. */
export type ColorUsage = { color: UserColor; count: number };

export function login(username: string, password: string): Promise<User> {
	return api<User>('/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) });
}

export function logout(): Promise<void> {
	return api<void>('/auth/logout', { method: 'POST' });
}

export function me(): Promise<User> {
	return api<User>('/auth/me');
}

/** Returns a safe in-app path to continue to after login. */
export function safeNext(next: string | null): string {
	if (!next) {
		return '/';
	}
	// Resolve against a fixed, fake base so this is testable in Node and so
	// the URL parser - not string prefix checks - decides what counts as
	// "same origin". That rejects host-embedding tricks like a leading
	// backslash or control character that some browsers normalize into a
	// scheme-relative or absolute URL before a naive startsWith('/') check
	// would catch it.
	let u: URL;
	try {
		u = new URL(next, 'http://rezepte.local');
	} catch {
		return '/';
	}
	if (u.origin !== 'http://rezepte.local' || u.pathname.startsWith('/login')) {
		return '/';
	}
	return u.pathname + u.search;
}

/**
 * True when `path` is a SvelteKit route, so `goto` can reach it.
 *
 * Everything under /api/ is served by the Go binary, not by the SPA - the
 * Scalar docs page, which sends a signed-out visitor here with ?next, is the
 * case that matters. `goto` would render the app's own 404 for it, so those
 * targets need a full page load instead.
 */
export function isAppPath(path: string): boolean {
	return !path.startsWith('/api/');
}

/** Changes the own password; other sessions of the user are ended server-side. */
export function changePassword(currentPassword: string, password: string): Promise<void> {
	return api<void>('/auth/me', {
		method: 'PATCH',
		body: JSON.stringify({ currentPassword, password })
	});
}

/** Changes the own display name, colour and/or locale. Its own path, because PATCH /auth/me is the password change. */
export function updateOwnProfile(patch: {
	displayName?: string;
	color?: UserColor;
	locale?: Locale;
}): Promise<User> {
	return api<User>('/auth/me/profile', { method: 'PATCH', body: JSON.stringify(patch) });
}

/**
 * The palette with a count per colour, so the picker can mark one as taken.
 * Counts, not names: a plain member may not list users.
 */
export async function listColorUsage(): Promise<ColorUsage[]> {
	const page = await api<{ items: ColorUsage[] }>('/auth/me/colors');
	return page.items;
}

import { api } from './client';

export type User = { id: string; username: string; role: 'admin' | 'user' };

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

/** Changes the own password; other sessions of the user are ended server-side. */
export function changePassword(currentPassword: string, password: string): Promise<void> {
	return api<void>('/auth/me', {
		method: 'PATCH',
		body: JSON.stringify({ currentPassword, password })
	});
}

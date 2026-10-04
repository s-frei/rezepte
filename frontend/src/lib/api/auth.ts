import type { Locale } from '$lib/paraglide/runtime';
import { avatarBody, cropQuery } from '$lib/user/avatar';
import type { Crop } from '$lib/user/crop';
import type { UserColor } from '$lib/user/color';
import { api } from './client';

/**
 * Re-exported rather than spelled out: Paraglide generates it from
 * `service/internal/i18n/locales.json`, so a language added there reaches the
 * API types without anyone editing a union. The service embeds the same file,
 * so both sides accept exactly the same set.
 */
export type { Locale };

export type User = {
	id: string;
	username: string;
	displayName: string;
	role: 'superadmin' | 'admin' | 'user';
	color: UserColor;
	locale: Locale;
	/** Whether an admin lets this person create public, no-login recipe links. */
	canSharePublicly: boolean;
	/** The current picture, null without one; see avatarUrl. */
	avatarId: string | null;
	/** The account's email address; empty when none. Never used to sign in. */
	email: string;
	/** Whether the address is confirmed, by its confirmation mail or by an identity provider. */
	emailVerified: boolean;
	/** False for an account that signs in only through a setup link or an identity provider. */
	hasPassword: boolean;
	/**
	 * Whether a confirmation mail went to the current, unconfirmed address and
	 * its link is still open. Only the own-account responses carry it.
	 */
	emailConfirmationPending?: boolean;
};

/** One palette color and how many accounts hold it. */
export type ColorUsage = { color: UserColor; count: number };

/** Sets the own picture; the server crops `crop` out of `file`. */
export function setOwnAvatar(file: File, crop: Crop): Promise<{ avatarId: string }> {
	return api<{ avatarId: string }>(`/auth/me/avatar${cropQuery(crop)}`, {
		method: 'PUT',
		body: avatarBody(file)
	});
}

/** Removes the own picture; the circle falls back to color and initial. */
export function removeOwnAvatar(): Promise<void> {
	return api<void>('/auth/me/avatar', { method: 'DELETE' });
}

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

/**
 * Changes the own password; other sessions of the user are ended server-side.
 * currentPassword is undefined for an account that has none yet - the field
 * is then omitted from the body rather than sent as an empty string, so the
 * request looks the same as a client that has never heard of the concept.
 */
export function changePassword(
	currentPassword: string | undefined,
	password: string
): Promise<void> {
	return api<void>('/auth/me', {
		method: 'PATCH',
		body: JSON.stringify(
			currentPassword === undefined ? { password } : { currentPassword, password }
		)
	});
}

/**
 * Changes the own display name, color and/or locale. Its own path, because PATCH /auth/me is the password change.
 * `emailConfirmationPending` tells whether a changed address's confirmation mail went out.
 */
export function updateOwnProfile(patch: {
	displayName?: string;
	color?: UserColor;
	locale?: Locale;
	email?: string;
}): Promise<User> {
	return api<User>('/auth/me/profile', {
		method: 'PATCH',
		body: JSON.stringify(patch)
	});
}

/**
 * The palette with a count per color, so the picker can mark one as taken.
 * Counts, not names: the picker only has to mark a color as taken.
 */
export async function listColorUsage(): Promise<ColorUsage[]> {
	const page = await api<{ items: ColorUsage[] }>('/auth/me/colors');
	return page.items;
}

/**
 * Looks up the account a setup link belongs to, without using it - what
 * `/welcome` greets the invited person with before they have typed anything.
 * Throws a 404 `ApiError` when the token is unknown, used or expired.
 * `reset` is a forgotten-password link: it only sets a password.
 */
export function inspectSetupLink(
	token: string
): Promise<{ username: string; displayName: string; purpose: 'setup' | 'reset' }> {
	return api('/auth/setup/inspect', { method: 'POST', body: JSON.stringify({ token }) });
}

/** Sets the account's password through its setup link and signs in as it. */
export function redeemSetupLink(token: string, password: string): Promise<User> {
	return api<User>('/auth/setup/password', {
		method: 'POST',
		body: JSON.stringify({ token, password })
	});
}

/** Whether a forgotten password can be reset by mail here. Public, like getOidc. */
export function getPasswordReset(): Promise<{ available: boolean }> {
	return api<{ available: boolean }>('/auth/password');
}

/** Asks for a reset mail. Resolves the same whether or not an account matches. */
export function forgotPassword(login: string): Promise<void> {
	return api<void>('/auth/password/forgot', { method: 'POST', body: JSON.stringify({ login }) });
}

/** Confirms the address a confirmation mail went to; 404 when the link no longer works. Never signs in. */
export function confirmEmail(token: string): Promise<{ address: string }> {
	return api('/auth/email/confirm', { method: 'POST', body: JSON.stringify({ token }) });
}

/** Mails a fresh confirmation link for the own address; 429 within a minute of the last. */
export function resendConfirmation(): Promise<void> {
	return api<void>('/auth/me/email/confirmation', { method: 'POST' });
}

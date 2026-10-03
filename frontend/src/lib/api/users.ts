import type { UserColor } from '$lib/user/color';
import type { Locale, User } from './auth';
import { avatarBody, cropQuery } from '$lib/user/avatar';
import type { Crop } from '$lib/user/crop';
import { api } from './client';

export type UserAccount = User & {
	createdAt: string;
	/** When the account's open setup link expires; absent when none is open. */
	setupLinkExpiresAt?: string;
	/** Whether the person connected an identity provider; true only in listUsers (other responses carry false). */
	hasIdentity: boolean;
};

export type UserRole = User['role'];

/**
 * A freshly issued setup link, shown once. `mailedTo` names the address a
 * mail was tried for; `mailError` says that send failed - the link is valid
 * either way.
 */
export type SetupLinkInfo = {
	path: string;
	expiresAt: string;
	mailedTo?: string;
	mailError?: 'send_failed';
};

/** The full URL a setup link's `path` resolves to, on this instance. */
export function setupLinkUrl(info: SetupLinkInfo): string {
	return window.location.origin + info.path;
}

/**
 * An account as every signed-in account sees it: who takes part and in which
 * role - the `Person` a recipe names, plus the role. The people list shows
 * nothing else, so the admin's view of it is built from this too;
 * `UserAccount` is what the admin-only operations return. Named after the
 * service's `PersonEntry` so it cannot be mistaken for that roleless `Person`.
 */
export type PersonEntry = Pick<
	User,
	'id' | 'username' | 'displayName' | 'color' | 'role' | 'avatarId'
>;

/** Every account, ordered by username; readable by any signed-in account. */
export async function listPeople(): Promise<PersonEntry[]> {
	const list = await api<{ items: PersonEntry[] }>('/people');
	return list.items;
}

export async function listUsers(): Promise<UserAccount[]> {
	const list = await api<{ items: UserAccount[] }>('/users');
	return list.items;
}

/**
 * Creates an account. Omitting `password` creates it without one: the
 * response then carries `setupLink`, the one-time link the person uses to
 * set their own.
 */
export function createUser(input: {
	username: string;
	password?: string;
	role: UserRole;
	displayName?: string;
	color?: UserColor;
	/** Omitted means the instance default (`REZEPTE_LOCALE`). */
	locale?: Locale;
	/** Where the setup link is mailed when mail is on; stored on the account either way. */
	email?: string;
}): Promise<UserAccount & { setupLink?: SetupLinkInfo }> {
	return api('/users', { method: 'POST', body: JSON.stringify(input) });
}

/** Issues a one-time setup link for id, replacing any open one, and mails it when mail is on and id has an address, unless `mail` is false. Session-only, like a password reset. */
export function issueSetupLink(id: string, opts: { mail?: boolean } = {}): Promise<SetupLinkInfo> {
	return api<SetupLinkInfo>(`/users/${encodeURIComponent(id)}/setup-link`, {
		method: 'POST',
		body: JSON.stringify(opts)
	});
}

/** Revokes id's open setup link, if there is one. Not an error when none is open. */
export function revokeSetupLink(id: string): Promise<void> {
	return api<void>(`/users/${encodeURIComponent(id)}/setup-link`, { method: 'DELETE' });
}

/**
 * Disconnects id's identity-provider account and ends every session of id.
 * Allowed without a password; issue a setup link afterwards so they get back in.
 */
export function unlinkUserIdentity(id: string): Promise<void> {
	return api<void>(`/users/${encodeURIComponent(id)}/identity`, { method: 'DELETE' });
}

/**
 * Changes role, profile, public sharing and/or resets the password; a reset
 * ends all of that user's sessions. Display name and color are the owner's
 * alone; `canSharePublicly` follows the same rank rule as a reset (only the
 * owner reaches an admin) and is refused (409) for the owner's own row.
 */
export function updateUser(
	id: string,
	patch: {
		password?: string;
		role?: UserRole;
		displayName?: string;
		color?: UserColor;
		canSharePublicly?: boolean;
		/** Same rank rule as a reset; session only. */
		email?: string;
	}
): Promise<UserAccount> {
	return api<UserAccount>(`/users/${encodeURIComponent(id)}`, {
		method: 'PATCH',
		body: JSON.stringify(patch)
	});
}

export function deleteUser(id: string): Promise<void> {
	return api<void>(`/users/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

/** Sets another account's picture; the owner alone may. */
export function setUserAvatar(id: string, file: File, crop: Crop): Promise<{ avatarId: string }> {
	return api<{ avatarId: string }>(`/users/${encodeURIComponent(id)}/avatar${cropQuery(crop)}`, {
		method: 'PUT',
		body: avatarBody(file)
	});
}

/** Removes another account's picture; the owner alone may. */
export function removeUserAvatar(id: string): Promise<void> {
	return api<void>(`/users/${encodeURIComponent(id)}/avatar`, { method: 'DELETE' });
}

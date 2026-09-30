import type { UserColor } from '$lib/user/color';
import type { Locale, User } from './auth';
import { avatarBody, cropQuery } from '$lib/user/avatar';
import type { Crop } from '$lib/user/crop';
import { api } from './client';

export type UserAccount = User & { createdAt: string };

export type UserRole = User['role'];

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

export function createUser(input: {
	username: string;
	password: string;
	role: UserRole;
	displayName?: string;
	color?: UserColor;
	/** Omitted means the instance default (`REZEPTE_LOCALE`). */
	locale?: Locale;
}): Promise<UserAccount> {
	return api<UserAccount>('/users', { method: 'POST', body: JSON.stringify(input) });
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

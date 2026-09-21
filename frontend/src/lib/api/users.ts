import type { UserColor } from '$lib/user/color';
import type { User } from './auth';
import { api } from './client';

export type UserAccount = User & { createdAt: string };

export type UserRole = User['role'];

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
}): Promise<UserAccount> {
	return api<UserAccount>('/users', { method: 'POST', body: JSON.stringify(input) });
}

/** Changes role, profile and/or resets the password; a reset ends all of that user's sessions. Display name and colour are the owner's alone. */
export function updateUser(
	id: string,
	patch: { password?: string; role?: UserRole; displayName?: string; color?: UserColor }
): Promise<UserAccount> {
	return api<UserAccount>(`/users/${encodeURIComponent(id)}`, {
		method: 'PATCH',
		body: JSON.stringify(patch)
	});
}

export function deleteUser(id: string): Promise<void> {
	return api<void>(`/users/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

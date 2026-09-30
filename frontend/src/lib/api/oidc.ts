import { api, ApiError } from './client';

/** Whether this instance offers sign-in through an identity provider, and its name. */
export type OidcInfo = { enabled: boolean; name: string };

export async function getOidc(): Promise<OidcInfo> {
	const info = await api<{ enabled: boolean; name?: string }>('/auth/oidc');
	return { enabled: info.enabled, name: info.name ?? '' };
}

/** The caller's connection to the provider, or null when there is none. */
export async function getOwnIdentity(): Promise<{ linkedAt: string } | null> {
	try {
		return await api<{ linkedAt: string }>('/auth/me/identity');
	} catch (e) {
		if (e instanceof ApiError && e.status === 404) return null;
		throw e;
	}
}

/** Disconnects the provider; 409 while the account has no password. */
export function unlinkOwnIdentity(): Promise<void> {
	return api<void>('/auth/me/identity', { method: 'DELETE' });
}

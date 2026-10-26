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

/** The sign-in provider as the owner sees it; the client secret itself never comes back. */
export type SignInSettings = {
	/** env: REZEPTE_OIDC_* pin it, read-only here; settings: configured here; none: off. */
	source: 'env' | 'settings' | 'none';
	issuer: string;
	clientId: string;
	secretSet: boolean;
	/** Empty means the button says "single sign-on". */
	name: string;
	/** Empty while publicUrlMissing. */
	redirectUri: string;
	publicUrlMissing: boolean;
	/** Accounts connected at the issuer in force, and of those the ones without a password. */
	connected: number;
	connectedWithoutPassword: number;
};

/** What the edit dialog sends; an omitted clientSecret keeps the stored one, "" removes it. */
export type SignInConfig = {
	issuer: string;
	clientId: string;
	clientSecret?: string;
	name: string;
};

export function getSignInSettings(): Promise<SignInSettings> {
	return api<SignInSettings>('/settings/sign-in');
}

/** 422 with a field error on body.issuer when no provider answers there; nothing is stored then. */
export function saveSignInSettings(config: SignInConfig): Promise<SignInSettings> {
	return api<SignInSettings>('/settings/sign-in', { method: 'PUT', body: JSON.stringify(config) });
}

/** Turns sign-in through a provider off; connected identities stay. */
export function clearSignInSettings(): Promise<void> {
	return api<void>('/settings/sign-in', { method: 'DELETE' });
}

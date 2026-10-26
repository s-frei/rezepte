import { describe, expect, it } from 'vitest';
import type { SignInSettings } from '$lib/api/oidc';
import { connectedWarning } from './sign-in';

const base: SignInSettings = {
	source: 'settings',
	issuer: 'https://auth.example.org',
	clientId: 'rezepte',
	secretSet: true,
	name: '',
	redirectUri: 'https://rezepte.example.org/api/v1/auth/oidc/callback',
	publicUrlMissing: false,
	connected: 0,
	connectedWithoutPassword: 0
};

describe('connectedWarning', () => {
	it('names one account, falls back to the default name, leaves out nobody without a password', () =>
		expect(connectedWarning({ ...base, connected: 1 })).toBe(
			'1 account is connected to single sign-on and can no longer sign in with it afterwards. The connections stay saved and apply again once single sign-on is set up again.'
		));

	it('counts several and those without a password', () =>
		expect(
			connectedWarning({ ...base, name: 'Authentik', connected: 3, connectedWithoutPassword: 1 })
		).toBe(
			'3 accounts are connected to Authentik and can no longer sign in with it afterwards. 1 of them has no password and needs a setup link. The connections stay saved and apply again once Authentik is set up again.'
		));
});

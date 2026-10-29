import { describe, expect, it } from 'vitest';
import type { SignInSettings } from '#lib/api/oidc.js';
import { connectedWarning, secretRequired, turnOffWarning } from './sign-in';

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

describe('turnOffWarning', () => {
	it('says what goes, even with nobody connected', () =>
		expect(turnOffWarning(base)).toBe(
			'Rezepte removes the provider, the client ID and the secret.'
		));

	it('adds the connected accounts when there are any', () =>
		expect(turnOffWarning({ ...base, name: 'Dex', connected: 1 })).toBe(
			'Rezepte removes the provider, the client ID and the secret. 1 account is connected to Dex and can no longer sign in with it afterwards. The connections stay saved and apply again once Dex is set up again.'
		));
});

describe('secretRequired', () => {
	const same = { issuer: base.issuer, clientId: base.clientId, secret: '', removeSecret: false };
	it('is not needed while issuer and client stay', () =>
		expect(secretRequired(base, same)).toBe(false));
	it('is needed once the issuer or client ID changes', () => {
		expect(secretRequired(base, { ...same, issuer: 'https://other.example' })).toBe(true);
		expect(secretRequired(base, { ...same, clientId: 'other' })).toBe(true);
	});
	it('is met by a typed secret, a removal or no stored secret', () => {
		const other = { ...same, clientId: 'other' };
		expect(secretRequired(base, { ...other, secret: 'x' })).toBe(false);
		expect(secretRequired(base, { ...other, removeSecret: true })).toBe(false);
		expect(secretRequired({ ...base, secretSet: false }, other)).toBe(false);
	});
});

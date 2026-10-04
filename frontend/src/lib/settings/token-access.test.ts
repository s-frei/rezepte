import { describe, expect, it } from 'vitest';
import type { ApiToken } from '$lib/api/tokens';
import { OPEN_TODAY, accessLevels, isExpired, lifetime, tokenSupply } from './token-access';

function token(expiresAt?: string): ApiToken {
	return {
		id: 't',
		name: 't',
		prefix: 'rzp_abcd',
		scopes: ['recipes:read'],
		ownerId: 'u',
		ownerUsername: 'u',
		ownerDisplayName: 'u',
		ownerColor: 'amber',
		createdAt: '2026-01-01T00:00:00Z',
		expiresAt
	};
}

const now = new Date('2026-10-02T12:00:00Z');

describe('accessLevels', () => {
	// Each level travels with every level below it, so the deepest scope of an
	// area names the whole grant.
	it('names the deepest level per area', () => {
		expect(accessLevels(['recipes:read', 'recipes:write', 'users:read'])).toEqual({
			recipes: 'write',
			users: 'read'
		});
	});

	it('reaches delete for recipes and none for an area without scopes', () => {
		expect(accessLevels(['recipes:read', 'recipes:write', 'recipes:delete'])).toEqual({
			recipes: 'delete',
			users: 'none'
		});
	});
});

describe('isExpired', () => {
	it('treats a token without expiry as valid', () => {
		expect(isExpired(token(), now)).toBe(false);
	});

	it('expires at the stored instant', () => {
		expect(isExpired(token('2026-10-02T12:00:00Z'), now)).toBe(true);
		expect(isExpired(token('2026-10-02T12:00:01Z'), now)).toBe(false);
	});
});

describe('tokenSupply', () => {
	it('is missing without tokens', () => {
		expect(tokenSupply([], now)).toEqual({ kind: 'missing' });
	});

	it('is expired when no token is left valid', () => {
		expect(tokenSupply([token('2026-01-01T00:00:00Z')], now)).toEqual({ kind: 'expired' });
	});

	it('counts only the valid tokens', () => {
		expect(tokenSupply([token(), token('2026-01-01T00:00:00Z'), token()], now)).toEqual({
			kind: 'valid',
			count: 2
		});
	});
});

describe('lifetime', () => {
	const span = (expiresAt?: string, lastUsedAt?: string): ApiToken => ({
		...token(expiresAt),
		createdAt: '2026-10-01T12:00:00Z',
		lastUsedAt
	});

	it('places today and the last use between issue and expiry', () => {
		const life = lifetime(span('2026-10-11T12:00:00Z', '2026-10-01T12:00:00Z'), now);
		expect(life).toEqual({ kind: 'running', spent: 0.1, lastUsed: 0 });
	});

	it('is spent once the expiry has passed', () => {
		const life = lifetime(span('2026-10-02T00:00:00Z', '2026-10-01T18:00:00Z'), now);
		expect(life.kind).toBe('spent');
		expect(life.spent).toBe(1);
		expect(life.lastUsed).toBe(0.5);
	});

	// Without an end there is no scale; today sits at a fixed point and the
	// rest of the bar stays open.
	it('puts today at a fixed point when the token never expires', () => {
		const life = lifetime(span(undefined, '2026-10-01T12:00:00Z'), now);
		expect(life).toEqual({ kind: 'open', spent: OPEN_TODAY, lastUsed: 0 });
	});

	it('leaves the last use out when the token was never used', () => {
		expect(lifetime(span('2026-10-11T12:00:00Z'), now).lastUsed).toBeUndefined();
	});
});

describe('lifetime of a shared link', () => {
	// A permanent link carries `expiresAt: null` rather than leaving it out.
	it('treats a null expiry as an open end', () => {
		expect(lifetime({ createdAt: '2026-10-01T12:00:00Z', expiresAt: null }, now)).toEqual({
			kind: 'open',
			spent: OPEN_TODAY,
			lastUsed: undefined
		});
	});
});

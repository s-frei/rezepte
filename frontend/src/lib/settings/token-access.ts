import type { ApiToken, TokenScope } from '$lib/api/tokens';

export type RecipeAccess = 'none' | 'read' | 'write' | 'delete';
export type UserAccess = 'none' | 'read' | 'write';

/**
 * The deepest level a token holds per area. A level always travels with every
 * level below it (see CreateTokenDialog's `scopesOf`), so the deepest scope
 * alone names the whole grant.
 */
export function accessLevels(scopes: readonly TokenScope[]): {
	recipes: RecipeAccess;
	users: UserAccess;
} {
	const deepest = <L extends string>(area: string, ladder: readonly L[], none: L): L =>
		ladder.findLast((level) => scopes.includes(`${area}:${level}` as TokenScope)) ?? none;
	return {
		recipes: deepest('recipes', ['read', 'write', 'delete'] as const, 'none'),
		users: deepest('users', ['read', 'write'] as const, 'none')
	};
}

/** Whether the token's expiry has passed; one without an expiry never does. */
export function isExpired(token: ApiToken, now: Date = new Date()): boolean {
	return token.expiresAt !== undefined && new Date(token.expiresAt) <= now;
}

/** What the API page's ingredient line says about the admin's tokens. */
export type TokenSupply =
	{ kind: 'missing' } | { kind: 'expired' } | { kind: 'valid'; count: number };

export function tokenSupply(tokens: readonly ApiToken[], now: Date = new Date()): TokenSupply {
	if (tokens.length === 0) {
		return { kind: 'missing' };
	}
	const count = tokens.filter((token) => !isExpired(token, now)).length;
	return count === 0 ? { kind: 'expired' } : { kind: 'valid', count };
}

/** Where today sits on the bar of a token that never expires. */
export const OPEN_TODAY = 0.8;

/**
 * A token's lifetime as fractions of its bar: how much of it has passed and
 * where its last use falls, each from 0 (issued) to 1 (expires). A token
 * without an expiry has no scale, so today sits at OPEN_TODAY and the bar
 * stays open past it.
 */
export type Lifetime = {
	kind: 'running' | 'spent' | 'open';
	spent: number;
	lastUsed?: number;
};

/** What a lifetime bar needs: an API token or a shared link both qualify. */
export type Lifespan = { createdAt: string; expiresAt?: string | null; lastUsedAt?: string };

export function lifetime(item: Lifespan, now: Date = new Date()): Lifetime {
	const start = new Date(item.createdAt).getTime();
	// Without an expiry the bar is scaled so that today lands on OPEN_TODAY.
	const end =
		item.expiresAt == null
			? start + (now.getTime() - start) / OPEN_TODAY
			: new Date(item.expiresAt).getTime();
	const at = (time: number) => Math.min(1, Math.max(0, (time - start) / Math.max(1, end - start)));
	const lastUsed =
		item.lastUsedAt === undefined ? undefined : at(new Date(item.lastUsedAt).getTime());
	if (item.expiresAt == null) {
		return { kind: 'open', spent: OPEN_TODAY, lastUsed };
	}
	const spent = at(now.getTime());
	return { kind: spent >= 1 ? 'spent' : 'running', spent, lastUsed };
}

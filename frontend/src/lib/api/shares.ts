import { api } from './client';

// Contract types, mirroring the service's share package; the OpenAPI
// document (docs/user/public/openapi.json) is the reference.

/** How many days a public link lasts; `null` is permanent. */
export type ShareLifetime = 1 | 7 | 30 | 365 | null;

/** Where a public link currently stands. */
export type ShareStatus = 'active' | 'paused' | 'limited';

/** Who created a link; only present in an admin's `listShares(true)`. */
export type ShareCreator = { id: string; displayName: string; color: string };

/** A public, revocable link to a recipe. */
export type PublicShare = {
	id: string;
	recipe: { id: string; slug: string; title: string };
	/** The public page's path, `/s/<token>`. */
	path: string;
	createdAt: string;
	/** Effective expiry: the earlier of the link's own and the instance
	 * maximum counted from `createdAt`. `null` is permanent. */
	expiresAt: string | null;
	status: ShareStatus;
	createdBy?: ShareCreator;
};

/** Why the service refused to open a public link: a 403's `body.reason`. */
export type ShareRefusal = 'sharing-off' | 'not-allowed';

/**
 * Opens a public link to a recipe. `days` is capped by the instance
 * maximum - see `$lib/recipe/lifetimes`. Throws `ApiError` with status 409
 * when the caller already has one; its `body.share` carries that link. A
 * 403 says why in `body.reason` (`ShareRefusal`).
 */
export function createPublicShare(recipeId: string, days: ShareLifetime): Promise<PublicShare> {
	return api<PublicShare>(`/recipes/${recipeId}/public-share`, {
		method: 'POST',
		body: JSON.stringify({ days })
	});
}

/** The caller's own public link to a recipe, or null when there is none. */
export async function getMyPublicShare(recipeId: string): Promise<PublicShare | null> {
	const { share } = await api<{ share: PublicShare | null }>(`/shares/by-recipe/${recipeId}`);
	return share;
}

/** The caller's own public links, or (admins only) everyone's with `all`. */
export async function listShares(all = false): Promise<PublicShare[]> {
	const page = await api<{ items: PublicShare[] }>(`/shares${all ? '?all=true' : ''}`);
	return page.items;
}

/** Revokes one link: the caller's own, or any as an admin. */
export function revokeShare(id: string): Promise<void> {
	return api<void>(`/shares/${id}`, { method: 'DELETE' });
}

/** Revokes every public link in the household. Admins only. */
export function revokeAllShares(): Promise<void> {
	return api<void>('/shares', { method: 'DELETE' });
}

// When a share link is renewed. The link's token shows the recipe in chat
// previews until `expiresAt` (see docs/memory/content/features/link-previews.mdx);
// the recipe page renews it ahead of that, so a tap can hand it out at once.

/** How long before `expiresAt` a link counts as stale and is renewed. */
export const RENEW_BEFORE_MS = 2 * 60_000;

/** Never renew more often than this, whatever the lifetime. */
const MIN_DELAY_MS = 60_000;

/** Milliseconds from `now` until the link should be renewed; null for a link without expiry. */
export function renewDelay(expiresAt: string | null, now: number): number | null {
	if (expiresAt === null) {
		return null;
	}
	return Math.max(MIN_DELAY_MS, Date.parse(expiresAt) - now - RENEW_BEFORE_MS);
}

/** Whether a link has less than RENEW_BEFORE_MS left, or none. */
export function isStale(expiresAt: string | null, now: number): boolean {
	return expiresAt !== null && Date.parse(expiresAt) - now < RENEW_BEFORE_MS;
}

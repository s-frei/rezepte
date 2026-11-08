import { m } from '#lib/paraglide/messages.js';
import type { PublicShare, ShareLifetime } from '#lib/api/shares.js';

export type StubFace =
	| { kind: 'household' }
	| { kind: 'days'; value: number }
	| { kind: 'months'; value: number }
	| { kind: 'today'; until: Date }
	| { kind: 'open' }
	| { kind: 'paused' }
	| { kind: 'limited' };

const dayNumber = (d: Date): number =>
	Math.round(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()) / 86_400_000);

/**
 * What the ticket stub shows for a public link: its status, or the time left
 * in local calendar days. A link not created yet passes its status and expiry
 * alone. The household face is the caller's.
 */
export function stubFace(share: Pick<PublicShare, 'status' | 'expiresAt'>, now: Date): StubFace {
	if (share.status !== 'active') {
		return { kind: share.status };
	}
	if (share.expiresAt === null) {
		return { kind: 'open' };
	}
	const until = new Date(share.expiresAt);
	const d = dayNumber(until) - dayNumber(now);
	if (d <= 0) {
		return { kind: 'today', until };
	}
	return d < 60 ? { kind: 'days', value: d } : { kind: 'months', value: Math.round(d / 30.44) };
}

/** Number and unit for a lifetime-bar cell; `null` is permanent. */
export function lifetimeCell(days: ShareLifetime): { value: string; unit: string } {
	switch (days) {
		case null:
			return { value: '∞', unit: '' };
		case 1:
			return { value: '1', unit: m.share_unit_day() };
		case 365:
			return { value: '1', unit: m.share_unit_year() };
		default:
			return { value: String(days), unit: m.share_unit_days() };
	}
}

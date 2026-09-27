import { m } from '$lib/paraglide/messages';
import type { ShareStatus } from '$lib/api/shares';

/**
 * The label for a public link's status, in the active language. The Shared
 * links list names all three; the create/manage dialog shows a pill only
 * for the two dormant ones and nothing at all for an active link, so it
 * reads this for `paused` and `limited` alone.
 */
export function shareStatusLabel(status: ShareStatus): string {
	switch (status) {
		case 'paused':
			return m.public_share_status_paused();
		case 'limited':
			return m.public_share_status_limited();
		default:
			return m.public_share_status_active();
	}
}

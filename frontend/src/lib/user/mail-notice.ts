import type { MailWindow } from '#lib/api/auth.js';

/**
 * What a dialog says before saving `typed` over `stored`: a confirmation mail
 * goes only for a new, non-empty address - the same one in other letters is
 * the same mailbox, the service keeps it and mails nothing - and only while
 * mail is on.
 */
export function mailNotice(
	stored: string,
	typed: string,
	mailOn: boolean
): 'send' | 'mail-off' | null {
	const next = typed.trim();
	if (!next || next.toLowerCase() === stored.toLowerCase()) return null;
	return mailOn ? 'send' : 'mail-off';
}

/**
 * Whether the save that answered `after` mailed a confirmation: its sentAt
 * differs from the one held before. The holder's second change within a
 * minute mails nothing, and the toast must not say it did.
 */
export function mailWentOut(before: string | undefined, after: MailWindow | undefined): boolean {
	return !!after && after.sentAt !== before;
}

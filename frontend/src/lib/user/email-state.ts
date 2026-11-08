import type { MailWindow, User } from '#lib/api/auth.js';

/** What the email row says about an address and the mail that confirms it. */
export type EmailState =
	| { kind: 'none' }
	| { kind: 'confirmed' }
	| { kind: 'setup-link'; window: MailWindow }
	| { kind: 'pending'; window: MailWindow }
	| { kind: 'expired'; window: MailWindow }
	| { kind: 'unsent'; mailOn: boolean | null };

/**
 * A mailed setup link comes first: redeeming it confirms the address, so a
 * confirmation beside it would only be a second mail.
 */
export function emailState(
	u: Pick<User, 'email' | 'emailVerified' | 'emailConfirmation' | 'emailSetupLink'>,
	/** Null until known: the row then says nothing about mail being off. */
	mailOn: boolean | null,
	now: Date = new Date()
): EmailState {
	if (!u.email) return { kind: 'none' };
	if (u.emailVerified) return { kind: 'confirmed' };
	if (u.emailSetupLink) return { kind: 'setup-link', window: u.emailSetupLink };
	const c = u.emailConfirmation;
	if (c) return { kind: new Date(c.expiresAt) > now ? 'pending' : 'expired', window: c };
	return { kind: 'unsent', mailOn };
}

/**
 * The send the row offers: only while mail is on, since a send with mail
 * off answers 409 and changes nothing. A setup link has none: resending it
 * is the setup-link row's.
 */
export function emailAction(
	state: EmailState,
	mailOn: boolean | null
): 'resend' | 'new-link' | 'send' | null {
	if (mailOn !== true) return null;
	if (state.kind === 'pending') return 'resend';
	if (state.kind === 'expired') return 'new-link';
	if (state.kind === 'unsent') return 'send';
	return null;
}

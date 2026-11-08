import { describe, expect, it } from 'vitest';
import { emailAction, emailState } from './email-state';

const now = new Date('2026-11-10T12:00:00Z');
const window = { sentAt: '2026-11-08T14:03:00Z', expiresAt: '2026-11-15T14:03:00Z' };
const expired = { sentAt: '2026-10-25T09:12:00Z', expiresAt: '2026-11-01T09:12:00Z' };
const base = { email: 'mila@example.com', emailVerified: false };

describe('emailState', () => {
	it('is none without an address', () => {
		expect(emailState({ email: '', emailVerified: false }, true, now)).toEqual({ kind: 'none' });
	});
	it('is confirmed for a verified address, whatever mail is on record', () => {
		expect(
			emailState({ ...base, emailVerified: true, emailConfirmation: window }, true, now)
		).toEqual({ kind: 'confirmed' });
	});
	it('shows a mailed setup link before an open confirmation', () => {
		expect(
			emailState({ ...base, emailSetupLink: window, emailConfirmation: window }, true, now)
		).toEqual({
			kind: 'setup-link',
			window
		});
	});
	it('is pending while the confirmation is open', () => {
		expect(emailState({ ...base, emailConfirmation: window }, true, now)).toEqual({
			kind: 'pending',
			window
		});
	});
	it('is expired once the link ran out', () => {
		expect(emailState({ ...base, emailConfirmation: expired }, true, now)).toEqual({
			kind: 'expired',
			window: expired
		});
	});
	it('is expired when expiresAt equals now', () => {
		const edge = { ...window, expiresAt: now.toISOString() };
		expect(emailState({ ...base, emailConfirmation: edge }, true, now).kind).toBe('expired');
	});
	it('is unsent without any mail, and says whether mail is on', () => {
		expect(emailState(base, true, now)).toEqual({ kind: 'unsent', mailOn: true });
		expect(emailState(base, false, now)).toEqual({ kind: 'unsent', mailOn: false });
	});
	it('says nothing about mail being off until that is known', () => {
		expect(emailState(base, null, now)).toEqual({ kind: 'unsent', mailOn: null });
	});
});

describe('emailAction', () => {
	it('offers a send only while mail is on', () => {
		expect(emailAction({ kind: 'pending', window }, true)).toBe('resend');
		expect(emailAction({ kind: 'expired', window: expired }, true)).toBe('new-link');
		expect(emailAction({ kind: 'unsent', mailOn: true }, true)).toBe('send');
		for (const mailOn of [false, null]) {
			expect(emailAction({ kind: 'pending', window }, mailOn)).toBeNull();
			expect(emailAction({ kind: 'expired', window: expired }, mailOn)).toBeNull();
			expect(emailAction({ kind: 'unsent', mailOn }, mailOn)).toBeNull();
		}
	});
	it('offers nothing beside a setup link or for a confirmed address', () => {
		expect(emailAction({ kind: 'setup-link', window }, true)).toBeNull();
		expect(emailAction({ kind: 'confirmed' }, true)).toBeNull();
	});
});

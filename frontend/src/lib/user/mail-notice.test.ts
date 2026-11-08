import { describe, expect, it } from 'vitest';
import { mailNotice, mailWentOut } from './mail-notice';

describe('mailNotice', () => {
	it('says nothing for an unchanged address or one in other letters', () => {
		expect(mailNotice('mila@example.com', 'mila@example.com', true)).toBeNull();
		expect(mailNotice('mila@example.com', ' Mila@Example.com ', true)).toBeNull();
	});
	it('says nothing when the field is cleared or blank', () => {
		expect(mailNotice('mila@example.com', '   ', true)).toBeNull();
	});
	it('announces the mail for a new address', () => {
		expect(mailNotice('mila@example.com', 'mila.neu@example.com', true)).toBe('send');
		expect(mailNotice('', 'mila@example.com', true)).toBe('send');
	});
	it('says mail is off instead', () => {
		expect(mailNotice('', 'mila@example.com', false)).toBe('mail-off');
	});
});

describe('mailWentOut', () => {
	const w = (sentAt: string) => ({ sentAt, expiresAt: '2026-11-15T00:00:00Z' });
	it('is true for a confirmation sent by this save', () => {
		expect(mailWentOut(undefined, w('2026-11-08T14:03:00Z'))).toBe(true);
		expect(mailWentOut('2026-11-01T09:00:00Z', w('2026-11-08T14:03:00Z'))).toBe(true);
	});
	it('is false when nothing new went out', () => {
		expect(mailWentOut('2026-11-08T14:03:00Z', w('2026-11-08T14:03:00Z'))).toBe(false);
		expect(mailWentOut('2026-11-08T14:03:00Z', undefined)).toBe(false);
	});
});

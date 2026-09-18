import { describe, expect, it } from 'vitest';
import { passwordErrorsFromApi, validateNewPassword } from './password';

describe('validateNewPassword', () => {
	it('accepts a matching pair of at least 8 characters', () => {
		expect(validateNewPassword('long-enough', 'long-enough')).toEqual({});
	});
	it('rejects short passwords', () => {
		expect(validateNewPassword('short', 'short').next).toBeTruthy();
	});
	it('rejects passwords over 128 characters', () => {
		const long = 'x'.repeat(129);
		expect(validateNewPassword(long, long).next).toBeTruthy();
	});
	it('rejects a repeat that differs', () => {
		const errors = validateNewPassword('long-enough', 'long-enough2');
		expect(errors.next).toBeUndefined();
		expect(errors.repeat).toBeTruthy();
	});
});

describe('passwordErrorsFromApi', () => {
	it('maps the API locations onto the fields', () => {
		const errors = passwordErrorsFromApi([
			{ location: 'body.currentPassword', message: 'current password is wrong' },
			{ location: 'body.password', message: 'expected length >= 8' }
		]);
		expect(errors.current).toBeTruthy();
		expect(errors.next).toBeTruthy();
		expect(errors.repeat).toBeUndefined();
	});
	it('ignores unknown locations', () => {
		expect(passwordErrorsFromApi([{ location: 'body.other', message: 'x' }])).toEqual({});
	});
});

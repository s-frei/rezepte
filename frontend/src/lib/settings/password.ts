import type { FieldError } from '$lib/api/client';
import { m } from '$lib/paraglide/messages';

export const PASSWORD_MIN = 8;
export const PASSWORD_MAX = 128;

export type PasswordErrors = { current?: string; next?: string; repeat?: string };

/** Client-side checks that mirror the API's 8-128 rule plus the repeat field. */
export function validateNewPassword(next: string, repeat: string): PasswordErrors {
	const errors: PasswordErrors = {};
	if (next.length < PASSWORD_MIN) {
		errors.next = m.settings_password_too_short({ min: PASSWORD_MIN });
	} else if (next.length > PASSWORD_MAX) {
		errors.next = m.settings_password_too_long({ max: PASSWORD_MAX });
	}
	if (repeat !== next) {
		errors.repeat = m.settings_password_mismatch();
	}
	return errors;
}

/** Maps a 422 problem body onto the form's fields; other locations are dropped. */
export function passwordErrorsFromApi(errors: FieldError[]): PasswordErrors {
	const out: PasswordErrors = {};
	for (const error of errors) {
		if (error.location === 'body.currentPassword') {
			out.current = m.settings_password_current_wrong();
		} else if (error.location === 'body.password') {
			out.next = m.settings_password_too_short({ min: PASSWORD_MIN });
		}
	}
	return out;
}

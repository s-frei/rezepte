import { describe, expect, it } from 'vitest';
import { ApiError } from '#lib/api/client.js';
import { mailProblem } from './mail-problem';

const at = (status: number, ...locations: string[]) =>
	new ApiError(status, {
		detail: 'English text for a developer',
		errors: locations.map((location) => ({ location, message: 'x' }))
	});

describe('mailProblem', () => {
	it('names the field a save or a test fails on', () => {
		expect(mailProblem(at(422, 'body.host'), 'fallback')).toBe(
			'Enter the address of your mail server.'
		);
		expect(mailProblem(at(422, 'body.config.security'), 'fallback')).toBe(
			'Signing in needs STARTTLS or TLS.'
		);
		expect(mailProblem(at(422, 'body.from'), 'fallback')).toBe(
			'Enter the from address on its own, such as rezepte@example.org.'
		);
		expect(mailProblem(at(422, 'body.config.password'), 'fallback')).toBe(
			'A different server or username needs its password.'
		);
		expect(mailProblem(at(422, 'body.port'), 'fallback')).toBe('A number from 1 to 65535');
	});

	it('puts a bad recipient before the settings', () =>
		expect(mailProblem(at(422, 'body.config.host', 'body.to'), 'fallback')).not.toBe(
			'Enter the address of your mail server.'
		));

	it('shows the server reply for a refused delivery and never the API text otherwise', () => {
		expect(mailProblem(at(502), 'fallback')).toBe('English text for a developer');
		expect(mailProblem(at(409), 'fallback')).toBe('fallback');
		expect(mailProblem(new Error('boom'), 'fallback')).toBe('fallback');
		expect(mailProblem(at(422, 'body.fromName'), 'fallback')).toMatch(/^Rezepte cannot send/);
	});
});

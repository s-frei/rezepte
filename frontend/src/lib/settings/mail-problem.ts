import { ApiError } from '#lib/api/client.js';
import { m } from '#lib/paraglide/messages.js';

/** The catalog copy for each mail setting the API names in a 422. */
const FIELD_PROBLEMS: Record<string, () => string> = {
	host: () => m.settings_mail_host_invalid(),
	port: () => m.settings_mail_field_port_invalid(),
	security: () => m.settings_mail_security_invalid(),
	from: () => m.settings_mail_from_invalid(),
	password: () => m.settings_mail_password_required()
};

/**
 * What the owner reads under the test row when a test or a save fails: the
 * mail server's own reply for a refused delivery (502), since that is what
 * there is to fix, and catalog copy for everything else - the API's own
 * words are English. A 422 names its field as `body.<field>` on a save and
 * `body.config.<field>` on a test.
 */
export function mailProblem(error: unknown, fallback: string): string {
	if (!(error instanceof ApiError)) return fallback;
	if (error.status === 502) return error.message;
	if (error.status !== 422) return fallback;
	const at = error.errors.map((e) => e.location);
	if (at.includes('body.to')) return m.settings_profile_email_invalid();
	for (const location of at) {
		const problem = FIELD_PROBLEMS[location.split('.').at(-1) ?? ''];
		if (problem) return problem();
	}
	return m.settings_mail_config_invalid();
}

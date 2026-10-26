import type { SignInSettings } from '$lib/api/oidc';
import { m } from '$lib/paraglide/messages';

/** The provider's name as the sign-in button shows it. */
export function providerName(s: SignInSettings): string {
	return s.name || m.settings_sign_in_default_name();
}

/**
 * What a switch or turning off costs, said before either happens: who
 * loses this way in, who of them has no other, and that nothing is lost
 * for good.
 */
export function connectedWarning(s: SignInSettings): string {
	const name = providerName(s);
	const count = s.connected;
	const without = s.connectedWithoutPassword;
	return [
		count === 1
			? m.settings_sign_in_connected_one({ name })
			: m.settings_sign_in_connected({ count, name }),
		without === 0
			? ''
			: without === 1
				? m.settings_sign_in_without_password_one()
				: m.settings_sign_in_without_password({ count: without }),
		m.settings_sign_in_connections_stay({ name })
	]
		.filter(Boolean)
		.join(' ');
}

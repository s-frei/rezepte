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

/**
 * Whether the server will refuse the save for a missing secret: the stored
 * secret belongs to the stored issuer and client, so a different one needs
 * a secret typed again (or the stored one removed) first.
 */
export function secretRequired(
	s: SignInSettings,
	next: { issuer: string; clientId: string; secret: string; removeSecret: boolean }
): boolean {
	return (
		s.secretSet &&
		!next.secret &&
		!next.removeSecret &&
		(next.issuer !== s.issuer || next.clientId !== s.clientId)
	);
}

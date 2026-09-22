import type { User } from './api/auth';
import { getLocale, setLocale } from '$lib/paraglide/runtime';

/** Current login state, shared across the app. Mutate `session.user`. */
export const session = $state<{ user: User | null }>({ user: null });

/**
 * Makes the browser render the language the account actually holds.
 *
 * The locale cookie is written on login and on a profile PATCH, and it lives
 * for a year - so a device signed in before the language was changed on
 * another one keeps rendering the old language for the rest of its session.
 * The account row is the source of truth and `GET /auth/me` returns it on
 * every session load, which is the one place both values are in hand.
 *
 * Unlike the settings control, this leaves the reload to `setLocale`. There
 * the PATCH response had already installed the new cookie, so Paraglide
 * compared the new locale against an already-current one, saw no change and
 * skipped its own reload - hence the manual one. Here the cookie is the stale
 * value, so Paraglide's comparison differs, it writes the cookie and reloads
 * itself, and a second reload would only be a redundant navigation.
 *
 * This cannot loop. `setLocale` writes the cookie before it reloads, the
 * cookie is the first strategy Paraglide consults, and the account row is
 * untouched - so after the reload `getLocale()` returns exactly the
 * `user.locale` the fresh `/auth/me` reports and this is a no-op.
 *
 * Returns true when a reload was started, so a caller can stop working on a
 * page that is about to be replaced.
 */
export function adoptAccountLocale(user: User): boolean {
	if (user.locale === getLocale()) {
		return false;
	}
	setLocale(user.locale);
	return true;
}

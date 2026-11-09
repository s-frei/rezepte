import { goto } from '$app/navigation';
import type { Path } from '$app/types';
import { resolvePath } from '#lib/paths.js';
import { logout } from '#lib/api/auth.js';
import { session } from '#lib/auth.svelte.js';
import { signedOutTarget } from '#lib/device-accounts.js';
import { disablePush } from '#lib/push.svelte.js';
import { announceSession } from '#lib/session-broadcast.js';
import { timers } from '#lib/recipe/timers.svelte.js';

/**
 * The one sign-out action (user menu and settings nav). The server call is
 * best-effort: a session that is already gone must still end up signed out
 * on the login page. Exactly one navigation happens here - callers must not
 * add another.
 */
export async function signOut(): Promise<void> {
	// Timers belong to whoever is signed in on this device; the next person
	// starts clean, and this device gets no push for them. Both first, while
	// the session still exists.
	await disablePush();
	timers.clear();
	try {
		await logout();
	} catch {
		// see above
	}
	const who = session.user;
	session.user = null;
	announceSession(null);
	// Someone else still known on this device is asked who cooks next; the
	// login page says once who left (a shared kitchen tablet), from
	// navigation state, so a reload and the URL carry nothing. logout() is
	// left out of the API client's 401 redirect, so this is the one navigation.
	const target = await signedOutTarget();
	await goto(resolvePath(target as Path), {
		state: who ? { signedOut: { displayName: who.displayName, color: who.color } } : {}
	});
}

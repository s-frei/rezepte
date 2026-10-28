import { goto } from '$app/navigation';
import { resolve } from '$app/paths';
import { logout } from '$lib/api/auth';
import { session } from '$lib/auth.svelte';
import { disablePush } from '$lib/push.svelte';
import { timers } from '$lib/recipe/timers.svelte';

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
	session.user = null;
	await goto(resolve('/login'));
}

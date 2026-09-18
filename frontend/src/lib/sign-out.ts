import { goto } from '$app/navigation';
import { resolve } from '$app/paths';
import { logout } from '$lib/api/auth';
import { session } from '$lib/auth.svelte';

/**
 * The one sign-out action (user menu and settings nav). The server call is
 * best-effort: a session that is already gone must still end up signed out
 * on the login page. Exactly one navigation happens here - callers must not
 * add another.
 */
export async function signOut(): Promise<void> {
	try {
		await logout();
	} catch {
		// see above
	}
	session.user = null;
	await goto(resolve('/login'));
}

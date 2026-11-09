import { redirect } from '@sveltejs/kit';
import { ApiError } from '#lib/api/client.js';
import { me } from '#lib/api/auth.js';
import { signedOutTarget } from '#lib/device-accounts.js';
import { adoptAccountLocale, authPages, session } from '#lib/auth.svelte.js';
import type { LayoutLoad } from './$types';

export const ssr = false;
export const prerender = false;

// A failed root load leaves SvelteKit its bare built-in error page, so a
// session that cannot be loaded (the server unreachable) never fails this
// load where SvelteKit still has a page to render: it returns `unreachable`
// and +error.svelte says so, without the shell (+layout.svelte).
//
// On a route, SvelteKit renders +error.svelte by running this load once more
// after it failed; that run passes. On a path no route matches, the one run
// is already the error page's, so it passes at once.
let failed = false;

export const load: LayoutLoad = async ({ url, route }) => {
	if (failed) {
		failed = false;
		return { user: null, unreachable: true };
	}
	// A public share is read without a session; a stranger has none to load
	// and a signed-in member sees the same page without a redirect. Never
	// calls `me()` here, so a member's expired cookie cannot bounce them
	// off a link someone just handed them.
	if (url.pathname.startsWith('/s/')) {
		return { user: session.user ?? null };
	}
	if (session.user) return { user: session.user };
	try {
		session.user = await me();
		// The one place the browser's language and the account's stored one
		// meet: `me()` runs once per session load, so this is where a language
		// changed on another device gets picked up.
		adoptAccountLocale(session.user);
		return { user: session.user };
	} catch (e) {
		if (e instanceof ApiError && e.status === 401) {
			if (authPages.includes(url.pathname)) return { user: null };
			redirect(302, await signedOutTarget(url.pathname + url.search));
		}
		if (route.id === null) return { user: null, unreachable: true };
		failed = true;
		throw e;
	}
};

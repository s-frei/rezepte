import { redirect } from '@sveltejs/kit';
import { ApiError } from '$lib/api/client';
import { me } from '$lib/api/auth';
import { adoptAccountLocale, session } from '$lib/auth.svelte';
import type { LayoutLoad } from './$types';

export const ssr = false;
export const prerender = false;

export const load: LayoutLoad = async ({ url }) => {
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
			if (url.pathname === '/login' || url.pathname === '/welcome') return { user: null };
			redirect(302, `/login?next=${encodeURIComponent(url.pathname + url.search)}`);
		}
		throw e;
	}
};

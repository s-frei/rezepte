import { redirect } from '@sveltejs/kit';
import { ApiError } from '$lib/api/client';
import { me } from '$lib/api/auth';
import { adoptAccountLocale, session } from '$lib/auth.svelte';
import type { LayoutLoad } from './$types';

export const ssr = false;
export const prerender = false;

export const load: LayoutLoad = async ({ url }) => {
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
			if (url.pathname === '/login') return { user: null };
			redirect(302, `/login?next=${encodeURIComponent(url.pathname + url.search)}`);
		}
		throw e;
	}
};

import { redirect } from '@sveltejs/kit';
import { ApiError } from '$lib/api/client';
import { me } from '$lib/api/auth';
import { session } from '$lib/auth.svelte';
import type { LayoutLoad } from './$types';

export const ssr = false;
export const prerender = false;

export const load: LayoutLoad = async ({ url }) => {
	if (session.user) return { user: session.user };
	try {
		session.user = await me();
		return { user: session.user };
	} catch (e) {
		if (e instanceof ApiError && e.status === 401) {
			if (url.pathname === '/login') return { user: null };
			redirect(302, `/login?next=${encodeURIComponent(url.pathname + url.search)}`);
		}
		throw e;
	}
};

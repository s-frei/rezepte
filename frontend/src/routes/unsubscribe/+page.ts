import { ApiError } from '$lib/api/client';
import { getUnsubscribe, type Unsubscribe } from '$lib/api/notifications';
import type { PageLoad } from './$types';

/**
 * Reads what the mailed link reaches. Opening changes nothing - mail
 * scanners open links - only a button on the page does. A forged, expired
 * or incomplete link is `gone`.
 */
export const load: PageLoad = async ({ url }) => {
	const link = {
		u: url.searchParams.get('u') ?? '',
		k: url.searchParams.get('k') ?? '',
		s: url.searchParams.get('s') ?? ''
	};
	const scope: 'kind' | 'group' = url.searchParams.get('scope') === 'group' ? 'group' : 'kind';
	let info: Unsubscribe | null = null;
	let failure: 'gone' | 'error' | null = null;
	try {
		info = await getUnsubscribe(link);
	} catch (e) {
		failure = e instanceof ApiError && (e.status === 410 || e.status === 422) ? 'gone' : 'error';
	}
	return { link, scope, info, failure };
};

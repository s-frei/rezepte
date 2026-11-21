import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { getLocalTimeZone, today } from '@internationalized/date';
import { ApiError } from '#lib/api/client.js';
import { getPlan, listPlans } from '#lib/api/mealplan.js';
import { planHref } from '#lib/mealplan/href.js';
import { mondayOf, weekDays } from '#lib/mealplan/week.js';
import type { PageLoad } from './$types';

/** The Monday of `from`; a missing or broken one is this week's. */
function monday(from: string | null, now: string): string {
	try {
		if (from) return mondayOf(from);
	} catch {
		// Not a date: this week.
	}
	return mondayOf(now);
}

/**
 * `/plans/<id>?from=<Monday>`: one plan on the week of `from`, else this one;
 * any other `from` (a Sunday, this week's Monday, garbage) moves on to that
 * address. A plan the reader cannot see (gone, left, removed, a stale
 * bookmark) leads to the overview, which says so (`?gone`); one they are
 * only invited to, to the overview under its invitation card. Never the
 * recipes' 404 page, and no toast here: a load also runs to preload a link.
 */
export const load: PageLoad = async ({ params, url }) => {
	const now = today(getLocalTimeZone()).toString();
	const asked = url.searchParams.get('from');
	const from = monday(asked, now);
	const canonical = from === mondayOf(now) ? null : from;
	// A stamp's jump (`day`, `entry`) survives the move to the canonical address.
	if (asked !== canonical)
		redirect(
			307,
			planHref(params.id, canonical ?? undefined, {
				day: url.searchParams.get('day'),
				entry: url.searchParams.get('entry')
			})
		);
	try {
		const plan = await getPlan(params.id, { from, to: weekDays(from)[6] });
		return { plan, today: now };
	} catch (err) {
		if (!(err instanceof ApiError && err.status === 404)) throw err;
		const invited = await listPlans()
			.then((r) => r.invitations.some((i) => i.planId === params.id))
			.catch(() => false);
		redirect(307, invited ? resolve('/plans') : `${resolve('/plans')}?gone`);
	}
};

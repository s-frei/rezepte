import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { getLocalTimeZone, parseDate, today } from '@internationalized/date';
import { ApiError } from '#lib/api/client.js';
import { getPlan, listPlans } from '#lib/api/mealplan.js';
import { planHref } from '#lib/mealplan/href.js';
import { mondayOf, shiftDay } from '#lib/mealplan/week.js';
import { narrow } from '#lib/ui/narrow.js';
import type { PageLoad } from './$types';

/** `from` as a day; null when missing or not a date. */
function day(from: string | null): string | null {
	try {
		return from ? parseDate(from).toString() : null;
	} catch {
		return null;
	}
}

/**
 * `/plans/<id>?from=<day>`: one plan, shown from `from`. A desktop's strip
 * starts at that day, a phone shows the week it falls in; without `from`
 * the strip has today in its middle and a phone shows this week. A `from`
 * that is no date moves on to the address without it. A phone loads its
 * week; from `md` the load holds six weeks around it (two before its week,
 * three after, within the server's 62 days), so the strip scrolls without a
 * request per day, and the page loads the next six near their end. A plan
 * the reader cannot see (gone, left, removed, a stale bookmark) leads to
 * the overview, which says so (`?gone`); one they are only invited to, to
 * the overview under its invitation card. Never the recipes' 404 page, and
 * no toast here: a load also runs to preload a link.
 */
export const load: PageLoad = async ({ params, url, untrack }) => {
	const now = today(getLocalTimeZone()).toString();
	// The query alone does not run the load again: the strip writes its
	// first day into `from` as it rests (`goto`), and asks for a load itself
	// (`refreshAll`) when it nears the ends of its six weeks; so does the
	// phone's pager.
	const [asked, jumpDay, entry] = untrack(() =>
		['from', 'day', 'entry'].map((key) => url.searchParams.get(key))
	);
	const start = day(asked);
	// A stamp's jump (`day`, `entry`) survives the move to the address without `from`.
	if (asked !== null && start === null)
		redirect(307, planHref(params.id, undefined, { day: jumpDay, entry }));
	const monday = mondayOf(start ?? now);
	// The page's own breakpoint (`narrow`): read once per load, a resize loads again.
	const wide = !narrow.current;
	try {
		const plan = await getPlan(
			params.id,
			wide
				? { from: shiftDay(monday, -14), to: shiftDay(monday, 27) }
				: { from: monday, to: shiftDay(monday, 6) }
		);
		return { plan, today: now, start };
	} catch (err) {
		if (!(err instanceof ApiError && err.status === 404)) throw err;
		const invited = await listPlans()
			.then((r) => r.invitations.some((i) => i.planId === params.id))
			.catch(() => false);
		redirect(307, invited ? resolve('/plans') : `${resolve('/plans')}?gone`);
	}
};

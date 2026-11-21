import { getLocalTimeZone, today } from '@internationalized/date';
import { isSignedOut } from '#lib/api/client.js';
import { getPlansWeek } from '#lib/api/mealplan.js';
import { mondayOf, weekDays } from '#lib/mealplan/week.js';
import type { PageLoad } from './$types';

/**
 * The plans overview: every plan the reader is in with this week of it, and
 * the invitations, in one request. A 401 is the page's to redirect
 * (`signedOut`), as on the lists: a load also runs as a tap's preload.
 */
export const load: PageLoad = async () => {
	const now = today(getLocalTimeZone()).toString();
	const first = mondayOf(now);
	try {
		const week = await getPlansWeek({ from: first, to: weekDays(first)[6] }, { redirect: false });
		return { plans: week.plans, invitations: week.invitations, today: now, signedOut: false };
	} catch (error) {
		if (!isSignedOut(error)) throw error;
		return { plans: [], invitations: [], today: now, signedOut: true };
	}
};

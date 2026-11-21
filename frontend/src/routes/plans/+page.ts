import { getLocalTimeZone, today } from '@internationalized/date';
import { getPlansWeek } from '#lib/api/mealplan.js';
import { mondayOf, weekDays } from '#lib/mealplan/week.js';
import type { PageLoad } from './$types';

/**
 * The plans overview: every plan the reader is in with this week of it, and
 * the invitations, in one request.
 */
export const load: PageLoad = async () => {
	const now = today(getLocalTimeZone()).toString();
	const first = mondayOf(now);
	const week = await getPlansWeek({ from: first, to: weekDays(first)[6] });
	return { plans: week.plans, invitations: week.invitations, today: now };
};

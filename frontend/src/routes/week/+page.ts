import { error } from '@sveltejs/kit';
import { getLocalTimeZone, today } from '@internationalized/date';
import { ApiError } from '#lib/api/client.js';
import { getPlan, listPlans } from '#lib/api/mealplan.js';
import { mondayOf, weekDays } from '#lib/mealplan/week.js';
import { m } from '#lib/paraglide/messages.js';
import { lastPlan } from '#lib/shell.svelte.js';
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
 * `/week?plan=<id>&from=<day>`, both optional: the plan opened last (while
 * the reader is still in it), else the personal plan, which the plan list
 * creates on first call; the week of `from`, else this one. A plan id the
 * reader cannot see is SvelteKit's 404, like a list's; one they are invited
 * to opens the default plan, under the invitation's card.
 */
export const load: PageLoad = async ({ url }) => {
	const now = today(getLocalTimeZone()).toString();
	const from = monday(url.searchParams.get('from'), now);
	const days = weekDays(from);
	const { plans, invitations } = await listPlans();
	// A plan still waiting for the reader's answer (the invitation's link)
	// opens the default plan: its invitation card waits above the week.
	const param = url.searchParams.get('plan');
	const asked = invitations.some((i) => i.planId === param) ? null : param;
	const last = lastPlan();
	const id =
		asked ??
		plans.find((p) => p.id === last)?.id ??
		plans.find((p) => p.personal)?.id ??
		plans[0].id;
	try {
		const plan = await getPlan(id, { from, to: days[6] });
		return { plans, invitations, plan, today: now };
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) error(404, m.not_found());
		throw err;
	}
};

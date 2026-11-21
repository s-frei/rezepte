import { resolve } from '$app/paths';

/**
 * Address of one plan, on the week of `from` (a Monday) when given; `day`
 * and `entry` make the page show that day (unfolded on a phone) and mark
 * that meal.
 */
export function planHref(
	id: string,
	from?: string,
	jump: { day?: string | null; entry?: string | null } = {}
): string {
	const base = resolve('/plans/[id]', { id });
	const query = new URLSearchParams();
	if (from) query.set('from', from);
	if (jump.day) query.set('day', jump.day);
	if (jump.entry) query.set('entry', jump.entry);
	return query.size ? `${base}?${query}` : base;
}

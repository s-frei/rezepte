import type { PlanRef } from '#lib/api/shopping.js';
import { firstName } from '#lib/components/switch/switch.js';
import { planHref } from '#lib/mealplan/href.js';
import { mondayOf } from '#lib/mealplan/week.js';
import { m } from '#lib/paraglide/messages.js';
import { getLocale } from '#lib/paraglide/runtime.js';

/**
 * A plan as a list names it: "Haushalt" or "Mein Plan" for a reader in it,
 * a link to the plan (`href`); for anyone else who put the recipe on the
 * list ("Geplant von Mila", "Geplant von dir", plain "Geplant" once that
 * account is gone), never a link and never the plan's name.
 */
export type PlanStamp = {
	key: string;
	label: string;
	href: string | null;
	member: boolean;
	personal: boolean;
};

/** A list entry's link to its plan: `PlanRef` plus the planned day where it has one. */
export type Planned = PlanRef & { planDay?: string | null };

/**
 * The plan at a day, and with `entry` that meal: the plan page unfolds a
 * folded day, scrolls to it and marks it. `thisMonday` leaves the current
 * week's `from` out, as the plan's own addresses do.
 */
export function planJumpHref(
	planId: string,
	day: string | null,
	entry: string | null,
	thisMonday: string
): string {
	const from = day ? mondayOf(day) : undefined;
	return planHref(planId, from === thisMonday ? undefined : from, { day, entry });
}

/** The stamp for one entry's plan; null for a recipe added by hand. `entry` makes the link mark the meal. */
export function planStamp(
	ref: Planned,
	me: string | undefined,
	thisMonday: string,
	entry = true
): PlanStamp | null {
	if (!ref.planId) return null;
	if (ref.planMember)
		return {
			key: ref.planId,
			label: ref.planPersonal ? m.plan_personal() : (ref.planName ?? ''),
			href: planJumpHref(
				ref.planId,
				ref.planDay ?? null,
				entry ? ref.planEntryId : null,
				thisMonday
			),
			member: true,
			personal: ref.planPersonal
		};
	const by = ref.planAddedBy;
	const label = !by
		? m.link_planned()
		: by.id === me
			? m.link_planned_by_you()
			: m.link_planned_by({ name: firstName(by.displayName) });
	// Plans the reader is not in are told apart only by who put them on the list.
	return { key: `by|${label}`, label, href: null, member: false, personal: false };
}

/** "Mein Plan" first, then the reader's plans by name, plans they are not in last. */
const rank = (s: PlanStamp) => (s.personal ? 0 : s.member ? 1 : 2);
const byRank = (a: PlanStamp, b: PlanStamp) =>
	rank(a) - rank(b) || a.label.localeCompare(b.label, getLocale());

/** Each plan the entries came from once, linked to its earliest planned day. */
export function uniquePlans(
	refs: Planned[],
	me: string | undefined,
	thisMonday: string
): PlanStamp[] {
	const first = new Map<string, Planned>();
	const stamps = new Map<string, PlanStamp>();
	for (const ref of refs) {
		const stamp = planStamp(ref, me, thisMonday, false);
		if (!stamp) continue;
		const seen = first.get(stamp.key);
		if (seen && !(ref.planDay && (!seen.planDay || ref.planDay < seen.planDay))) continue;
		first.set(stamp.key, ref);
		stamps.set(stamp.key, stamp);
	}
	return [...stamps.values()].sort(byRank);
}

/** Dates as written (`YYYY-MM-DD`): a locale's collation would sort '~' or '9' by other rules. */
const order = (a: string, b: string) => (a < b ? -1 : a > b ? 1 : 0);

export type PlanGroup<T> = {
	key: string;
	/** Its head links to the group's first day. */
	plan: PlanStamp | null;
	/** The week's Monday; '' for the tray and the ones added by hand. */
	monday: string;
	items: T[];
};

/**
 * Entries by plan and week, by day inside (the tray last): the order of
 * `uniquePlans`, earlier weeks first, then the ones added by hand
 * (`plan: null`).
 */
export function groupByPlan<T extends Planned>(
	items: T[],
	me: string | undefined,
	thisMonday: string
): PlanGroup<T>[] {
	const groups = new Map<string, PlanGroup<T>>();
	for (const item of items) {
		const plan = planStamp(item, me, thisMonday, false);
		const monday = item.planDay ? mondayOf(item.planDay) : '';
		const key = plan ? `${plan.key}|${monday}` : 'single';
		let group = groups.get(key);
		if (!group) {
			group = { key, plan, monday, items: [] };
			groups.set(key, group);
		}
		group.items.push(item);
	}
	const all = [...groups.values()];
	for (const g of all) {
		g.items.sort((a, b) => order(a.planDay ?? '9', b.planDay ?? '9'));
		// The head leads to the group's first day.
		g.plan &&= planStamp(g.items[0], me, thisMonday, false);
	}
	// Weeks by their Monday, so January follows December; the tray ('') after them.
	return all.sort(
		(a, b) =>
			Number(!a.plan) - Number(!b.plan) ||
			(a.plan && b.plan ? byRank(a.plan, b.plan) : 0) ||
			Number(!a.monday) - Number(!b.monday) ||
			order(a.monday, b.monday)
	);
}

/** "„Wocheneinkauf“ und „Markt Samstag“". */
export function quotedList(names: string[]): string {
	return new Intl.ListFormat(getLocale(), { type: 'conjunction' }).format(
		names.map((name) => m.quoted({ name }))
	);
}

/** Lists by name, so none moves when counts change: an entry's stamps, its popover and its ⋯. */
export function byListName<T extends { name: string }>(lists: T[]): T[] {
	return [...lists].sort((a, b) => a.name.localeCompare(b.name, getLocale()));
}

/**
 * A plan's own lists (the overview card, the plan's head): the one with the
 * most of the plan's entries first, then by name, so a one-meal list never
 * crowds out the week's main one on a one-line row.
 */
export function byPlanUse<T extends { name: string; entries?: number }>(lists: T[]): T[] {
	return byListName(lists).sort((a, b) => (b.entries ?? 0) - (a.entries ?? 0));
}

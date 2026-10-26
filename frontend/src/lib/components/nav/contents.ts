import type UserRound from '@lucide/svelte/icons/user-round';
import type { ResolvedPathname } from '$app/types';

/** A Lucide icon component, as the nav lists draw them. */
export type IconComponent = typeof UserRound;

/**
 * One entry of a phone's running head and its contents sheet: a section of
 * a long page, or a page of an area. The contents sheet lists every entry,
 * the running head names the current one.
 */
export type ContentsEntry = {
	id: string;
	label: string;
	/**
	 * What sits right of the dotted leader, where a cookbook prints the page
	 * number: a count, a title, a setting. Optional.
	 */
	summary?: string;
	/** The summary is a placeholder (an untitled recipe) and reads as one. */
	placeholder?: boolean;
	/**
	 * Set for a page: the entry is a link. Unset: the sheet calls `onselect`.
	 * Typed as resolved, so the caller has already run it through `resolve`.
	 */
	href?: ResolvedPathname;
	/** The heading of the group the entry belongs to, as settings splits its pages. Optional. */
	group?: string;
	/** Drawn before the label, as the settings pages carry one. Optional. */
	icon?: IconComponent;
};

/**
 * Consecutive entries with the same `group`, in order: what the lists draw
 * as named groups under a rule. Entries without one form a single run.
 */
export function byGroup<T extends { id: string; group?: string }>(
	entries: T[]
): { label: T['group']; entries: T[] }[] {
	const runs: { label: T['group']; entries: T[] }[] = [];
	for (const entry of entries) {
		const last = runs.at(-1);
		if (last && last.label === entry.group) last.entries.push(entry);
		else runs.push({ label: entry.group, entries: [entry] });
	}
	return runs;
}

/**
 * An action that is not an entry - signing out - listed under a rule at the
 * foot of the contents sheet.
 */
export type ContentsAction = {
	id: string;
	label: string;
	tone?: 'destructive';
	icon?: IconComponent;
	onselect: () => void;
};

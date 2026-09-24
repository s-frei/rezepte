import type { ResolvedPathname } from '$app/types';

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
};

/**
 * An action that is not an entry - signing out - listed under a rule at the
 * foot of the contents sheet.
 */
export type ContentsAction = {
	id: string;
	label: string;
	tone?: 'destructive';
	onselect: () => void;
};

import { resolve } from '$app/paths';

/** Address of one list. */
export const listHref = (id: string) => resolve('/lists/[id]', { id });

/** Address of one list's shopping mode. */
export const shopHref = (id: string) => resolve('/lists/[id]/shop', { id });

/** A list at the block of one plan entry: the list scrolls to it and marks it. */
export const listEntryHref = (id: string, planEntryId: string) =>
	`${listHref(id)}?${new URLSearchParams({ entry: planEntryId })}`;

import { resolve } from '$app/paths';

/** Address of one list. */
export const listHref = (id: string) => resolve('/lists/[id]', { id });

/** Address of one list's shopping mode. */
export const shopHref = (id: string) => resolve('/lists/[id]/shop', { id });

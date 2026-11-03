import { resolve } from '$app/paths';

/** Address of one list. */
export const listHref = (id: string) => resolve('/lists/[id]', { id });

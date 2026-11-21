import { resolve } from '$app/paths';

/** Address of one plan, on the week of `from` (a Monday) when given. */
export function planHref(id: string, from?: string): string {
	const base = resolve('/plans/[id]', { id });
	return from ? `${base}?${new URLSearchParams({ from })}` : base;
}

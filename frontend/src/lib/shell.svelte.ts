import type { Snippet } from 'svelte';

/**
 * Lets a page contribute to the desktop top bar - breadcrumb text next to
 * the logo, and a snippet of actions rendered where the top bar's default
 * buttons ("Importieren" / "+ Neues Rezept") sit - without threading props
 * through `+layout.svelte` -> `AppShell` -> `TopBar`.
 *
 * A page sets these from an `$effect` and must reset them in its cleanup so
 * they don't leak into the next page it navigates to:
 *
 * ```ts
 * $effect(() => {
 * 	shell.breadcrumb = recipe.title;
 * 	shell.actions = pageActions; // a snippet declared in the page's markup
 * 	return () => {
 * 		shell.breadcrumb = undefined;
 * 		shell.actions = undefined;
 * 	};
 * });
 * ```
 */
export const shell = $state<{ breadcrumb?: string; actions?: Snippet }>({
	breadcrumb: undefined,
	actions: undefined
});

/**
 * Routes that own the bottom of a phone screen for their own action bar and
 * therefore hide the bottom nav: the detail page (cook mode CTA), and both
 * editors (save bar). They all offer their own way back, so navigation is
 * never lost - see `docs/memory/content/architecture/design-system.mdx`.
 */
const ROUTES_WITHOUT_BOTTOM_NAV = new Set([
	'/recipes/[slug]',
	'/recipes/[slug]/edit',
	'/recipes/new'
]);

export function showsBottomNav(routeId: string | null): boolean {
	return routeId === null || !ROUTES_WITHOUT_BOTTOM_NAV.has(routeId);
}

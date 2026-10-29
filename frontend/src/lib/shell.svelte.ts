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
 * The phone's bottom nav, as far as what sits above it needs to know: whether
 * it is docked to the edge or floats (smaller, lifted), and whether it is
 * moving `still` - placed without its morph after a navigation. The timer
 * player follows both, so the two stay one stack. Only `BottomNav` writes
 * them. `typing`: a field has the focus, so a phone's software keyboard is
 * up and both step aside; `AppShell` tracks it.
 */
export const bottomNav = $state({ docked: false, still: false, typing: false });

/** The places the phone's bottom nav can mark as current. */
export type NavPlace = 'recipes' | 'you';

/**
 * Every page route and what the phone's bottom nav does on it. The bar shows
 * exactly when one of its places is current, so a bar with nothing marked
 * can never appear. `null` routes go without it: the detail page and both
 * editors own the screen with their own top bar and their own way back
 * ("one bar per phone screen"), and the bare routes - login, cook mode, a
 * public share - render no shell at all. `shell.test.ts` fails when a route
 * is missing here, so a new one has to decide. See
 * `docs/memory/content/architecture/design-system.mdx`.
 */
export const NAV_PLACES: Record<string, NavPlace | null> = {
	'/': 'recipes',
	// Share → Rezepte lands here and moves on to the overview at once.
	'/import': 'recipes',
	'/settings': 'you',
	'/settings/api': 'you',
	'/settings/mail': 'you',
	'/settings/notifications': 'you',
	'/settings/shares': 'you',
	'/settings/sign-in': 'you',
	'/settings/transfer': 'you',
	'/settings/users': 'you',
	'/recipes/new': null,
	'/recipes/[slug]': null,
	'/recipes/[slug]/edit': null,
	'/recipes/[slug]/cook': null,
	'/s/[token]': null,
	'/login': null,
	'/welcome': null,
	'/forgot-password': null,
	'/confirm-email': null,
	'/unsubscribe': null
};

/**
 * The bottom nav's current place on a route, or `null` for no bar. A path no
 * route matches has no id, and the error page owns the screen on any route.
 */
export function navPlace(routeId: string | null, hasError: boolean): NavPlace | null {
	if (hasError || routeId === null) return null;
	return NAV_PLACES[routeId] ?? null;
}

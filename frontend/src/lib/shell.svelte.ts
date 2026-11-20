import type { Snippet } from 'svelte';
import { resolve } from '$app/paths';
import { load, save } from '#lib/storage.js';

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
export const shell = $state<{
	breadcrumb?: string;
	actions?: Snippet;
	add?: { label: string; open: () => void; readonly expanded: boolean };
}>({
	breadcrumb: undefined,
	actions: undefined,
	/**
	 * What a phone's "+" does on this page instead of opening the new-things
	 * sheet: its name, what it opens and whether that is open (a getter, so
	 * the button follows the page's state). Reset like `actions`.
	 */
	add: undefined
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
export type NavPlace = 'recipes' | 'lists' | 'you';

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
	'/lists': 'lists',
	'/lists/new': 'lists',
	'/lists/[id]': 'lists',
	'/lists/[id]/shop': null,
	'/week': 'lists',
	'/week/new': 'lists',
	'/settings': 'you',
	'/settings/api': 'you',
	'/settings/mail': 'you',
	'/settings/notifications': 'you',
	'/settings/shares': 'you',
	'/settings/sign-in': 'you',
	'/settings/transfer': 'you',
	'/settings/users': 'you',
	'/settings/users/[username]': 'you',
	'/recipes/new': null,
	'/recipes/[slug]': null,
	'/recipes/[slug]/edit': null,
	'/recipes/[slug]/cook': null,
	'/s/[token]': null,
	'/login': null,
	'/welcome': null,
	'/forgot-password': null,
	'/confirm-email': null,
	'/unsubscribe': null,
	'/switch': null
};

/**
 * The bottom nav's current place on a route, or `null` for no bar. A path no
 * route matches has no id, and the error page owns the screen on any route.
 */
export function navPlace(routeId: string | null, hasError: boolean): NavPlace | null {
	if (hasError || routeId === null) return null;
	return NAV_PLACES[routeId] ?? null;
}

/** The two sides of the lists place: the shopping lists and the week's meal plan. */
export type PlaceSide = 'lists' | 'week';

const SIDE_KEY = 'rezepte:place-side';

/**
 * The side the place opened on last, per device like the list view: the nav
 * links to it, so the place comes back where it was left. A first visit opens
 * the lists.
 */
export const placeSide = $state({
	current: (load(SIDE_KEY) === 'week' ? 'week' : 'lists') as PlaceSide
});

export function setPlaceSide(side: PlaceSide): void {
	placeSide.current = side;
	save(SIDE_KEY, side);
}

/** Where the place's nav link goes: the side last used. */
export const placeHref = (): string => resolve(placeSide.current === 'week' ? '/week' : '/lists');

const PLAN_KEY = 'rezepte:last-plan';

/** The plan the week opened on last on this device; the week reopens on it while the reader is still in it. */
export const lastPlan = (): string | null => load(PLAN_KEY);

export const rememberPlan = (id: string): void => save(PLAN_KEY, id);

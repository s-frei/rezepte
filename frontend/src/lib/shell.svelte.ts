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

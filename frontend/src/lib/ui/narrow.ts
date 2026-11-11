import { MediaQuery } from 'svelte/reactivity';

/**
 * Below `md`, the bottom nav's breakpoint, by width alone: a dialog there is
 * a bottom sheet (`ResponsiveDialog`), a person card a sheet (`PersonPeek`).
 * Created on first read, so importing it outside a browser costs nothing.
 */
let query: MediaQuery | undefined;

export const narrow = {
	get current(): boolean {
		query ??= new MediaQuery('max-width: 767.98px');
		return query.current;
	}
};

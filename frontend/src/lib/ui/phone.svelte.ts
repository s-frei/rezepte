import { MediaQuery } from 'svelte/reactivity';

/**
 * A touch screen with little room - a phone, upright below `md:` or on its
 * side under 32rem tall - gets sheets; everything else gets cards at the
 * pointer. By pointer as well as width: a narrow window with a mouse has no
 * taps to open a sheet with.
 */
let query: MediaQuery | undefined;

export const phone = {
	get value(): boolean {
		query ??= new MediaQuery(
			'(pointer: coarse) and ((max-width: 47.99rem) or (max-height: 32rem))'
		);
		return query.current;
	}
};

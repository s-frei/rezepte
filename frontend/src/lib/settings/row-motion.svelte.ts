import { prefersReducedMotion } from 'svelte/motion';

/**
 * How long a settings list's row takes to open or close. Only the change
 * handed to `play` - an add or a remove the viewer just made - animates; a
 * load or a filter swaps the rows at once. The rows pass `duration` to
 * their `transition:`, whose parameters Svelte reads when it starts, in the
 * update right after the change, so a frame later it is back to 0.
 */
export function rowMotion(ms = 200) {
	let duration = $state(0);
	return {
		get duration() {
			return duration;
		},
		play(change: () => void) {
			duration = prefersReducedMotion.current ? 0 : ms;
			change();
			requestAnimationFrame(() => (duration = 0));
		}
	};
}

<script lang="ts" module>
	export type RequiredState = 'empty' | 'filled' | 'error';

	/** The error wins: a filled field can still be wrong, and then it is the error that matters. */
	export function requiredState(filled: boolean, error: boolean): RequiredState {
		return error ? 'error' : filled ? 'filled' : 'empty';
	}
</script>

<script lang="ts">
	import { prefersReducedMotion } from 'svelte/motion';
	import { draw } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';

	let { state }: { state: RequiredState } = $props();

	// Amber while there is still something to fill in, a green check once
	// there is not, red only after a save found a problem - never red before
	// the person has done anything, which would read as an error they had not
	// made yet.
	const tone = {
		empty: 'text-primary',
		filled: 'text-success-foreground',
		error: 'text-destructive'
	};
</script>

<!-- Hidden from assistive tech: the field carries `aria-required`, and a
     screen reader reading "star" or "check" after every label would add
     nothing.

     The check takes the star's place, ink on ink: the star stays in the
     flow, only invisible, so it alone gives the mark its width and baseline
     and the label never moves. The check hangs in a zero-width box in front
     of it, so its ink starts where the star's does (its viewBox is cut to
     the stroke), and `vertical-align` puts its middle where the star's ink
     sits (see the style below). -->
<span
	aria-hidden="true"
	data-required-mark
	data-state={state}
	class="inline-block transition-colors motion-reduce:transition-none {tone[state]}"
	>{#if state === 'filled'}<span class="inline-block w-0"
			><!-- Lucide's check turned around so it draws from the short stroke,
			     as Checkbox's does. The transition is local, so a field that is
			     already filled when the form opens shows its check at once; only
			     filling it in draws it. --><svg
				viewBox="2.5 4.5 19 14"
				fill="none"
				stroke="currentColor"
				stroke-width="3"
				stroke-linecap="round"
				stroke-linejoin="round"
				class="inline-block h-[calc(0.8em*14/24)] w-[calc(0.8em*19/24)] max-w-none overflow-visible"
			>
				<path
					d="m4 12 5 5L20 6"
					in:draw={{ duration: prefersReducedMotion.current ? 0 : 300, easing: cubicOut }}
				/>
			</svg></span
		>{/if}<span class={state === 'filled' ? 'invisible' : ''}>*</span></span
>

<style>
	/* The star hangs from the cap line, its middle at about 0.86 of the cap
	   height in Karla and Literata alike; one em value missed one font or
	   the other by most of a pixel. The em line is for browsers without
	   the `cap` unit. */
	svg {
		vertical-align: calc(0.58em - 0.4em * 14 / 24);
		vertical-align: calc(0.86cap - 0.4em * 14 / 24);
	}
</style>

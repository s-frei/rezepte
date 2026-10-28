<script lang="ts">
	import type { IngredientGroup, Step, StepTime } from '$lib/api/recipes';
	import Timer from '@lucide/svelte/icons/timer';
	import TimerRing from '$lib/components/timer/TimerRing.svelte';
	import { m } from '$lib/paraglide/messages';
	import { segmentStep } from '$lib/recipe/references';
	import { fraction, type Timer as CookTimer } from '$lib/recipe/timers';
	import { timers } from '$lib/recipe/timers.svelte';

	let {
		step,
		groups,
		servings,
		baseServings,
		onstarttime,
		activeTimer
	}: {
		step: Step;
		groups: IngredientGroup[];
		/** The servings the user chose. */
		servings: number;
		/** The recipe's stored servings - what every quantity is written for. */
		baseServings: number;
		/** Cook mode: every time becomes a button that starts its timer. */
		onstarttime?: (time: StepTime) => void;
		/** Cook mode: the timer a time already runs, drawn as a live ring. */
		activeTimer?: (time: StepTime) => CookTimer | undefined;
	} = $props();

	const segments = $derived(segmentStep(step, groups, servings, baseServings));

	const timeClass =
		'inline-block max-w-full font-semibold hyphens-none text-time-foreground underline decoration-dotted decoration-[0.08em] underline-offset-[0.22em]';

	/**
	 * A time with each number tied to the word after it by a no-break space:
	 * "20 bis 25 Minuten" may wrap, but never between "25" and "Minuten".
	 */
	const tieNumbers = (text: string) => text.replace(/(\p{N}) /gu, '$1\u00a0');

	// No ARIA: the visible text is already the accessible text and reads
	// correctly as "Saft (400 ml) mit Zucker (100 g) aufkochen". A label would
	// only get in the way. Sizes stay in em so the annotation grows with cook
	// mode's display type. A quantity never breaks across lines: "(100" at the
	// end of one line and "g)" at the start of the next is the one thing a cook
	// glancing at the step must not have to piece together, and at cook mode's
	// size it happened on every longer step. A time is one inline block, so the
	// clock stays with its number; never hyphenated, as Safari would split "Mi-nuten".
	// In cook mode the time is a button with the same look; the `::after`
	// makes its target taller without moving a letter - only taller, since a
	// sideways overhang would widen the step past its column.
	// (Comment kept here, not as an HTML comment above the markup, so it never
	// ships into the rendered DOM.)
</script>

{#snippet mark(time: StepTime, text: string)}{@const active =
		activeTimer?.(time)}{#if active}<TimerRing
			fraction={fraction(active, timers.now)}
			class="mr-[0.15em] inline size-[0.82em] align-[-0.08em]"
		/>{:else}<Timer
			class="mr-[0.15em] inline size-[0.82em] align-[-0.08em]"
			aria-hidden="true"
		/>{/if}{tieNumbers(text)}{/snippet}

{#each segments as segment, index (index)}{#if segment.time && onstarttime}<button
			type="button"
			data-time
			aria-label={m.timer_start({ phrase: segment.time.phrase })}
			onclick={() => onstarttime(segment.time!)}
			class="{timeClass} relative cursor-pointer rounded-word after:absolute after:inset-x-0 after:-inset-y-2 after:content-[''] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			>{@render mark(segment.time, segment.text)}</button
		>{:else if segment.time}<span data-time class={timeClass}
			>{@render mark(segment.time, segment.text)}</span
		>{:else}{segment.text}{#if segment.quantity}<span
				class="ml-[0.2em] font-semibold whitespace-nowrap text-primary">({segment.quantity})</span
			>{/if}{/if}{/each}

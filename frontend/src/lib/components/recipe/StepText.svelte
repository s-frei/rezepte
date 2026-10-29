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

	const timeClass = 'max-w-full font-semibold hyphens-none text-time-foreground';

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
	// Elsewhere it is underlined as a mark. In cook mode it is a button and
	// looks like one: a teal chip, the timers' color, ending in a play disc
	// (the triangle a hole, so the hover tint shows through) until its timer
	// runs. The chip is a flex row, so its icons are centered on the text by
	// flex alignment rather than by a baseline offset, which every engine
	// rounds differently: the text is trimmed by `text-box` to the digits'
	// cap height and baseline, padded to be at least as tall as the icons so
	// it sets the row's height, and baseline-aligned, which makes its baseline
	// the chip's in the sentence. Where the chip is wider than the line, the
	// row wraps, centered, so the icons stack above and below the text (320 px
	// at the largest size); the text never breaks inside a word. The stopwatch's body sits below its box's
	// center, under the knob; lifted by half that, it reads as centered. The `::after` makes its target taller without
	// moving a letter - only taller, since a sideways overhang would widen the
	// step past its column.
	// (Comment kept here, not as an HTML comment above the markup, so it never
	// ships into the rendered DOM.)
</script>

{#snippet clock(time: StepTime, iconClass: string)}{@const active =
		activeTimer?.(time)}{#if active}<TimerRing
			fraction={fraction(active, timers.now)}
			class="mr-[0.15em] size-[0.82em] {iconClass}"
		/>{:else}<Timer
			class="mr-[0.15em] size-[0.82em] -translate-y-[0.035em] {iconClass}"
			aria-hidden="true"
		/>{/if}{/snippet}

{#each segments as segment, index (index)}{#if segment.time && onstarttime}<button
			type="button"
			data-time
			aria-label={m.timer_start({ phrase: segment.time.phrase })}
			onclick={() => onstarttime(segment.time!)}
			class="{timeClass} relative inline-flex cursor-pointer flex-wrap items-center justify-center rounded-time bg-user-teal hover-tint px-[0.28em] py-[0.12em] after:absolute after:inset-x-0 after:-inset-y-2 after:content-[''] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			>{@render clock(segment.time, 'shrink-0')}<span
				class="self-baseline py-[0.08em] [text-box:trim-both_cap_alphabetic]"
				>{tieNumbers(segment.text)}</span
			>{#if !activeTimer?.(segment.time)}<svg
					viewBox="0 0 24 24"
					fill="currentColor"
					class="ml-[0.26em] size-[0.86em] shrink-0"
					aria-hidden="true"
					><path
						fill-rule="evenodd"
						d="M12 0a12 12 0 1 0 0 24a12 12 0 1 0 0-24ZM9.5 7v10l8-5Z"
					/></svg
				>{/if}</button
		>{:else if segment.time}<span
			data-time
			class="{timeClass} inline-block underline decoration-dotted decoration-[0.08em] underline-offset-[0.22em]"
			>{@render clock(segment.time, 'inline align-[-0.08em]')}{tieNumbers(segment.text)}</span
		>{:else}{segment.text}{#if segment.quantity}<span
				class="ml-[0.2em] font-semibold whitespace-nowrap text-primary">({segment.quantity})</span
			>{/if}{/if}{/each}

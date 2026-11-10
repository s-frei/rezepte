<script lang="ts">
	import type { IngredientGroup, Step, StepPhoto, StepTime } from '#lib/api/recipes.js';
	import ImageIcon from '@lucide/svelte/icons/image';
	import Timer from '@lucide/svelte/icons/timer';
	import TimerRing from '#lib/components/timer/TimerRing.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { segmentStep, type Segment } from '#lib/recipe/references.js';
	import { fraction, type Timer as CookTimer } from '#lib/recipe/timers.js';
	import { timers } from '#lib/recipe/timers.svelte.js';

	let {
		step,
		groups,
		servings,
		baseServings,
		onstarttime,
		activeTimer,
		onphoto
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
		/** A photo phrase becomes a trigger that hands over its photo and itself; without it the phrase is plain text (share card, print). */
		onphoto?: (photo: StepPhoto, trigger: HTMLElement) => void;
	} = $props();

	const uid = $props.id();

	const segments = $derived(segmentStep(step, groups, servings, baseServings));

	/**
	 * The segments with each photo phrase gathered: consecutive segments that
	 * carry the same photo are one trigger.
	 */
	const runs = $derived(
		segments.reduce<{ photo?: StepPhoto; segments: Segment[] }[]>((acc, segment) => {
			const last = acc[acc.length - 1];
			if (segment.photo && last?.photo === segment.photo) last.segments.push(segment);
			else acc.push({ photo: segment.photo, segments: [segment] });
			return acc;
		}, [])
	);

	const timeClass = 'max-w-full font-semibold hyphens-none text-time-foreground';

	/**
	 * A time with each number tied to the word after it by a no-break space:
	 * "20 bis 25 Minuten" may wrap, but never between "25" and "Minuten".
	 */
	const tieNumbers = (text: string) => text.replace(/(\p{N}) /gu, '$1\u00a0');

	// No ARIA on the text: the visible text is already the accessible text and reads
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
	// A photo trigger is named by a hidden "Show photo:" plus its own content,
	// so its quantities are read too - an `aria-label` would replace them. It
	// opens like a native button: Enter on keydown, Space on keyup (on keydown
	// focus moved into the card and the keyup clicked "View large" there).
	// Only the icon and the first letter are kept together: a whole first
	// word in `nowrap` could no longer hyphenate and overflowed cook mode's
	// column at its largest size.
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

{#snippet piece(segment: Segment)}{#if segment.time && onstarttime}<button
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
			>{/if}{/if}{/snippet}

{#each runs as run, index (index)}{#if run.photo && onphoto}{@const photo =
			run.photo}{@const first = run.segments[0]}{@const head = String.fromCodePoint(
			first.text.codePointAt(0)!
		)}<span hidden id="{uid}-{index}-label">{m.step_photo_show()}</span><span
			role="button"
			tabindex="0"
			id="{uid}-{index}"
			aria-labelledby="{uid}-{index}-label {uid}-{index}"
			data-step-photo
			onclick={(event) => onphoto(photo, event.currentTarget)}
			onkeydown={(event) => {
				if (event.key === ' ') event.preventDefault();
				if (event.key !== 'Enter') return;
				event.preventDefault();
				onphoto(photo, event.currentTarget);
			}}
			onkeyup={(event) => {
				if (event.key !== ' ') return;
				event.preventDefault();
				onphoto(photo, event.currentTarget);
			}}
			class="photo-link cursor-pointer text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			><span class="whitespace-nowrap"
				><ImageIcon
					class="mr-[0.25em] inline size-[0.8em] align-[-0.06em] text-text-muted print:hidden"
					aria-hidden="true"
				/>{head}</span
			>{@render piece({
				...first,
				text: first.text.slice(head.length)
			})}{#each run.segments.slice(1) as segment, at (at)}{@render piece(segment)}{/each}</span
		>{:else}{#each run.segments as segment, at (at)}{@render piece(segment)}{/each}{/if}{/each}

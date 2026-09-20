<script lang="ts">
	import { Slider as BitsSlider } from 'bits-ui';

	// A single-thumb slider over a small, fixed number of stops: `value` is
	// the position on the scale, not the quantity behind it, so the stops sit
	// evenly on the track however far apart their values are. The caller maps
	// position to meaning and hands back the text for each - this component
	// knows the look and the mechanics, nothing about minutes.
	let {
		value = $bindable(),
		max,
		label,
		valueText,
		muted = false,
		oncommit
	}: {
		/** Position on the scale, 0 to `max`. */
		value: number;
		/** Highest position; the scale has `max + 1` stops. */
		max: number;
		/** Accessible name of the slider. */
		label: string;
		/** What the current position means, read out in place of the bare
		 * number - "bis 1 Std 30 Min" rather than "4". */
		valueText: string;
		/** Draws the filled part in `handle` rather than `primary`. The length
		 * of that part says how far along the scale the thumb sits; this says
		 * whether the setting is doing anything at all, so a scale resting at
		 * a stop that means "off" stays quiet instead of reading as a control
		 * turned all the way up. */
		muted?: boolean;
		/** Fires when the user lets go - after a drag, not during it. Filters
		 * that refetch belong here: dragging across eight stops would
		 * otherwise be eight round trips for seven results nobody read. */
		oncommit?: (value: number) => void;
	} = $props();
</script>

<!--
	`py-2.5` is the touch target, not spacing: the visible thumb is 24px, and
	the row it sits in has to clear the 44px the design system asks of an
	action on a phone. Padding on the root grows the pointer area around the
	whole track without moving anything on screen.
-->
<BitsSlider.Root
	type="single"
	bind:value
	min={0}
	{max}
	step={1}
	onValueCommit={(next) => oncommit?.(next)}
	class="relative flex w-full touch-none items-center py-2.5 select-none"
>
	{#snippet children({ tickItems })}
		<span class="relative h-1.5 w-full grow rounded-pill bg-background">
			<BitsSlider.Range class="absolute h-full rounded-pill {muted ? 'bg-handle' : 'bg-primary'}" />
			<!-- The stops themselves, so the scale reads as a set of choices
			     rather than a continuum the thumb happens to jump across. The
			     ones already behind the thumb sit on the filled part and would
			     disappear against it, so they switch to `surface` at half
			     strength there instead of `handle`.

			     `top-1/2 -mt-0.5` and not `-translate-y-1/2`: see the note on
			     the thumb below. Horizontal placement is the library's own,
			     including the nudge that keeps the first and last stop inside
			     the track rather than half outside it. -->
			{#each tickItems as tick (tick.index)}
				<BitsSlider.Tick
					index={tick.index}
					class="absolute top-1/2 -mt-0.5 size-1 rounded-pill {tick.index <= value
						? 'bg-surface/55'
						: 'bg-handle'}"
				/>
			{/each}
			<!-- The name and the spoken value belong here, not on the root:
			     Bits UI puts `role="slider"` and `aria-valuenow` on the thumb,
			     so that is the element a screen reader announces. On the root
			     they would sit on a plain span and the thumb would be read as
			     an unnamed slider at "4" - a position on a scale nobody said
			     the meaning of. `aria-valuetext` replaces that number with
			     what the position actually bounds. -->
			<!-- The thumb carries the same "is this doing anything" signal as
			     the filled track, because at the tightest stop there is no
			     filled track left to carry it: the range is zero wide there,
			     so a scale set as far as it goes would otherwise show less
			     colour than any other setting.

			     `bg-surface-elevated`, not `bg-surface`: the panel around it
			     is `bg-surface` already, and in dark mode `shadow-card` is
			     `none`, so a `surface` thumb there had nothing left to set it
			     apart and read as a hole punched through the track.

			     Centred with `top-1/2 -mt-3` (half of `size-6`), because Bits
			     UI positions this element with an inline `left` and an inline
			     `translate` alone and never a `top` - the vertical axis is
			     left at the static position, which for a 24px circle in a 6px
			     track is most of a thumb too low. A Tailwind `-translate-y-1/2`
			     cannot fix it either: in Tailwind v4 that utility writes the
			     very `translate` property the inline style already holds, and
			     the inline one wins. `top` and `margin` are untouched by the
			     library, so they are what is left to centre it. -->
			<BitsSlider.Thumb
				index={0}
				aria-label={label}
				aria-valuetext={valueText}
				class="block size-6 rounded-full border-[1.5px] bg-surface-elevated shadow-card transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary {muted
					? 'border-border'
					: 'border-primary'} top-1/2 -mt-3"
			/>
		</span>
	{/snippet}
</BitsSlider.Root>

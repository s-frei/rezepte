<script lang="ts">
	import { untrack, type Snippet } from 'svelte';
	import { on } from 'svelte/events';
	import { prefersReducedMotion } from 'svelte/motion';
	import { dayNumber } from '#lib/mealplan/week.js';
	import { m } from '#lib/paraglide/messages.js';

	/**
	 * The desktop's days: one horizontal strip of columns that snaps by the
	 * day, `count` of them in view. A trackpad, Shift+wheel (a day a notch),
	 * the arrow keys (Shift: a week) and a mouse dragged on the strip's empty
	 * parts scroll it. `start` follows the first day in view and `position`
	 * the exact scroll in days; once the scroll rests, `onsettle` names the
	 * first day. While a card is dragged (`free`) the strip neither snaps nor
	 * lets the drag library scroll it (it scrolls any parent whose `overflow`
	 * names `auto`, at up to 30px a frame): held near an edge, the card steps
	 * the strip a day at a time, and the page hears of it only after the drop.
	 */
	let {
		days,
		start = $bindable(),
		count,
		free = false,
		onat,
		onsettle,
		column
	}: {
		days: string[];
		/** The first day in view; the strip opens on it. */
		start: string;
		/** The first day in view as a ruler's coordinate (`dayNumber`, fractional while it moves): unlike `position`, a load leaves it alone. */
		onat?: (at: number) => void;
		/** Columns in view. */
		count: number;
		/** A card is being dragged. */
		free?: boolean;
		onsettle?: (first: string) => void;
		column: Snippet<[string]>;
	} = $props();

	let scroller = $state<HTMLElement>();
	let width = $state(0);
	/** A mouse drags the strip itself (or the timeline's lens does): no snap. */
	let held = $state(false);
	let releasing = false;
	/**
	 * The day a step is on its way to, so quick steps add up. Snap is off
	 * meanwhile: a layout change under a moving strip (a photo loading, a
	 * refetch) would snap it to wherever it is and drop the step.
	 */
	let aiming = $state<number>();
	/** A day asked for before a load brings it. */
	let wanted: string | undefined;
	let resting: ReturnType<typeof setTimeout> | undefined;

	const last = () => Math.max(0, days.length - count);
	const clamp = (index: number) => Math.max(0, Math.min(last(), Math.round(index)));
	/** A column and its gap, fractional: whole pixels drift a day's worth over six weeks. */
	function step() {
		const [a, b] = (scroller?.children ?? []) as HTMLCollectionOf<HTMLElement>;
		if (!a) return 1;
		return b
			? b.getBoundingClientRect().left - a.getBoundingClientRect().left
			: a.getBoundingClientRect().width;
	}

	/** The exact scroll in days. */
	let position = 0;

	function measure() {
		if (!scroller) return;
		position = scroller.scrollLeft / step();
		onat?.(dayNumber(days[0]) + position);
		start = days[Math.min(days.length - 1, Math.round(position))];
	}

	/** Day `index` to the left edge. */
	function scrollToIndex(index: number) {
		if (!scroller) return;
		const to = clamp(index);
		if (Math.abs(scroller.scrollLeft - to * step()) < 1) return;
		// A rest timed before the step would end it before it moves.
		clearTimeout(resting);
		aiming = to;
		scroller.scrollTo({
			left: to * step(),
			behavior: prefersReducedMotion.current ? 'instant' : 'smooth'
		});
	}
	/** `day` to the left edge, now or once a load brings it. */
	export function aim(day: string) {
		const at = days.indexOf(day);
		if (at >= 0 && at <= last()) scrollToIndex(at);
		else wanted = day;
	}
	export const scrollByDays = (by: number) => scrollToIndex((aiming ?? Math.round(position)) + by);
	/** The day the strip is on its way to, else the first in view. */
	export const aimed = () => days[aiming ?? Math.round(position)];
	/** Follows a pointer elsewhere (the timeline's lens) to day `day` at the left edge: no snap, no smoothing. */
	export function seekDay(day: number) {
		held = true;
		const at = day - dayNumber(days[0]);
		if (scroller) scroller.scrollLeft = Math.max(0, Math.min(last(), at)) * step();
	}
	/** The pointer let go: on to the nearest day (`bias` leans a flick on), then the snap comes back. */
	export function release(bias = 0) {
		releasing = true;
		const target = clamp(position + bias);
		if (Math.abs(target - position) < 0.01) return settle();
		scrollToIndex(target);
	}

	// The scroll at rest: snap back on, and the page follows (address,
	// range), but not under a dragged card: a load would swap the columns,
	// the drag's zones among them; the drop's own rest tells it.
	function settle() {
		aiming = undefined;
		if (releasing) {
			releasing = false;
			held = false;
		}
		if (start && !free) onsettle?.(start);
	}
	$effect(() => () => clearTimeout(resting));
	function scrolled() {
		measure();
		clearTimeout(resting);
		resting = setTimeout(settle, 140);
	}

	// Opens on `start`; when a load moves the days on, the first day (or the
	// one a step is on its way to, or the one asked for) stays in place. A
	// refetch of the same days leaves the scroll alone, a step in flight
	// included, and a day asked for waits on for the load that brings it.
	let shown: string[] = [];
	$effect(() => {
		const next = days;
		if (!scroller) return;
		untrack(() => {
			const before = shown;
			const same = before.length > 0 && before.length === next.length && before[0] === next[0];
			shown = next;
			if (same) return;
			const lead = wanted ?? (before.length ? before[aiming ?? Math.round(position)] : start);
			const at = next.indexOf(lead);
			if (scroller && at >= 0) scroller.scrollLeft = at * step();
			measure();
			if (wanted) {
				wanted = undefined;
				settle();
			}
		});
	});
	$effect(() => {
		void width;
		void count;
		untrack(measure);
	});

	// A dropped card leaves the strip between two days: on to the nearest.
	let wasFree = false;
	$effect(() => {
		if (wasFree && !free) untrack(() => release());
		wasFree = free;
	});

	// A dragged card held near an edge steps the strip a day at a time: a
	// third of a second in the edge first, so passing it does not scroll;
	// after three days at a faster beat, for a card going weeks away.
	const EDGE = 48;
	$effect(() => {
		if (!free || !scroller) return;
		const strip = scroller;
		let x = -1;
		let y = -1;
		let since = 0;
		let stepped = 0;
		let steps = 0;
		let frame = requestAnimationFrame(tick);
		function track(event: MouseEvent | TouchEvent) {
			const point = 'touches' in event ? event.touches[0] : event;
			if (!point) return;
			x = point.clientX;
			y = point.clientY;
		}
		function tick(now: number) {
			frame = requestAnimationFrame(tick);
			const box = strip.getBoundingClientRect();
			const side =
				y < box.top || y > box.bottom
					? 0
					: x >= box.left && x < box.left + EDGE
						? -1
						: x <= box.right && x > box.right - EDGE
							? 1
							: 0;
			if (!side) {
				since = steps = 0;
				return;
			}
			since ||= now;
			if (now - since < 350 || now - stepped < (steps < 3 ? 700 : 350)) return;
			stepped = now;
			steps++;
			scrollByDays(side);
		}
		const stop = [
			on(window, 'mousemove', track, { capture: true, passive: true }),
			on(window, 'touchmove', track, { capture: true, passive: true })
		];
		return () => {
			cancelAnimationFrame(frame);
			for (const off of stop) off();
		};
	});

	function key(event: KeyboardEvent) {
		if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
		if (event.altKey || event.ctrlKey || event.metaKey || event.defaultPrevented) return;
		if ((event.target as Element).closest('input, textarea, [role="slider"]')) return;
		event.preventDefault();
		scrollByDays((event.key === 'ArrowLeft' ? -1 : 1) * (event.shiftKey ? 7 : 1));
	}

	// Shift and a mouse wheel: a day a notch. Chromium and WebKit leave that
	// to the system (macOS turns a mouse's Shift+wheel into `deltaX`, which
	// scrolls natively and never reaches here; a Playwright Shift+wheel
	// scrolls neither engine); elsewhere it arrives as `deltaY`, and snap
	// would undo the small steps a browser makes of it. A pause drops what
	// is left over.
	let wheeled = 0;
	let wheeledAt = 0;
	function wheel(event: WheelEvent) {
		if (!event.shiftKey || event.deltaX !== 0 || event.deltaY === 0) return;
		event.preventDefault();
		if (event.timeStamp - wheeledAt > 300) wheeled = 0;
		wheeledAt = event.timeStamp;
		wheeled += event.deltaY;
		if (Math.abs(wheeled) < 40) return;
		scrollByDays(Math.sign(wheeled));
		wheeled = 0;
	}
	const wheels = (node: HTMLElement) => on(node, 'wheel', wheel, { passive: false });

	/** A mouse dragged on the strip's empty parts scrolls it, as a finger would. */
	function grab(event: PointerEvent) {
		if (!scroller || free || event.pointerType !== 'mouse' || event.button !== 0) return;
		if ((event.target as Element).closest('a, button, input, [data-entry-id]')) return;
		const x = event.clientX;
		const from = scroller.scrollLeft;
		let dx = 0;
		const stop = [
			on(window, 'pointermove', (move) => {
				dx = move.clientX - x;
				// A click is no drag: the strip takes hold past a few pixels.
				if (!held && Math.abs(dx) < 4) return;
				held = true;
				if (scroller) scroller.scrollLeft = from - dx;
			}),
			on(window, 'pointerup', up),
			on(window, 'pointercancel', up)
		];
		function up() {
			for (const off of stop) off();
			// A flick goes on to the next day in its direction.
			if (held) release(Math.abs(dx) > 24 ? -Math.sign(dx) * 0.35 : 0);
		}
	}
</script>

<!-- A Tab stop of its own, so the arrow keys reach it (a scrolling region
     must be); the padding keeps the cards' shadows inside it top and bottom.
     Focused, it keeps the week bar and the timeline in view above it. -->
<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
<div
	bind:this={scroller}
	bind:clientWidth={width}
	role="region"
	aria-label={m.plan_days()}
	tabindex="0"
	onscroll={scrolled}
	onpointerdown={grab}
	onkeydown={key}
	{@attach wheels}
	style:--days={count}
	class="@container relative flex scroll-mt-56 [scrollbar-width:none] gap-2 overscroll-x-contain rounded-2xl pt-1 pb-6 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary {free
		? 'overflow-hidden'
		: 'overflow-x-auto'} {held || free || aiming !== undefined ? '' : 'snap-x snap-mandatory'} {held
		? 'cursor-grabbing select-none'
		: 'cursor-grab'}"
>
	{#each days as day (day)}
		<div
			class="flex shrink-0 snap-start"
			style:width="calc((100cqw - (var(--days) - 1) * 0.5rem) / var(--days))"
		>
			{@render column(day)}
		</div>
	{/each}
</div>

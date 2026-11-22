<script lang="ts">
	import ChevronLeft from '@lucide/svelte/icons/chevron-left';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import type { Snippet } from 'svelte';
	import { prefersReducedMotion } from 'svelte/motion';
	import type { HTMLAttributes } from 'svelte/elements';

	let {
		frame = '',
		fade,
		arrows,
		children,
		...rest
	}: HTMLAttributes<HTMLDivElement> & {
		/** Classes for the positioned box around the scroller (margins, sizing). */
		frame?: string;
		/** Gradient start color, e.g. `from-background`; must match what is behind the row. */
		fade: string;
		/** Arrow buttons over the fades for a mouse, named by these labels; a touch screen keeps its swipe. */
		arrows?: { previous: string; next: string };
		children: Snippet;
	} = $props();

	let start = $state(false);
	let end = $state(false);
	// A classic scrollbar takes room below the content; the fades stop above it.
	let bar = $state(0);
	let scroller: HTMLElement;
	// Where the arrows' last smooth scroll is headed, so a click during it
	// steps on from there and still lands on an item's edge.
	let goal: number | null = null;

	/** With arrows, a fade is a solid bed under the arrow and a short tail (`w-14`, `from-75%`). */
	const INSET = 56;

	function measure(el: HTMLElement) {
		start = el.scrollLeft > 1;
		end = el.scrollLeft + el.clientWidth < el.scrollWidth - 1;
		bar = el.offsetHeight - el.clientHeight;
		if (goal !== null && Math.abs(el.scrollLeft - goal) < 1) goal = null;
	}

	/** The items in the row, looking through `display: contents` wrappers. */
	function items(parent: Element): Element[] {
		return [...parent.children].flatMap((child) =>
			getComputedStyle(child).display === 'contents' ? items(child) : [child]
		);
	}

	function observe(el: HTMLElement) {
		scroller = el;
		measure(el);
		// The items too: one that changes width changes the width to scroll
		// without resizing the scroller, and so does a new one. A change starts
		// the watch over, so removed items are let go.
		const observer = new ResizeObserver(() => measure(el));
		const watch = () => [el, ...items(el)].forEach((item) => observer.observe(item));
		watch();
		const mutations = new MutationObserver(() => (observer.disconnect(), watch(), measure(el)));
		mutations.observe(el, { childList: true, subtree: true });
		const settle = () => (goal = null);
		el.addEventListener('scrollend', settle);
		return () => {
			observer.disconnect();
			mutations.disconnect();
			el.removeEventListener('scrollend', settle);
		};
	}

	// About one visible width, snapped so the item the edge cuts lands whole
	// just clear of the arrow on the other side. Measured in the row's own
	// coordinates from where the scroll is headed, not where it is.
	function step(way: 1 | -1) {
		const span = scroller.clientWidth;
		const from = goal ?? scroller.scrollLeft;
		const origin = scroller.getBoundingClientRect().left - scroller.scrollLeft;
		// A hidden item (the desktop's status line) has an empty box at 0.
		const boxes = items(scroller)
			.map((item) => item.getBoundingClientRect())
			.filter((box) => box.width > 0)
			.map((box) => ({ left: box.left - origin, right: box.right - origin }));
		const cut =
			way > 0
				? boxes.find((box) => box.right > from + span - INSET)
				: boxes.findLast((box) => box.left < from + INSET);
		let to = !cut ? from : way > 0 ? cut.left - INSET : cut.right - span + INSET;
		// An item wider than the view would leave the row where it is.
		if ((to - from) * way < INSET) to = from + way * (span - 2 * INSET);
		goal = Math.max(0, Math.min(to, scroller.scrollWidth - span));
		scroller.scrollTo({
			left: goal,
			behavior: prefersReducedMotion.current ? 'instant' : 'smooth'
		});
	}

	const edge = 'pointer-events-none absolute top-0 transition-opacity';
	const width = $derived(arrows ? 'w-6 pointer-fine:w-14 pointer-fine:from-75%' : 'w-6');
	// Out of the Tab order: a focused item scrolls itself into view. A dense
	// desktop control's size (32px), so the bed under it leaves the row room.
	const arrow =
		'absolute top-1/2 hidden size-8 -translate-y-1/2 items-center justify-center rounded-full border border-border bg-surface text-text shadow-card transition-[opacity,visibility,border-color,scale] hover:border-handle focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.96] pointer-fine:inline-flex';
</script>

<!-- A row that scrolls sideways fades out only on the side that hides more
     items. Overlays rather than a mask: a mask would fade the scrollbar too.
     With `arrows`, a mouse also gets a round button over each fade. -->
<div class="relative {frame}">
	<div {...rest} onscroll={(event) => measure(event.currentTarget)} {@attach observe}>
		{@render children()}
	</div>
	<div
		class="{edge} {width} left-0 bg-linear-to-r {fade} {start ? 'opacity-100' : 'opacity-0'}"
		style:bottom="{bar}px"
	></div>
	<div
		class="{edge} {width} right-0 bg-linear-to-l {fade} {end ? 'opacity-100' : 'opacity-0'}"
		style:bottom="{bar}px"
	></div>
	{#if arrows}
		<button
			type="button"
			tabindex="-1"
			aria-label={arrows.previous}
			onclick={() => step(-1)}
			class="{arrow} left-1 {start ? '' : 'invisible opacity-0'}"
			style:margin-top="{-bar / 2}px"
		>
			<ChevronLeft class="size-4" aria-hidden="true" />
		</button>
		<button
			type="button"
			tabindex="-1"
			aria-label={arrows.next}
			onclick={() => step(1)}
			class="{arrow} right-1 {end ? '' : 'invisible opacity-0'}"
			style:margin-top="{-bar / 2}px"
		>
			<ChevronRight class="size-4" aria-hidden="true" />
		</button>
	{/if}
</div>

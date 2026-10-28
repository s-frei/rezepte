<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLAttributes } from 'svelte/elements';

	let {
		frame = '',
		fade,
		children,
		...rest
	}: HTMLAttributes<HTMLDivElement> & {
		/** Classes for the positioned box around the scroller (margins, sizing). */
		frame?: string;
		/** Gradient start color, e.g. `from-background`; must match what is behind the row. */
		fade: string;
		children: Snippet;
	} = $props();

	let start = $state(false);
	let end = $state(false);
	// A classic scrollbar takes room below the content; the fades stop above it.
	let bar = $state(0);

	function measure(el: HTMLElement) {
		start = el.scrollLeft > 1;
		end = el.scrollLeft + el.clientWidth < el.scrollWidth - 1;
		bar = el.offsetHeight - el.clientHeight;
	}

	function observe(el: HTMLElement) {
		measure(el);
		const observer = new ResizeObserver(() => measure(el));
		observer.observe(el);
		return () => observer.disconnect();
	}

	const edge = 'pointer-events-none absolute top-0 w-6 transition-opacity';
</script>

<!-- A row that scrolls sideways fades out only on the side that hides more
     pictures. Overlays rather than a mask: a mask would fade the scrollbar too. -->
<div class="relative {frame}">
	<div {...rest} onscroll={(event) => measure(event.currentTarget)} {@attach observe}>
		{@render children()}
	</div>
	<div
		class="{edge} left-0 bg-linear-to-r {fade} {start ? 'opacity-100' : 'opacity-0'}"
		style:bottom="{bar}px"
	></div>
	<div
		class="{edge} right-0 bg-linear-to-l {fade} {end ? 'opacity-100' : 'opacity-0'}"
		style:bottom="{bar}px"
	></div>
</div>

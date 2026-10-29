<script lang="ts">
	import type { Snippet } from 'svelte';
	import { cardSize } from '#lib/recipe/card-size.svelte.js';
	import { placeCards } from '#lib/recipe/masonry.js';

	// Layout only - the caller decides what goes in each cell (real
	// `RecipeCard`s or skeleton placeholders while loading), so both share
	// the exact same grid.
	let { children }: { children: Snippet } = $props();

	// The fallback for browsers without `grid-lanes`: the cards leave the
	// grid flow and `placeCards` names each one's column and `top`, which
	// become `left` and `top` of an absolutely positioned card; the grid
	// itself keeps only its column tracks (for the count and the width) and
	// gets the tallest column's height. Grid rows are not an option: Gecko
	// caps grid lines at 10 000, so a list taller than 10 000px with 1px rows
	// would stack every later card on the last line. The cards stay direct
	// children in DOM order, so tab and screen-reader order is newest-first,
	// as it is with `grid-lanes`. `left`/`top` rather than `transform`: the
	// hover tilt uses the individual `translate` and `rotate` properties.
	//
	// The card size is written here, not in the template, so the tracks
	// change and the cards are placed in the same synchronous pass: by the
	// time Svelte's update settles (`tick`), the new grid is laid out, which
	// is what lets the overview animate the change from one layout to the
	// next. Reading the size also attaches the layout afresh on a change,
	// which the observer below would miss: new tracks leave the grid's width
	// alone, and the width is all it answers to.
	function masonry(grid: HTMLElement) {
		grid.dataset.size = cardSize.desktop;
		grid.toggleAttribute('data-wide', cardSize.phone === 1);
		if (CSS.supports('display', 'grid-lanes')) return;
		grid.style.position = 'relative';

		let frame = 0;
		let width = -1;
		const layout = () => {
			frame = 0;
			const cards = [...grid.children] as HTMLElement[];
			// Absolute cards add no tracks, so the computed track list holds
			// only the real ones, whatever the cards were last placed for. The
			// grid's height stays until the new one is known, or the browser
			// would clamp the scroll position of a reader far down the list.
			const style = getComputedStyle(grid);
			const tracks = style.gridTemplateColumns.split(' ');
			const columns = tracks.length;
			const track = parseFloat(tracks[0]);
			const columnGap = parseFloat(style.columnGap);
			// The width first: a card's height depends on it, and the last
			// layout may have had another column count. The vertical gap is
			// whole pixels (the gaps are rem, so a reader's larger default
			// font makes them fractional) and heights are ceiled, so the
			// cards' rounded boxes never overlap.
			for (const card of cards) {
				card.style.position = 'absolute';
				card.style.width = `${track}px`;
			}
			const gap = Math.round(columnGap);
			// The computed height, not the bounding box: a hovered card is
			// tilted, and its box would be a few pixels taller.
			const heights = cards.map((card) => Math.ceil(parseFloat(getComputedStyle(card).height)));
			const bottoms = new Array<number>(columns).fill(0);
			placeCards(heights, columns, gap).forEach(({ column, top }, i) => {
				cards[i].style.left = `${column * (track + columnGap)}px`;
				cards[i].style.top = `${top}px`;
				bottoms[column] = top + heights[i];
			});
			grid.style.height = `${Math.max(0, ...bottoms)}px`;
		};
		const schedule = () => {
			if (!frame) frame = requestAnimationFrame(layout);
		};

		// The grid only counts when its width changes: that is a new column
		// count or track width, and writing the height is not. Cards count
		// on any resize (an image loading, a title wrapping differently);
		// the width written to them settles after one more pass.
		const resizes = new ResizeObserver((entries) => {
			for (const entry of entries) {
				if (entry.target !== grid) schedule();
				else if (entry.contentRect.width !== width) {
					width = entry.contentRect.width;
					schedule();
				}
			}
		});
		resizes.observe(grid);
		for (const card of grid.children) resizes.observe(card, { box: 'border-box' });

		// "Load more" appends cards; earlier ones keep their place because
		// `placeCards` only looks back.
		// The layout runs right away, not on the next frame: the observer
		// answers before the browser paints, so new cards never show for a
		// frame in the grid's own flow, stacked over the first row.
		const mutations = new MutationObserver((records) => {
			for (const record of records) {
				for (const node of record.addedNodes) {
					if (node instanceof Element) resizes.observe(node, { box: 'border-box' });
				}
				for (const node of record.removedNodes) {
					if (node instanceof Element) resizes.unobserve(node);
				}
			}
			cancelAnimationFrame(frame);
			layout();
		});
		mutations.observe(grid, { childList: true });
		// The same for the first cards, which are already there.
		layout();

		return () => {
			cancelAnimationFrame(frame);
			resizes.disconnect();
			mutations.disconnect();
		};
	}
</script>

<!-- Masonry: every card is as tall as its content and goes into the
     shortest column, in DOM order. Browsers with `display: grid-lanes` lay
     it out natively; the others get the `masonry` attachment above on the
     same column tracks.
     The reader picks the card size (see card-size.svelte.ts). A phone shows
     two columns, or one, and then `data-wide` gives the cards the desktop
     look (the `desk:` variant). From `md` up each size is a minimum column
     width the row is filled with: `auto-fill` rather than `auto-fit`, so
     one search hit keeps a card's width instead of stretching over the
     whole row. A card there carries the star and the tasty heart over its
     photo, and 220px, the smallest, is what leaves the dish visible under
     them; the page's widest content (1216px) holds five of those. -->
<!-- Medium is the default track; `data-size` overrides it. Large never
     falls below two columns: at 768px one 704px column would stand a
     portrait photo almost 900px tall. -->
<div
	{@attach masonry}
	class="grid grid-cols-2 items-start gap-2.5 supports-[display:grid-lanes]:[display:grid-lanes] max-md:data-wide:grid-cols-1 md:grid-cols-[repeat(auto-fill,minmax(280px,1fr))] md:gap-5 md:data-[size=large]:grid-cols-[repeat(auto-fill,minmax(min(360px,calc((100%_-_20px)/2)),1fr))] md:data-[size=small]:grid-cols-[repeat(auto-fill,minmax(220px,1fr))]"
>
	{@render children()}
</div>
